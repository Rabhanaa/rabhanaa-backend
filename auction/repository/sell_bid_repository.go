package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
)

type SellBidRepository interface {
	Create(ctx context.Context, params sqlc.CreateSellBidParams) (sqlc.SellBid, error)
	GetByID(ctx context.Context, id int32) (sqlc.SellBid, error)
	GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.SellBid, error)
	GetByAuctionAndBidder(ctx context.Context, auctionID, bidderID int32) (sqlc.SellBid, error)
	ListByAuction(ctx context.Context, auctionID int32) ([]sqlc.ListSellBidsByAuctionRow, error)
	ListByBidder(ctx context.Context, params sqlc.ListSellBidsByBidderParams) ([]sqlc.ListSellBidsByBidderRow, error)
	CountByAuction(ctx context.Context, auctionID int32) (int64, error)
	CountActiveBidsByBidder(ctx context.Context, bidderID int32) (int64, error)
	SelectBid(ctx context.Context, id int32) error
	MarkNotChosen(ctx context.Context, auctionID int32) error
	GetLosingBidderIDs(ctx context.Context, auctionID, exceptBidID int32) ([]int32, error)
}

type sellBidRepository struct {
	queries *sqlc.Queries
}

func NewSellBidRepository(queries *sqlc.Queries) SellBidRepository {
	return &sellBidRepository{queries: queries}
}

func (r *sellBidRepository) Create(ctx context.Context, params sqlc.CreateSellBidParams) (sqlc.SellBid, error) {
	return r.queries.CreateSellBid(ctx, params)
}

func (r *sellBidRepository) GetByID(ctx context.Context, id int32) (sqlc.SellBid, error) {
	return r.queries.GetSellBidByID(ctx, id)
}

func (r *sellBidRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.SellBid, error) {
	return r.queries.GetSellBidByPublicID(ctx, pgtype.UUID{Bytes: publicID, Valid: true})
}

func (r *sellBidRepository) GetByAuctionAndBidder(ctx context.Context, auctionID, bidderID int32) (sqlc.SellBid, error) {
	return r.queries.GetSellBidByAuctionAndBidder(ctx, sqlc.GetSellBidByAuctionAndBidderParams{
		AuctionID: auctionID,
		BidderID:  bidderID,
	})
}

func (r *sellBidRepository) ListByAuction(ctx context.Context, auctionID int32) ([]sqlc.ListSellBidsByAuctionRow, error) {
	return r.queries.ListSellBidsByAuction(ctx, auctionID)
}

func (r *sellBidRepository) ListByBidder(ctx context.Context, params sqlc.ListSellBidsByBidderParams) ([]sqlc.ListSellBidsByBidderRow, error) {
	return r.queries.ListSellBidsByBidder(ctx, params)
}

func (r *sellBidRepository) CountByAuction(ctx context.Context, auctionID int32) (int64, error) {
	return r.queries.CountSellBidsByAuction(ctx, auctionID)
}

func (r *sellBidRepository) CountActiveBidsByBidder(ctx context.Context, bidderID int32) (int64, error) {
	return r.queries.CountActiveSellBidsByBidder(ctx, bidderID)
}

func (r *sellBidRepository) SelectBid(ctx context.Context, id int32) error {
	return r.queries.SelectSellBid(ctx, id)
}

func (r *sellBidRepository) MarkNotChosen(ctx context.Context, auctionID int32) error {
	return r.queries.MarkNotChosenSellBids(ctx, auctionID)
}

func (r *sellBidRepository) GetLosingBidderIDs(ctx context.Context, auctionID, exceptBidID int32) ([]int32, error) {
	return r.queries.GetLosingBidderIDsForAuction(ctx, sqlc.GetLosingBidderIDsForAuctionParams{
		AuctionID: auctionID,
		ID:        exceptBidID,
	})
}
