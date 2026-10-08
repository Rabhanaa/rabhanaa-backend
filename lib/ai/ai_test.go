package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

var testSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"title", "tags"},
	"properties": map[string]any{
		"title": map[string]any{"type": "string"},
		"tags":  map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
	},
}

var testReq = Request{System: "sys", User: "usr", SchemaName: "article", Schema: testSchema}

// capture records the last request a fake provider received.
type capture struct {
	path    string
	headers http.Header
	body    map[string]any
}

func fakeServer(t *testing.T, c *capture, status int, response string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.path = r.URL.Path
		c.headers = r.Header.Clone()
		c.body = nil
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			json.Unmarshal(b, &c.body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOpenAICompatible(t *testing.T) {
	var c capture
	srv := fakeServer(t, &c, 200, `{"choices":[{"message":{"content":"{\"title\":\"خبر\",\"tags\":[1]}"}}]}`)
	openAIBaseURL, openRouterBaseURL = srv.URL+"/v1", srv.URL+"/api/v1"

	out, err := CompleteJSON(context.Background(), Config{Provider: OpenAI, APIKey: "sk-test", Model: "gpt-x"}, testReq)
	if err != nil || out != `{"title":"خبر","tags":[1]}` {
		t.Fatalf("CompleteJSON = %q, %v", out, err)
	}
	if c.path != "/v1/chat/completions" || c.headers.Get("Authorization") != "Bearer sk-test" {
		t.Errorf("path %q auth %q", c.path, c.headers.Get("Authorization"))
	}
	rf := c.body["response_format"].(map[string]any)
	js := rf["json_schema"].(map[string]any)
	if rf["type"] != "json_schema" || js["strict"] != true || js["name"] != "article" || c.body["model"] != "gpt-x" {
		t.Errorf("unexpected body: %v", c.body)
	}
	msgs := c.body["messages"].([]any)
	if len(msgs) != 2 || msgs[0].(map[string]any)["role"] != "system" {
		t.Errorf("messages = %v", msgs)
	}

	// OpenRouter: same protocol, own base URL and attribution headers.
	_, err = CompleteJSON(context.Background(), Config{Provider: OpenRouter, APIKey: "or-key", Model: "openai/gpt-x", AppURL: "https://rabhanaa.com", AppName: "Rabhanaa"}, testReq)
	if err != nil {
		t.Fatal(err)
	}
	if c.path != "/api/v1/chat/completions" || c.headers.Get("HTTP-Referer") != "https://rabhanaa.com" || c.headers.Get("X-Title") != "Rabhanaa" {
		t.Errorf("openrouter path %q headers %v", c.path, c.headers)
	}
}

func TestProviderErrors(t *testing.T) {
	var c capture
	srv := fakeServer(t, &c, 401, `{"error":{"message":"Incorrect API key provided"}}`)
	openAIBaseURL = srv.URL

	_, err := CompleteJSON(context.Background(), Config{Provider: OpenAI, APIKey: "bad", Model: "m"}, testReq)
	var pe *ProviderError
	if !errors.As(err, &pe) || pe.Status != 401 || pe.Message != "Incorrect API key provided" {
		t.Fatalf("got %v", err)
	}

	if _, err := CompleteJSON(context.Background(), Config{Provider: OpenAI, Model: "m"}, testReq); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no key: got %v", err)
	}
	if _, err := CompleteJSON(context.Background(), Config{Provider: "claude", APIKey: "k", Model: "m"}, testReq); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("unknown provider: got %v", err)
	}

	refusal := fakeServer(t, &c, 200, `{"choices":[{"message":{"content":"","refusal":"cannot help"}}]}`)
	openAIBaseURL = refusal.URL
	if _, err := CompleteJSON(context.Background(), Config{Provider: OpenAI, APIKey: "k", Model: "m"}, testReq); !errors.As(err, &pe) {
		t.Errorf("refusal: got %v", err)
	}

	// OpenRouter can report an upstream failure inside a 200.
	inBand := fakeServer(t, &c, 200, `{"error":{"message":"upstream overloaded"}}`)
	openRouterBaseURL = inBand.URL
	if _, err := CompleteJSON(context.Background(), Config{Provider: OpenRouter, APIKey: "k", Model: "m"}, testReq); !errors.As(err, &pe) || pe.Message != "upstream overloaded" {
		t.Errorf("in-band error: got %v", err)
	}
}

func TestGemini(t *testing.T) {
	var c capture
	srv := fakeServer(t, &c, 200, `{"candidates":[{"content":{"parts":[
		{"text":"thinking about it","thought":true},
		{"text":"{\"title\":\"خبر\",\"tags\":[]}"}
	]},"finishReason":"STOP"}]}`)
	geminiBaseURL = srv.URL + "/v1beta"

	out, err := CompleteJSON(context.Background(), Config{Provider: Gemini, APIKey: "g-key", Model: "models/gemini-x"}, testReq)
	if err != nil || out != `{"title":"خبر","tags":[]}` {
		t.Fatalf("CompleteJSON = %q, %v", out, err)
	}
	if c.path != "/v1beta/models/gemini-x:generateContent" || c.headers.Get("x-goog-api-key") != "g-key" {
		t.Errorf("path %q key %q", c.path, c.headers.Get("x-goog-api-key"))
	}
	gc := c.body["generationConfig"].(map[string]any)
	schema := gc["responseSchema"].(map[string]any)
	if gc["responseMimeType"] != "application/json" || schema["type"] != "OBJECT" {
		t.Errorf("generationConfig = %v", gc)
	}
	if _, ok := schema["additionalProperties"]; ok {
		t.Error("additionalProperties must not reach Gemini")
	}
	items := schema["properties"].(map[string]any)["tags"].(map[string]any)["items"].(map[string]any)
	if items["type"] != "INTEGER" {
		t.Errorf("nested items type = %v", items["type"])
	}

	blocked := fakeServer(t, &c, 200, `{"promptFeedback":{"blockReason":"SAFETY"}}`)
	geminiBaseURL = blocked.URL
	var pe *ProviderError
	if _, err := CompleteJSON(context.Background(), Config{Provider: Gemini, APIKey: "k", Model: "m"}, testReq); !errors.As(err, &pe) {
		t.Errorf("blocked: got %v", err)
	}
}

func TestListModels(t *testing.T) {
	var c capture
	ctx := context.Background()

	oa := fakeServer(t, &c, 200, `{"data":[{"id":"gpt-z"},{"id":"text-embedding-3-small"},{"id":"o4-mini"},{"id":"gpt-z-audio"},{"id":"dall-e-3"},{"id":"gpt-a"}]}`)
	openAIBaseURL = oa.URL
	ids, err := ListModels(ctx, OpenAI, "k")
	if err != nil || !reflect.DeepEqual(ids, []string{"gpt-a", "gpt-z", "o4-mini"}) {
		t.Errorf("openai = %v, %v", ids, err)
	}

	gm := fakeServer(t, &c, 200, `{"models":[
		{"name":"models/gemini-b","supportedGenerationMethods":["generateContent","countTokens"]},
		{"name":"models/embedding-1","supportedGenerationMethods":["embedContent"]},
		{"name":"models/gemini-a","supportedGenerationMethods":["generateContent"]}]}`)
	geminiBaseURL = gm.URL
	ids, err = ListModels(ctx, Gemini, "k")
	if err != nil || !reflect.DeepEqual(ids, []string{"gemini-a", "gemini-b"}) {
		t.Errorf("gemini = %v, %v", ids, err)
	}

	or := fakeServer(t, &c, 200, `{"data":[
		{"id":"vendor/plain","supported_parameters":["temperature"]},
		{"id":"vendor/json","supported_parameters":["response_format"]},
		{"id":"vendor/strict","supported_parameters":["structured_outputs","tools"]}]}`)
	openRouterBaseURL = or.URL
	ids, err = ListModels(ctx, OpenRouter, "")
	if err != nil || !reflect.DeepEqual(ids, []string{"vendor/json", "vendor/strict"}) {
		t.Errorf("openrouter = %v, %v", ids, err)
	}
	if c.headers.Get("Authorization") != "" {
		t.Error("OpenRouter list sent an Authorization header with no key")
	}

	if _, err := ListModels(ctx, OpenAI, ""); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no key: got %v", err)
	}
}

func TestExtractJSON(t *testing.T) {
	cases := map[string]string{
		`{"a":1}`:                     `{"a":1}`,
		"```json\n{\"a\":1}\n```":     `{"a":1}`,
		"Here you go: {\"a\":{}} ok.": `{"a":{}}`,
	}
	for in, want := range cases {
		if got := extractJSON(in); got != want {
			t.Errorf("extractJSON(%q) = %q, want %q", in, got, want)
		}
	}
}
