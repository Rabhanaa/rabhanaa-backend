package model

import (
	"github.com/jackc/pgx/v5/pgtype"
)

type UserStatus string

const (
	StatusPendingDocuments UserStatus = "pending_documents"
	StatusPendingReview    UserStatus = "pending_review"
	StatusActive           UserStatus = "active"
	StatusRejected         UserStatus = "rejected"
	StatusSuspended        UserStatus = "suspended"
)

type OnboardingStatus string

const (
	OnboardingRegistered      OnboardingStatus = "registered"
	OnboardingPickInterests   OnboardingStatus = "pick_interests"
	OnboardingSetLocation     OnboardingStatus = "set_location"
	OnboardingUploadDocuments OnboardingStatus = "upload_documents"
	OnboardingPendingReview   OnboardingStatus = "pending_review"
	OnboardingRejected        OnboardingStatus = "rejected"
	OnboardingSuspended       OnboardingStatus = "suspended"
	OnboardingActive          OnboardingStatus = "active"
)

type User struct {
	ID                int32
	PublicID          pgtype.UUID
	Email             string
	Phone             pgtype.Text
	PasswordHash      pgtype.Text
	Name              string
	JobID             pgtype.Int4
	JobKey            string
	RegionID          pgtype.Int4
	Status            UserStatus
	OTPHash           pgtype.Text
	OTPExpiresAt      pgtype.Timestamptz
	FCMToken          pgtype.Text
	Latitude          pgtype.Numeric
	Longitude         pgtype.Numeric
	IsAdmin           bool
	RejectionReason   pgtype.Text
	PasswordChangedAt pgtype.Timestamptz
	CreatedAt         pgtype.Timestamptz
	UpdatedAt         pgtype.Timestamptz
}
