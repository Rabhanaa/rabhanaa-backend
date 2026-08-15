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
	"github.com/shopspring/decimal"

	"rabhana/auction/model"
	"rabhana/auction/repository"
	"rabhana/db/sqlc"
	notifModel "rabhana/notification/model"
	"rabhana/pkg/errs"
)

type SellAuctionService struct {
	auctionRepo        repository.SellAuctionRepository
	queries            *sqlc.Queries
	notificationSender NotificationSender
	auctionDuration    time.Duration
	uploadService      UploadService
	defaultImageURL    string
	regionFilter       bool
	postApproval       bool
}

type NotificationSender interface {
	SendToUser(ctx context.Context, userID int32, title, body string, data map[string]string)
	Send(ctx context.Context, userID int32, event notifModel.EventType, data map[string]string)
}

type UploadService interface {
	UploadFile(ctx context.Context, file []byte, filename string) (string, error)
}

func NewSellAuctionService(
	auctionRepo repository.SellAuctionRepository,
	queries *sqlc.Queries,
	notificationSender NotificationSender,
	auctionDurationHours int,
	uploadService UploadService,
	defaultImageURL string,
	regionFilter bool,
	postApproval bool,
) *SellAuctionService {
	duration := time.Duration(auctionDurationHours) * time.Hour
	if duration == 0 {
		duration = 24 * time.Hour
	}
	return &SellAuctionService{
		auctionRepo:        auctionRepo,
		queries:            queries,
		notificationSender: notificationSender,
		auctionDuration:    duration,
		uploadService:      uploadService,
		defaultImageURL:    defaultImageURL,
		regionFilter:       regionFilter,
		postApproval:       postApproval,
	}
}

func (s *SellAuctionService) CreateSellAuction(ctx context.Context, userID int32, req model.CreateSellAuctionRequest, imageFile []byte, imageFilename string) (*model.SellAuctionResponse, error) {
	unitPrice := decimal.NewFromFloat(req.UnitPrice)
	quantity := decimal.NewFromFloat(req.Quantity)

	buyAllFromOne := true
	if req.BuyAllFromOne != nil {
		buyAllFromOne = *req.BuyAllFromOne
	}

	var imageURL string
	if req.ImageURL != "" {
		imageURL = req.ImageURL
	} else if len(imageFile) > 0 {
		url, err := s.uploadService.UploadFile(ctx, imageFile, imageFilename)
		if err != nil {
			return nil, fmt.Errorf("failed to upload image: %w", err)
		}
		imageURL = url
	} else if s.defaultImageURL != "" {
		imageURL = s.defaultImageURL
	}

	endTime := time.Now().Add(s.auctionDuration)

	var regionID int32
	if req.RegionID != nil {
		regionID = *req.RegionID
	}

	// Check subscription tier and monthly limits
	userSub, err := s.queries.GetUserWithSubscription(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrNoSubscription
		}
		return nil, fmt.Errorf("failed to get user subscription: %w", err)
	}

	// Check if tier allows creating sell auctions
	if !userSub.CanCreateSellAuctions.Valid || !userSub.CanCreateSellAuctions.Bool {
		return nil, errs.ErrInsufficientTier
	}

	// Check monthly limit (skip if unlimited/zico)
	if userSub.MaxSellAuctionsPerMonth.Valid {
		maxAuctions := userSub.MaxSellAuctionsPerMonth.Int32
		current := userSub.AuctionsCreatedThisMonth.Int32

		// Reset if new month
		if userSub.MonthResetAt.Time.Before(time.Now().AddDate(0, -1, 0)) {
			s.queries.ResetMonthlyCounts(ctx)
			current = 0
		}

		if current >= maxAuctions {
			return nil, errs.ErrMonthlyLimit
		}

		// Increment counter
		s.queries.IncrementAuctionCount(ctx, userSub.SubscriptionID.Int32)
	}

	// Lookup names for caching
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Retailers buy on this platform and sell to consumers off it, so a retail
	// sell post has no audience here. This is a role check and is separate from
	// the tier check above, which is about what a subscription allows.
	if user.JobKey == RetailerRoleKey {
		return nil, errs.ErrRetailerCannotSell
	}

	region, err := s.queries.GetRegionByID(ctx, regionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get region: %w", err)
	}

	interest, err := s.queries.GetInterestByID(ctx, req.InterestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get interest: %w", err)
	}

	// Handle optional description
	description := pgtype.Text{Valid: req.Description != nil}
	if req.Description != nil {
		description.String = *req.Description
	}

	auction, err := s.auctionRepo.Create(ctx, sqlc.CreateSellAuctionParams{
		OwnerID:       userID,
		RegionID:      regionID,
		InterestID:    req.InterestID,
		Title:         req.Title,
		Description:   description,
		ImageUrl:      imageURL,
		Unit:          req.Unit,
		Quantity:      decimalToNumeric(quantity),
		UnitPrice:     decimalToNumeric(unitPrice),
		BuyAllFromOne: buyAllFromOne,
		EndTime:       pgtype.Timestamptz{Time: endTime, Valid: true},
		OwnerName:     user.Name,
		RegionName:    region.NameAr,
		InterestName:  interest.NameAr,
		Status:        s.initialStatus(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create auction: %w", err)
	}

	go func(interestNameAr, auctionTitle, auctionPublicID string, interestID, auctionID, ownerID, postRegionID int32, ownerJobKey string) {
		bgCtx := context.Background()
		users, err := s.queries.GetActiveUsersByInterest(bgCtx, sqlc.GetActiveUsersByInterestParams{
			InterestID:     interestID,
			ExcludeUserID:  ownerID,
			FilterRegionID: s.notifyRegion(postRegionID),
			// A retailer's feed only shows supply-side sellers, so a post from
			// anyone else must not notify them either.
			ExcludeRetailers: !isSupplySideRole(ownerJobKey),
		})
		if err != nil {
			slog.Error("broadcast sell auction: get interested users failed", "auction_id", auctionID, "error", err)
			return
		}
		title := interestNameAr + " - صفقة جديدة"
		for _, uid := range users {
			s.notificationSender.SendToUser(bgCtx, uid, title, auctionTitle, map[string]string{
				"type":       "new_sell_auction",
				"auction_id": auctionPublicID,
			})
		}
		if err := s.queries.MarkSellAuctionNotified(bgCtx, auctionID); err != nil {
			slog.Error("broadcast sell auction: mark notified failed", "auction_id", auctionID, "error", err)
		}
	}(interest.NameAr, auction.Title, auction.PublicID.String(), auction.InterestID, auction.ID, auction.OwnerID, auction.RegionID, user.JobKey)

	return s.toResponse(auction, userID), nil
}

func (s *SellAuctionService) CancelSellAuction(ctx context.Context, userID int32, auctionPublicID uuid.UUID) error {
	auction, err := s.auctionRepo.GetByPublicID(ctx, auctionPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrAuctionNotFound
		}
		return fmt.Errorf("failed to get auction: %w", err)
	}

	if auction.OwnerID != userID {
		return errs.ErrNotAuctionOwner
	}

	if auction.Status != "active" {
		return errs.ErrAuctionNotActive
	}

	if auction.BidCount > 0 {
		return errs.ErrCannotCancelWithBids
	}

	sellCancellations, _ := s.auctionRepo.CountMonthlyCancellations(ctx, userID)
	if sellCancellations >= 3 {
		return errs.ErrMaxCancellations
	}

	return s.auctionRepo.Cancel(ctx, auction.ID)
}

func (s *SellAuctionService) GetSellAuctionDetail(ctx context.Context, auctionPublicID uuid.UUID, requestingUserID int32) (*model.SellAuctionResponse, error) {
	auction, err := s.auctionRepo.GetByPublicID(ctx, auctionPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrAuctionNotFound
		}
		return nil, fmt.Errorf("failed to get auction: %w", err)
	}

	// A post that is not live is visible only to its owner (an admin reaches it
	// through the moderation endpoints), so a rejected or suspended post is not
	// still reachable by anyone holding the link.
	if auction.Status != "active" && auction.OwnerID != requestingUserID {
		return nil, errs.ErrAuctionNotFound
	}

	return s.toResponse(auction, requestingUserID), nil
}

func (s *SellAuctionService) ListMySellAuctions(ctx context.Context, userID int32, page, pageSize int32) ([]model.SellAuctionResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	auctions, err := s.auctionRepo.ListByOwner(ctx, sqlc.ListSellAuctionsByOwnerParams{
		OwnerID: userID,
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list auctions: %w", err)
	}

	total, err := s.auctionRepo.CountByOwner(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count auctions: %w", err)
	}

	var responses []model.SellAuctionResponse
	for _, auction := range auctions {
		responses = append(responses, *s.toResponse(auction, userID))
	}

	return responses, total, nil
}

func (s *SellAuctionService) ListActiveAuctions(ctx context.Context, userID int32, page, pageSize int32, interestIDs []int32) ([]model.SellAuctionResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	v := s.viewer(ctx, userID)
	auctions, err := s.auctionRepo.ListActive(ctx, sqlc.ListActiveSellAuctionsParams{
		Limit:                 pageSize,
		Offset:                (page - 1) * pageSize,
		ExcludeOwnerID:        userID,
		ExcludeBiddedAuctions: nil,
		UserID:                userID,
		UserInterestIds:       interestIDs,
		FilterRegionID:        v.regionID,
		OwnerJobKeys:          v.ownerJobKeys(),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list auctions: %w", err)
	}

	total, err := s.auctionRepo.CountActive(ctx, userID, v.regionID, v.ownerJobKeys())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count auctions: %w", err)
	}

	var responses []model.SellAuctionResponse
	for _, auction := range auctions {
		responses = append(responses, *s.toResponse(auction, userID))
	}

	return responses, total, nil
}

func (s *SellAuctionService) SearchAuctions(ctx context.Context, userID int32, searchTerm string, page, pageSize int32, interestIDs []int32) ([]model.SellAuctionResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	v := s.viewer(ctx, userID)
	auctions, err := s.auctionRepo.Search(ctx, sqlc.SearchSellAuctionsParams{
		SearchTerm:      searchTerm,
		Limit:           pageSize,
		Offset:          (page - 1) * pageSize,
		ExcludeOwnerID:  userID,
		UserInterestIds: interestIDs,
		FilterRegionID:  v.regionID,
		OwnerJobKeys:    v.ownerJobKeys(),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search auctions: %w", err)
	}

	total, err := s.auctionRepo.CountSearch(ctx, searchTerm, userID, v.regionID, v.ownerJobKeys())
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	var responses []model.SellAuctionResponse
	for _, auction := range auctions {
		responses = append(responses, *s.toResponse(auction, userID))
	}

	return responses, total, nil
}

func (s *SellAuctionService) toResponse(auction sqlc.SellAuction, requestingUserID int32) *model.SellAuctionResponse {
	quantityStr := numericToString(auction.Quantity)
	priceStr := numericToString(auction.UnitPrice)

	var desc *string
	if auction.Description.Valid {
		desc = &auction.Description.String
	}

	return &model.SellAuctionResponse{
		PublicID: auction.PublicID.String(),
		// OwnerName:     auction.OwnerName,
		RegionName:    auction.RegionName,
		InterestName:  auction.InterestName,
		Title:         auction.Title,
		Description:   desc,
		ImageURL:      auction.ImageUrl,
		Unit:          auction.Unit,
		Quantity:      quantityStr,
		UnitPrice:     priceStr,
		BuyAllFromOne: auction.BuyAllFromOne,
		BidCount:      auction.BidCount,
		EndTime:       auction.EndTime.Time.Format(time.RFC3339),
		Status:        auction.Status,
		// Only the owner is told why their post was rejected or suspended.
		ModerationReason: moderationReasonFor(auction.ModerationReason, auction.OwnerID == requestingUserID),
		IsOwner:          auction.OwnerID == requestingUserID,
		IsExpired:        auction.EndTime.Time.Before(time.Now()),
		CreatedAt:        auction.CreatedAt.Time.Format(time.RFC3339),
	}
}

// notifyRegion scopes a broadcast to the post's governorate when region
// filtering is on, so nobody is notified about a post their feed hides.
// Returns 0 (no filter) when the feature is off.
func (s *SellAuctionService) notifyRegion(postRegionID int32) int32 {
	if s.regionFilter {
		return postRegionID
	}
	return 0
}

// viewer resolves the region and role filters for whoever is asking. The lookup
// is skipped entirely when neither feature needs it.
func (s *SellAuctionService) viewer(ctx context.Context, userID int32) viewerContext {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		slog.Error("failed to load viewer for listing filters", "error", err, "user_id", userID)
		return viewerContext{}
	}

	v := viewerContext{isRetailer: user.JobKey == RetailerRoleKey}
	if s.regionFilter && user.RegionID.Valid {
		v.regionID = user.RegionID.Int32
	}
	return v
}

// initialStatus decides whether a new post goes live immediately or waits for an
// admin. end_time is still written at creation because the column is NOT NULL,
// but it is recomputed on approval — nothing reads it while the post is not active.
func (s *SellAuctionService) initialStatus() string {
	if s.postApproval {
		return "pending_approval"
	}
	return "active"
}

// SupplySideRoles are the merchant types a retailer is allowed to see sell posts
// from. Kept in sync with SUPPLY_SIDE_ROLES in frontend RegisterPage.tsx, which
// decides who is asked the supplies-to-retail question — if these two disagree,
// a merchant can be offered that question yet stay invisible to retailers.
var SupplySideRoles = []string{"importer", "wholesaler", "distributor", "processor", "supplier"}

// RetailerRoleKey matches the jobs row added in migration 040.
const RetailerRoleKey = "retailer"

// viewerContext is what a listing request needs to know about whoever is asking:
// which governorate to scope to (#11) and whether they are a retailer (#7). One
// lookup serves both — the region filter already paid for this query.
type viewerContext struct {
	regionID   int32
	isRetailer bool
}

// ownerJobKeys returns the roles a viewer may see sell posts from; empty means
// no restriction.
func (v viewerContext) ownerJobKeys() []string {
	if v.isRetailer {
		return SupplySideRoles
	}
	return nil
}

// isSupplySideRole reports whether posts from this role appear in retailer feeds.
func isSupplySideRole(jobKey string) bool {
	for _, r := range SupplySideRoles {
		if r == jobKey {
			return true
		}
	}
	return false
}
