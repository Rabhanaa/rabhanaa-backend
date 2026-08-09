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

type SellBiddingService struct {
	bidRepo            repository.SellBidRepository
	supplyOfferRepo    repository.SupplyOfferRepository
	auctionRepo        repository.SellAuctionRepository
	queries            *sqlc.Queries
	notificationSender NotificationSender
	maxBidsPerAuction  int
	maxActiveBids      int
}

func NewSellBiddingService(
	bidRepo repository.SellBidRepository,
	supplyOfferRepo repository.SupplyOfferRepository,
	auctionRepo repository.SellAuctionRepository,
	queries *sqlc.Queries,
	notificationSender NotificationSender,
	maxBidsPerAuction int,
	maxActiveBids int,
) *SellBiddingService {
	if maxBidsPerAuction == 0 {
		maxBidsPerAuction = 10
	}
	if maxActiveBids == 0 {
		maxActiveBids = 3
	}
	return &SellBiddingService{
		bidRepo:            bidRepo,
		supplyOfferRepo:    supplyOfferRepo,
		auctionRepo:        auctionRepo,
		queries:            queries,
		notificationSender: notificationSender,
		maxBidsPerAuction:  maxBidsPerAuction,
		maxActiveBids:      maxActiveBids,
	}
}

func (s *SellBiddingService) PlaceBid(ctx context.Context, bidderID int32, auctionPublicID uuid.UUID, req model.PlaceSellBidRequest) error {
	auction, err := s.auctionRepo.GetByPublicID(ctx, auctionPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrAuctionNotFound
		}
		return fmt.Errorf("failed to get auction: %w", err)
	}

	if auction.Status != "active" {
		return errs.ErrAuctionNotActive
	}

	if auction.EndTime.Time.Before(time.Now()) {
		return errs.ErrAuctionExpired
	}

	if auction.OwnerID == bidderID {
		return errs.ErrCannotBidOwnAuction
	}

	_, err = s.bidRepo.GetByAuctionAndBidder(ctx, auction.ID, bidderID)
	if err == nil {
		return errs.ErrAlreadyBid
	}

	if auction.BidCount >= int32(s.maxBidsPerAuction) {
		return errs.ErrMaxBidders
	}

	activeBids, err := s.bidRepo.CountActiveBidsByBidder(ctx, bidderID)
	if err != nil {
		return fmt.Errorf("failed to count active bids: %w", err)
	}

	activeOffers, err := s.supplyOfferRepo.CountActiveOffersBySupplier(ctx, bidderID)
	if err != nil {
		return fmt.Errorf("failed to count active offers: %w", err)
	}

	totalActive := activeBids + activeOffers
	if totalActive >= int64(s.maxActiveBids) {
		return errs.ErrMaxActiveBids
	}

	// Check subscription tier and monthly limits
	userSub, err := s.queries.GetUserWithSubscription(ctx, bidderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrNoSubscription
		}
		return fmt.Errorf("failed to get user subscription: %w", err)
	}

	// Check if tier allows placing bids
	if !userSub.CanPlaceBids.Valid || !userSub.CanPlaceBids.Bool {
		return errs.ErrInsufficientTier
	}

	// Check monthly limit (skip if unlimited/zico)
	if userSub.MaxBidsPerMonth.Valid {
		maxBids := userSub.MaxBidsPerMonth.Int32
		current := userSub.BidsPlacedThisMonth.Int32

		// Reset if new month
		if userSub.MonthResetAt.Time.Before(time.Now().AddDate(0, -1, 0)) {
			s.queries.ResetMonthlyCounts(ctx)
			current = 0
		}

		if current >= maxBids {
			return errs.ErrMonthlyLimit
		}

		// Increment counter
		s.queries.IncrementBidCount(ctx, userSub.SubscriptionID.Int32)
	}

	bidAmount := decimal.NewFromFloat(req.Amount)
	minBid := decimal.NewFromBigInt(auction.UnitPrice.Int, auction.UnitPrice.Exp).Mul(decimal.NewFromFloat(0.85))
	if bidAmount.LessThan(minBid) {
		return errs.ErrBidBelowMinimum
	}

	// Fetch bidder's region
	bidder, err := s.queries.GetUserByID(ctx, bidderID)
	if err != nil {
		return fmt.Errorf("failed to get bidder: %w", err)
	}

	var regionName string
	if bidder.RegionID.Valid {
		region, err := s.queries.GetRegionByID(ctx, bidder.RegionID.Int32)
		if err != nil {
			return fmt.Errorf("failed to get region: %w", err)
		}
		regionName = region.NameAr
	}

	// Generate fake name for bidder (e.g., "مزاد1", "مزاد2", etc.)
	fakeName := generateFakeBidderName(auction.ID, int(auction.BidCount+1))

	_, err = s.bidRepo.Create(ctx, sqlc.CreateSellBidParams{
		AuctionID:        auction.ID,
		BidderID:         bidderID,
		Amount:           decimalToNumeric(bidAmount),
		BidderRegionName: regionName,
		BidderFakeName:   fakeName,
		AuctionTitle:     auction.Title,
		AuctionUnitPrice: auction.UnitPrice,
		AuctionQuantity:  auction.Quantity,
		AuctionUnit:      auction.Unit,
	})
	if err != nil {
		return fmt.Errorf("failed to create bid: %w", err)
	}

	s.auctionRepo.IncrementBidCount(ctx, auction.ID)

	s.notificationSender.Send(ctx, auction.OwnerID, notifModel.EventNewBid, map[string]string{
		"auction_id": auction.PublicID.String(),
	})

	return nil
}

func (s *SellBiddingService) ListMyBids(ctx context.Context, bidderID int32, page, pageSize int32) ([]model.SellBidResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	bids, err := s.bidRepo.ListByBidder(ctx, sqlc.ListSellBidsByBidderParams{
		BidderID: bidderID,
		Limit:    pageSize,
		Offset:   (page - 1) * pageSize,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list bids: %w", err)
	}

	var responses []model.SellBidResponse
	for _, bid := range bids {
		responses = append(responses, model.SellBidResponse{
			PublicID:         bid.PublicID.String(),
			Amount:           numericToString(bid.Amount),
			IsSelected:       bid.IsSelected,
			IsMyBid:          true,
			AuctionTitle:     bid.AuctionTitle,
			AuctionUnitPrice: numericToString(bid.AuctionUnitPrice),
			AuctionQuantity:  numericToString(bid.AuctionQuantity),
			AuctionUnit:      bid.AuctionUnit,
			CreatedAt:        bid.CreatedAt.Time.Format(time.RFC3339),
		})
	}

	return responses, 0, nil
}

func (s *SellBiddingService) ListBidsByAuction(ctx context.Context, auctionPublicID uuid.UUID, userID int32) ([]model.SellBidResponse, error) {
	auction, err := s.auctionRepo.GetByPublicID(ctx, auctionPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrAuctionNotFound
		}
		return nil, fmt.Errorf("failed to get auction: %w", err)
	}

	// Only auction owner can see all bids
	if auction.OwnerID != userID {
		return nil, errs.ErrNotAuctionOwner
	}

	bids, err := s.bidRepo.ListByAuction(ctx, auction.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list bids: %w", err)
	}

	var responses []model.SellBidResponse
	for _, bid := range bids {
		responses = append(responses, model.SellBidResponse{
			PublicID:     bid.PublicID.String(),
			BidderName:   bid.BidderName,   // Fake name for anonymity
			BidderRegion: bid.BidderRegion, // Real region
			Amount:       numericToString(bid.Amount),
			IsSelected:   bid.IsSelected,
			IsMyBid:      bid.BidderID == userID,
			CreatedAt:    bid.CreatedAt.Time.Format(time.RFC3339),
		})
	}

	return responses, nil
}

// generateFakeBidderName generates a fake name for bidder anonymity
func generateFakeBidderName(auctionID int32, bidNumber int) string {
	return fmt.Sprintf("مزاد%d", bidNumber)
}
