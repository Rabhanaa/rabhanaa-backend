package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/auth/model"
	"rabhana/db/sqlc"
	"rabhana/pkg/errs"
)

func (s *AuthService) ChangePassword(ctx context.Context, userID int32, req model.ChangePasswordRequest) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.ErrInvalidCredentials
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	if !CheckPassword(req.CurrentPassword, user.PasswordHash.String) {
		return errs.ErrInvalidCredentials
	}

	if !ValidatePassword(req.NewPassword) {
		return errors.New("INVALID_PASSWORD")
	}

	newHash, err := HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return s.repo.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		ID:           userID,
		PasswordHash: pgtype.Text{String: newHash, Valid: true},
	})
}
