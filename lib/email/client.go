// Package email sends transactional mail through Resend.
//
// One HTTP call is not worth a dependency, so this talks to the REST API
// directly with net/http, following the same shape as lib/firebase and
// lib/minio: a client that degrades to disabled rather than refusing to start.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const resendEndpoint = "https://api.resend.com/emails"

type Config struct {
	APIKey   string
	From     string
	FromName string
}

type Client struct {
	http     *http.Client
	apiKey   string
	from     string
	fromName string
	enabled  bool
}

// NewClient returns a disabled client when no API key is configured, so the
// server still boots without mail credentials. Callers check Enabled() when
// they need to know whether anything was actually delivered.
func NewClient(cfg Config) *Client {
	c := &Client{
		http:     &http.Client{Timeout: 10 * time.Second},
		apiKey:   cfg.APIKey,
		from:     cfg.From,
		fromName: cfg.FromName,
		enabled:  cfg.APIKey != "",
	}
	if !c.enabled {
		slog.Warn("email disabled", "reason", "RESEND_API_KEY not set")
	}
	return c
}

func (c *Client) Enabled() bool { return c.enabled }

type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

type sendResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Name    string `json:"name"`
}

// Send delivers one HTML email. It returns an error the caller may choose to
// swallow — a failed reset email must not reveal anything to the requester.
func (c *Client) Send(ctx context.Context, to, subject, html string) error {
	if !c.enabled {
		return fmt.Errorf("email: client disabled")
	}

	from := c.from
	if c.fromName != "" {
		from = fmt.Sprintf("%s <%s>", c.fromName, c.from)
	}

	body, err := json.Marshal(sendRequest{From: from, To: []string{to}, Subject: subject, HTML: html})
	if err != nil {
		return fmt.Errorf("email: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("email: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("email: send: %w", err)
	}
	defer resp.Body.Close()

	var parsed sendResponse
	_ = json.NewDecoder(resp.Body).Decode(&parsed)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Resend reports a bad key, an unverified sending domain, or a rejected
		// recipient here — the three things that actually go wrong in practice.
		return fmt.Errorf("email: resend returned %d: %s %s", resp.StatusCode, parsed.Name, parsed.Message)
	}

	slog.Info("email sent", "id", parsed.ID, "subject", subject)
	return nil
}
