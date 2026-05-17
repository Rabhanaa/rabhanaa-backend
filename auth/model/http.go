package model

import "time"

type RegisterRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=100"`
	Email        string `json:"email" binding:"required,email"`
	Phone        string `json:"phone" binding:"required"`
	RegionID     int32  `json:"region_id" binding:"required,gt=0"`
	JobID        int32  `json:"job_id" binding:"required,gt=0"`
	Password     string `json:"password" binding:"required,min=8,max=16"`
	SignupSource string `json:"signup_source"`
}

type ProfileRequest struct {
	JobID    int32 `json:"job_id" binding:"required"`
	RegionID int32 `json:"region_id" binding:"required"`
}

type InterestsRequest struct {
	InterestIDs []int32 `json:"interest_ids" binding:"required,min=1"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type GetOTPRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type VerifyOTPRequest struct {
	Phone string `json:"phone" binding:"required"`
	OTP   string `json:"otp" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=16"`
}

type SetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8,max=16"`
}

type AuthResponse struct {
	AccessToken string       `json:"access_token"`
	User        UserResponse `json:"user"`
}

type AdminSuspendRequest struct {
	Reason        string `json:"reason" binding:"required,min=3,max=500"`
	DurationHours *int   `json:"duration_hours" binding:"omitempty,min=1,max=8760"`
}

type AdminBanRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=500"`
}

type UserResponse struct {
	PublicID        string  `json:"public_id"`
	Name            string  `json:"name"`
	Email           string  `json:"email"`
	Phone           string  `json:"phone"`
	Status          string  `json:"status"`
	IsAdmin         bool    `json:"is_admin"`
	RegionID        *int32  `json:"region_id,omitempty"`
	JobID           *int32  `json:"job_id,omitempty"`
	RegionName      string  `json:"region_name,omitempty"`
	JobName         string  `json:"job_name,omitempty"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
	Interests       []int32 `json:"interests,omitempty"`
	Subscribed      bool    `json:"subscribed"`
	InTrial         bool    `json:"in_trial"`

	// Lifecycle fields (admin detail only)
	SuspendedUntil         *time.Time `json:"suspended_until,omitempty"`
	SuspensionReason       string     `json:"suspension_reason,omitempty"`
	BannedAt               *time.Time `json:"banned_at,omitempty"`
	BannedReason           string     `json:"banned_reason,omitempty"`
	StatusChangedByAdminID *string    `json:"status_changed_by_admin_id,omitempty"`
	StatusChangedAt        *time.Time `json:"status_changed_at,omitempty"`
}
