package service

import (
	"context"
	"fmt"

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
	onboardingStatus := determineOnboardingStatus(data, accountStatus)

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

	return &UserStatusResponse{
		Status:        onboardingStatus,
		AccountStatus: accountStatus,
		NextStep:      nextStep,
	}, nil
}

func determineOnboardingStatus(data sqlc.GetUserStatusDataRow, accountStatus model.UserStatus) model.OnboardingStatus {
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
		return model.OnboardingUploadDocuments
	default:
		return model.OnboardingRegistered
	}
}
