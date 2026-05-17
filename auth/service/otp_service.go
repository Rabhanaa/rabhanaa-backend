package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/auth/model"
	"rabhana/db/sqlc"
	"rabhana/pkg/errs"
)

func (s *AuthService) GetOTP(ctx context.Context, phone string) error {
	user, err := s.repo.GetUserByEmail(ctx, phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrInvalidPhone
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	otp := generateOTP()
	otpHash := hashOTP(otp)

	if err := s.repo.UpdateUserOTP(ctx, sqlc.UpdateUserOTPParams{
		ID:           user.ID,
		OtpHash:      pgtype.Text{String: otpHash, Valid: true},
		OtpExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Duration(s.config.OTPExpirationMinutes) * time.Minute), Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to update OTP: %w", err)
	}

	return nil
}

func (s *AuthService) VerifyOTP(ctx context.Context, req model.VerifyOTPRequest, deviceInfo, ipAddress string) (*model.AuthResponse, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrInvalidPhone
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if !user.OtpExpiresAt.Valid || user.OtpExpiresAt.Time.Before(time.Now()) {
		return nil, errs.ErrOTPExpired
	}

	if !checkOTP(req.OTP, user.OtpHash.String) {
		return nil, errs.ErrInvalidOTP
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

	if err := s.repo.ClearUserOTP(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("failed to clear OTP: %w", err)
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

	publicID := uuid.UUID(user.PublicID.Bytes)
	token, err := s.GenerateToken(publicID, user.Role.Valid && user.Role.String == "admin")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &model.AuthResponse{
		AccessToken: token,
		User: model.UserResponse{
			PublicID: publicID.String(),
			Name:     user.Name,
			Email:    user.Email,
			Phone:    user.Phone.String,
			Status:   user.Status,
			IsAdmin:  user.Role.Valid && user.Role.String == "admin",
		},
	}, nil
}

func generateOTP() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func hashOTP(otp string) string {
	hash := sha256.Sum256([]byte(otp))
	return hex.EncodeToString(hash[:])
}

func checkOTP(otp, hash string) bool {
	return hashOTP(otp) == hash
}
