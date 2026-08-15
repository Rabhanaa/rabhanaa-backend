package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
	"rabhana/lib/email"
	"rabhana/pkg/errs"
)

const (
	// A 6-digit code has a million combinations; five guesses per code keeps
	// brute force impractical without frustrating someone mistyping.
	maxResetAttempts = 5
	// Throttles, enforced against password_reset_codes since there is no Redis.
	resetMinGap     = time.Minute
	resetMaxPerHour = 5
)

// RequestPasswordReset emails a reset code.
//
// It reports success to the caller in every case that is not an internal
// failure — an unknown address, a throttled user and a genuine send all look
// identical from outside. Anything else turns this endpoint into a way to test
// whether an email has an account.
func (s *AuthService) RequestPasswordReset(ctx context.Context, rawEmail string) {
	address := strings.ToLower(strings.TrimSpace(rawEmail))

	user, err := s.repo.GetUserByEmail(ctx, address)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("password reset: lookup failed", "error", err)
		}
		return
	}

	q := s.repo.GetQueries()

	// Two throttles: a short gap so a held-down button cannot mail-bomb anyone,
	// and an hourly ceiling.
	recentCount, err := q.CountRecentPasswordResetCodes(ctx, sqlc.CountRecentPasswordResetCodesParams{
		UserID: user.ID,
		Since:  pgtype.Timestamptz{Time: time.Now().Add(-resetMinGap), Valid: true},
	})
	if err != nil {
		slog.Error("password reset: throttle check failed", "error", err, "user_id", user.ID)
		return
	}
	if recentCount > 0 {
		slog.Info("password reset: throttled (too soon)", "user_id", user.ID)
		return
	}

	hourlyCount, err := q.CountRecentPasswordResetCodes(ctx, sqlc.CountRecentPasswordResetCodesParams{
		UserID: user.ID,
		Since:  pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	})
	if err != nil {
		slog.Error("password reset: hourly throttle check failed", "error", err, "user_id", user.ID)
		return
	}
	if hourlyCount >= resetMaxPerHour {
		slog.Warn("password reset: hourly limit reached", "user_id", user.ID)
		return
	}

	code := generateCode()
	ttl := time.Duration(s.config.PasswordResetTTLMinutes) * time.Minute

	if _, err := q.CreatePasswordResetCode(ctx, sqlc.CreatePasswordResetCodeParams{
		UserID:    user.ID,
		CodeHash:  hashCode(code),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(ttl), Valid: true},
	}); err != nil {
		slog.Error("password reset: failed to store code", "error", err, "user_id", user.ID)
		return
	}

	// Without a mail provider configured there is nowhere to send it, so log it
	// instead — that is what makes the flow testable in local development. This
	// branch is unreachable once RESEND_API_KEY is set.
	if s.emailClient == nil || !s.emailClient.Enabled() {
		slog.Warn("password reset: email disabled, code not delivered",
			"email", address, "code", code, "ttl_minutes", s.config.PasswordResetTTLMinutes)
		return
	}

	subject, html, err := email.PasswordResetEmail(code, s.config.PasswordResetTTLMinutes)
	if err != nil {
		slog.Error("password reset: template failed", "error", err)
		return
	}
	if err := s.emailClient.Send(ctx, user.Email, subject, html); err != nil {
		slog.Error("password reset: send failed", "error", err, "user_id", user.ID)
	}
}

// VerifyResetCode checks a code without spending it, so the UI can move to the
// new-password step before asking for one.
func (s *AuthService) VerifyResetCode(ctx context.Context, rawEmail, code string) error {
	_, _, err := s.checkResetCode(ctx, rawEmail, code)
	return err
}

// ResetPassword sets a new password and revokes everything the old one could
// reach.
func (s *AuthService) ResetPassword(ctx context.Context, rawEmail, code, newPassword string) error {
	user, resetCode, err := s.checkResetCode(ctx, rawEmail, code)
	if err != nil {
		return err
	}

	if !ValidatePassword(newPassword) {
		return errs.ErrInvalidPassword
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	q := s.repo.GetQueries()

	if err := s.repo.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: pgtype.Text{String: hash, Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	if err := q.ConsumePasswordResetCode(ctx, resetCode.ID); err != nil {
		slog.Error("password reset: failed to consume code", "error", err, "code_id", resetCode.ID)
	}
	if err := q.InvalidatePasswordResetCodes(ctx, user.ID); err != nil {
		slog.Error("password reset: failed to retire other codes", "error", err, "user_id", user.ID)
	}

	// The revocation that actually works. InvalidateUserSessions is called too,
	// but middleware.Auth never reads user_sessions — password_changed_at is
	// what invalidates tokens already in the wild.
	if err := q.MarkPasswordChanged(ctx, user.ID); err != nil {
		slog.Error("password reset: failed to mark password changed", "error", err, "user_id", user.ID)
	}
	if err := s.repo.InvalidateUserSessions(ctx, user.ID); err != nil {
		slog.Error("password reset: failed to invalidate sessions", "error", err, "user_id", user.ID)
	}

	// Push still reaches the device even though its session is gone, which is
	// the point: if this was not the account owner, they find out.
	if s.notificationSender != nil {
		s.notificationSender.SendToUser(ctx, user.ID,
			"تم تغيير كلمة المرور",
			"تم تغيير كلمة مرور حسابك. إذا لم تقم بذلك، تواصل معنا فوراً.",
			map[string]string{"type": "password_changed"})
	}

	return nil
}

// checkResetCode resolves the user and their newest usable code, counting a
// failed guess against that code's allowance.
func (s *AuthService) checkResetCode(ctx context.Context, rawEmail, code string) (sqlc.User, sqlc.PasswordResetCode, error) {
	address := strings.ToLower(strings.TrimSpace(rawEmail))

	user, err := s.repo.GetUserByEmail(ctx, address)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Same error as a wrong code: at this point the caller already
			// needs a valid code, so there is nothing to gain by distinguishing.
			return sqlc.User{}, sqlc.PasswordResetCode{}, errs.ErrInvalidResetCode
		}
		return sqlc.User{}, sqlc.PasswordResetCode{}, fmt.Errorf("failed to get user: %w", err)
	}

	q := s.repo.GetQueries()
	resetCode, err := q.GetLivePasswordResetCode(ctx, sqlc.GetLivePasswordResetCodeParams{
		UserID:      user.ID,
		MaxAttempts: maxResetAttempts,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, sqlc.PasswordResetCode{}, errs.ErrResetCodeExpired
		}
		return sqlc.User{}, sqlc.PasswordResetCode{}, fmt.Errorf("failed to get reset code: %w", err)
	}

	if !checkCode(code, resetCode.CodeHash) {
		if err := q.IncrementPasswordResetAttempts(ctx, resetCode.ID); err != nil {
			slog.Error("password reset: failed to record attempt", "error", err, "code_id", resetCode.ID)
		}
		return sqlc.User{}, sqlc.PasswordResetCode{}, errs.ErrInvalidResetCode
	}

	return user, resetCode, nil
}
