package model

import "time"

type TimeSeriesPoint struct {
	Bucket time.Time `json:"bucket"`
	Value  int64     `json:"value"`
}

type GMVPoint struct {
	Bucket time.Time `json:"bucket"`
	Orders int64     `json:"orders"`
	GMV    string    `json:"gmv"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type IssueSummary struct {
	Total      int64 `json:"total"`
	Resolved   int64 `json:"resolved"`
	Unresolved int64 `json:"unresolved"`
}

type SourceCount struct {
	Source string `json:"source"`
	Count  int64  `json:"count"`
}

type SourceDayPoint struct {
	Bucket time.Time `json:"bucket"`
	Source string    `json:"source"`
	Value  int64     `json:"value"`
}

type TierCount struct {
	TierName      string `json:"tier_name"`
	DisplayNameEN string `json:"display_name_en"`
	Count         int64  `json:"count"`
}

type SubscriptionStats struct {
	ActiveByTier    []TierCount `json:"active_by_tier"`
	InactiveTotal   int64       `json:"inactive_total"`
}

type InterestCount struct {
	ID     int32  `json:"id"`
	NameAr string `json:"name_ar"`
	NameEn string `json:"name_en"`
	Count  int64  `json:"count"`
}

type InterestUser struct {
	PublicID  string    `json:"public_id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type OverviewSummary struct {
	ActiveSessions       int64        `json:"active_sessions"`
	ProfileCompletionRate float64     `json:"profile_completion_rate"`
	UsersTotal           int64        `json:"users_total"`
	UsersNewInRange      int64        `json:"users_new_in_range"`
	ActiveSubscriptions  int64        `json:"active_subscriptions"`
	SubscriptionsByTier  []TierCount  `json:"subscriptions_by_tier"`
	OrdersInRange        int64        `json:"orders_in_range"`
	GMVInRange           string       `json:"gmv_in_range"`
	OpenIssues           int64        `json:"open_issues"`
}

type MetricKey string

const (
	MetricNewUsers       MetricKey = "new_users"
	MetricFailedLogins   MetricKey = "failed_logins"
	MetricBids           MetricKey = "bids"
	MetricClosedAuctions MetricKey = "closed_auctions"
	MetricOrders         MetricKey = "orders"
	MetricBuyRequests    MetricKey = "buy_requests"
	MetricIssues         MetricKey = "issues"
)

type Granularity string

const (
	GranularityDay   Granularity = "day"
	GranularityWeek  Granularity = "week"
	GranularityMonth Granularity = "month"
)

func (g Granularity) IsValid() bool {
	switch g {
	case GranularityDay, GranularityWeek, GranularityMonth:
		return true
	}
	return false
}
