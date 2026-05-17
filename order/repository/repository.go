package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
)

type OrderRepository interface {
	CreateFromSellAuction(ctx context.Context, params sqlc.CreateOrderFromSellAuctionParams) (sqlc.Order, error)
	CreateFromBuyRequest(ctx context.Context, params sqlc.CreateOrderFromBuyRequestParams) (sqlc.Order, error)
	GetByID(ctx context.Context, id int32) (sqlc.Order, error)
	GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.Order, error)
	ListByUser(ctx context.Context, params sqlc.ListOrdersByUserParams) ([]sqlc.Order, error)
	CountByUser(ctx context.Context, userID int32) (int64, error)
	ConfirmAsSeller(ctx context.Context, id int32) error
	ConfirmAsBuyer(ctx context.Context, id int32) error
	Complete(ctx context.Context, id int32) error
	CheckExistsForSellAuction(ctx context.Context, auctionID int32) (bool, error)
	CheckExistsForBuyRequestAndSupplier(ctx context.Context, requestID, supplierID int32) (bool, error)
	GetOrdersPendingConfirmation(ctx context.Context) ([]sqlc.Order, error)
	CancelOrder(ctx context.Context, orderID int32) error
}

type orderRepository struct {
	queries *sqlc.Queries
}

func NewOrderRepository(queries *sqlc.Queries) OrderRepository {
	return &orderRepository{queries: queries}
}

func (r *orderRepository) CreateFromSellAuction(ctx context.Context, params sqlc.CreateOrderFromSellAuctionParams) (sqlc.Order, error) {
	return r.queries.CreateOrderFromSellAuction(ctx, params)
}

func (r *orderRepository) CreateFromBuyRequest(ctx context.Context, params sqlc.CreateOrderFromBuyRequestParams) (sqlc.Order, error) {
	return r.queries.CreateOrderFromBuyRequest(ctx, params)
}

func (r *orderRepository) GetByID(ctx context.Context, id int32) (sqlc.Order, error) {
	return r.queries.GetOrderByID(ctx, id)
}

func (r *orderRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.Order, error) {
	return r.queries.GetOrderByPublicID(ctx, pgtype.UUID{Bytes: publicID, Valid: true})
}

func (r *orderRepository) ListByUser(ctx context.Context, params sqlc.ListOrdersByUserParams) ([]sqlc.Order, error) {
	return r.queries.ListOrdersByUser(ctx, params)
}

func (r *orderRepository) CountByUser(ctx context.Context, userID int32) (int64, error) {
	return r.queries.CountOrdersByUser(ctx, userID)
}

func (r *orderRepository) ConfirmAsSeller(ctx context.Context, id int32) error {
	return r.queries.ConfirmOrderAsSeller(ctx, id)
}

func (r *orderRepository) ConfirmAsBuyer(ctx context.Context, id int32) error {
	return r.queries.ConfirmOrderAsBuyer(ctx, id)
}

func (r *orderRepository) Complete(ctx context.Context, id int32) error {
	return r.queries.CompleteOrder(ctx, id)
}

func (r *orderRepository) CheckExistsForSellAuction(ctx context.Context, auctionID int32) (bool, error) {
	return r.queries.CheckOrderExistsForSellAuction(ctx, pgtype.Int4{Int32: auctionID, Valid: true})
}

func (r *orderRepository) CheckExistsForBuyRequestAndSupplier(ctx context.Context, requestID, supplierID int32) (bool, error) {
	return r.queries.CheckOrderExistsForBuyRequestAndSupplier(ctx, sqlc.CheckOrderExistsForBuyRequestAndSupplierParams{
		BuyRequestID: pgtype.Int4{Int32: requestID, Valid: true},
		SellerID:     supplierID,
	})
}

func (r *orderRepository) GetOrdersPendingConfirmation(ctx context.Context) ([]sqlc.Order, error) {
	return r.queries.GetOrdersPendingConfirmation(ctx)
}

func (r *orderRepository) CancelOrder(ctx context.Context, orderID int32) error {
	return r.queries.CancelOrder(ctx, orderID)
}
