package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
)

type SellAuctionRepository interface {
	Create(ctx context.Context, params sqlc.CreateSellAuctionParams) (sqlc.SellAuction, error)
	GetByID(ctx context.Context, id int32) (sqlc.SellAuction, error)
	GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.SellAuction, error)
	GetByPublicIDForUpdate(ctx context.Context, publicID uuid.UUID) (sqlc.SellAuction, error)
	ListActive(ctx context.Context, params sqlc.ListActiveSellAuctionsParams) ([]sqlc.SellAuction, error)
	CountActive(ctx context.Context, excludeOwnerID, filterRegionID int32) (int64, error)
	Search(ctx context.Context, params sqlc.SearchSellAuctionsParams) ([]sqlc.SellAuction, error)
	CountSearch(ctx context.Context, searchTerm string, excludeOwnerID, filterRegionID int32) (int64, error)
	ListByOwner(ctx context.Context, params sqlc.ListSellAuctionsByOwnerParams) ([]sqlc.SellAuction, error)
	CountByOwner(ctx context.Context, ownerID int32) (int64, error)
	UpdateStatus(ctx context.Context, params sqlc.UpdateSellAuctionStatusParams) error
	IncrementBidCount(ctx context.Context, id int32) error
	SelectWinner(ctx context.Context, params sqlc.SelectSellWinnerParams) error
	Cancel(ctx context.Context, id int32) error
	GetExpiredActive(ctx context.Context) ([]sqlc.SellAuction, error)
	GetExpiredPendingSelection(ctx context.Context, windowHours int32) ([]sqlc.SellAuction, error)
	GetSoonExpiringSelection(ctx context.Context, windowHours int32) ([]sqlc.SellAuction, error)
	MarkSelectionWarned(ctx context.Context, id int32) error
	CountMonthlyCancellations(ctx context.Context, ownerID int32) (int64, error)
	RevertToPendingSelection(ctx context.Context, auctionID int32) error
}

type sellAuctionRepository struct {
	queries *sqlc.Queries
}

func NewSellAuctionRepository(queries *sqlc.Queries) SellAuctionRepository {
	return &sellAuctionRepository{queries: queries}
}

func (r *sellAuctionRepository) Create(ctx context.Context, params sqlc.CreateSellAuctionParams) (sqlc.SellAuction, error) {
	return r.queries.CreateSellAuction(ctx, params)
}

func (r *sellAuctionRepository) GetByID(ctx context.Context, id int32) (sqlc.SellAuction, error) {
	return r.queries.GetSellAuctionByID(ctx, id)
}

func (r *sellAuctionRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.SellAuction, error) {
	return r.queries.GetSellAuctionByPublicID(ctx, pgtype.UUID{Bytes: publicID, Valid: true})
}

func (r *sellAuctionRepository) GetByPublicIDForUpdate(ctx context.Context, publicID uuid.UUID) (sqlc.SellAuction, error) {
	return r.queries.GetSellAuctionByPublicIDForUpdate(ctx, pgtype.UUID{Bytes: publicID, Valid: true})
}

func (r *sellAuctionRepository) ListActive(ctx context.Context, params sqlc.ListActiveSellAuctionsParams) ([]sqlc.SellAuction, error) {
	return r.queries.ListActiveSellAuctions(ctx, params)
}

func (r *sellAuctionRepository) CountActive(ctx context.Context, excludeOwnerID, filterRegionID int32) (int64, error) {
	return r.queries.CountActiveSellAuctions(ctx, sqlc.CountActiveSellAuctionsParams{
		FilterRegionID:        filterRegionID,
		ExcludeOwnerID:        excludeOwnerID,
		ExcludeBiddedAuctions: nil,
		UserID:                0,
	})
}

func (r *sellAuctionRepository) Search(ctx context.Context, params sqlc.SearchSellAuctionsParams) ([]sqlc.SellAuction, error) {
	return r.queries.SearchSellAuctions(ctx, params)
}

func (r *sellAuctionRepository) CountSearch(ctx context.Context, searchTerm string, excludeOwnerID, filterRegionID int32) (int64, error) {
	return r.queries.CountSearchSellAuctions(ctx, sqlc.CountSearchSellAuctionsParams{
		FilterRegionID:        filterRegionID,
		SearchTerm:            searchTerm,
		ExcludeOwnerID:        excludeOwnerID,
		ExcludeBiddedAuctions: nil,
		UserID:                0,
	})
}

func (r *sellAuctionRepository) ListByOwner(ctx context.Context, params sqlc.ListSellAuctionsByOwnerParams) ([]sqlc.SellAuction, error) {
	return r.queries.ListSellAuctionsByOwner(ctx, params)
}

func (r *sellAuctionRepository) CountByOwner(ctx context.Context, ownerID int32) (int64, error) {
	return r.queries.CountSellAuctionsByOwner(ctx, ownerID)
}

func (r *sellAuctionRepository) UpdateStatus(ctx context.Context, params sqlc.UpdateSellAuctionStatusParams) error {
	return r.queries.UpdateSellAuctionStatus(ctx, params)
}

func (r *sellAuctionRepository) IncrementBidCount(ctx context.Context, id int32) error {
	return r.queries.IncrementSellAuctionBidCount(ctx, id)
}

func (r *sellAuctionRepository) SelectWinner(ctx context.Context, params sqlc.SelectSellWinnerParams) error {
	return r.queries.SelectSellWinner(ctx, params)
}

func (r *sellAuctionRepository) Cancel(ctx context.Context, id int32) error {
	return r.queries.CancelSellAuction(ctx, id)
}

func (r *sellAuctionRepository) GetExpiredActive(ctx context.Context) ([]sqlc.SellAuction, error) {
	return r.queries.GetExpiredActiveSellAuctions(ctx)
}

func (r *sellAuctionRepository) GetExpiredPendingSelection(ctx context.Context, windowHours int32) ([]sqlc.SellAuction, error) {
	return r.queries.GetExpiredPendingSelectionSellAuctions(ctx, windowHours)
}

func (r *sellAuctionRepository) GetSoonExpiringSelection(ctx context.Context, windowHours int32) ([]sqlc.SellAuction, error) {
	return r.queries.GetSoonExpiringSelectionSellAuctions(ctx, windowHours)
}

func (r *sellAuctionRepository) MarkSelectionWarned(ctx context.Context, id int32) error {
	return r.queries.MarkSellAuctionSelectionWarned(ctx, id)
}

func (r *sellAuctionRepository) CountMonthlyCancellations(ctx context.Context, ownerID int32) (int64, error) {
	return r.queries.CountMonthlySellCancellations(ctx, ownerID)
}

func (r *sellAuctionRepository) RevertToPendingSelection(ctx context.Context, auctionID int32) error {
	return r.queries.RevertSellAuctionToPendingSelection(ctx, auctionID)
}
