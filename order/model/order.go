package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type OrderStatus string

const (
	OrderStatusCreated         OrderStatus = "created"
	OrderStatusSellerConfirmed OrderStatus = "seller_confirmed"
	OrderStatusBuyerConfirmed  OrderStatus = "buyer_confirmed"
	OrderStatusCompleted       OrderStatus = "completed"
	OrderStatusCancelled       OrderStatus = "cancelled"
)

type Order struct {
	ID                   int32
	PublicID             uuid.UUID
	SellAuctionID        pgtype.Int4
	BuyRequestID         pgtype.Int4
	SellerID             int32
	BuyerID              int32
	FinalPrice           decimal.Decimal
	Quantity             decimal.Decimal
	Unit                 string
	Status               OrderStatus
	SellerConfirmedAt    pgtype.Timestamptz
	BuyerConfirmedAt     pgtype.Timestamptz
	ConfirmationDeadline time.Time
	CompletedAt          pgtype.Timestamptz
	CancelledAt          pgtype.Timestamptz
	CreatedAt            time.Time
}

type OrderResponse struct {
	PublicID             string  `json:"public_id"`
	SourceType           string  `json:"source_type"`
	SourceID             string  `json:"source_id"`
	SellerName           string  `json:"seller_name"`
	SellerPhone          string  `json:"seller_phone"`
	SellerRegion         string  `json:"seller_region"`
	BuyerName            string  `json:"buyer_name"`
	BuyerPhone           string  `json:"buyer_phone"`
	BuyerRegion          string  `json:"buyer_region"`
	FinalPrice           string  `json:"final_price"`
	UnitPrice            string  `json:"unit_price"`
	TotalPrice           string  `json:"total_price"`
	Quantity             string  `json:"quantity"`
	Unit                 string  `json:"unit"`
	Status               string  `json:"status"`
	ConfirmationDeadline *string `json:"confirmation_deadline"`
	MaskedMessage        *string `json:"masked_message,omitempty"`
	IsSellerConfirmed    bool    `json:"is_seller_confirmed"`
	IsBuyerConfirmed     bool    `json:"is_buyer_confirmed"`
	IAmSeller            bool    `json:"i_am_seller"`
	IAmBuyer             bool    `json:"i_am_buyer"`
	CreatedAt            string  `json:"created_at"`
}
