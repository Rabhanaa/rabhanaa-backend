// Package ai calls a text-generation provider for a JSON answer.
//
// Three providers sit behind one call. OpenAI and OpenRouter share a client
// because OpenRouter speaks the OpenAI chat-completions protocol; Gemini has its
// own. Everything provider-specific stays in this package, so switching
// provider is a setting rather than a code change.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	OpenAI     = "openai"
	Gemini     = "gemini"
	OpenRouter = "openrouter"
)

// Base URLs are variables so tests can point them at a local server.
var (
	openAIBaseURL     = "https://api.openai.com/v1"
	openRouterBaseURL = "https://openrouter.ai/api/v1"
	geminiBaseURL     = "https://generativelanguage.googleapis.com/v1beta"
)

var (
	ErrUnknownProvider = errors.New("ai: unknown provider")
	ErrNotConfigured   = errors.New("ai: api key or model not set")
	ErrEmptyResponse   = errors.New("ai: empty response")
)

// ProviderError is a failure reported by the provider itself — a bad key, no
// quota, an unknown model. Its message is worth showing the admin as is.
type ProviderError struct {
	Provider string
	Status   int
	Message  string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s (HTTP %d)", e.Provider, e.Message, e.Status)
}

type Config struct {
	Provider string
	APIKey   string
	Model    string
	// Sent to OpenRouter, which shows it on its dashboard. Optional.
	AppURL  string
	AppName string
}

// Request asks for one JSON object matching Schema, a JSON Schema object.
type Request struct {
	System     string
	User       string
	SchemaName string
	Schema     map[string]any
}

// Generation can take a while on reasoning models, but must finish before the
// 100-second proxy timeout in front of the API, or the admin gets a bare 524
// instead of a readable error.
var httpClient = &http.Client{Timeout: 90 * time.Second}

// CompleteJSON returns the provider's JSON answer as raw text.
func CompleteJSON(ctx context.Context, cfg Config, req Request) (string, error) {
	if cfg.APIKey == "" || cfg.Model == "" {
		return "", ErrNotConfigured
	}
	switch cfg.Provider {
	case OpenAI:
		return chatCompletion(ctx, openAIBaseURL, cfg, req, nil)
	case OpenRouter:
		headers := map[string]string{}
		if cfg.AppURL != "" {
			headers["HTTP-Referer"] = cfg.AppURL
		}
		if cfg.AppName != "" {
			headers["X-Title"] = cfg.AppName
		}
		return chatCompletion(ctx, openRouterBaseURL, cfg, req, headers)
	case Gemini:
		return geminiGenerate(ctx, cfg, req)
	default:
		return "", ErrUnknownProvider
	}
}

// ------------------------------------------------- OpenAI-compatible (OpenAI, OpenRouter)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Refusal string `json:"refusal"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Message string `json:"message"`
}

func chatCompletion(ctx context.Context, baseURL string, cfg Config, req Request, extraHeaders map[string]string) (string, error) {
	// No temperature or token limit: several current models reject one or the
	// other, and the defaults are fine for this.
	body := map[string]any{
		"model": cfg.Model,
		"messages": []chatMessage{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   req.SchemaName,
				"strict": true,
				"schema": req.Schema,
			},
		},
	}
	headers := map[string]string{"Authorization": "Bearer " + cfg.APIKey}
	for k, v := range extraHeaders {
		headers[k] = v
	}

	var out chatResponse
	if err := postJSON(ctx, cfg.Provider, baseURL+"/chat/completions", headers, body, &out); err != nil {
		return "", err
	}
	// OpenRouter reports some upstream failures inside a 200.
	if out.Error != nil && out.Error.Message != "" {
		return "", &ProviderError{Provider: cfg.Provider, Status: http.StatusOK, Message: out.Error.Message}
	}
	if len(out.Choices) == 0 {
		return "", ErrEmptyResponse
	}
	msg := out.Choices[0].Message
	if msg.Refusal != "" {
		return "", &ProviderError{Provider: cfg.Provider, Status: http.StatusOK, Message: "refused: " + msg.Refusal}
	}
	if strings.TrimSpace(msg.Content) == "" {
		return "", ErrEmptyResponse
	}
	return extractJSON(msg.Content), nil
}

// ------------------------------------------------------------------ Gemini

type geminiPart struct {
	Text    string `json:"text,omitempty"`
	Thought bool   `json:"thought,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

func geminiModelPath(model string) string {
	return "models/" + strings.TrimPrefix(model, "models/")
}

func geminiGenerate(ctx context.Context, cfg Config, req Request) (string, error) {
	body := map[string]any{
		"systemInstruction": geminiContent{Parts: []geminiPart{{Text: req.System}}},
		"contents":          []geminiContent{{Role: "user", Parts: []geminiPart{{Text: req.User}}}},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"responseSchema":   toGeminiSchema(req.Schema),
		},
	}
	url := geminiBaseURL + "/" + geminiModelPath(cfg.Model) + ":generateContent"

	var out geminiResponse
	if err := postJSON(ctx, cfg.Provider, url, map[string]string{"x-goog-api-key": cfg.APIKey}, body, &out); err != nil {
		return "", err
	}
	if out.PromptFeedback != nil && out.PromptFeedback.BlockReason != "" {
		return "", &ProviderError{Provider: Gemini, Status: http.StatusOK, Message: "blocked: " + out.PromptFeedback.BlockReason}
	}
	if len(out.Candidates) == 0 {
		return "", ErrEmptyResponse
	}
	cand := out.Candidates[0]
	var text strings.Builder
	for _, p := range cand.Content.Parts {
		// Thinking models return their reasoning as separate parts.
		if !p.Thought {
			text.WriteString(p.Text)
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		if cand.FinishReason != "" && cand.FinishReason != "STOP" {
			return "", &ProviderError{Provider: Gemini, Status: http.StatusOK, Message: "stopped: " + cand.FinishReason}
		}
		return "", ErrEmptyResponse
	}
	return extractJSON(text.String()), nil
}

// toGeminiSchema converts a JSON Schema to the OpenAPI subset Gemini's
// responseSchema accepts: upper-case type names, no additionalProperties.
func toGeminiSchema(schema map[string]any) map[string]any {
	out := make(map[string]any, len(schema))
	for k, v := range schema {
		switch k {
		case "additionalProperties":
			continue
		case "type":
			if s, ok := v.(string); ok {
				out[k] = strings.ToUpper(s)
				continue
			}
		case "properties":
			if props, ok := v.(map[string]any); ok {
				converted := make(map[string]any, len(props))
				for name, p := range props {
					if pm, ok := p.(map[string]any); ok {
						converted[name] = toGeminiSchema(pm)
					}
				}
				out[k] = converted
				continue
			}
		case "items":
			if im, ok := v.(map[string]any); ok {
				out[k] = toGeminiSchema(im)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// ------------------------------------------------------------------ models

// ListModels returns the model ids an admin can pick from, using the given
// key. OpenRouter's list is public and is narrowed to models that support
// structured output, which the news writer relies on.
func ListModels(ctx context.Context, provider, apiKey string) ([]string, error) {
	switch provider {
	case OpenAI:
		if apiKey == "" {
			return nil, ErrNotConfigured
		}
		var out struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := getJSON(ctx, OpenAI, openAIBaseURL+"/models", map[string]string{"Authorization": "Bearer " + apiKey}, &out); err != nil {
			return nil, err
		}
		var ids []string
		for _, m := range out.Data {
			if isOpenAIChatModel(m.ID) {
				ids = append(ids, m.ID)
			}
		}
		sort.Strings(ids)
		return ids, nil

	case Gemini:
		if apiKey == "" {
			return nil, ErrNotConfigured
		}
		var ids []string
		pageToken := ""
		for page := 0; page < 5; page++ {
			url := geminiBaseURL + "/models?pageSize=1000"
			if pageToken != "" {
				url += "&pageToken=" + pageToken
			}
			var out struct {
				Models []struct {
					Name                       string   `json:"name"`
					SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
				} `json:"models"`
				NextPageToken string `json:"nextPageToken"`
			}
			if err := getJSON(ctx, Gemini, url, map[string]string{"x-goog-api-key": apiKey}, &out); err != nil {
				return nil, err
			}
			for _, m := range out.Models {
				for _, method := range m.SupportedGenerationMethods {
					if method == "generateContent" {
						ids = append(ids, strings.TrimPrefix(m.Name, "models/"))
						break
					}
				}
			}
			if out.NextPageToken == "" {
				break
			}
			pageToken = out.NextPageToken
		}
		sort.Strings(ids)
		return ids, nil

	case OpenRouter:
		var out struct {
			Data []struct {
				ID                  string   `json:"id"`
				SupportedParameters []string `json:"supported_parameters"`
			} `json:"data"`
		}
		headers := map[string]string{}
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
		if err := getJSON(ctx, OpenRouter, openRouterBaseURL+"/models", headers, &out); err != nil {
			return nil, err
		}
		var structured, all []string
		for _, m := range out.Data {
			all = append(all, m.ID)
			for _, p := range m.SupportedParameters {
				if p == "structured_outputs" || p == "response_format" {
					structured = append(structured, m.ID)
					break
				}
			}
		}
		ids := structured
		if len(ids) == 0 {
			ids = all
		}
		sort.Strings(ids)
		return ids, nil
	}
	return nil, ErrUnknownProvider
}

// The models endpoint also lists embedding, audio, image and moderation
// models, none of which can write an article.
func isOpenAIChatModel(id string) bool {
	if !(strings.HasPrefix(id, "gpt-") || strings.HasPrefix(id, "chatgpt-") ||
		(len(id) > 1 && id[0] == 'o' && id[1] >= '0' && id[1] <= '9')) {
		return false
	}
	for _, skip := range []string{"audio", "realtime", "tts", "transcribe", "image", "embedding", "moderation", "search"} {
		if strings.Contains(id, skip) {
			return false
		}
	}
	return true
}

// ------------------------------------------------------------------ http

func postJSON(ctx context.Context, provider, url string, headers map[string]string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return do(provider, req, headers, out)
}

func getJSON(ctx context.Context, provider, url string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return do(provider, req, headers, out)
}

func do(provider string, req *http.Request, headers map[string]string, out any) error {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ai: %s request failed: %w", provider, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("ai: %s read failed: %w", provider, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &ProviderError{Provider: provider, Status: resp.StatusCode, Message: errorMessage(data)}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("ai: %s returned invalid JSON: %w", provider, err)
	}
	return nil
}

// errorMessage pulls the human-readable part out of the three providers' error
// bodies, which all nest it as error.message.
func errorMessage(body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 300 {
		msg = msg[:300]
	}
	if msg == "" {
		return "no details"
	}
	return msg
}

// extractJSON tolerates models that wrap their JSON in a markdown fence or a
// sentence, which happens on OpenRouter models without structured output.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		return s
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
