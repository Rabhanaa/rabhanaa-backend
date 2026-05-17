package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type SupplyOffer struct {
	ID              int32
	PublicID        uuid.UUID
	BuyRequestID    int32
	SupplierID      int32
	PricePerUnit    decimal.Decimal
	OfferedQuantity decimal.Decimal
	IsAccepted      bool
	AcceptedAt      pgtype.Timestamptz
	RequestTitle    string
	RequestQuantity decimal.Decimal
	RequestUnit     string
	CreatedAt       time.Time
}

type PlaceSupplyOfferRequest struct {
	PricePerUnit    float64 `json:"price_per_unit" binding:"required,gt=0"`
	OfferedQuantity float64 `json:"offered_quantity" binding:"omitempty,gt=0"`
}

type SupplyOfferResponse struct {
	PublicID        string  `json:"public_id"`
	SupplierName    string  `json:"supplier_name"`
	SupplierRegion  string  `json:"supplier_region"`
	SupplierPhone   *string `json:"supplier_phone,omitempty"`
	PricePerUnit    string  `json:"price_per_unit"`
	OfferedQuantity string  `json:"offered_quantity"`
	IsAccepted      bool    `json:"is_accepted"`
	IsMyOffer       bool    `json:"is_my_offer"`
	CreatedAt       string  `json:"created_at"`
}
