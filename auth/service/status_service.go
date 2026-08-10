package service

import (
	"context"
	"fmt"
	"log/slog"

	"rabhana/auth/model"
	"rabhana/db/sqlc"
)

type UserStatusResponse struct {
	Status        model.OnboardingStatus `json:"status"`
	AccountStatus model.UserStatus       `json:"account_status"`
	NextStep      string                 `json:"next_step"`
}

func (s *AuthService) GetUserStatus(ctx context.Context, userID int32) (*UserStatusResponse, error) {
	data, err := s.repo.GetUserStatusData(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user status data: %w", err)
	}

	accountStatus := model.UserStatus(data.Status)
	requireDocuments := s.config.RequireDocuments
	onboardingStatus := determineOnboardingStatus(data, accountStatus, requireDocuments)

	// Lazy migration: with documents switched off, a user who finished the rest
	// of onboarding is effectively active, but their row still says
	// pending_documents — which keeps them in the admin verification queue
	// forever. Flip it on read, the same way expired suspensions are restored.
	if !requireDocuments &&
		accountStatus == model.StatusPendingDocuments &&
		onboardingStatus == model.OnboardingActive {
		if err := s.repo.UpdateUserStatus(ctx, sqlc.UpdateUserStatusParams{
			ID:     userID,
			Status: string(model.StatusActive),
		}); err != nil {
			slog.Error("failed to activate user with documents disabled", "error", err, "user_id", userID)
		} else {
			accountStatus = model.StatusActive
		}
	}

	nextStep := map[model.OnboardingStatus]string{
		model.OnboardingRegistered:      "",
		model.OnboardingPickInterests:   "interests",
		model.OnboardingSetLocation:     "location",
		model.OnboardingUploadDocuments: "documents",
		model.OnboardingPendingReview:   "",
		model.OnboardingRejected:        "documents",
		model.OnboardingSuspended:       "",
		model.OnboardingActive:          "",
	}[onboardingStatus]

	// A rejected user is told to re-upload documents. With uploads disabled
	// that route no longer exists, so don't send them to a dead end.
	if !requireDocuments && nextStep == "documents" {
		nextStep = ""
	}

	return &UserStatusResponse{
		Status:        onboardingStatus,
		AccountStatus: accountStatus,
		NextStep:      nextStep,
	}, nil
}

func determineOnboardingStatus(data sqlc.GetUserStatusDataRow, accountStatus model.UserStatus, requireDocuments bool) model.OnboardingStatus {
	switch accountStatus {
	case model.StatusSuspended:
		return model.OnboardingSuspended
	case model.StatusPendingReview:
		return model.OnboardingPendingReview
	case model.StatusRejected:
		return model.OnboardingRejected
	case model.StatusActive:
		return model.OnboardingActive
	case model.StatusPendingDocuments:
		if data.InterestsCount < 5 {
			return model.OnboardingPickInterests
		}
		if !data.Latitude.Valid || !data.Longitude.Valid {
			return model.OnboardingSetLocation
		}
		if !requireDocuments {
			return model.OnboardingActive
		}
		return model.OnboardingUploadDocuments
	default:
		return model.OnboardingRegistered
	}
}
