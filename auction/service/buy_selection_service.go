package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	auctionRepo "rabhana/auction/repository"
	authRepo "rabhana/auth/repository"
	"rabhana/db/sqlc"
	notifModel "rabhana/notification/model"
	orderRepo "rabhana/order/repository"
	"rabhana/pkg/errs"
)

type BuySelectionService struct {
	requestRepo          auctionRepo.BuyRequestRepository
	offerRepo            auctionRepo.SupplyOfferRepository
	orderRepo            orderRepo.OrderRepository
	userRepo             authRepo.Repository
	notificationSender   NotificationSender
	selectionWindowHours int
}

func NewBuySelectionService(
	requestRepo auctionRepo.BuyRequestRepository,
	offerRepo auctionRepo.SupplyOfferRepository,
	orderRepo orderRepo.OrderRepository,
	userRepo authRepo.Repository,
	notificationSender NotificationSender,
	selectionWindowHours int,
) *BuySelectionService {
	return &BuySelectionService{
		requestRepo:          requestRepo,
		offerRepo:            offerRepo,
		orderRepo:            orderRepo,
		userRepo:             userRepo,
		notificationSender:   notificationSender,
		selectionWindowHours: selectionWindowHours,
	}
}

func (s *BuySelectionService) AcceptOffer(ctx context.Context, ownerID int32, requestPublicID, offerPublicID uuid.UUID) error {
	request, err := s.requestRepo.GetByPublicIDForUpdate(ctx, requestPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrAuctionNotFound
		}
		return fmt.Errorf("failed to get request: %w", err)
	}

	if request.OwnerID != ownerID {
		return errs.ErrNotAuctionOwner
	}

	if request.Status != "pending_selection" && request.Status != "partially_fulfilled" && request.Status != "active" {
		return errs.ErrAuctionNotActive
	}

	selectionDeadline := request.EndTime.Time.Add(time.Duration(s.selectionWindowHours) * time.Hour)
	if time.Now().After(selectionDeadline) {
		return errs.ErrSelectionWindowExpired
	}

	offer, err := s.offerRepo.GetByPublicID(ctx, offerPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("OFFER_NOT_FOUND")
		}
		return fmt.Errorf("failed to get offer: %w", err)
	}

	if offer.BuyRequestID != request.ID {
		return errors.New("OFFER_NOT_IN_REQUEST")
	}

	if offer.IsAccepted {
		return errs.ErrAlreadyConfirmed
	}

	acceptedQty, err := s.offerRepo.SumAcceptedQuantityByRequest(ctx, request.ID)
	if err != nil {
		return fmt.Errorf("failed to sum accepted quantity: %w", err)
	}

	var totalAccepted float64
	if acceptedQty != nil {
		switch v := acceptedQty.(type) {
		case float64:
			totalAccepted = v
		case int64:
			totalAccepted = float64(v)
		}
	}

	requestQty := 0.0
	if request.Quantity.Valid && request.Quantity.Int != nil {
		requestQty = float64(request.Quantity.Int.Int64()) * 0.001
	}

	offeredQty := 0.0
	if offer.OfferedQuantity.Valid && offer.OfferedQuantity.Int != nil {
		offeredQty = float64(offer.OfferedQuantity.Int.Int64()) * 0.001
	}

	remaining := requestQty - totalAccepted
	if offeredQty > remaining {
		return errs.ErrInvalidQuantity
	}

	if err := s.offerRepo.AcceptOffer(ctx, offer.ID); err != nil {
		return fmt.Errorf("failed to accept offer: %w", err)
	}

	s.requestRepo.IncrementAcceptedOfferCount(ctx, request.ID)

	newFulfilled := totalAccepted + offeredQty
	s.requestRepo.UpdateFulfilledQuantity(ctx, sqlc.UpdateBuyRequestFulfilledQuantityParams{
		ID:                request.ID,
		FulfilledQuantity: pgtype.Numeric{Int: big.NewInt(int64(newFulfilled * 1000)), Valid: true},
	})

	if newFulfilled >= requestQty {
		s.requestRepo.UpdateStatus(ctx, sqlc.UpdateBuyRequestStatusParams{
			ID:     request.ID,
			Status: "fulfilled",
		})
	} else {
		s.requestRepo.UpdateStatus(ctx, sqlc.UpdateBuyRequestStatusParams{
			ID:     request.ID,
			Status: "partially_fulfilled",
		})
	}

	// Fetch seller (supplier) and buyer user data with region
	seller, err := s.userRepo.GetUserWithRegion(ctx, offer.SupplierID)
	if err != nil {
		return fmt.Errorf("failed to get seller data: %w", err)
	}

	buyer, err := s.userRepo.GetUserWithRegion(ctx, request.OwnerID)
	if err != nil {
		return fmt.Errorf("failed to get buyer data: %w", err)
	}

	order, err := s.orderRepo.CreateFromBuyRequest(ctx, sqlc.CreateOrderFromBuyRequestParams{
		BuyRequestID:   pgtype.Int4{Int32: request.ID, Valid: true},
		SellerID:       offer.SupplierID,
		BuyerID:        request.OwnerID,
		FinalPrice:     offer.PricePerUnit,
		Quantity:       offer.OfferedQuantity,
		Unit:           request.Unit,
		SellerName:     seller.Name,
		SellerPhone:    getPgTextValue(seller.Phone),
		SellerRegion:   seller.RegionName,
		BuyerName:      buyer.Name,
		BuyerPhone:     getPgTextValue(buyer.Phone),
		BuyerRegion:    buyer.RegionName,
		SourcePublicID: request.PublicID,
	})
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	s.notificationSender.Send(ctx, offer.SupplierID, notifModel.EventOfferAccepted, map[string]string{
		"request_id": request.PublicID.String(),
		"order_id":   order.PublicID.String(),
	})

	s.notificationSender.Send(ctx, request.OwnerID, notifModel.EventOrderCreated, map[string]string{
		"request_id": request.PublicID.String(),
		"order_id":   order.PublicID.String(),
	})

	wasFullyFulfilled := newFulfilled >= requestQty
	if wasFullyFulfilled {
		others, err := s.offerRepo.GetUnacceptedSupplierIDs(ctx, request.ID)
		if err != nil {
			slog.Error("failed to fetch unaccepted suppliers", "request_id", request.ID, "error", err)
		} else {
			for _, uid := range others {
				s.notificationSender.Send(ctx, uid, notifModel.EventOfferNotAccepted, map[string]string{
					"request_id": request.PublicID.String(),
				})
			}
		}
	}

	return nil
}
