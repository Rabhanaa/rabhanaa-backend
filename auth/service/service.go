package service

import (
	"context"

	"rabhana/db/sqlc"
)

type AuthRepository interface {
	GetQueries() *sqlc.Queries
	CreateUser(ctx context.Context, params sqlc.CreateUserParams) (sqlc.User, error)
	GetUserByID(ctx context.Context, id int32) (sqlc.User, error)
	GetUserByPublicID(ctx context.Context, publicID interface{}) (sqlc.User, error) // accepts uuid.UUID
	GetUserByEmail(ctx context.Context, email string) (sqlc.User, error)
	GetUserWithRegionAndJob(ctx context.Context, userID int32) (sqlc.GetUserWithRegionAndJobRow, error)
	GetUserWithRegionAndJobByPublicID(ctx context.Context, publicID interface{}) (sqlc.GetUserWithRegionAndJobByPublicIDRow, error)
	UpdateUserStatus(ctx context.Context, params sqlc.UpdateUserStatusParams) error
	UpdateUserStatusWithReason(ctx context.Context, params sqlc.UpdateUserStatusWithReasonParams) error
	UpdateUserPassword(ctx context.Context, params sqlc.UpdateUserPasswordParams) error
	UpdateUserOTP(ctx context.Context, params sqlc.UpdateUserOTPParams) error
	ClearUserOTP(ctx context.Context, id int32) error
	UpdateUserFCMToken(ctx context.Context, params sqlc.UpdateUserFCMTokenParams) error
	ListUsersByStatus(ctx context.Context, params sqlc.ListUsersByStatusParams) ([]sqlc.User, error)
	CountUsersByStatus(ctx context.Context, status string) (int64, error)
	ListAllUsersAnyStatus(ctx context.Context, limit, offset int32) ([]sqlc.User, error)
	CountAllUsersAnyStatus(ctx context.Context) (int64, error)
	SearchUsers(ctx context.Context, params sqlc.SearchUsersParams) ([]sqlc.User, error)
	SearchUsersCount(ctx context.Context, query, status string) (int64, error)

	GetUserInterests(ctx context.Context, userID int32) ([]sqlc.GetUserInterestsRow, error)
	GetUserInterestIDs(ctx context.Context, userID int32) ([]int32, error)
	AddUserInterest(ctx context.Context, params sqlc.AddUserInterestParams) error
	DeleteUserInterests(ctx context.Context, userID int32) error
	UpdateUserInterestsCount(ctx context.Context, params sqlc.UpdateUserInterestsCountParams) error
	GetUserStatusData(ctx context.Context, id int32) (sqlc.GetUserStatusDataRow, error)

	CreateUserDocument(ctx context.Context, params sqlc.CreateUserDocumentParams) (sqlc.UserDocument, error)
	GetUserDocuments(ctx context.Context, userID int32) ([]sqlc.UserDocument, error)
	CountUserDocuments(ctx context.Context, userID int32) (int64, error)
	GetMissingDocumentTypes(ctx context.Context, userID int32) ([]string, error)

	UpdateUserProfile(ctx context.Context, params sqlc.UpdateUserProfileParams) error
	UpdateUserProfileWithNames(ctx context.Context, params sqlc.UpdateUserProfileWithNamesParams) error
	UpdateUserCachedNames(ctx context.Context, params sqlc.UpdateUserCachedNamesParams) error

	GetRegionByID(ctx context.Context, id int32) (sqlc.Region, error)
	GetJobByID(ctx context.Context, id int32) (sqlc.Job, error)

	SuspendUser(ctx context.Context, arg sqlc.SuspendUserParams) (int64, error)
	UnsuspendUser(ctx context.Context, arg sqlc.UnsuspendUserParams) (int64, error)
	LazyRestoreExpiredSuspension(ctx context.Context, userID int32) (int64, error)
	BanUser(ctx context.Context, arg sqlc.BanUserParams) (int64, error)
	UnbanUser(ctx context.Context, arg sqlc.UnbanUserParams) (int64, error)
	InvalidateAllSessionsForUser(ctx context.Context, userID int32) error

	CreateSession(ctx context.Context, params sqlc.CreateSessionParams) (sqlc.UserSession, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (sqlc.UserSession, error)
	InvalidateUserSessions(ctx context.Context, userID int32) error
	InvalidateSession(ctx context.Context, id int32) error

	CreateLoginHistory(ctx context.Context, params sqlc.CreateLoginHistoryParams) error
	UpdateUserLocation(ctx context.Context, params sqlc.UpdateUserLocationParams) error
	HasActiveSubscription(ctx context.Context, userID int32) (bool, error)
}
