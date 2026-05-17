package repository

import (
	"context"
	"time"

	analyticsModel "rabhana/analytics/model"
)

type Repository interface {
	CountUsersByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error)
	CountFailedLoginsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error)
	CountActiveSessions(ctx context.Context) (int64, error)
	ProfileCompletionRatio(ctx context.Context) (completed, total int64, err error)
	CountBidsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error)
	CountClosedAuctionsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error)
	OrdersGMVByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.GMVPoint, error)
	CountIssuesByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error)
	IssuesByStatus(ctx context.Context, from, to time.Time) ([]analyticsModel.StatusCount, error)
	ActiveSubscriptionsByTier(ctx context.Context) ([]analyticsModel.TierCount, error)
	InactiveSubscriptionsCount(ctx context.Context) (int64, error)
	CountBuyRequestsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error)
	UsersByStatus(ctx context.Context) ([]analyticsModel.StatusCount, error)
	UsersBySource(ctx context.Context, from, to time.Time) ([]analyticsModel.SourceCount, error)
	UsersBySourceByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.SourceDayPoint, error)
	OverviewUsers(ctx context.Context, from, to time.Time) (total, newInRange int64, err error)
	OverviewOrders(ctx context.Context, from, to time.Time) (count int64, gmv string, err error)
	CountOpenIssues(ctx context.Context) (int64, error)
	UsersCountByInterest(ctx context.Context) ([]analyticsModel.InterestCount, error)
	UsersByInterest(ctx context.Context, interestID int32) ([]analyticsModel.InterestUser, error)
}
