package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// getPgTextValue extracts string value from pgtype.Text
func getPgTextValue(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

type SellSelectionService struct {
	auctionRepo          auctionRepo.SellAuctionRepository
	bidRepo              auctionRepo.SellBidRepository
	orderRepo            orderRepo.OrderRepository
	userRepo             authRepo.Repository
	notificationSender   NotificationSender
	selectionWindowHours int
}

func NewSellSelectionService(
	auctionRepo auctionRepo.SellAuctionRepository,
	bidRepo auctionRepo.SellBidRepository,
	orderRepo orderRepo.OrderRepository,
	userRepo authRepo.Repository,
	notificationSender NotificationSender,
	selectionWindowHours int,
) *SellSelectionService {
	return &SellSelectionService{
		auctionRepo:          auctionRepo,
		bidRepo:              bidRepo,
		orderRepo:            orderRepo,
		userRepo:             userRepo,
		notificationSender:   notificationSender,
		selectionWindowHours: selectionWindowHours,
	}
}

func (s *SellSelectionService) SelectWinner(ctx context.Context, ownerID int32, auctionPublicID, bidPublicID uuid.UUID) error {
	auction, err := s.auctionRepo.GetByPublicIDForUpdate(ctx, auctionPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrAuctionNotFound
		}
		return fmt.Errorf("failed to get auction: %w", err)
	}

	if auction.OwnerID != ownerID {
		return errs.ErrNotAuctionOwner
	}

	if auction.Status != "pending_selection" && auction.Status != "active" {
		return errs.ErrAuctionNotActive
	}

	selectionDeadline := auction.EndTime.Time.Add(time.Duration(s.selectionWindowHours) * time.Hour)
	if time.Now().After(selectionDeadline) {
		return errs.ErrSelectionWindowExpired
	}

	bid, err := s.bidRepo.GetByPublicID(ctx, bidPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("BID_NOT_FOUND")
		}
		return fmt.Errorf("failed to get bid: %w", err)
	}

	if bid.AuctionID != auction.ID {
		return errors.New("BID_NOT_IN_AUCTION")
	}

	if err := s.bidRepo.SelectBid(ctx, bid.ID); err != nil {
		return fmt.Errorf("failed to select bid: %w", err)
	}

	if err := s.auctionRepo.SelectWinner(ctx, sqlc.SelectSellWinnerParams{
		ID:            auction.ID,
		SelectedBidID: pgtype.Int4{Int32: bid.ID, Valid: true},
		WinnerID:      pgtype.Int4{Int32: bid.BidderID, Valid: true},
		FinalPrice:    bid.Amount,
	}); err != nil {
		return fmt.Errorf("failed to update auction: %w", err)
	}

	// Fetch seller and buyer user data with region
	seller, err := s.userRepo.GetUserWithRegion(ctx, auction.OwnerID)
	if err != nil {
		return fmt.Errorf("failed to get seller data: %w", err)
	}

	buyer, err := s.userRepo.GetUserWithRegion(ctx, bid.BidderID)
	if err != nil {
		return fmt.Errorf("failed to get buyer data: %w", err)
	}

	_, err = s.orderRepo.CreateFromSellAuction(ctx, sqlc.CreateOrderFromSellAuctionParams{
		SellAuctionID:  pgtype.Int4{Int32: auction.ID, Valid: true},
		SellerID:       auction.OwnerID,
		BuyerID:        bid.BidderID,
		FinalPrice:     bid.Amount,
		Quantity:       auction.Quantity,
		Unit:           auction.Unit,
		SellerName:     seller.Name,
		SellerPhone:    getPgTextValue(seller.Phone),
		SellerRegion:   seller.RegionName,
		BuyerName:      buyer.Name,
		BuyerPhone:     getPgTextValue(buyer.Phone),
		BuyerRegion:    buyer.RegionName,
		SourcePublicID: auction.PublicID,
	})
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	s.notificationSender.Send(ctx, bid.BidderID, notifModel.EventWinnerSelected, map[string]string{
		"auction_id": auction.PublicID.String(),
	})

	s.notificationSender.Send(ctx, auction.OwnerID, notifModel.EventOrderCreated, map[string]string{
		"auction_id": auction.PublicID.String(),
	})

	losers, err := s.bidRepo.GetLosingBidderIDs(ctx, auction.ID, bid.ID)
	if err != nil {
		slog.Error("failed to fetch losing bidders", "auction_id", auction.ID, "error", err)
	} else {
		for _, uid := range losers {
			s.notificationSender.Send(ctx, uid, notifModel.EventBidNotSelected, map[string]string{
				"auction_id": auction.PublicID.String(),
			})
		}
	}

	return nil
}
