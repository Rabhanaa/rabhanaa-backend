package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type SellAuctionStatus string

const (
	SellStatusActive           SellAuctionStatus = "active"
	SellStatusCancelled        SellAuctionStatus = "cancelled"
	SellStatusExpired          SellAuctionStatus = "expired"
	SellStatusPendingSelection SellAuctionStatus = "pending_selection"
	SellStatusWinnerSelected   SellAuctionStatus = "winner_selected"
)

type SellAuction struct {
	ID            int32
	PublicID      uuid.UUID
	OwnerID       int32
	RegionID      int32
	InterestID    int32
	Title         string
	Description   pgtype.Text
	ImageURL      string
	Unit          string
	Quantity      decimal.Decimal
	UnitPrice     decimal.Decimal
	BuyAllFromOne bool
	BidCount      int32
	EndTime       time.Time
	Status        SellAuctionStatus
	SelectedBidID pgtype.Int4
	WinnerID      pgtype.Int4
	FinalPrice    pgtype.Numeric
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (a *SellAuction) MinBidAmount() decimal.Decimal {
	return a.UnitPrice.Mul(decimal.NewFromFloat(0.85))
}

func (a *SellAuction) IsExpired() bool {
	return time.Now().After(a.EndTime)
}

func (a *SellAuction) SelectionDeadline() time.Time {
	return a.EndTime.Add(24 * time.Hour)
}

type CreateSellAuctionRequest struct {
	InterestID    int32   `json:"interest_id" binding:"required"`
	RegionID      *int32  `json:"region_id"`
	Title         string  `json:"title" binding:"required,min=5,max=200"`
	Description   *string `json:"description,omitempty"`
	ImageURL      string  `json:"image_url"`
	Unit          string  `json:"unit" binding:"required,oneof=kg ton piece box"`
	Quantity      float64 `json:"quantity" binding:"required,gt=0"`
	UnitPrice     float64 `json:"unit_price" binding:"required,gt=0"`
	BuyAllFromOne *bool   `json:"buy_all_from_one"`
}

type SellAuctionResponse struct {
	PublicID string `json:"public_id"`
	// OwnerName     string  `json:"owner_name"`
	RegionName    string  `json:"region_name"`
	InterestName  string  `json:"interest_name"`
	Title         string  `json:"title"`
	Description   *string `json:"description,omitempty"`
	ImageURL      string  `json:"image_url"`
	Unit          string  `json:"unit"`
	Quantity      string  `json:"quantity"`
	UnitPrice     string  `json:"unit_price"`
	BuyAllFromOne bool    `json:"buy_all_from_one"`
	BidCount      int32   `json:"bid_count"`
	EndTime       string  `json:"end_time"`
	Status        string  `json:"status"`
	IsOwner       bool    `json:"is_owner"`
	IsExpired     bool    `json:"is_expired"`
	CreatedAt     string  `json:"created_at"`
}
