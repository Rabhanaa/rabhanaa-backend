package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/auth/model"
	"rabhana/db/sqlc"
	"rabhana/pkg/errs"
)

func (s *AuthService) RegisterUser(ctx context.Context, req model.RegisterRequest) (*model.AuthResponse, error) {
	if !ValidatePhone(req.Phone) {
		return nil, errs.ErrInvalidPhone
	}

	if !ValidatePassword(req.Password) {
		return nil, errors.New("INVALID_PASSWORD")
	}

	signupSource := NormalizeSignupSource(req.SignupSource)
	if !IsValidSignupSource(signupSource) {
		return nil, errs.ErrInvalidSignupSource
	}

	_, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err == nil {
		return nil, errs.ErrEmailAlreadyExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}

	// Validate region exists
	if err := s.validateRegionExists(ctx, req.RegionID); err != nil {
		return nil, err
	}

	// Validate job exists
	if err := s.validateJobExists(ctx, req.JobID); err != nil {
		return nil, err
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, sqlc.CreateUserParams{
		Email:            req.Email,
		Phone:            pgtype.Text{String: req.Phone, Valid: true},
		PasswordHash:     pgtype.Text{String: passwordHash, Valid: true},
		Name:             req.Name,
		RegionID:         pgtype.Int4{Int32: req.RegionID, Valid: true},
		JobID:            pgtype.Int4{Int32: req.JobID, Valid: true},
		Status:           "pending_documents",
		SignupSource:     signupSource,
		SuppliesToRetail: req.SuppliesToRetail,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if err := s.repo.UpdateUserCachedNames(ctx, sqlc.UpdateUserCachedNamesParams{
		ID:   user.ID,
		ID_2: req.JobID,
		ID_3: req.RegionID,
	}); err != nil {
		return nil, fmt.Errorf("failed to update cached names: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create free tier subscription for new user
	_, err = s.repo.GetQueries().CreateUserSubscription(ctx, sqlc.CreateUserSubscriptionParams{
		UserID:    user.ID,
		TierName:  "free",
		StartedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ExpiresAt: pgtype.Timestamptz{Valid: false}, // Never expires
		IsActive:  true,
		IsPrimary: true,
	})
	if err != nil {
		// Log error but don't fail registration - admin can fix later
		fmt.Printf("Warning: failed to create free subscription for user %d: %v\n", user.ID, err)
	}

	region, err := s.repo.GetRegionByID(ctx, req.RegionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get region: %w", err)
	}

	job, err := s.repo.GetJobByID(ctx, req.JobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	publicID := uuid.UUID(user.PublicID.Bytes)
	fmt.Printf("[REGISTER] User created: ID=%d, publicID=%s, email=%s\n", user.ID, publicID.String(), user.Email)
	fmt.Printf("[REGISTER] Generating token for publicID=%s, isAdmin=%v\n", publicID.String(), user.Role.Valid && user.Role.String == "admin")
	token, err := s.GenerateToken(publicID, user.Role.Valid && user.Role.String == "admin")
	if err != nil {
		fmt.Printf("[REGISTER] ❌ Token generation failed: %v\n", err)
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	fmt.Printf("[REGISTER] ✅ Token generated, length=%d chars\n", len(token))

	return &model.AuthResponse{
		AccessToken: token,
		User: model.UserResponse{
			PublicID:         publicID.String(),
			Name:             user.Name,
			Email:            user.Email,
			Phone:            user.Phone.String,
			Status:           user.Status,
			IsAdmin:          user.Role.Valid && user.Role.String == "admin",
			RegionID:         &user.RegionID.Int32,
			JobID:            &user.JobID.Int32,
			RegionName:       region.NameAr,
			JobName:          job.NameAr,
			SuppliesToRetail: user.SuppliesToRetail,
		},
	}, nil
}

// validateRegionExists checks if the given region ID exists in the database
func (s *AuthService) validateRegionExists(ctx context.Context, regionID int32) error {
	_, err := s.repo.GetRegionByID(ctx, regionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrInvalidRegionID
		}
		return fmt.Errorf("failed to validate region: %w", err)
	}
	return nil
}

// validateJobExists checks if the given job ID exists in the database
func (s *AuthService) validateJobExists(ctx context.Context, jobID int32) error {
	_, err := s.repo.GetJobByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrInvalidJobID
		}
		return fmt.Errorf("failed to validate job: %w", err)
	}
	return nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID int32, req model.ProfileRequest) error {
	return s.repo.UpdateUserProfileWithNames(ctx, sqlc.UpdateUserProfileWithNamesParams{
		ID:       userID,
		JobID:    pgtype.Int4{Int32: req.JobID, Valid: true},
		RegionID: pgtype.Int4{Int32: req.RegionID, Valid: true},
	})
}

func (s *AuthService) UpdateInterests(ctx context.Context, userID int32, req model.InterestsRequest) error {
	if len(req.InterestIDs) < s.config.MinInterests {
		return errors.New("INSUFFICIENT_INTERESTS")
	}

	if err := s.repo.DeleteUserInterests(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete existing interests: %w", err)
	}

	for _, interestID := range req.InterestIDs {
		if err := s.repo.AddUserInterest(ctx, sqlc.AddUserInterestParams{
			UserID:     userID,
			InterestID: interestID,
		}); err != nil {
			return fmt.Errorf("failed to add interest %d: %w", interestID, err)
		}
	}

	if err := s.repo.UpdateUserInterestsCount(ctx, sqlc.UpdateUserInterestsCountParams{
		ID:             userID,
		InterestsCount: int32(len(req.InterestIDs)),
	}); err != nil {
		return fmt.Errorf("failed to update interests count: %w", err)
	}

	return nil
}

func (s *AuthService) GetUserByPublicID(ctx context.Context, publicID uuid.UUID) (*model.User, error) {
	user, err := s.repo.GetUserByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return dbUserToModel(user), nil
}

func dbUserToModel(u sqlc.User) *model.User {
	return &model.User{
		ID:              u.ID,
		PublicID:        u.PublicID,
		Email:           u.Email,
		Phone:           u.Phone,
		PasswordHash:    u.PasswordHash,
		Name:            u.Name,
		JobID:           u.JobID,
		RegionID:        u.RegionID,
		Status:          model.UserStatus(u.Status),
		OTPHash:         u.OtpHash,
		OTPExpiresAt:    u.OtpExpiresAt,
		FCMToken:        u.FcmToken,
		Latitude:        u.Latitude,
		Longitude:       u.Longitude,
		IsAdmin:         u.Role.Valid && u.Role.String == "admin",
		RejectionReason: u.RejectionReason,
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
	}
}
