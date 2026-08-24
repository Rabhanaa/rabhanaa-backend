package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/auth/model"
	"rabhana/db/sqlc"
	notifModel "rabhana/notification/model"
	"rabhana/pkg/errs"
)

func (s *AuthService) ListPendingUsers(ctx context.Context, limit, offset int32) ([]model.UserResponse, int64, error) {
	users, err := s.repo.ListUsersByStatus(ctx, sqlc.ListUsersByStatusParams{
		Status: "pending_review",
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}

	var responses []model.UserResponse
	for _, u := range users {
		publicID, _ := uuid.FromBytes(u.PublicID.Bytes[:])
		response := model.UserResponse{
			PublicID: publicID.String(),
			Name:     u.Name,
			Email:    u.Email,
			Phone:    u.Phone.String,
			Status:   u.Status,
			IsAdmin:  u.Role.Valid && u.Role.String == "admin",
		}
		if u.JobID.Valid {
			response.JobID = &u.JobID.Int32
		}
		if u.RegionID.Valid {
			response.RegionID = &u.RegionID.Int32
		}
		if u.RejectionReason.Valid {
			response.RejectionReason = &u.RejectionReason.String
		}
		if u.StatusChangedAt.Valid {
			t := u.StatusChangedAt.Time
			response.StatusChangedAt = &t
		}
		// The role matters in these lists now that carriers (#14) wait for approval
		// alongside merchants — approving a shipping company is a different
		// decision from approving a trader. Cached on the row by 022 and 040, so
		// the joined query the old TODO asked for is not needed.
		response.RegionName = u.RegionName
		response.JobName = u.JobName
		response.JobKey = u.JobKey
		responses = append(responses, response)
	}

	total, err := s.repo.CountUsersByStatus(ctx, "pending_review")
	if err != nil {
		return nil, 0, err
	}

	return responses, total, nil
}

func (s *AuthService) ApproveUser(ctx context.Context, userPublicID uuid.UUID) error {
	user, err := s.repo.GetUserByPublicID(ctx, userPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrInvalidCredentials
		}
		return err
	}

	if user.Status != "pending_review" {
		return errors.New("INVALID_STATUS")
	}

	if err := s.repo.UpdateUserStatus(ctx, sqlc.UpdateUserStatusParams{
		ID:     user.ID,
		Status: "active",
	}); err != nil {
		return err
	}

	s.notificationSender.Send(ctx, user.ID, notifModel.EventAccountApproved, map[string]string{
		"user_id": userPublicID.String(),
	})
	return nil
}

func (s *AuthService) RejectUser(ctx context.Context, userPublicID uuid.UUID, reason string) error {
	user, err := s.repo.GetUserByPublicID(ctx, userPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrInvalidCredentials
		}
		return err
	}

	if user.Status != "pending_review" {
		return errors.New("INVALID_STATUS")
	}

	if err := s.repo.UpdateUserStatusWithReason(ctx, sqlc.UpdateUserStatusWithReasonParams{
		ID:              user.ID,
		Status:          "rejected",
		RejectionReason: pgtype.Text{String: reason, Valid: true},
	}); err != nil {
		return err
	}

	s.notificationSender.Send(ctx, user.ID, notifModel.EventAccountRejected, map[string]string{
		"user_id": userPublicID.String(),
		"reason":  reason,
	})
	return nil
}

// GetUserDocumentsForAdmin returns documents for a user identified by public UUID.
// It returns object keys (not URLs) — callers are responsible for generating presigned URLs.
func (s *AuthService) GetUserDocumentsForAdmin(ctx context.Context, userPublicID uuid.UUID) (*GetUserDocumentsResponse, error) {
	user, err := s.repo.GetUserByPublicID(ctx, userPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return s.GetUserDocuments(ctx, user.ID)
}

func (s *AuthService) GetUserDetailForAdmin(ctx context.Context, userPublicID uuid.UUID) (*model.UserResponse, error) {
	u, err := s.repo.GetUserWithRegionAndJobByPublicID(ctx, userPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrInvalidCredentials
		}
		return nil, err
	}

	publicID, _ := uuid.FromBytes(u.PublicID.Bytes[:])
	resp := &model.UserResponse{
		PublicID:   publicID.String(),
		Name:       u.Name,
		Email:      u.Email,
		Phone:      u.Phone.String,
		Status:     u.Status,
		IsAdmin:    u.Role.Valid && u.Role.String == "admin",
		RegionName: u.RegionName,
		JobName:    u.JobName,
	}
	if u.JobID.Valid {
		resp.JobID = &u.JobID.Int32
	}
	if u.RegionID.Valid {
		resp.RegionID = &u.RegionID.Int32
	}
	if u.RejectionReason.Valid {
		resp.RejectionReason = &u.RejectionReason.String
	}

	if u.SuspendedUntil.Valid {
		t := u.SuspendedUntil.Time
		resp.SuspendedUntil = &t
	}
	if u.SuspensionReason.Valid {
		resp.SuspensionReason = u.SuspensionReason.String
	}
	if u.BannedAt.Valid {
		t := u.BannedAt.Time
		resp.BannedAt = &t
	}
	if u.BannedReason.Valid {
		resp.BannedReason = u.BannedReason.String
	}
	if u.StatusChangedAt.Valid {
		t := u.StatusChangedAt.Time
		resp.StatusChangedAt = &t
	}
	if u.StatusChangedByAdminID.Valid {
		admin, err := s.repo.GetUserByID(ctx, u.StatusChangedByAdminID.Int32)
		if err == nil {
			adminPublicID := uuid.UUID(admin.PublicID.Bytes).String()
			resp.StatusChangedByAdminID = &adminPublicID
		}
	}

	interests, err := s.repo.GetUserInterests(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	for _, i := range interests {
		resp.Interests = append(resp.Interests, i.ID)
	}

	return resp, nil
}

func (s *AuthService) ListAllUsers(ctx context.Context, status string, limit, offset int32) ([]model.UserResponse, error) {
	var users []sqlc.User
	var err error

	if status != "" {
		users, err = s.repo.ListUsersByStatus(ctx, sqlc.ListUsersByStatusParams{
			Status: status,
			Limit:  limit,
			Offset: offset,
		})
	} else {
		users, err = s.repo.ListAllUsersAnyStatus(ctx, limit, offset)
	}
	if err != nil {
		return nil, err
	}

	var responses []model.UserResponse
	for _, u := range users {
		publicID, _ := uuid.FromBytes(u.PublicID.Bytes[:])
		response := model.UserResponse{
			PublicID: publicID.String(),
			Name:     u.Name,
			Email:    u.Email,
			Phone:    u.Phone.String,
			Status:   u.Status,
			IsAdmin:  u.Role.Valid && u.Role.String == "admin",
		}
		if u.JobID.Valid {
			response.JobID = &u.JobID.Int32
		}
		if u.RegionID.Valid {
			response.RegionID = &u.RegionID.Int32
		}
		if u.RejectionReason.Valid {
			response.RejectionReason = &u.RejectionReason.String
		}
		if u.StatusChangedAt.Valid {
			t := u.StatusChangedAt.Time
			response.StatusChangedAt = &t
		}
		// The role matters in these lists now that carriers (#14) wait for approval
		// alongside merchants — approving a shipping company is a different
		// decision from approving a trader. Cached on the row by 022 and 040, so
		// the joined query the old TODO asked for is not needed.
		response.RegionName = u.RegionName
		response.JobName = u.JobName
		response.JobKey = u.JobKey
		responses = append(responses, response)
	}

	return responses, nil
}

func (s *AuthService) CountUsersByStatus(ctx context.Context, status string) (int64, error) {
	return s.repo.CountUsersByStatus(ctx, status)
}

func (s *AuthService) CountAllUsersAnyStatus(ctx context.Context) (int64, error) {
	return s.repo.CountAllUsersAnyStatus(ctx)
}

func (s *AuthService) assertAdminCanActOn(ctx context.Context, actingAdminID int32, targetPublicID uuid.UUID) (*sqlc.User, error) {
	target, err := s.repo.GetUserByPublicID(ctx, targetPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrInvalidCredentials
		}
		return nil, err
	}
	if target.ID == actingAdminID {
		return nil, errs.ErrCannotActOnSelf
	}
	if target.Role.Valid && target.Role.String == "admin" {
		return nil, errs.ErrCannotActOnAdmin
	}
	return &target, nil
}

func (s *AuthService) SuspendUser(ctx context.Context, actingAdminID int32, targetPublicID uuid.UUID, reason string, durationHours *int) error {
	target, err := s.assertAdminCanActOn(ctx, actingAdminID, targetPublicID)
	if err != nil {
		return err
	}

	if target.Status == "banned" {
		return errs.ErrInvalidStatusTransition
	}

	var suspendedUntil pgtype.Timestamptz
	if durationHours != nil {
		suspendedUntil = pgtype.Timestamptz{Time: time.Now().Add(time.Duration(*durationHours) * time.Hour), Valid: true}
	}

	rows, err := s.repo.SuspendUser(ctx, sqlc.SuspendUserParams{
		ID:                     target.ID,
		SuspensionReason:       pgtype.Text{String: reason, Valid: true},
		SuspendedUntil:         suspendedUntil,
		StatusChangedByAdminID: pgtype.Int4{Int32: actingAdminID, Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return errs.ErrInvalidStatusTransition
	}

	if err := s.repo.InvalidateAllSessionsForUser(ctx, target.ID); err != nil {
		slog.Info("failed to invalidate sessions after suspend", "user_id", target.ID, "error", err)
	}

	s.notificationSender.SendToUser(ctx, target.ID, "تم تعليق حسابك مؤقتاً", reason, map[string]string{
		"type": "account_suspended",
	})
	return nil
}

func (s *AuthService) UnsuspendUser(ctx context.Context, actingAdminID int32, targetPublicID uuid.UUID) error {
	target, err := s.assertAdminCanActOn(ctx, actingAdminID, targetPublicID)
	if err != nil {
		return err
	}

	rows, err := s.repo.UnsuspendUser(ctx, sqlc.UnsuspendUserParams{
		ID:                     target.ID,
		StatusChangedByAdminID: pgtype.Int4{Int32: actingAdminID, Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return errs.ErrInvalidStatusTransition
	}

	s.notificationSender.SendToUser(ctx, target.ID, "تم إعادة تفعيل حسابك", "تم رفع التعليق عن حسابك", map[string]string{
		"type": "account_unsuspended",
	})
	return nil
}

func (s *AuthService) BanUser(ctx context.Context, actingAdminID int32, targetPublicID uuid.UUID, reason string) error {
	target, err := s.assertAdminCanActOn(ctx, actingAdminID, targetPublicID)
	if err != nil {
		return err
	}

	rows, err := s.repo.BanUser(ctx, sqlc.BanUserParams{
		ID:                     target.ID,
		BannedReason:           pgtype.Text{String: reason, Valid: true},
		StatusChangedByAdminID: pgtype.Int4{Int32: actingAdminID, Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return errs.ErrInvalidStatusTransition
	}

	if err := s.repo.InvalidateAllSessionsForUser(ctx, target.ID); err != nil {
		slog.Info("failed to invalidate sessions after ban", "user_id", target.ID, "error", err)
	}

	s.notificationSender.SendToUser(ctx, target.ID, "تم حظر حسابك", reason, map[string]string{
		"type": "account_banned",
	})
	return nil
}

func (s *AuthService) UnbanUser(ctx context.Context, actingAdminID int32, targetPublicID uuid.UUID) error {
	target, err := s.assertAdminCanActOn(ctx, actingAdminID, targetPublicID)
	if err != nil {
		return err
	}

	rows, err := s.repo.UnbanUser(ctx, sqlc.UnbanUserParams{
		ID:                     target.ID,
		StatusChangedByAdminID: pgtype.Int4{Int32: actingAdminID, Valid: true},
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return errs.ErrInvalidStatusTransition
	}

	s.notificationSender.SendToUser(ctx, target.ID, "تم إعادة تفعيل حسابك", "تم رفع الحظر عن حسابك", map[string]string{
		"type": "account_unbanned",
	})
	return nil
}

func (s *AuthService) SearchUsers(ctx context.Context, q, status string, limit, offset int32) ([]model.UserResponse, int64, error) {
	users, err := s.repo.SearchUsers(ctx, sqlc.SearchUsersParams{
		Query:  q,
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}

	var responses []model.UserResponse
	for _, u := range users {
		publicID, _ := uuid.FromBytes(u.PublicID.Bytes[:])
		response := model.UserResponse{
			PublicID: publicID.String(),
			Name:     u.Name,
			Email:    u.Email,
			Phone:    u.Phone.String,
			Status:   u.Status,
			IsAdmin:  u.Role.Valid && u.Role.String == "admin",
		}
		if u.JobID.Valid {
			response.JobID = &u.JobID.Int32
		}
		if u.RegionID.Valid {
			response.RegionID = &u.RegionID.Int32
		}
		if u.RejectionReason.Valid {
			response.RejectionReason = &u.RejectionReason.String
		}
		if u.StatusChangedAt.Valid {
			t := u.StatusChangedAt.Time
			response.StatusChangedAt = &t
		}
		// The role matters in these lists now that carriers (#14) wait for approval
		// alongside merchants — approving a shipping company is a different
		// decision from approving a trader. Cached on the row by 022 and 040, so
		// the joined query the old TODO asked for is not needed.
		response.RegionName = u.RegionName
		response.JobName = u.JobName
		response.JobKey = u.JobKey
		responses = append(responses, response)
	}

	total, err := s.repo.SearchUsersCount(ctx, q, status)
	if err != nil {
		return nil, 0, err
	}

	return responses, total, nil
}
