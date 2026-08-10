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
	"rabhana/pkg/errs"
)

type BuyRequestService struct {
	requestRepo        repository.BuyRequestRepository
	queries            *sqlc.Queries
	notificationSender NotificationSender
	auctionDuration    time.Duration
	uploadService      UploadService
	defaultImageURL    string
	regionFilter       bool
}

func NewBuyRequestService(
	requestRepo repository.BuyRequestRepository,
	queries *sqlc.Queries,
	notificationSender NotificationSender,
	auctionDurationHours int,
	uploadService UploadService,
	defaultImageURL string,
	regionFilter bool,
) *BuyRequestService {
	duration := time.Duration(auctionDurationHours) * time.Hour
	if duration == 0 {
		duration = 24 * time.Hour
	}
	return &BuyRequestService{
		requestRepo:        requestRepo,
		queries:            queries,
		notificationSender: notificationSender,
		auctionDuration:    duration,
		uploadService:      uploadService,
		defaultImageURL:    defaultImageURL,
		regionFilter:       regionFilter,
	}
}

func (s *BuyRequestService) CreateBuyRequest(ctx context.Context, userID int32, req model.CreateBuyRequestRequest, imageFile []byte, imageFilename string) (*model.BuyRequestResponse, error) {
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

	// Check if tier allows creating buy requests
	if !userSub.CanCreateBuyRequests.Valid || !userSub.CanCreateBuyRequests.Bool {
		return nil, errs.ErrInsufficientTier
	}

	// Check monthly limit (skip if unlimited/zico)
	if userSub.MaxBuyRequestsPerMonth.Valid {
		maxRequests := userSub.MaxBuyRequestsPerMonth.Int32
		current := userSub.RequestsCreatedThisMonth.Int32

		// Reset if new month
		if userSub.MonthResetAt.Time.Before(time.Now().AddDate(0, -1, 0)) {
			s.queries.ResetMonthlyCounts(ctx)
			current = 0
		}

		if current >= maxRequests {
			return nil, errs.ErrMonthlyLimit
		}

		// Increment counter
		s.queries.IncrementRequestCount(ctx, userSub.SubscriptionID.Int32)
	}

	// Lookup names for caching
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
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

	request, err := s.requestRepo.Create(ctx, sqlc.CreateBuyRequestParams{
		OwnerID:       userID,
		RegionID:      regionID,
		InterestID:    req.InterestID,
		Title:         req.Title,
		Description:   description,
		ImageUrl:      imageURL,
		Unit:          req.Unit,
		Quantity:      decimalToNumeric(quantity),
		BuyAllFromOne: buyAllFromOne,
		EndTime:       pgtype.Timestamptz{Time: endTime, Valid: true},
		OwnerName:     user.Name,
		RegionName:    region.NameAr,
		InterestName:  interest.NameAr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	go func(interestNameAr, requestTitle, requestPublicID string, interestID, requestID, ownerID, postRegionID int32) {
		bgCtx := context.Background()
		users, err := s.queries.GetActiveUsersByInterest(bgCtx, sqlc.GetActiveUsersByInterestParams{
			InterestID:     interestID,
			ExcludeUserID:  ownerID,
			FilterRegionID: s.notifyRegion(postRegionID),
		})
		if err != nil {
			slog.Error("broadcast buy request: get interested users failed", "request_id", requestID, "error", err)
			return
		}
		title := interestNameAr + " - طلب شراء جديد"
		for _, uid := range users {
			s.notificationSender.SendToUser(bgCtx, uid, title, requestTitle, map[string]string{
				"type":       "new_buy_request",
				"request_id": requestPublicID,
			})
		}
		if err := s.queries.MarkBuyRequestNotified(bgCtx, requestID); err != nil {
			slog.Error("broadcast buy request: mark notified failed", "request_id", requestID, "error", err)
		}
	}(interest.NameAr, request.Title, request.PublicID.String(), request.InterestID, request.ID, request.OwnerID, request.RegionID)

	return s.toResponse(request, userID), nil
}

func (s *BuyRequestService) CancelBuyRequest(ctx context.Context, userID int32, requestPublicID uuid.UUID) error {
	request, err := s.requestRepo.GetByPublicID(ctx, requestPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrAuctionNotFound
		}
		return fmt.Errorf("failed to get request: %w", err)
	}

	if request.OwnerID != userID {
		return errs.ErrNotAuctionOwner
	}

	if request.Status != "active" {
		return errs.ErrAuctionNotActive
	}

	if request.OfferCount > 0 {
		return errs.ErrCannotCancelWithBids
	}

	cancellations, _ := s.requestRepo.CountMonthlyCancellations(ctx, userID)
	if cancellations >= 3 {
		return errs.ErrMaxCancellations
	}

	return s.requestRepo.Cancel(ctx, request.ID)
}

func (s *BuyRequestService) GetBuyRequestDetail(ctx context.Context, requestPublicID uuid.UUID, requestingUserID int32) (*model.BuyRequestResponse, error) {
	request, err := s.requestRepo.GetByPublicID(ctx, requestPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrAuctionNotFound
		}
		return nil, fmt.Errorf("failed to get request: %w", err)
	}

	return s.toResponse(request, requestingUserID), nil
}

func (s *BuyRequestService) ListMyBuyRequests(ctx context.Context, userID int32, page, pageSize int32) ([]model.BuyRequestResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	requests, err := s.requestRepo.ListByOwner(ctx, sqlc.ListBuyRequestsByOwnerParams{
		OwnerID: userID,
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list requests: %w", err)
	}

	total, err := s.requestRepo.CountByOwner(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count requests: %w", err)
	}

	var responses []model.BuyRequestResponse
	for _, request := range requests {
		responses = append(responses, *s.toResponse(request, userID))
	}

	return responses, total, nil
}

func (s *BuyRequestService) ListActiveBuyRequests(ctx context.Context, userID int32, page, pageSize int32, interestIDs []int32) ([]model.BuyRequestResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filterRegion := s.viewerRegion(ctx, userID)
	requests, err := s.requestRepo.ListActive(ctx, sqlc.ListActiveBuyRequestsParams{
		Limit:                  pageSize,
		Offset:                 (page - 1) * pageSize,
		ExcludeOwnerID:         userID,
		ExcludeOfferedRequests: nil,
		UserID:                 userID,
		UserInterestIds:        interestIDs,
		FilterRegionID:         filterRegion,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list requests: %w", err)
	}

	total, err := s.requestRepo.CountActive(ctx, userID, filterRegion)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count requests: %w", err)
	}

	var responses []model.BuyRequestResponse
	for _, request := range requests {
		responses = append(responses, *s.toResponse(request, userID))
	}

	return responses, total, nil
}

func (s *BuyRequestService) SearchBuyRequests(ctx context.Context, userID int32, searchTerm string, page, pageSize int32, interestIDs []int32) ([]model.BuyRequestResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filterRegion := s.viewerRegion(ctx, userID)
	requests, err := s.requestRepo.Search(ctx, sqlc.SearchBuyRequestsParams{
		SearchTerm:      searchTerm,
		Limit:           pageSize,
		Offset:          (page - 1) * pageSize,
		ExcludeOwnerID:  userID,
		UserInterestIds: interestIDs,
		FilterRegionID:  filterRegion,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search requests: %w", err)
	}

	total, err := s.requestRepo.CountSearch(ctx, searchTerm, userID, filterRegion)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	var responses []model.BuyRequestResponse
	for _, request := range requests {
		responses = append(responses, *s.toResponse(request, userID))
	}

	return responses, total, nil
}

func (s *BuyRequestService) toResponse(request sqlc.BuyRequest, requestingUserID int32) *model.BuyRequestResponse {
	quantityStr := numericToString(request.Quantity)
	fulfilledStr := numericToString(request.FulfilledQuantity)

	var desc *string
	if request.Description.Valid {
		desc = &request.Description.String
	}

	return &model.BuyRequestResponse{
		PublicID: request.PublicID.String(),
		// OwnerName:          request.OwnerName,
		RegionName:         request.RegionName,
		InterestName:       request.InterestName,
		Title:              request.Title,
		Description:        desc,
		ImageURL:           request.ImageUrl,
		Unit:               request.Unit,
		Quantity:           quantityStr,
		BuyAllFromOne:      request.BuyAllFromOne,
		OfferCount:         request.OfferCount,
		AcceptedOfferCount: request.AcceptedOfferCount,
		FulfilledQuantity:  fulfilledStr,
		EndTime:            request.EndTime.Time.Format(time.RFC3339),
		Status:             request.Status,
		IsOwner:            request.OwnerID == requestingUserID,
		IsExpired:          request.EndTime.Time.Before(time.Now()),
		CreatedAt:          request.CreatedAt.Time.Format(time.RFC3339),
	}
}

// See SellAuctionService.notifyRegion.
func (s *BuyRequestService) notifyRegion(postRegionID int32) int32 {
	if s.regionFilter {
		return postRegionID
	}
	return 0
}

// viewerRegion returns the governorate a listing should be filtered to, or 0
// for "no filter". The user lookup only happens while the feature is enabled,
// so the default configuration costs nothing extra per request.
func (s *BuyRequestService) viewerRegion(ctx context.Context, userID int32) int32 {
	if !s.regionFilter {
		return 0
	}
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		slog.Error("region filter: failed to load viewer", "error", err, "user_id", userID)
		return 0
	}
	if !user.RegionID.Valid {
		return 0
	}
	return user.RegionID.Int32
}
