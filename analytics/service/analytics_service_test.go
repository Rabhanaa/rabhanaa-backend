package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	analyticsModel "rabhana/analytics/model"
)

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) CountUsersByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.TimeSeriesPoint), args.Error(1)
}
func (m *mockRepo) CountFailedLoginsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.TimeSeriesPoint), args.Error(1)
}
func (m *mockRepo) CountActiveSessions(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockRepo) ProfileCompletionRatio(ctx context.Context) (int64, int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Get(1).(int64), args.Error(2)
}
func (m *mockRepo) CountBidsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.TimeSeriesPoint), args.Error(1)
}
func (m *mockRepo) CountClosedAuctionsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.TimeSeriesPoint), args.Error(1)
}
func (m *mockRepo) OrdersGMVByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.GMVPoint, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.GMVPoint), args.Error(1)
}
func (m *mockRepo) CountIssuesByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.TimeSeriesPoint), args.Error(1)
}
func (m *mockRepo) IssuesByStatus(ctx context.Context, from, to time.Time) ([]analyticsModel.StatusCount, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.StatusCount), args.Error(1)
}
func (m *mockRepo) ActiveSubscriptionsByTier(ctx context.Context) ([]analyticsModel.TierCount, error) {
	args := m.Called(ctx)
	return args.Get(0).([]analyticsModel.TierCount), args.Error(1)
}
func (m *mockRepo) InactiveSubscriptionsCount(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockRepo) CountBuyRequestsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.TimeSeriesPoint), args.Error(1)
}
func (m *mockRepo) UsersByStatus(ctx context.Context) ([]analyticsModel.StatusCount, error) {
	args := m.Called(ctx)
	return args.Get(0).([]analyticsModel.StatusCount), args.Error(1)
}
func (m *mockRepo) UsersBySource(ctx context.Context, from, to time.Time) ([]analyticsModel.SourceCount, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.SourceCount), args.Error(1)
}
func (m *mockRepo) UsersBySourceByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.SourceDayPoint, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]analyticsModel.SourceDayPoint), args.Error(1)
}
func (m *mockRepo) OverviewUsers(ctx context.Context, from, to time.Time) (int64, int64, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).(int64), args.Get(1).(int64), args.Error(2)
}
func (m *mockRepo) OverviewOrders(ctx context.Context, from, to time.Time) (int64, string, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).(int64), args.Get(1).(string), args.Error(2)
}
func (m *mockRepo) CountOpenIssues(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockRepo) UsersCountByInterest(ctx context.Context) ([]analyticsModel.InterestCount, error) {
	args := m.Called(ctx)
	return args.Get(0).([]analyticsModel.InterestCount), args.Error(1)
}
func (m *mockRepo) UsersByInterest(ctx context.Context, interestID int32) ([]analyticsModel.InterestUser, error) {
	args := m.Called(ctx, interestID)
	return args.Get(0).([]analyticsModel.InterestUser), args.Error(1)
}

func TestValidateRange(t *testing.T) {
	svc := NewAnalyticsService(nil)

	now := time.Now().UTC()
	assert.NoError(t, svc.ValidateRange(now.AddDate(0,0,-10), now))
	assert.ErrorContains(t, svc.ValidateRange(now, now.AddDate(0,0,-1)), "from must be before to")
	assert.ErrorContains(t, svc.ValidateRange(now.AddDate(-2,0,0), now), "must not exceed 366 days")
}

func TestTimeSeriesCache(t *testing.T) {
	repo := new(mockRepo)
	svc := NewAnalyticsService(repo)
	ctx := context.Background()
	from := time.Now().UTC().AddDate(0,0,-7)
	to := time.Now().UTC()

	repo.On("CountUsersByDay", ctx, from, to).Return([]analyticsModel.TimeSeriesPoint{
		{Bucket: from, Value: 5},
	}, nil).Once()

	pts, err := svc.TimeSeries(ctx, analyticsModel.MetricNewUsers, from, to, analyticsModel.GranularityDay)
	assert.NoError(t, err)
	assert.Len(t, pts, 1)
	assert.Equal(t, int64(5), pts[0].Value)

	// second call should hit cache, repo not called again
	pts2, err := svc.TimeSeries(ctx, analyticsModel.MetricNewUsers, from, to, analyticsModel.GranularityDay)
	assert.NoError(t, err)
	assert.Equal(t, pts, pts2)
	repo.AssertExpectations(t)
}

func TestTimeSeriesGranularityWeek(t *testing.T) {
	svc := NewAnalyticsService(nil)
	ctx := context.Background()
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	repo := new(mockRepo)
	svc = NewAnalyticsService(repo)

	repo.On("CountUsersByDay", ctx, from, to).Return([]analyticsModel.TimeSeriesPoint{
		{Bucket: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Value: 1},
		{Bucket: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Value: 2},
		{Bucket: time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC), Value: 3},
		{Bucket: time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC), Value: 4},
	}, nil).Once()

	pts, err := svc.TimeSeries(ctx, analyticsModel.MetricNewUsers, from, to, analyticsModel.GranularityWeek)
	assert.NoError(t, err)
	assert.Len(t, pts, 2)
	assert.Equal(t, int64(3), pts[0].Value) // Jan 1+2 -> week of Jan 1 (Mon)
	assert.Equal(t, int64(7), pts[1].Value) // Jan 8+9 -> week of Jan 8
}

func TestTimeSeriesGranularityMonth(t *testing.T) {
	repo := new(mockRepo)
	svc := NewAnalyticsService(repo)
	ctx := context.Background()
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

	repo.On("CountUsersByDay", ctx, from, to).Return([]analyticsModel.TimeSeriesPoint{
		{Bucket: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), Value: 10},
		{Bucket: time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), Value: 20},
		{Bucket: time.Date(2024, 2, 10, 0, 0, 0, 0, time.UTC), Value: 30},
	}, nil).Once()

	pts, err := svc.TimeSeries(ctx, analyticsModel.MetricNewUsers, from, to, analyticsModel.GranularityMonth)
	assert.NoError(t, err)
	assert.Len(t, pts, 2)
	assert.Equal(t, int64(30), pts[0].Value)
	assert.Equal(t, int64(30), pts[1].Value)
}

func TestOverview(t *testing.T) {
	repo := new(mockRepo)
	svc := NewAnalyticsService(repo)
	ctx := context.Background()
	from := time.Now().UTC().AddDate(0,0,-7)
	to := time.Now().UTC()

	repo.On("CountActiveSessions", mock.Anything).Return(int64(42), nil)
	repo.On("ProfileCompletionRatio", mock.Anything).Return(int64(80), int64(100), nil)
	repo.On("OverviewUsers", mock.Anything, from, to).Return(int64(1000), int64(50), nil)
	repo.On("ActiveSubscriptionsByTier", mock.Anything).Return([]analyticsModel.TierCount{
		{TierName: "basic", Count: 10},
		{TierName: "pro", Count: 5},
	}, nil)
	repo.On("OverviewOrders", mock.Anything, from, to).Return(int64(20), "1500.50", nil)
	repo.On("CountOpenIssues", mock.Anything).Return(int64(3), nil)

	ov, err := svc.Overview(ctx, from, to)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), ov.ActiveSessions)
	assert.InDelta(t, 0.8, ov.ProfileCompletionRate, 0.001)
	assert.Equal(t, int64(1000), ov.UsersTotal)
	assert.Equal(t, int64(50), ov.UsersNewInRange)
	assert.Equal(t, int64(15), ov.ActiveSubscriptions)
	assert.Equal(t, int64(20), ov.OrdersInRange)
	assert.Equal(t, "1500.50", ov.GMVInRange)
	assert.Equal(t, int64(3), ov.OpenIssues)
}

func TestIssueBreakdown(t *testing.T) {
	repo := new(mockRepo)
	svc := NewAnalyticsService(repo)
	ctx := context.Background()
	from := time.Now().UTC().AddDate(0, 0, -7)
	to := time.Now().UTC()

	repo.On("IssuesByStatus", ctx, from, to).Return([]analyticsModel.StatusCount{
		{Status: "open", Count: 4},
		{Status: "replied", Count: 2},
		{Status: "closed", Count: 7},
	}, nil).Once()

	s, err := svc.IssueBreakdown(ctx, from, to)
	assert.NoError(t, err)
	assert.Equal(t, int64(13), s.Total)
	assert.Equal(t, int64(7), s.Resolved)
	assert.Equal(t, int64(6), s.Unresolved)

	// second call hits cache
	s2, err := svc.IssueBreakdown(ctx, from, to)
	assert.NoError(t, err)
	assert.Equal(t, s, s2)
	repo.AssertExpectations(t)
}

func TestStatusDistribution(t *testing.T) {
	repo := new(mockRepo)
	svc := NewAnalyticsService(repo)
	ctx := context.Background()

	repo.On("UsersByStatus", ctx).Return([]analyticsModel.StatusCount{
		{Status: "active", Count: 10},
		{Status: "pending_review", Count: 5},
	}, nil).Once()

	dist, err := svc.StatusDistribution(ctx)
	assert.NoError(t, err)
	assert.Len(t, dist, 2)
}
