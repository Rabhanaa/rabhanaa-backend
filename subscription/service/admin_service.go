package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
	"rabhana/pkg/errs"
)

type AdminService struct {
	queries  *sqlc.Queries
	userRepo UserResolver
}

type UserResolver interface {
	GetUserByPublicID(ctx context.Context, publicID interface{}) (sqlc.User, error)
}

func NewAdminService(q *sqlc.Queries, ur UserResolver) *AdminService {
	return &AdminService{queries: q, userRepo: ur}
}

func (s *AdminService) ListUserSubscriptions(ctx context.Context, userPublicID uuid.UUID) ([]sqlc.GetAllUserSubscriptionsRow, error) {
	user, err := s.userRepo.GetUserByPublicID(ctx, userPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrInvalidCredentials
		}
		return nil, err
	}
	return s.queries.GetAllUserSubscriptions(ctx, user.ID)
}

func (s *AdminService) GrantSubscription(ctx context.Context, userPublicID uuid.UUID, tierName string, startedAt, expiresAt time.Time) (sqlc.UserSubscription, bool, error) {
	user, err := s.userRepo.GetUserByPublicID(ctx, userPublicID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.UserSubscription{}, false, errs.ErrInvalidCredentials
		}
		return sqlc.UserSubscription{}, false, err
	}

	// Deactivate all current active subscriptions before assigning the new one
	if err := s.queries.DeactivateAllUserSubscriptions(ctx, user.ID); err != nil {
		return sqlc.UserSubscription{}, false, err
	}

	_, err = s.queries.GetUserSubscriptionByUserAndTier(ctx, sqlc.GetUserSubscriptionByUserAndTierParams{
		UserID:   user.ID,
		TierName: tierName,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.UserSubscription{}, false, err
	}

	if err == nil {
		// Reactivate the existing record for this tier
		sub, err := s.queries.ReactivateUserSubscription(ctx, sqlc.ReactivateUserSubscriptionParams{
			UserID:    user.ID,
			TierName:  tierName,
			StartedAt: pgtype.Timestamptz{Time: startedAt, Valid: true},
			ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
			IsPrimary: true,
		})
		return sub, true, err
	}

	// Create new subscription
	sub, err := s.queries.CreateUserSubscription(ctx, sqlc.CreateUserSubscriptionParams{
		UserID:    user.ID,
		TierName:  tierName,
		StartedAt: pgtype.Timestamptz{Time: startedAt, Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		IsActive:  true,
		IsPrimary: true,
	})
	return sub, false, err
}

func (s *AdminService) UpdateSubscription(ctx context.Context, subID int32, startedAt, expiresAt *time.Time, isActive, isPrimary *bool) (sqlc.UserSubscription, error) {
	params := sqlc.UpdateUserSubscriptionParams{
		ID: subID,
	}
	if startedAt != nil {
		params.StartedAt = pgtype.Timestamptz{Time: *startedAt, Valid: true}
	}
	if expiresAt != nil {
		params.ExpiresAt = pgtype.Timestamptz{Time: *expiresAt, Valid: true}
	}
	if isActive != nil {
		params.IsActive = pgtype.Bool{Bool: *isActive, Valid: true}
	}
	if isPrimary != nil {
		params.IsPrimary = pgtype.Bool{Bool: *isPrimary, Valid: true}
	}

	return s.queries.UpdateUserSubscription(ctx, params)
}

func (s *AdminService) CancelSubscription(ctx context.Context, subID int32) (sqlc.UserSubscription, error) {
	return s.UpdateSubscription(ctx, subID, nil, nil, boolPtr(false), nil)
}

func boolPtr(b bool) *bool {
	return &b
}
