package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"rabhana/auction/model"
	"rabhana/auction/repository"
	"rabhana/db/sqlc"
	notifModel "rabhana/notification/model"
	"rabhana/pkg/errs"
)

type SupplyOfferingService struct {
	offerRepo           repository.SupplyOfferRepository
	sellBidRepo         repository.SellBidRepository
	requestRepo         repository.BuyRequestRepository
	queries             *sqlc.Queries
	notificationSender  NotificationSender
	maxOffersPerRequest int
	maxActiveOffers     int
}

func NewSupplyOfferingService(
	offerRepo repository.SupplyOfferRepository,
	sellBidRepo repository.SellBidRepository,
	requestRepo repository.BuyRequestRepository,
	queries *sqlc.Queries,
	notificationSender NotificationSender,
	maxOffersPerRequest int,
	maxActiveOffers int,
) *SupplyOfferingService {
	if maxOffersPerRequest == 0 {
		maxOffersPerRequest = 10
	}
	if maxActiveOffers == 0 {
		maxActiveOffers = 3
	}
	return &SupplyOfferingService{
		offerRepo:           offerRepo,
		sellBidRepo:         sellBidRepo,
		requestRepo:         requestRepo,
		queries:             queries,
		notificationSender:  notificationSender,
		maxOffersPerRequest: maxOffersPerRequest,
		maxActiveOffers:     maxActiveOffers,
	}
}

func (s *SupplyOfferingService) PlaceOffer(ctx context.Context, supplierID int32, requestPublicID uuid.UUID, req model.PlaceSupplyOfferRequest) error {
	request, err := s.requestRepo.GetByPublicIDForUpdate(ctx, requestPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrAuctionNotFound
		}
		return fmt.Errorf("failed to get request: %w", err)
	}

	if request.Status != "active" {
		return errs.ErrAuctionNotActive
	}

	if request.EndTime.Time.Before(time.Now()) {
		return errs.ErrAuctionExpired
	}

	if request.OwnerID == supplierID {
		return errs.ErrCannotBidOwnAuction
	}

	_, err = s.offerRepo.GetByRequestAndSupplier(ctx, request.ID, supplierID)
	if err == nil {
		return errs.ErrAlreadyBid
	}

	if request.OfferCount >= int32(s.maxOffersPerRequest) {
		return errs.ErrMaxBidders
	}

	activeOffers, err := s.offerRepo.CountActiveOffersBySupplier(ctx, supplierID)
	if err != nil {
		return fmt.Errorf("failed to count active offers: %w", err)
	}

	activeBids, err := s.sellBidRepo.CountActiveBidsByBidder(ctx, supplierID)
	if err != nil {
		return fmt.Errorf("failed to count active bids: %w", err)
	}

	totalActive := activeOffers + activeBids
	if totalActive >= int64(s.maxActiveOffers) {
		return errs.ErrMaxActiveBids
	}

	// Check subscription tier and monthly limits
	userSub, err := s.queries.GetUserWithSubscription(ctx, supplierID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNoSubscription
		}
		return fmt.Errorf("failed to get user subscription: %w", err)
	}

	// Check if tier allows making offers
	if !userSub.CanMakeOffers.Valid || !userSub.CanMakeOffers.Bool {
		return errs.ErrInsufficientTier
	}

	// Check monthly limit (skip if unlimited/zico)
	if userSub.MaxOffersPerMonth.Valid {
		maxOffers := userSub.MaxOffersPerMonth.Int32
		current := userSub.OffersMadeThisMonth.Int32

		// Reset if new month
		if userSub.MonthResetAt.Time.Before(time.Now().AddDate(0, -1, 0)) {
			s.queries.ResetMonthlyCounts(ctx)
			current = 0
		}

		if current >= maxOffers {
			return errs.ErrMonthlyLimit
		}

		// Increment counter
		s.queries.IncrementOfferCount(ctx, userSub.SubscriptionID.Int32)
	}

	requestQty := decimal.NewFromBigInt(request.Quantity.Int, request.Quantity.Exp)

	var fulfilledQty decimal.Decimal
	if request.FulfilledQuantity.Valid && request.FulfilledQuantity.Int != nil {
		fulfilledQty = decimal.NewFromBigInt(request.FulfilledQuantity.Int, request.FulfilledQuantity.Exp)
	}
	remainingQty := requestQty.Sub(fulfilledQty)

	if req.OfferedQuantity == 0 {
		req.OfferedQuantity, _ = remainingQty.Float64()
	}

	offeredQty := decimal.NewFromFloat(req.OfferedQuantity)

	if request.BuyAllFromOne {
		// For buy_all_from_one: must provide exact quantity
		if !offeredQty.Equal(requestQty) {
			return errs.ErrInvalidQuantity
		}
	} else {
		// For partial fulfillment: must be >0 and <= remaining open quantity
		if offeredQty.LessThanOrEqual(decimal.Zero) || offeredQty.GreaterThan(remainingQty) {
			return errs.ErrInvalidQuantity
		}
	}

	pricePerUnit := decimal.NewFromFloat(req.PricePerUnit)

	// Fetch supplier's region
	supplier, err := s.queries.GetUserByID(ctx, supplierID)
	if err != nil {
		return fmt.Errorf("failed to get supplier: %w", err)
	}

	var regionName string
	if supplier.RegionID.Valid {
		region, err := s.queries.GetRegionByID(ctx, supplier.RegionID.Int32)
		if err != nil {
			return fmt.Errorf("failed to get region: %w", err)
		}
		regionName = region.NameAr
	}

	// Generate fake name for supplier (e.g., "مورد1", "مورد2", etc.)
	fakeName := generateFakeSupplierName(request.ID, int(request.OfferCount+1))

	_, err = s.offerRepo.Create(ctx, sqlc.CreateSupplyOfferParams{
		BuyRequestID:       request.ID,
		SupplierID:         supplierID,
		PricePerUnit:       decimalToNumeric(pricePerUnit),
		OfferedQuantity:    decimalToNumeric(offeredQty),
		SupplierRegionName: regionName,
		SupplierFakeName:   fakeName,
		RequestTitle:       request.Title,
		RequestQuantity:    request.Quantity,
		RequestUnit:        request.Unit,
	})
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	s.requestRepo.IncrementOfferCount(ctx, request.ID)

	s.notificationSender.Send(ctx, request.OwnerID, notifModel.EventNewOffer, map[string]string{
		"request_id": request.PublicID.String(),
	})

	return nil
}

func (s *SupplyOfferingService) ListMyOffers(ctx context.Context, supplierID int32, page, pageSize int32) ([]model.SupplyOfferResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offers, err := s.offerRepo.ListBySupplier(ctx, sqlc.ListSupplyOffersBySupplierParams{
		SupplierID: supplierID,
		Limit:      pageSize,
		Offset:     (page - 1) * pageSize,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list offers: %w", err)
	}

	var responses []model.SupplyOfferResponse
	for _, offer := range offers {
		responses = append(responses, model.SupplyOfferResponse{
			PublicID:        offer.PublicID.String(),
			PricePerUnit:    numericToString(offer.PricePerUnit),
			OfferedQuantity: numericToString(offer.OfferedQuantity),
			IsAccepted:      offer.IsAccepted,
			IsMyOffer:       true,
			CreatedAt:       offer.CreatedAt.Time.Format(time.RFC3339),
		})
	}

	return responses, 0, nil
}

func (s *SupplyOfferingService) ListOffersByRequest(ctx context.Context, requestPublicID uuid.UUID, userID int32) ([]model.SupplyOfferResponse, error) {
	request, err := s.requestRepo.GetByPublicID(ctx, requestPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrAuctionNotFound
		}
		return nil, fmt.Errorf("failed to get request: %w", err)
	}

	// Any authenticated user can see offers (suppliers view anonymous offers for market transparency)
	offers, err := s.offerRepo.ListByRequest(ctx, request.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list offers: %w", err)
	}

	var responses []model.SupplyOfferResponse
	for _, offer := range offers {
		responses = append(responses, model.SupplyOfferResponse{
			PublicID:        offer.PublicID.String(),
			SupplierName:    offer.SupplierName,   // Fake name for anonymity
			SupplierRegion:  offer.SupplierRegion, // Real region
			PricePerUnit:    numericToString(offer.PricePerUnit),
			OfferedQuantity: numericToString(offer.OfferedQuantity),
			IsAccepted:      offer.IsAccepted,
			IsMyOffer:       offer.SupplierID == userID,
			CreatedAt:       offer.CreatedAt.Time.Format(time.RFC3339),
		})
	}

	return responses, nil
}

// generateFakeSupplierName generates a fake name for supplier anonymity
func generateFakeSupplierName(requestID int32, offerNumber int) string {
	return fmt.Sprintf("مورد%d", offerNumber)
}
