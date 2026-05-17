package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type SellBid struct {
	ID               int32
	PublicID         uuid.UUID
	AuctionID        int32
	BidderID         int32
	Amount           decimal.Decimal
	IsSelected       bool
	AuctionTitle     string
	AuctionUnitPrice decimal.Decimal
	AuctionQuantity  decimal.Decimal
	AuctionUnit      string
	CreatedAt        time.Time
}

type PlaceSellBidRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

type SellBidResponse struct {
	PublicID         string  `json:"public_id"`
	BidderName       string  `json:"bidder_name"`
	BidderRegion     string  `json:"bidder_region"`
	BidderPhone      *string `json:"bidder_phone,omitempty"`
	Amount           string  `json:"amount"`
	IsSelected       bool    `json:"is_selected"`
	IsMyBid          bool    `json:"is_my_bid"`
	AuctionTitle     string  `json:"auction_title"`
	AuctionUnitPrice string  `json:"auction_unit_price"`
	AuctionQuantity  string  `json:"auction_quantity"`
	AuctionUnit      string  `json:"auction_unit"`
	CreatedAt        string  `json:"created_at"`
}
