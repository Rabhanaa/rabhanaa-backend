// Package service holds settings an admin changes without a redeploy.
//
// Every other flag on this platform is an env var read once at boot
// (POST_APPROVAL_ENABLED, REQUIRE_DOCUMENTS, …), which is right for
// infrastructure and wrong for a policy the client wants to flip themselves.
//
// The rule worth keeping: env vars for deploy-time infrastructure, this table
// for behaviour the client owns. Two config systems is a maintenance trap, so
// the keys live in one whitelist below and nowhere else.
package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"rabhana/db/sqlc"
)

// Keys. A setting that is not listed here cannot be read or written.
const (
	// KeyCarrierQuoteStage decides where shipping companies quote (#14).
	KeyCarrierQuoteStage = "carrier_quote_stage"

	// Platform commission (#13). The rate is snapshotted onto every charge when
	// it accrues, so changing it here only affects future sales.
	KeyCommissionRatePercent = "commission_rate_percent"
	// KeyCommissionWeekCloseDay is the weekday the weekly invoice run fires, in
	// Africa/Cairo.
	KeyCommissionWeekCloseDay = "commission_week_close_day"
	// KeyCommissionGraceDays is how long a seller has to pay before the admin
	// worklist flags them. Stored onto each invoice as due_at at issue time.
	KeyCommissionGraceDays = "commission_grace_days"
)

// Values for KeyCarrierQuoteStage.
const (
	// StageOrder — carriers quote once a deal exists. The default: before a
	// winner is picked there is no buyer, so there is no destination and a
	// transport price would be a guess.
	StageOrder = "order"
	// StagePost — carriers quote on live posts. Indicative only, for the same
	// reason.
	StagePost = "post"
	// StageBoth — both surfaces at once.
	StageBoth = "both"
)

// A setting is validated by a function rather than a value set, because #13
// added numeric settings (a rate, a number of days) that an enum cannot express.
type validator func(string) bool

func oneOf(values ...string) validator {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}
	return func(candidate string) bool {
		_, ok := set[candidate]
		return ok
	}
}

// Bounded on both sides: a rate of 150% or a grace period of ten years is a
// typo, and this table is the only thing standing between a typo and the
// platform's billing.
func decimalRange(min, max float64) validator {
	return func(candidate string) bool {
		d, err := decimal.NewFromString(candidate)
		if err != nil {
			return false
		}
		return d.GreaterThanOrEqual(decimal.NewFromFloat(min)) &&
			d.LessThanOrEqual(decimal.NewFromFloat(max))
	}
}

func intRange(min, max int) validator {
	return func(candidate string) bool {
		n, err := strconv.Atoi(candidate)
		if err != nil {
			return false
		}
		return n >= min && n <= max
	}
}

// CommissionWeekDays is the accepted set, ordered for display. Exported so the
// admin API offers exactly what the validator accepts.
var CommissionWeekDays = []string{
	"saturday", "sunday", "monday", "tuesday", "wednesday", "thursday", "friday",
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday,
	"wednesday": time.Wednesday, "thursday": time.Thursday,
	"friday": time.Friday, "saturday": time.Saturday,
}

var allowed = map[string]validator{
	KeyCarrierQuoteStage:      oneOf(StageOrder, StagePost, StageBoth),
	KeyCommissionRatePercent:  decimalRange(0, 100),
	KeyCommissionWeekCloseDay: oneOf(CommissionWeekDays...),
	KeyCommissionGraceDays:    intRange(0, 90),
}

var defaults = map[string]string{
	KeyCarrierQuoteStage:      StageOrder,
	KeyCommissionRatePercent:  "1.5",
	KeyCommissionWeekCloseDay: "saturday",
	KeyCommissionGraceDays:    "3",
}

var ErrUnknownSetting = errors.New("UNKNOWN_SETTING")
var ErrInvalidSettingValue = errors.New("INVALID_SETTING_VALUE")

type Service struct {
	queries *sqlc.Queries

	// Read on nearly every carrier request, written by one admin now and then.
	// The API runs as a single instance — the in-process cron already requires
	// that — so an in-process cache needs no cross-replica invalidation. If the
	// API is ever scaled out, this cache and the cron both need revisiting.
	mu    sync.RWMutex
	cache map[string]string
}

func NewService(queries *sqlc.Queries) *Service {
	return &Service{queries: queries, cache: map[string]string{}}
}

// Load fills the cache at boot. A failure is not fatal: Get falls back to the
// documented default, which is the same behaviour as a missing env var.
func (s *Service) Load(ctx context.Context) {
	rows, err := s.queries.ListAppSettings(ctx)
	if err != nil {
		slog.Error("failed to load app settings, using defaults", "error", err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rows {
		if _, ok := allowed[r.Key]; ok {
			s.cache[r.Key] = r.Value
		}
	}
}

// Get returns the current value, or the default when unset or unreadable.
func (s *Service) Get(key string) string {
	s.mu.RLock()
	v, ok := s.cache[key]
	s.mu.RUnlock()
	if ok && v != "" {
		return v
	}
	return defaults[key]
}

// All returns every whitelisted setting with its effective value, so the admin
// screen shows what is actually in force rather than only what is stored.
func (s *Service) All() map[string]string {
	out := make(map[string]string, len(allowed))
	for key := range allowed {
		out[key] = s.Get(key)
	}
	return out
}

// Set validates against the whitelist before writing. An unknown key or an
// unrecognised value is rejected rather than stored — a typo here would silently
// change platform behaviour.
func (s *Service) Set(ctx context.Context, key, value string, adminID int32) error {
	valid, ok := allowed[key]
	if !ok {
		return ErrUnknownSetting
	}
	if !valid(value) {
		return ErrInvalidSettingValue
	}

	if _, err := s.queries.UpsertAppSetting(ctx, sqlc.UpsertAppSettingParams{
		Key:              key,
		Value:            value,
		UpdatedByAdminID: pgtype.Int4{Int32: adminID, Valid: adminID > 0},
	}); err != nil {
		return err
	}

	s.mu.Lock()
	s.cache[key] = value
	s.mu.Unlock()

	slog.Info("app setting changed", "key", key, "value", value, "admin_id", adminID)
	return nil
}

// CarrierQuoteStage is the one caller that matters today, wrapped so callers do
// not repeat the key.
func (s *Service) CarrierQuoteStage() string {
	return s.Get(KeyCarrierQuoteStage)
}

// QuotesOnOrders and QuotesOnPosts read the stage as the two questions the
// carrier and merchant code actually asks.
func (s *Service) QuotesOnOrders() bool {
	stage := s.CarrierQuoteStage()
	return stage == StageOrder || stage == StageBoth
}

func (s *Service) QuotesOnPosts() bool {
	stage := s.CarrierQuoteStage()
	return stage == StagePost || stage == StageBoth
}

// CommissionRate is the platform's cut as a percentage (#13). Callers snapshot
// this onto the charge they create — never read it back to recompute an old one.
// A malformed stored value falls back to the default rather than billing zero,
// which would silently stop collection.
func (s *Service) CommissionRate() decimal.Decimal {
	d, err := decimal.NewFromString(s.Get(KeyCommissionRatePercent))
	if err != nil {
		slog.Error("invalid commission rate stored, using default",
			"value", s.Get(KeyCommissionRatePercent), "default", defaults[KeyCommissionRatePercent])
		d, _ = decimal.NewFromString(defaults[KeyCommissionRatePercent])
	}
	return d
}

// CommissionWeekCloseDay is the weekday the weekly invoice run fires.
func (s *Service) CommissionWeekCloseDay() time.Weekday {
	if day, ok := weekdays[s.Get(KeyCommissionWeekCloseDay)]; ok {
		return day
	}
	return time.Saturday
}

// CommissionGraceDays is how long after issue an invoice becomes overdue.
func (s *Service) CommissionGraceDays() int {
	n, err := strconv.Atoi(s.Get(KeyCommissionGraceDays))
	if err != nil || n < 0 {
		return 3
	}
	return n
}
