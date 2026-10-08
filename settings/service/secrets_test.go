package service

import (
	"context"
	"errors"
	"testing"
)

func TestSecretsNeverListedAsSettings(t *testing.T) {
	s := NewService(nil, nil)
	all := s.All()
	for key := range secretKeys {
		if _, ok := all[key]; ok {
			t.Errorf("%s is readable through All()", key)
		}
		// Set is the plain-text path; secrets must not be writable through it.
		if err := s.Set(context.Background(), key, "sk-anything", 1); !errors.Is(err, ErrUnknownSetting) {
			t.Errorf("Set(%s) = %v, want ErrUnknownSetting", key, err)
		}
	}
}

func TestSetSecretGuards(t *testing.T) {
	s := NewService(nil, nil)
	ctx := context.Background()
	if err := s.SetSecret(ctx, KeyCommissionRatePercent, "x", 1); !errors.Is(err, ErrUnknownSetting) {
		t.Errorf("non-secret key: got %v", err)
	}
	for _, bad := range []string{"", "   ", "has space", "line\nbreak"} {
		if err := s.SetSecret(ctx, KeyOpenAIAPIKey, bad, 1); !errors.Is(err, ErrInvalidSettingValue) {
			t.Errorf("SetSecret(%q) = %v, want ErrInvalidSettingValue", bad, err)
		}
	}
	// Valid value, but no encryption key configured: refused, not stored in
	// plain text. Returns before touching the database (queries is nil here).
	if err := s.SetSecret(ctx, KeyOpenAIAPIKey, "sk-valid-key", 1); !errors.Is(err, ErrSecretsUnavailable) {
		t.Errorf("no box: got %v, want ErrSecretsUnavailable", err)
	}
	if s.SecretsAvailable() {
		t.Error("SecretsAvailable with no box")
	}
}

func TestSecretHintsMask(t *testing.T) {
	s := NewService(nil, nil)
	s.secrets[KeyOpenAIAPIKey] = "sk-proj-abcdefghijklmnop1234"
	s.secrets[KeyGeminiAPIKey] = "short"
	hints := s.SecretHints()
	if hints[KeyOpenAIAPIKey] != "••••1234" {
		t.Errorf("openai hint = %q", hints[KeyOpenAIAPIKey])
	}
	if hints[KeyGeminiAPIKey] != "••••" {
		t.Errorf("a short key must not be revealed: %q", hints[KeyGeminiAPIKey])
	}
	if hints[KeyOpenRouterAPIKey] != "" {
		t.Errorf("unset key hint = %q", hints[KeyOpenRouterAPIKey])
	}

	key, model := s.AICredentials(AIProviderOpenAI)
	if key != "sk-proj-abcdefghijklmnop1234" || model != "" {
		t.Errorf("AICredentials = %q, %q", key, model)
	}
	if k, m := s.AICredentials("claude"); k != "" || m != "" {
		t.Error("unknown provider returned credentials")
	}
}

func TestAISettingValidators(t *testing.T) {
	for _, ok := range []string{"gpt-4.1-mini", "gemini-2.5-flash", "openai/gpt-4o:extended", "meta-llama/llama-3.3-70b-instruct", "o4-mini"} {
		if !isModelName(ok) {
			t.Errorf("isModelName(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", " gpt", "gpt 4", "../etc", "<script>", string(make([]byte, 200))} {
		if isModelName(bad) {
			t.Errorf("isModelName(%q) = true", bad)
		}
	}
	if !allowed[KeyAIProvider](AIProviderOpenRouter) || allowed[KeyAIProvider]("claude") {
		t.Error("provider validator")
	}
	if !allowed[KeyNewsNotifyMode](NewsNotifyByInterest) || allowed[KeyNewsNotifyMode]("everyone") || allowed[KeyNewsNotifyMode]("all_pro") {
		t.Error("notify mode validator")
	}
	s := NewService(nil, nil)
	if s.AIProvider() != AIProviderOpenAI || s.NewsNotifyMode() != NewsNotifyAll {
		t.Errorf("defaults: provider %q, notify %q", s.AIProvider(), s.NewsNotifyMode())
	}
	// Values from before every mode included non-Pro members still work.
	for stored, want := range map[string]string{"all_pro": NewsNotifyAll, "pro_by_interest": NewsNotifyByInterest, "garbage": NewsNotifyAll} {
		s.cache[KeyNewsNotifyMode] = stored
		if got := s.NewsNotifyMode(); got != want {
			t.Errorf("stored %q read as %q, want %q", stored, got, want)
		}
	}
}
