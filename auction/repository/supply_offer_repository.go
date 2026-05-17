package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
)

type SupplyOfferRepository interface {
	Create(ctx context.Context, params sqlc.CreateSupplyOfferParams) (sqlc.SupplyOffer, error)
	GetByID(ctx context.Context, id int32) (sqlc.SupplyOffer, error)
	GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.SupplyOffer, error)
	GetByRequestAndSupplier(ctx context.Context, requestID, supplierID int32) (sqlc.SupplyOffer, error)
	ListByRequest(ctx context.Context, requestID int32) ([]sqlc.ListSupplyOffersByRequestRow, error)
	ListBySupplier(ctx context.Context, params sqlc.ListSupplyOffersBySupplierParams) ([]sqlc.ListSupplyOffersBySupplierRow, error)
	CountByRequest(ctx context.Context, requestID int32) (int64, error)
	CountActiveOffersBySupplier(ctx context.Context, supplierID int32) (int64, error)
	AcceptOffer(ctx context.Context, id int32) error
	SumAcceptedQuantityByRequest(ctx context.Context, requestID int32) (interface{}, error)
	ListAcceptedOffersByRequest(ctx context.Context, requestID int32) ([]sqlc.SupplyOffer, error)
	MarkNotChosen(ctx context.Context, buyRequestID int32) error
	GetUnacceptedSupplierIDs(ctx context.Context, requestID int32) ([]int32, error)
}

type supplyOfferRepository struct {
	queries *sqlc.Queries
}

func NewSupplyOfferRepository(queries *sqlc.Queries) SupplyOfferRepository {
	return &supplyOfferRepository{queries: queries}
}

func (r *supplyOfferRepository) Create(ctx context.Context, params sqlc.CreateSupplyOfferParams) (sqlc.SupplyOffer, error) {
	return r.queries.CreateSupplyOffer(ctx, params)
}

func (r *supplyOfferRepository) GetByID(ctx context.Context, id int32) (sqlc.SupplyOffer, error) {
	return r.queries.GetSupplyOfferByID(ctx, id)
}

func (r *supplyOfferRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (sqlc.SupplyOffer, error) {
	return r.queries.GetSupplyOfferByPublicID(ctx, pgtype.UUID{Bytes: publicID, Valid: true})
}

func (r *supplyOfferRepository) GetByRequestAndSupplier(ctx context.Context, requestID, supplierID int32) (sqlc.SupplyOffer, error) {
	return r.queries.GetSupplyOfferByRequestAndSupplier(ctx, sqlc.GetSupplyOfferByRequestAndSupplierParams{
		BuyRequestID: requestID,
		SupplierID:   supplierID,
	})
}

func (r *supplyOfferRepository) ListByRequest(ctx context.Context, requestID int32) ([]sqlc.ListSupplyOffersByRequestRow, error) {
	return r.queries.ListSupplyOffersByRequest(ctx, requestID)
}

func (r *supplyOfferRepository) ListBySupplier(ctx context.Context, params sqlc.ListSupplyOffersBySupplierParams) ([]sqlc.ListSupplyOffersBySupplierRow, error) {
	return r.queries.ListSupplyOffersBySupplier(ctx, params)
}

func (r *supplyOfferRepository) CountByRequest(ctx context.Context, requestID int32) (int64, error) {
	return r.queries.CountSupplyOffersByRequest(ctx, requestID)
}

func (r *supplyOfferRepository) CountActiveOffersBySupplier(ctx context.Context, supplierID int32) (int64, error) {
	return r.queries.CountActiveSupplyOffersBySupplier(ctx, supplierID)
}

func (r *supplyOfferRepository) AcceptOffer(ctx context.Context, id int32) error {
	return r.queries.AcceptSupplyOffer(ctx, id)
}

func (r *supplyOfferRepository) SumAcceptedQuantityByRequest(ctx context.Context, requestID int32) (interface{}, error) {
	return r.queries.SumAcceptedQuantityByRequest(ctx, requestID)
}

func (r *supplyOfferRepository) ListAcceptedOffersByRequest(ctx context.Context, requestID int32) ([]sqlc.SupplyOffer, error) {
	return r.queries.ListAcceptedOffersByRequest(ctx, requestID)
}

func (r *supplyOfferRepository) MarkNotChosen(ctx context.Context, buyRequestID int32) error {
	return r.queries.MarkNotChosenSupplyOffers(ctx, buyRequestID)
}

func (r *supplyOfferRepository) GetUnacceptedSupplierIDs(ctx context.Context, requestID int32) ([]int32, error) {
	return r.queries.GetUnacceptedSupplierIDsForRequest(ctx, requestID)
}
