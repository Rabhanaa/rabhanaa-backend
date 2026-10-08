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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"rabhana/db/sqlc"
	"rabhana/lib/secretbox"
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
	// KeyCommissionReminderDays is how often an unpaid invoice re-notifies the
	// seller once it is due. Read at send time, so changing it takes effect on
	// the next sweep.
	KeyCommissionReminderDays = "commission_reminder_days"
	// KeyCommissionStartDate is the earliest sale that may be charged, as
	// YYYY-MM-DD, or CommissionStartAll to bill the whole history. It exists
	// because switching commission on without it invoiced merchants for deals
	// they had closed months earlier, before any commission was announced.
	KeyCommissionStartDate = "commission_start_date"

	// News (AI-written articles for Pro members). The provider the editor uses
	// by default, and each provider's model. Models have no default: names
	// change too often to hardcode, so the admin picks one from the provider's
	// live list.
	KeyAIProvider      = "ai_provider"
	KeyOpenAIModel     = "openai_model"
	KeyGeminiModel     = "gemini_model"
	KeyOpenRouterModel = "openrouter_model"
	// KeyNewsNotifyMode decides who is pushed when a story is published.
	KeyNewsNotifyMode = "news_notify_mode"
)

// Secrets. Kept out of the allowed map on purpose, so they can never be read
// back through All() or written in plain text through Set(): they go through
// SetSecret, which encrypts, and the API only ever returns a masked hint.
const (
	KeyOpenAIAPIKey     = "openai_api_key"
	KeyGeminiAPIKey     = "gemini_api_key"
	KeyOpenRouterAPIKey = "openrouter_api_key"
)

// Values for KeyAIProvider.
const (
	AIProviderOpenAI     = "openai"
	AIProviderGemini     = "gemini"
	AIProviderOpenRouter = "openrouter"
)

// AIProviders is the accepted set, ordered for display.
var AIProviders = []string{AIProviderOpenAI, AIProviderGemini, AIProviderOpenRouter}

// Values for KeyNewsNotifyMode. Every mode reaches Pro and non-Pro members
// alike: a non-Pro member who taps the push is shown the story's teaser and
// an upgrade prompt.
const (
	NewsNotifyOff        = "off"
	NewsNotifyAll        = "all"
	NewsNotifyByInterest = "by_interest"
)

var NewsNotifyModes = []string{NewsNotifyOff, NewsNotifyAll, NewsNotifyByInterest}

// Values stored before every mode included non-Pro members. Never released,
// but local and test databases may hold them.
var legacyNewsNotifyModes = map[string]string{"all_pro": NewsNotifyAll, "pro_by_interest": NewsNotifyByInterest}

var secretKeys = map[string]bool{
	KeyOpenAIAPIKey:     true,
	KeyGeminiAPIKey:     true,
	KeyOpenRouterAPIKey: true,
}

// The settings key holding each provider's API key and model.
var (
	aiKeyFor   = map[string]string{AIProviderOpenAI: KeyOpenAIAPIKey, AIProviderGemini: KeyGeminiAPIKey, AIProviderOpenRouter: KeyOpenRouterAPIKey}
	aiModelFor = map[string]string{AIProviderOpenAI: KeyOpenAIModel, AIProviderGemini: KeyGeminiModel, AIProviderOpenRouter: KeyOpenRouterModel}
)

// CommissionStartAll disables the cutoff and bills every completed sale ever
// recorded. Reachable from the admin screen, but a deliberate choice.
const CommissionStartAll = "all"

const commissionStartDateLayout = "2006-01-02"

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

// Model ids as the three providers spell them: "gpt-…", "gemini-…",
// "vendor/model:variant" on OpenRouter.
var modelNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+-]{0,127}$`)

func isModelName(candidate string) bool {
	return modelNamePattern.MatchString(candidate)
}

func isStartDate(candidate string) bool {
	if candidate == CommissionStartAll {
		return true
	}
	_, err := time.Parse(commissionStartDateLayout, candidate)
	return err == nil
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
	// At least one day: a smaller value would notify on every cron tick.
	KeyCommissionReminderDays: intRange(1, 30),
	KeyCommissionStartDate:    isStartDate,
	KeyAIProvider:             oneOf(AIProviders...),
	KeyOpenAIModel:            isModelName,
	KeyGeminiModel:            isModelName,
	KeyOpenRouterModel:        isModelName,
	KeyNewsNotifyMode:         oneOf(NewsNotifyModes...),
}

var defaults = map[string]string{
	KeyCarrierQuoteStage:      StageOrder,
	KeyCommissionRatePercent:  "1.5",
	KeyCommissionWeekCloseDay: "saturday",
	KeyCommissionGraceDays:    "3",
	KeyCommissionReminderDays: "2",
	// Defaults to the whole history so an existing deployment's behaviour does
	// not change silently; production sets a real date.
	KeyCommissionStartDate: CommissionStartAll,
	KeyAIProvider:          AIProviderOpenAI,
	KeyNewsNotifyMode:      NewsNotifyAll,
}

var ErrUnknownSetting = errors.New("UNKNOWN_SETTING")
var ErrInvalidSettingValue = errors.New("INVALID_SETTING_VALUE")

// ErrSecretsUnavailable means SETTINGS_ENCRYPTION_KEY is not configured, so
// there is no way to store a secret without writing it in plain text.
var ErrSecretsUnavailable = errors.New("SECRETS_UNAVAILABLE")

type Service struct {
	queries *sqlc.Queries

	// Read on nearly every carrier request, written by one admin now and then.
	// The API runs as a single instance — the in-process cron already requires
	// that — so an in-process cache needs no cross-replica invalidation. If the
	// API is ever scaled out, this cache and the cron both need revisiting.
	mu    sync.RWMutex
	cache map[string]string

	// Nil when SETTINGS_ENCRYPTION_KEY is unset: everything else keeps working,
	// only storing secrets is refused.
	box *secretbox.Box
	// Decrypted secrets. Plain text lives only in process memory.
	secrets map[string]string
}

func NewService(queries *sqlc.Queries, box *secretbox.Box) *Service {
	return &Service{queries: queries, cache: map[string]string{}, box: box, secrets: map[string]string{}}
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
			continue
		}
		if secretKeys[r.Key] {
			if s.box == nil {
				slog.Warn("secret setting stored but SETTINGS_ENCRYPTION_KEY is not set; ignoring it", "key", r.Key)
				continue
			}
			plain, err := s.box.Open(r.Value)
			if err != nil {
				// Most likely the encryption key changed. The admin re-enters it.
				slog.Error("cannot decrypt secret setting; it must be entered again", "key", r.Key, "error", err)
				continue
			}
			s.secrets[r.Key] = plain
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

// SecretsAvailable reports whether secrets can be stored at all.
func (s *Service) SecretsAvailable() bool {
	return s.box != nil
}

// SetSecret encrypts and stores a secret. The value is never logged.
func (s *Service) SetSecret(ctx context.Context, key, value string, adminID int32) error {
	if !secretKeys[key] {
		return ErrUnknownSetting
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 || strings.ContainsAny(value, " \t\r\n") {
		return ErrInvalidSettingValue
	}
	if s.box == nil {
		return ErrSecretsUnavailable
	}
	sealed, err := s.box.Seal(value)
	if err != nil {
		return err
	}
	if _, err := s.queries.UpsertAppSetting(ctx, sqlc.UpsertAppSettingParams{
		Key:              key,
		Value:            sealed,
		UpdatedByAdminID: pgtype.Int4{Int32: adminID, Valid: adminID > 0},
	}); err != nil {
		return err
	}

	s.mu.Lock()
	s.secrets[key] = value
	s.mu.Unlock()

	slog.Info("secret setting changed", "key", key, "admin_id", adminID)
	return nil
}

// ClearSecret removes a stored secret.
func (s *Service) ClearSecret(ctx context.Context, key string, adminID int32) error {
	if !secretKeys[key] {
		return ErrUnknownSetting
	}
	if err := s.queries.DeleteAppSetting(ctx, key); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.secrets, key)
	s.mu.Unlock()

	slog.Info("secret setting cleared", "key", key, "admin_id", adminID)
	return nil
}

// Secret returns the plain value, or "" when unset.
func (s *Service) Secret(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.secrets[key]
}

// SecretHints is what the admin screen may see: for each secret, "" when unset
// or a mask showing only the last four characters, enough to tell which key is
// in place without exposing it.
func (s *Service) SecretHints() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(secretKeys))
	for key := range secretKeys {
		out[key] = maskSecret(s.secrets[key])
	}
	return out
}

func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "••••"
	}
	return "••••" + v[len(v)-4:]
}

// AIProvider is the provider the news editor uses unless told otherwise.
func (s *Service) AIProvider() string {
	return s.Get(KeyAIProvider)
}

// IsAIProvider reports whether name is one of the supported providers.
func IsAIProvider(name string) bool {
	_, ok := aiKeyFor[name]
	return ok
}

// AICredentials returns the API key and model configured for a provider.
// Either may be empty when the admin has not set it yet.
func (s *Service) AICredentials(provider string) (apiKey, model string) {
	keySetting, ok := aiKeyFor[provider]
	if !ok {
		return "", ""
	}
	return s.Secret(keySetting), s.Get(aiModelFor[provider])
}

// NewsNotifyMode decides who is pushed when a story is published.
func (s *Service) NewsNotifyMode() string {
	mode := s.Get(KeyNewsNotifyMode)
	if mapped, ok := legacyNewsNotifyModes[mode]; ok {
		return mapped
	}
	for _, m := range NewsNotifyModes {
		if m == mode {
			return mode
		}
	}
	return defaults[KeyNewsNotifyMode]
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

// CommissionReminderDays is the gap between repeat reminders on an unpaid
// invoice. Floored at one day so a misconfiguration cannot turn the every-minute
// cron into a notification flood.
func (s *Service) CommissionReminderDays() int {
	n, err := strconv.Atoi(s.Get(KeyCommissionReminderDays))
	if err != nil || n < 1 {
		return 2
	}
	return n
}

// CommissionGraceDays is how long after issue an invoice becomes overdue.
func (s *Service) CommissionGraceDays() int {
	n, err := strconv.Atoi(s.Get(KeyCommissionGraceDays))
	if err != nil || n < 0 {
		return 3
	}
	return n
}

// CommissionStartDate is the earliest completed sale that may be charged. The
// zero time means no cutoff.
//
// An unparsable stored value returns the zero time — billing more than intended
// is recoverable by deleting charges, whereas silently billing nothing would
// look like the feature working correctly while collecting nothing.
func (s *Service) CommissionStartDate() time.Time {
	v := s.Get(KeyCommissionStartDate)
	if v == "" || v == CommissionStartAll {
		return time.Time{}
	}
	d, err := time.Parse(commissionStartDateLayout, v)
	if err != nil {
		slog.Error("invalid commission start date stored, billing all history", "value", v)
		return time.Time{}
	}
	return d
}
