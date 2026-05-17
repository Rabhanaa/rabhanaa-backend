package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rabhana/auth/model"
	"rabhana/pkg/errs"
)

func (s *AuthService) GetCurrentUser(ctx context.Context, userID int32) (*model.UserResponse, error) {
	user, err := s.repo.GetUserWithRegionAndJob(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrInvalidCredentials
		}
		return nil, err
	}

	interestIDs, err := s.repo.GetUserInterestIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	publicID, _ := uuid.FromBytes(user.PublicID.Bytes[:])
	response := &model.UserResponse{
		PublicID:   publicID.String(),
		Name:       user.Name,
		Email:      user.Email,
		Phone:      user.Phone.String,
		Status:     user.Status,
		IsAdmin:    user.Role.Valid && user.Role.String == "admin",
		RegionName: user.RegionName,
		JobName:    user.JobName,
		Interests:  interestIDs,
	}

	if user.JobID.Valid {
		response.JobID = &user.JobID.Int32
	}
	if user.RegionID.Valid {
		response.RegionID = &user.RegionID.Int32
	}
	if user.RejectionReason.Valid {
		response.RejectionReason = &user.RejectionReason.String
	}

	hasSubscription, err := s.repo.HasActiveSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}
	response.Subscribed = hasSubscription
	response.InTrial = time.Since(user.CreatedAt.Time) <= 72*time.Hour

	return response, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID int32) error {
	return s.repo.InvalidateSession(ctx, sessionID)
}
