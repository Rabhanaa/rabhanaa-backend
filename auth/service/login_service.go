package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func (s *AuthService) Login(ctx context.Context, req model.LoginRequest, deviceInfo, ipAddress string) (*model.AuthResponse, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if !CheckPassword(req.Password, user.PasswordHash.String) {
		s.repo.CreateLoginHistory(ctx, sqlc.CreateLoginHistoryParams{
			UserID:     user.ID,
			DeviceInfo: pgtype.Text{String: deviceInfo, Valid: true},
			IpAddress:  pgtype.Text{String: ipAddress, Valid: true},
			Success:    false,
		})
		return nil, errs.ErrInvalidCredentials
	}

	if user.Status == "banned" {
		return nil, errs.ErrUserBanned
	}
	if user.Status == "suspended" {
		if user.SuspendedUntil.Valid && user.SuspendedUntil.Time.Before(time.Now()) {
			if _, err := s.repo.LazyRestoreExpiredSuspension(ctx, user.ID); err != nil {
				return nil, err
			}
			user.Status = "active"
		} else {
			return nil, errs.ErrUserSuspended
		}
	}

	if err := s.repo.InvalidateUserSessions(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("failed to invalidate sessions: %w", err)
	}

	tokenHash := generateTokenHash()
	_, err = s.repo.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID:     user.ID,
		TokenHash:  tokenHash,
		DeviceInfo: pgtype.Text{String: deviceInfo, Valid: true},
		IpAddress:  pgtype.Text{String: ipAddress, Valid: true},
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(time.Duration(s.config.JWTExpirationMinutes) * time.Minute), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.repo.CreateLoginHistory(ctx, sqlc.CreateLoginHistoryParams{
		UserID:     user.ID,
		DeviceInfo: pgtype.Text{String: deviceInfo, Valid: true},
		IpAddress:  pgtype.Text{String: ipAddress, Valid: true},
		Success:    true,
	})

	publicID := uuid.UUID(user.PublicID.Bytes)
	token, err := s.GenerateToken(publicID, user.Role.Valid && user.Role.String == "admin")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &model.AuthResponse{
		AccessToken: token,
		User: model.UserResponse{
			PublicID:         publicID.String(),
			Name:             user.Name,
			Email:            user.Email,
			Phone:            user.Phone.String,
			Status:           user.Status,
			IsAdmin:          user.Role.Valid && user.Role.String == "admin",
			SuppliesToRetail: user.SuppliesToRetail,
			JobKey:           user.JobKey,
		},
	}, nil
}

func generateTokenHash() string {
	hash := sha256.Sum256([]byte(uuid.New().String()))
	return hex.EncodeToString(hash[:])
}
