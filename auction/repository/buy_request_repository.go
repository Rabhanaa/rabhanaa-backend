package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
)

type BuyRequestRepository interface {
	Create(ctx context.Context, params sqlc.CreateBuyRequestParams) (sqlc.BuyRequest, error)
	GetByID(ctx context.Context, id int32) (sqlc.BuyRequest, error)
	GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.BuyRequest, error)
	GetByPublicIDForUpdate(ctx context.Context, publicID uuid.UUID) (sqlc.BuyRequest, error)
	ListActive(ctx context.Context, params sqlc.ListActiveBuyRequestsParams) ([]sqlc.BuyRequest, error)
	CountActive(ctx context.Context, excludeOwnerID int32) (int64, error)
	Search(ctx context.Context, params sqlc.SearchBuyRequestsParams) ([]sqlc.BuyRequest, error)
	CountSearch(ctx context.Context, searchTerm string, excludeOwnerID int32) (int64, error)
	ListByOwner(ctx context.Context, params sqlc.ListBuyRequestsByOwnerParams) ([]sqlc.BuyRequest, error)
	CountByOwner(ctx context.Context, ownerID int32) (int64, error)
	UpdateStatus(ctx context.Context, params sqlc.UpdateBuyRequestStatusParams) error
	IncrementOfferCount(ctx context.Context, id int32) error
	IncrementAcceptedOfferCount(ctx context.Context, id int32) error
	UpdateFulfilledQuantity(ctx context.Context, params sqlc.UpdateBuyRequestFulfilledQuantityParams) error
	Cancel(ctx context.Context, id int32) error
	GetExpiredActive(ctx context.Context) ([]sqlc.BuyRequest, error)
	GetExpiredPendingSelection(ctx context.Context) ([]sqlc.BuyRequest, error)
	GetSoonExpiringSelection(ctx context.Context) ([]sqlc.BuyRequest, error)
	CountMonthlyCancellations(ctx context.Context, ownerID int32) (int64, error)
	RevertToPendingSelection(ctx context.Context, requestID int32) error
}

type buyRequestRepository struct {
	queries *sqlc.Queries
}

func NewBuyRequestRepository(queries *sqlc.Queries) BuyRequestRepository {
	return &buyRequestRepository{queries: queries}
}

func (r *buyRequestRepository) Create(ctx context.Context, params sqlc.CreateBuyRequestParams) (sqlc.BuyRequest, error) {
	return r.queries.CreateBuyRequest(ctx, params)
}

func (r *buyRequestRepository) GetByID(ctx context.Context, id int32) (sqlc.BuyRequest, error) {
	return r.queries.GetBuyRequestByID(ctx, id)
}

func (r *buyRequestRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.BuyRequest, error) {
	return r.queries.GetBuyRequestByPublicID(ctx, pgtype.UUID{Bytes: publicID, Valid: true})
}

func (r *buyRequestRepository) GetByPublicIDForUpdate(ctx context.Context, publicID uuid.UUID) (sqlc.BuyRequest, error) {
	return r.queries.GetBuyRequestByPublicIDForUpdate(ctx, pgtype.UUID{Bytes: publicID, Valid: true})
}

func (r *buyRequestRepository) ListActive(ctx context.Context, params sqlc.ListActiveBuyRequestsParams) ([]sqlc.BuyRequest, error) {
	return r.queries.ListActiveBuyRequests(ctx, params)
}

func (r *buyRequestRepository) CountActive(ctx context.Context, excludeOwnerID int32) (int64, error) {
	return r.queries.CountActiveBuyRequests(ctx, sqlc.CountActiveBuyRequestsParams{
		ExcludeOwnerID:         excludeOwnerID,
		ExcludeOfferedRequests: nil,
		UserID:                 0,
	})
}

func (r *buyRequestRepository) Search(ctx context.Context, params sqlc.SearchBuyRequestsParams) ([]sqlc.BuyRequest, error) {
	return r.queries.SearchBuyRequests(ctx, params)
}

func (r *buyRequestRepository) CountSearch(ctx context.Context, searchTerm string, excludeOwnerID int32) (int64, error) {
	return r.queries.CountSearchBuyRequests(ctx, sqlc.CountSearchBuyRequestsParams{
		SearchTerm:             searchTerm,
		ExcludeOwnerID:         excludeOwnerID,
		ExcludeOfferedRequests: nil,
		UserID:                 0,
	})
}

func (r *buyRequestRepository) ListByOwner(ctx context.Context, params sqlc.ListBuyRequestsByOwnerParams) ([]sqlc.BuyRequest, error) {
	return r.queries.ListBuyRequestsByOwner(ctx, params)
}

func (r *buyRequestRepository) CountByOwner(ctx context.Context, ownerID int32) (int64, error) {
	return r.queries.CountBuyRequestsByOwner(ctx, ownerID)
}

func (r *buyRequestRepository) UpdateStatus(ctx context.Context, params sqlc.UpdateBuyRequestStatusParams) error {
	return r.queries.UpdateBuyRequestStatus(ctx, params)
}

func (r *buyRequestRepository) IncrementOfferCount(ctx context.Context, id int32) error {
	return r.queries.IncrementBuyRequestOfferCount(ctx, id)
}

func (r *buyRequestRepository) IncrementAcceptedOfferCount(ctx context.Context, id int32) error {
	return r.queries.IncrementBuyRequestAcceptedOfferCount(ctx, id)
}

func (r *buyRequestRepository) UpdateFulfilledQuantity(ctx context.Context, params sqlc.UpdateBuyRequestFulfilledQuantityParams) error {
	return r.queries.UpdateBuyRequestFulfilledQuantity(ctx, params)
}

func (r *buyRequestRepository) Cancel(ctx context.Context, id int32) error {
	return r.queries.CancelBuyRequest(ctx, id)
}

func (r *buyRequestRepository) GetExpiredActive(ctx context.Context) ([]sqlc.BuyRequest, error) {
	return r.queries.GetExpiredActiveBuyRequests(ctx)
}

func (r *buyRequestRepository) GetExpiredPendingSelection(ctx context.Context) ([]sqlc.BuyRequest, error) {
	return r.queries.GetExpiredPendingSelectionBuyRequests(ctx)
}

func (r *buyRequestRepository) GetSoonExpiringSelection(ctx context.Context) ([]sqlc.BuyRequest, error) {
	return r.queries.GetSoonExpiringSelectionBuyRequests(ctx)
}

func (r *buyRequestRepository) CountMonthlyCancellations(ctx context.Context, ownerID int32) (int64, error) {
	return r.queries.CountMonthlyBuyCancellations(ctx, ownerID)
}

func (r *buyRequestRepository) RevertToPendingSelection(ctx context.Context, requestID int32) error {
	return r.queries.RevertBuyRequestToPendingSelection(ctx, requestID)
}
