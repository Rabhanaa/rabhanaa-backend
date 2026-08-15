package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type BuyRequestStatus string

const (
	BuyStatusActive             BuyRequestStatus = "active"
	BuyStatusCancelled          BuyRequestStatus = "cancelled"
	BuyStatusExpired            BuyRequestStatus = "expired"
	BuyStatusPendingSelection   BuyRequestStatus = "pending_selection"
	BuyStatusFulfilled          BuyRequestStatus = "fulfilled"
	BuyStatusPartiallyFulfilled BuyRequestStatus = "partially_fulfilled"
)

type BuyRequest struct {
	ID                 int32
	PublicID           uuid.UUID
	OwnerID            int32
	RegionID           int32
	InterestID         int32
	Title              string
	Description        pgtype.Text
	ImageURL           string
	Unit               string
	Quantity           decimal.Decimal
	BuyAllFromOne      bool
	OfferCount         int32
	AcceptedOfferCount int32
	FulfilledQuantity  decimal.Decimal
	EndTime            time.Time
	Status             BuyRequestStatus
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateBuyRequestRequest struct {
	InterestID    int32   `json:"interest_id" binding:"required"`
	RegionID      *int32  `json:"region_id"`
	Title         string  `json:"title" binding:"required,min=5,max=200"`
	Description   *string `json:"description,omitempty"`
	ImageURL      string  `json:"image_url"`
	Unit          string  `json:"unit" binding:"required,oneof=kg ton piece box"`
	Quantity      float64 `json:"quantity" binding:"required,gt=0"`
	BuyAllFromOne *bool   `json:"buy_all_from_one"`
}

type BuyRequestResponse struct {
	PublicID string `json:"public_id"`
	// OwnerName          string  `json:"owner_name"`
	RegionName         string  `json:"region_name"`
	InterestName       string  `json:"interest_name"`
	Title              string  `json:"title"`
	Description        *string `json:"description,omitempty"`
	ImageURL           string  `json:"image_url"`
	Unit               string  `json:"unit"`
	Quantity           string  `json:"quantity"`
	BuyAllFromOne      bool    `json:"buy_all_from_one"`
	OfferCount         int32   `json:"offer_count"`
	AcceptedOfferCount int32   `json:"accepted_offer_count"`
	FulfilledQuantity  string  `json:"fulfilled_quantity"`
	EndTime            string  `json:"end_time"`
	Status             string  `json:"status"`
	ModerationReason   *string `json:"moderation_reason,omitempty"`
	IsOwner            bool    `json:"is_owner"`
	IsExpired          bool    `json:"is_expired"`
	CreatedAt          string  `json:"created_at"`
}
