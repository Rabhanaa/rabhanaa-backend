package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/auth/service"
	"rabhana/db/sqlc"
)

type Repository struct {
	queries *sqlc.Queries
}

func NewRepository(queries *sqlc.Queries) *Repository {
	return &Repository{queries: queries}
}

func (r *Repository) GetQueries() *sqlc.Queries {
	return r.queries
}

var _ service.AuthRepository = (*Repository)(nil)

func (r *Repository) CreateUser(ctx context.Context, params sqlc.CreateUserParams) (sqlc.User, error) {
	return r.queries.CreateUser(ctx, params)
}

func (r *Repository) GetUserByID(ctx context.Context, id int32) (sqlc.User, error) {
	return r.queries.GetUserByID(ctx, id)
}

func (r *Repository) GetUserByPublicID(ctx context.Context, publicID interface{}) (sqlc.User, error) {
	uid := publicID.(uuid.UUID)
	pgUUID := pgtype.UUID{Bytes: uid, Valid: true}
	return r.queries.GetUserByPublicID(ctx, pgUUID)
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (sqlc.User, error) {
	return r.queries.GetUserByEmail(ctx, email)
}

func (r *Repository) UpdateUserStatus(ctx context.Context, params sqlc.UpdateUserStatusParams) error {
	return r.queries.UpdateUserStatus(ctx, params)
}

func (r *Repository) UpdateUserStatusWithReason(ctx context.Context, params sqlc.UpdateUserStatusWithReasonParams) error {
	return r.queries.UpdateUserStatusWithReason(ctx, params)
}

func (r *Repository) UpdateUserPassword(ctx context.Context, params sqlc.UpdateUserPasswordParams) error {
	return r.queries.UpdateUserPassword(ctx, params)
}

func (r *Repository) UpdateUserOTP(ctx context.Context, params sqlc.UpdateUserOTPParams) error {
	return r.queries.UpdateUserOTP(ctx, params)
}

func (r *Repository) ClearUserOTP(ctx context.Context, id int32) error {
	return r.queries.ClearUserOTP(ctx, id)
}

func (r *Repository) UpdateUserFCMToken(ctx context.Context, params sqlc.UpdateUserFCMTokenParams) error {
	return r.queries.UpdateUserFCMToken(ctx, params)
}

func (r *Repository) ListUsersByStatus(ctx context.Context, params sqlc.ListUsersByStatusParams) ([]sqlc.User, error) {
	return r.queries.ListUsersByStatus(ctx, params)
}

func (r *Repository) CountUsersByStatus(ctx context.Context, status string) (int64, error) {
	return r.queries.CountUsersByStatus(ctx, status)
}

func (r *Repository) ListAllUsersAnyStatus(ctx context.Context, limit, offset int32) ([]sqlc.User, error) {
	return r.queries.ListAllUsersAnyStatus(ctx, sqlc.ListAllUsersAnyStatusParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *Repository) CountAllUsersAnyStatus(ctx context.Context) (int64, error) {
	return r.queries.CountAllUsersAnyStatus(ctx)
}

func (r *Repository) SearchUsers(ctx context.Context, params sqlc.SearchUsersParams) ([]sqlc.User, error) {
	return r.queries.SearchUsers(ctx, params)
}

func (r *Repository) SearchUsersCount(ctx context.Context, query, status string) (int64, error) {
	return r.queries.SearchUsersCount(ctx, sqlc.SearchUsersCountParams{
		Query:  query,
		Status: status,
	})
}

func (r *Repository) GetUserInterests(ctx context.Context, userID int32) ([]sqlc.GetUserInterestsRow, error) {
	return r.queries.GetUserInterests(ctx, userID)
}

func (r *Repository) GetUserInterestIDs(ctx context.Context, userID int32) ([]int32, error) {
	return r.queries.GetUserInterestIDs(ctx, userID)
}

func (r *Repository) AddUserInterest(ctx context.Context, params sqlc.AddUserInterestParams) error {
	return r.queries.AddUserInterest(ctx, params)
}

func (r *Repository) DeleteUserInterests(ctx context.Context, userID int32) error {
	return r.queries.DeleteUserInterests(ctx, userID)
}

func (r *Repository) UpdateUserInterestsCount(ctx context.Context, params sqlc.UpdateUserInterestsCountParams) error {
	return r.queries.UpdateUserInterestsCount(ctx, params)
}

func (r *Repository) GetUserStatusData(ctx context.Context, id int32) (sqlc.GetUserStatusDataRow, error) {
	return r.queries.GetUserStatusData(ctx, id)
}

func (r *Repository) CreateUserDocument(ctx context.Context, params sqlc.CreateUserDocumentParams) (sqlc.UserDocument, error) {
	return r.queries.CreateUserDocument(ctx, params)
}

func (r *Repository) GetUserDocuments(ctx context.Context, userID int32) ([]sqlc.UserDocument, error) {
	return r.queries.GetUserDocuments(ctx, userID)
}

func (r *Repository) CountUserDocuments(ctx context.Context, userID int32) (int64, error) {
	return r.queries.CountUserDocuments(ctx, userID)
}

func (r *Repository) GetMissingDocumentTypes(ctx context.Context, userID int32) ([]string, error) {
	result, err := r.queries.GetMissingDocumentTypes(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Convert []interface{} to []string
	missing := make([]string, len(result))
	for i, v := range result {
		if s, ok := v.(string); ok {
			missing[i] = s
		}
	}
	return missing, nil
}

func (r *Repository) UpdateUserProfile(ctx context.Context, params sqlc.UpdateUserProfileParams) error {
	return r.queries.UpdateUserProfile(ctx, params)
}

func (r *Repository) GetRegionByID(ctx context.Context, id int32) (sqlc.Region, error) {
	return r.queries.GetRegionByID(ctx, id)
}

func (r *Repository) GetJobByID(ctx context.Context, id int32) (sqlc.Job, error) {
	return r.queries.GetJobByID(ctx, id)
}

func (r *Repository) CreateSession(ctx context.Context, params sqlc.CreateSessionParams) (sqlc.UserSession, error) {
	return r.queries.CreateSession(ctx, params)
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (sqlc.UserSession, error) {
	return r.queries.GetSessionByTokenHash(ctx, tokenHash)
}

func (r *Repository) SuspendUser(ctx context.Context, arg sqlc.SuspendUserParams) (int64, error) {
	return r.queries.SuspendUser(ctx, arg)
}

func (r *Repository) UnsuspendUser(ctx context.Context, arg sqlc.UnsuspendUserParams) (int64, error) {
	return r.queries.UnsuspendUser(ctx, arg)
}

func (r *Repository) LazyRestoreExpiredSuspension(ctx context.Context, userID int32) (int64, error) {
	return r.queries.LazyRestoreExpiredSuspension(ctx, userID)
}

func (r *Repository) BanUser(ctx context.Context, arg sqlc.BanUserParams) (int64, error) {
	return r.queries.BanUser(ctx, arg)
}

func (r *Repository) UnbanUser(ctx context.Context, arg sqlc.UnbanUserParams) (int64, error) {
	return r.queries.UnbanUser(ctx, arg)
}

func (r *Repository) InvalidateAllSessionsForUser(ctx context.Context, userID int32) error {
	return r.queries.InvalidateUserSessions(ctx, userID)
}

func (r *Repository) InvalidateUserSessions(ctx context.Context, userID int32) error {
	return r.queries.InvalidateUserSessions(ctx, userID)
}

func (r *Repository) InvalidateSession(ctx context.Context, id int32) error {
	return r.queries.InvalidateSession(ctx, id)
}

func (r *Repository) DeleteExpiredSessions(ctx context.Context) error {
	return r.queries.DeleteExpiredSessions(ctx)
}

func (r *Repository) CreateLoginHistory(ctx context.Context, params sqlc.CreateLoginHistoryParams) error {
	return r.queries.CreateLoginHistory(ctx, params)
}

func (r *Repository) GetUserWithRegion(ctx context.Context, userID int32) (sqlc.GetUserWithRegionRow, error) {
	return r.queries.GetUserWithRegion(ctx, userID)
}

func (r *Repository) GetUserWithRegionAndJob(ctx context.Context, userID int32) (sqlc.GetUserWithRegionAndJobRow, error) {
	return r.queries.GetUserWithRegionAndJob(ctx, userID)
}

func (r *Repository) GetUserWithRegionAndJobByPublicID(ctx context.Context, publicID interface{}) (sqlc.GetUserWithRegionAndJobByPublicIDRow, error) {
	uid := publicID.(uuid.UUID)
	pgUUID := pgtype.UUID{Bytes: uid, Valid: true}
	return r.queries.GetUserWithRegionAndJobByPublicID(ctx, pgUUID)
}

func (r *Repository) UpdateUserProfileWithNames(ctx context.Context, params sqlc.UpdateUserProfileWithNamesParams) error {
	return r.queries.UpdateUserProfileWithNames(ctx, params)
}

func (r *Repository) UpdateUserCachedNames(ctx context.Context, params sqlc.UpdateUserCachedNamesParams) error {
	return r.queries.UpdateUserCachedNames(ctx, params)
}

func (r *Repository) UpdateUserLocation(ctx context.Context, params sqlc.UpdateUserLocationParams) error {
	return r.queries.UpdateUserLocation(ctx, params)
}

func (r *Repository) HasActiveSubscription(ctx context.Context, userID int32) (bool, error) {
	return r.queries.HasActiveSubscription(ctx, userID)
}
