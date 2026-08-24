package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rabhana/auth/model"
	"rabhana/db/sqlc"
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

	// Lazy migration for accounts created before #12: with documents switched
	// off they are waiting for a step that no longer exists. GetUserStatus does
	// the same repair, but nothing calls it any more — /auth/me is the endpoint
	// the app actually loads on every start, so it has to happen here too.
	if !s.config.RequireDocuments && user.Status == string(model.StatusPendingDocuments) {
		if err := s.repo.UpdateUserStatus(ctx, sqlc.UpdateUserStatusParams{
			ID:     userID,
			Status: string(model.StatusActive),
		}); err != nil {
			slog.Error("failed to activate user with documents disabled", "error", err, "user_id", userID)
		} else {
			user.Status = string(model.StatusActive)
		}
	}

	interestIDs, err := s.repo.GetUserInterestIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	publicID, _ := uuid.FromBytes(user.PublicID.Bytes[:])
	response := &model.UserResponse{
		PublicID:         publicID.String(),
		Name:             user.Name,
		Email:            user.Email,
		Phone:            user.Phone.String,
		Status:           user.Status,
		IsAdmin:          user.Role.Valid && user.Role.String == "admin",
		RegionName:       user.RegionName,
		JobName:          user.JobName,
		JobKey:           user.JobKey,
		Interests:        interestIDs,
		SuppliesToRetail: user.SuppliesToRetail,
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
