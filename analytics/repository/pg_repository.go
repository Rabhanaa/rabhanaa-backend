package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	analyticsModel "rabhana/analytics/model"
	"rabhana/db/sqlc"
)

type pgRepository struct {
	q *sqlc.Queries
}

func NewRepository(q *sqlc.Queries) Repository {
	return &pgRepository{q: q}
}

func toTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func pgDateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return time.Date(int(d.Time.Year()), d.Time.Month(), int(d.Time.Day()), 0, 0, 0, 0, time.UTC)
}

func (r *pgRepository) CountUsersByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	rows, err := r.q.AnalyticsCountUsersByDay(ctx, sqlc.AnalyticsCountUsersByDayParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.TimeSeriesPoint, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.TimeSeriesPoint{Bucket: pgDateToTime(row.Bucket), Value: row.Value}
	}
	return out, nil
}

func (r *pgRepository) CountFailedLoginsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	rows, err := r.q.AnalyticsCountFailedLoginsByDay(ctx, sqlc.AnalyticsCountFailedLoginsByDayParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.TimeSeriesPoint, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.TimeSeriesPoint{Bucket: pgDateToTime(row.Bucket), Value: row.Value}
	}
	return out, nil
}

func (r *pgRepository) CountActiveSessions(ctx context.Context) (int64, error) {
	return r.q.AnalyticsCountActiveSessions(ctx)
}

func (r *pgRepository) ProfileCompletionRatio(ctx context.Context) (int64, int64, error) {
	row, err := r.q.AnalyticsProfileCompletionRatio(ctx)
	if err != nil {
		return 0, 0, err
	}
	return row.Completed, row.Total, nil
}

func (r *pgRepository) CountBidsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	rows, err := r.q.AnalyticsCountBidsByDay(ctx, sqlc.AnalyticsCountBidsByDayParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.TimeSeriesPoint, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.TimeSeriesPoint{Bucket: pgDateToTime(row.Bucket), Value: row.Value}
	}
	return out, nil
}

func (r *pgRepository) CountClosedAuctionsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	rows, err := r.q.AnalyticsCountClosedAuctionsByDay(ctx, sqlc.AnalyticsCountClosedAuctionsByDayParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.TimeSeriesPoint, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.TimeSeriesPoint{Bucket: pgDateToTime(row.Bucket), Value: row.Value}
	}
	return out, nil
}

func (r *pgRepository) CountIssuesByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	rows, err := r.q.AnalyticsCountIssuesByDay(ctx, sqlc.AnalyticsCountIssuesByDayParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.TimeSeriesPoint, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.TimeSeriesPoint{Bucket: pgDateToTime(row.Bucket), Value: row.Value}
	}
	return out, nil
}

func (r *pgRepository) OrdersGMVByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.GMVPoint, error) {
	rows, err := r.q.AnalyticsOrdersGMVByDay(ctx, sqlc.AnalyticsOrdersGMVByDayParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.GMVPoint, len(rows))
	for i, row := range rows {
		gmv := "0"
		if row.Gmv.Valid {
			gmv = row.Gmv.Int.String()
			if row.Gmv.Exp != 0 {
				// format as decimal string using big.Int and exponent
				gmv = numericToString(row.Gmv)
			}
		}
		out[i] = analyticsModel.GMVPoint{
			Bucket: pgDateToTime(row.Bucket),
			Orders: row.Orders,
			GMV:    gmv,
		}
	}
	return out, nil
}

func (r *pgRepository) IssuesByStatus(ctx context.Context, from, to time.Time) ([]analyticsModel.StatusCount, error) {
	rows, err := r.q.AnalyticsIssuesByStatus(ctx, sqlc.AnalyticsIssuesByStatusParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.StatusCount, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.StatusCount{Status: row.Status, Count: row.Count}
	}
	return out, nil
}

func (r *pgRepository) ActiveSubscriptionsByTier(ctx context.Context) ([]analyticsModel.TierCount, error) {
	rows, err := r.q.AnalyticsActiveSubscriptionsByTier(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.TierCount, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.TierCount{
			TierName:      row.TierName,
			DisplayNameEN: row.DisplayNameEn,
			Count:         row.Count,
		}
	}
	return out, nil
}

func (r *pgRepository) InactiveSubscriptionsCount(ctx context.Context) (int64, error) {
	return r.q.AnalyticsInactiveSubscriptionsCount(ctx)
}

func (r *pgRepository) CountBuyRequestsByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.TimeSeriesPoint, error) {
	rows, err := r.q.AnalyticsCountBuyRequestsByDay(ctx, sqlc.AnalyticsCountBuyRequestsByDayParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.TimeSeriesPoint, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.TimeSeriesPoint{Bucket: pgDateToTime(row.Bucket), Value: row.Value}
	}
	return out, nil
}

func (r *pgRepository) UsersByStatus(ctx context.Context) ([]analyticsModel.StatusCount, error) {
	rows, err := r.q.AnalyticsUsersByStatus(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.StatusCount, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.StatusCount{Status: row.Status, Count: row.Count}
	}
	return out, nil
}

func (r *pgRepository) UsersBySource(ctx context.Context, from, to time.Time) ([]analyticsModel.SourceCount, error) {
	rows, err := r.q.AnalyticsUsersBySource(ctx, sqlc.AnalyticsUsersBySourceParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.SourceCount, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.SourceCount{Source: row.Source, Count: row.Count}
	}
	return out, nil
}

func (r *pgRepository) UsersBySourceByDay(ctx context.Context, from, to time.Time) ([]analyticsModel.SourceDayPoint, error) {
	rows, err := r.q.AnalyticsUsersBySourceByDay(ctx, sqlc.AnalyticsUsersBySourceByDayParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.SourceDayPoint, len(rows))
	for i, row := range rows {
		out[i] = analyticsModel.SourceDayPoint{
			Bucket: pgDateToTime(row.Bucket),
			Source: row.Source,
			Value:  row.Value,
		}
	}
	return out, nil
}

func (r *pgRepository) OverviewUsers(ctx context.Context, from, to time.Time) (int64, int64, error) {
	row, err := r.q.AnalyticsOverviewUsers(ctx, sqlc.AnalyticsOverviewUsersParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return 0, 0, err
	}
	return row.Total, row.NewInRange, nil
}

func (r *pgRepository) OverviewOrders(ctx context.Context, from, to time.Time) (int64, string, error) {
	row, err := r.q.AnalyticsOverviewOrders(ctx, sqlc.AnalyticsOverviewOrdersParams{
		FromTime: toTimestamptz(from),
		ToTime:   toTimestamptz(to),
	})
	if err != nil {
		return 0, "0", err
	}
	gmv := "0"
	if row.Gmv.Valid {
		gmv = numericToString(row.Gmv)
	}
	return row.Count, gmv, nil
}

func (r *pgRepository) CountOpenIssues(ctx context.Context) (int64, error) {
	return r.q.AnalyticsCountOpenIssues(ctx)
}

func (r *pgRepository) UsersCountByInterest(ctx context.Context) ([]analyticsModel.InterestCount, error) {
	rows, err := r.q.AnalyticsUsersCountByInterest(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.InterestCount, len(rows))
	for i, row := range rows {
		nameEn := ""
		if row.NameEn.Valid {
			nameEn = row.NameEn.String
		}
		out[i] = analyticsModel.InterestCount{
			ID:     row.ID,
			NameAr: row.NameAr,
			NameEn: nameEn,
			Count:  row.Count,
		}
	}
	return out, nil
}

func (r *pgRepository) UsersByInterest(ctx context.Context, interestID int32) ([]analyticsModel.InterestUser, error) {
	rows, err := r.q.AnalyticsUsersByInterest(ctx, interestID)
	if err != nil {
		return nil, err
	}
	out := make([]analyticsModel.InterestUser, len(rows))
	for i, row := range rows {
		publicID := ""
		if row.PublicID.Valid {
			if u, err := uuid.FromBytes(row.PublicID.Bytes[:]); err == nil {
				publicID = u.String()
			}
		}
		phone := ""
		if row.Phone.Valid {
			phone = row.Phone.String
		}
		createdAt := time.Time{}
		if row.CreatedAt.Valid {
			createdAt = row.CreatedAt.Time
		}
		out[i] = analyticsModel.InterestUser{
			PublicID:  publicID,
			Name:      row.Name,
			Phone:     phone,
			Status:    row.Status,
			CreatedAt: createdAt,
		}
	}
	return out, nil
}

// numericToString converts a pgtype.Numeric to a decimal string representation.
func numericToString(n pgtype.Numeric) string {
	if !n.Valid || n.Int == nil {
		return "0"
	}
	s := n.Int.String()
	if n.Exp == 0 {
		return s
	}
	if n.Exp > 0 {
		for i := int32(0); i < n.Exp; i++ {
			s += "0"
		}
		return s
	}
	// negative exponent — insert decimal point
	exp := int(-n.Exp)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	for len(s) <= exp {
		s = "0" + s
	}
	pos := len(s) - exp
	s = s[:pos] + "." + s[pos:]
	// trim trailing zeros after decimal
	i := len(s) - 1
	for i > pos && s[i] == '0' {
		i--
	}
	if s[i] == '.' {
		s = s[:i]
	} else {
		s = s[:i+1]
	}
	if neg {
		s = "-" + s
	}
	return s
}
