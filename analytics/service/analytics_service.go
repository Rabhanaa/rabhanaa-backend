package service

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	analyticsModel "rabhana/analytics/model"
	analyticsRepo "rabhana/analytics/repository"
)

const (
	maxRangeDays = 366
	cacheTTL     = 60 * time.Second
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

type AnalyticsService struct {
	repo  analyticsRepo.Repository
	mu    sync.Mutex
	cache map[string]cacheEntry
}

func NewAnalyticsService(repo analyticsRepo.Repository) *AnalyticsService {
	return &AnalyticsService{
		repo:  repo,
		cache: make(map[string]cacheEntry),
	}
}

// quantize rounds a time to the nearest cacheTTL boundary so that distinct
// per-second `to` values from frontends don't blow up the cache.
func quantize(t time.Time) int64 {
	return t.UTC().Truncate(cacheTTL).Unix()
}

func (s *AnalyticsService) get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.cache[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.value, true
}

func (s *AnalyticsService) set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	// opportunistic eviction: sweep expired entries on each write so the
	// cache size stays bounded by the number of live keys × TTL.
	for k, e := range s.cache {
		if now.After(e.expiresAt) {
			delete(s.cache, k)
		}
	}
	s.cache[key] = cacheEntry{value: value, expiresAt: now.Add(cacheTTL)}
}

func (s *AnalyticsService) ValidateRange(from, to time.Time) error {
	if from.After(to) {
		return fmt.Errorf("from must be before to")
	}
	if to.Sub(from) > maxRangeDays*24*time.Hour {
		return fmt.Errorf("date range must not exceed %d days", maxRangeDays)
	}
	return nil
}

func (s *AnalyticsService) Overview(ctx context.Context, from, to time.Time) (analyticsModel.OverviewSummary, error) {
	key := fmt.Sprintf("overview:%d:%d", quantize(from), quantize(to))
	if v, ok := s.get(key); ok {
		return v.(analyticsModel.OverviewSummary), nil
	}

	var (
		sessions               int64
		completed, total       int64
		usersTotal, usersNew   int64
		tiers                  []analyticsModel.TierCount
		ordersCount            int64
		gmv                    string
		openIssues             int64
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := s.repo.CountActiveSessions(gctx)
		sessions = v
		return err
	})
	g.Go(func() error {
		c, t, err := s.repo.ProfileCompletionRatio(gctx)
		completed, total = c, t
		return err
	})
	g.Go(func() error {
		t, n, err := s.repo.OverviewUsers(gctx, from, to)
		usersTotal, usersNew = t, n
		return err
	})
	g.Go(func() error {
		v, err := s.repo.ActiveSubscriptionsByTier(gctx)
		tiers = v
		return err
	})
	g.Go(func() error {
		c, v, err := s.repo.OverviewOrders(gctx, from, to)
		ordersCount, gmv = c, v
		return err
	})
	g.Go(func() error {
		v, err := s.repo.CountOpenIssues(gctx)
		openIssues = v
		return err
	})
	if err := g.Wait(); err != nil {
		return analyticsModel.OverviewSummary{}, err
	}

	var completionRate float64
	if total > 0 {
		completionRate = float64(completed) / float64(total)
	}
	var activeSubs int64
	for _, t := range tiers {
		activeSubs += t.Count
	}

	result := analyticsModel.OverviewSummary{
		ActiveSessions:        sessions,
		ProfileCompletionRate: completionRate,
		UsersTotal:            usersTotal,
		UsersNewInRange:       usersNew,
		ActiveSubscriptions:   activeSubs,
		SubscriptionsByTier:   tiers,
		OrdersInRange:         ordersCount,
		GMVInRange:            gmv,
		OpenIssues:            openIssues,
	}
	s.set(key, result)
	return result, nil
}

func (s *AnalyticsService) TimeSeries(ctx context.Context, metric analyticsModel.MetricKey, from, to time.Time, granularity analyticsModel.Granularity) ([]analyticsModel.TimeSeriesPoint, error) {
	key := fmt.Sprintf("ts:%s:%d:%d:%s", metric, quantize(from), quantize(to), granularity)
	if v, ok := s.get(key); ok {
		return v.([]analyticsModel.TimeSeriesPoint), nil
	}

	var points []analyticsModel.TimeSeriesPoint
	var err error

	switch metric {
	case analyticsModel.MetricNewUsers:
		points, err = s.repo.CountUsersByDay(ctx, from, to)
	case analyticsModel.MetricFailedLogins:
		points, err = s.repo.CountFailedLoginsByDay(ctx, from, to)
	case analyticsModel.MetricBids:
		points, err = s.repo.CountBidsByDay(ctx, from, to)
	case analyticsModel.MetricClosedAuctions:
		points, err = s.repo.CountClosedAuctionsByDay(ctx, from, to)
	case analyticsModel.MetricOrders:
		gmvRows, e := s.repo.OrdersGMVByDay(ctx, from, to)
		if e != nil {
			return nil, e
		}
		pts := make([]analyticsModel.TimeSeriesPoint, len(gmvRows))
		for i, r := range gmvRows {
			pts[i] = analyticsModel.TimeSeriesPoint{Bucket: r.Bucket, Value: r.Orders}
		}
		points = pts
	case analyticsModel.MetricBuyRequests:
		points, err = s.repo.CountBuyRequestsByDay(ctx, from, to)
	case analyticsModel.MetricIssues:
		points, err = s.repo.CountIssuesByDay(ctx, from, to)
	default:
		return nil, fmt.Errorf("unknown metric: %s", metric)
	}

	if err != nil {
		return nil, err
	}

	points = rollup(points, granularity)
	s.set(key, points)
	return points, nil
}

func (s *AnalyticsService) OrdersGMV(ctx context.Context, from, to time.Time) ([]analyticsModel.GMVPoint, error) {
	key := fmt.Sprintf("gmv:%d:%d", quantize(from), quantize(to))
	if v, ok := s.get(key); ok {
		return v.([]analyticsModel.GMVPoint), nil
	}
	points, err := s.repo.OrdersGMVByDay(ctx, from, to)
	if err != nil {
		return nil, err
	}
	s.set(key, points)
	return points, nil
}

func (s *AnalyticsService) StatusDistribution(ctx context.Context) ([]analyticsModel.StatusCount, error) {
	key := "status_dist"
	if v, ok := s.get(key); ok {
		return v.([]analyticsModel.StatusCount), nil
	}
	result, err := s.repo.UsersByStatus(ctx)
	if err != nil {
		return nil, err
	}
	s.set(key, result)
	return result, nil
}

func (s *AnalyticsService) UsersBySource(ctx context.Context, from, to time.Time) ([]analyticsModel.SourceCount, error) {
	key := fmt.Sprintf("source_dist:%d:%d", quantize(from), quantize(to))
	if v, ok := s.get(key); ok {
		return v.([]analyticsModel.SourceCount), nil
	}
	result, err := s.repo.UsersBySource(ctx, from, to)
	if err != nil {
		return nil, err
	}
	s.set(key, result)
	return result, nil
}

func (s *AnalyticsService) UsersBySourceByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.SourceDayPoint, error) {
	key := fmt.Sprintf("source_by_day:%d:%d", quantize(from), quantize(to))
	if v, ok := s.get(key); ok {
		return v.([]analyticsModel.SourceDayPoint), nil
	}
	result, err := s.repo.UsersBySourceByDay(ctx, from, to)
	if err != nil {
		return nil, err
	}
	s.set(key, result)
	return result, nil
}

func (s *AnalyticsService) IssueBreakdown(ctx context.Context, from, to time.Time) (analyticsModel.IssueSummary, error) {
	key := fmt.Sprintf("issues_summary:%d:%d", quantize(from), quantize(to))
	if v, ok := s.get(key); ok {
		return v.(analyticsModel.IssueSummary), nil
	}
	rows, err := s.repo.IssuesByStatus(ctx, from, to)
	if err != nil {
		return analyticsModel.IssueSummary{}, err
	}
	var summary analyticsModel.IssueSummary
	for _, r := range rows {
		summary.Total += r.Count
		if r.Status == "closed" {
			summary.Resolved += r.Count
		} else {
			summary.Unresolved += r.Count
		}
	}
	s.set(key, summary)
	return summary, nil
}

func rollup(points []analyticsModel.TimeSeriesPoint, granularity analyticsModel.Granularity) []analyticsModel.TimeSeriesPoint {
	if granularity == analyticsModel.GranularityDay || len(points) == 0 {
		return points
	}

	bucket := func(t time.Time) time.Time {
		switch granularity {
		case analyticsModel.GranularityWeek:
			wd := int(t.Weekday())
			if wd == 0 {
				wd = 7
			}
			return t.AddDate(0, 0, -(wd - 1))
		case analyticsModel.GranularityMonth:
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
		return t
	}

	m := make(map[string]analyticsModel.TimeSeriesPoint)
	for _, p := range points {
		b := bucket(p.Bucket)
		key := b.Format("2006-01-02")
		if existing, ok := m[key]; ok {
			existing.Value += p.Value
			m[key] = existing
		} else {
			m[key] = analyticsModel.TimeSeriesPoint{Bucket: b, Value: p.Value}
		}
	}

	out := make([]analyticsModel.TimeSeriesPoint, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bucket.Before(out[j].Bucket) })
	return out
}

func (s *AnalyticsService) UsersCountByInterest(ctx context.Context) ([]analyticsModel.InterestCount, error) {
	key := "interest_counts"
	if v, ok := s.get(key); ok {
		return v.([]analyticsModel.InterestCount), nil
	}
	result, err := s.repo.UsersCountByInterest(ctx)
	if err != nil {
		return nil, err
	}
	s.set(key, result)
	return result, nil
}

func (s *AnalyticsService) UsersByInterest(ctx context.Context, interestID int32) ([]analyticsModel.InterestUser, error) {
	key := fmt.Sprintf("interest_users:%d", interestID)
	if v, ok := s.get(key); ok {
		return v.([]analyticsModel.InterestUser), nil
	}
	result, err := s.repo.UsersByInterest(ctx, interestID)
	if err != nil {
		return nil, err
	}
	s.set(key, result)
	return result, nil
}

func (s *AnalyticsService) SubscriptionStats(ctx context.Context) (analyticsModel.SubscriptionStats, error) {
	key := "sub_stats"
	if v, ok := s.get(key); ok {
		return v.(analyticsModel.SubscriptionStats), nil
	}

	var (
		tiers    []analyticsModel.TierCount
		inactive int64
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := s.repo.ActiveSubscriptionsByTier(gctx)
		tiers = v
		return err
	})
	g.Go(func() error {
		v, err := s.repo.InactiveSubscriptionsCount(gctx)
		inactive = v
		return err
	})
	if err := g.Wait(); err != nil {
		return analyticsModel.SubscriptionStats{}, err
	}

	result := analyticsModel.SubscriptionStats{
		ActiveByTier:  tiers,
		InactiveTotal: inactive,
	}
	s.set(key, result)
	return result, nil
}
