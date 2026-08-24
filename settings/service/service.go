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
	"sync"

	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
)

// Keys. A setting that is not listed here cannot be read or written.
const (
	// KeyCarrierQuoteStage decides where shipping companies quote (#14).
	KeyCarrierQuoteStage = "carrier_quote_stage"
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

var allowed = map[string]map[string]struct{}{
	KeyCarrierQuoteStage: {
		StageOrder: {},
		StagePost:  {},
		StageBoth:  {},
	},
}

var defaults = map[string]string{
	KeyCarrierQuoteStage: StageOrder,
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
	values, ok := allowed[key]
	if !ok {
		return ErrUnknownSetting
	}
	if _, ok := values[value]; !ok {
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
