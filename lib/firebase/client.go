package firebase

import (
	"context"
	"log/slog"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type Client struct {
	messaging *messaging.Client
	enabled   bool
	baseURL   string
}

// NewClient initializes a Firebase client.
// credentialsFile: path to a service account JSON file (used when FIREBASE_CREDENTIALS_PATH is set).
// credentialsJSON: raw JSON content of the service account (used when FIREBASE_CREDENTIALS_JSON is set).
// baseURL: the app's public HTTPS base URL used to build deep-link URLs in WebpushFCMOptions.
// If both credentials are empty, returns a disabled client (push notifications silently skipped).
func NewClient(ctx context.Context, credentialsFile, credentialsJSON, baseURL string) (*Client, error) {
	var opt option.ClientOption
	switch {
	case credentialsJSON != "":
		opt = option.WithCredentialsJSON([]byte(credentialsJSON))
	case credentialsFile != "":
		opt = option.WithCredentialsFile(credentialsFile)
	default:
		return &Client{enabled: false, baseURL: baseURL}, nil
	}

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, err
	}

	mc, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	slog.Info("Firebase client initialized")
	return &Client{messaging: mc, enabled: true, baseURL: baseURL}, nil
}

// buildLink returns a deep-link URL based on the notification type in data, or "" if no link applies.
func (c *Client) buildLink(data map[string]string) string {
	if c.baseURL == "" || !strings.HasPrefix(c.baseURL, "https://") {
		return ""
	}
	switch data["type"] {
	case "new_sell_auction", "auction_ended", "new_bid", "auction_motivation":
		if id := data["auction_id"]; id != "" {
			return c.baseURL + "/auctions/sell/" + id
		}
	case "new_buy_request", "request_ended", "new_offer", "request_motivation":
		if id := data["request_id"]; id != "" {
			return c.baseURL + "/auctions/buy/" + id
		}
	case "selection_expiring":
		if id := data["auction_id"]; id != "" {
			return c.baseURL + "/auctions/sell/" + id
		}
		if id := data["request_id"]; id != "" {
			return c.baseURL + "/auctions/buy/" + id
		}
	// Shipping quotes (#14). Only the merchant owns the deal, so only the
	// merchant is sent to it.
	case "shipping_quote_received":
		if id := data["order_id"]; id != "" {
			return c.baseURL + "/orders/" + id
		}
		if id := data["auction_id"]; id != "" {
			return c.baseURL + "/auctions/sell/" + id
		}
		if id := data["request_id"]; id != "" {
			return c.baseURL + "/auctions/buy/" + id
		}
	// A verdict only ever goes to a carrier, and a carrier is not a party to the
	// order — sending it to the deal hands it NOT_ORDER_PARTICIPANT and a blank
	// screen. Its own quote list is where the answer lives.
	//
	// This has to agree with resolveNotificationLink in the frontend, and it is
	// the side that wins: the link is copied into data as _url, which that
	// function returns before it ever reaches its own switch. A disagreement
	// here silently overrides the frontend for push, while in-app taps (whose
	// stored rows carry no _url) still follow the frontend — the two paths then
	// go to different places for the same notification.
	case "shipping_quote_accepted", "shipping_quote_rejected":
		return c.baseURL + "/carrier/quotes"
	// Moderation verdicts (#18) — either kind of post, so try both ids.
	case "post_approved", "post_rejected", "post_suspended":
		if id := data["auction_id"]; id != "" {
			return c.baseURL + "/auctions/sell/" + id
		}
		if id := data["request_id"]; id != "" {
			return c.baseURL + "/auctions/buy/" + id
		}
	case "new_order", "order_confirmed":
		if id := data["order_id"]; id != "" {
			return c.baseURL + "/orders/" + id
		}
	}
	return ""
}

func (c *Client) SendNotification(ctx context.Context, token, title, body string, data map[string]string) (string, error) {
	if !c.enabled {
		return "", nil
	}

	combined := make(map[string]string, len(data)+2)
	for k, v := range data {
		combined[k] = v
	}
	combined["_title"] = title
	combined["_body"] = body
	linkURL := c.buildLink(data)
	if linkURL != "" {
		combined["_url"] = linkURL
	}

	var fcmOpts *messaging.WebpushFCMOptions
	if linkURL != "" {
		fcmOpts = &messaging.WebpushFCMOptions{Link: linkURL}
	}

	msg := &messaging.Message{
		Token: token,
		Data:  combined,
		Webpush: &messaging.WebpushConfig{
			Headers:    map[string]string{"Urgency": "high", "TTL": "86400"},
			FCMOptions: fcmOpts,
		},
		Android: &messaging.AndroidConfig{Priority: "high"},
		APNS:    &messaging.APNSConfig{Headers: map[string]string{"apns-priority": "10"}},
	}

	return c.messaging.Send(ctx, msg)
}

func (c *Client) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) (*messaging.BatchResponse, error) {
	if !c.enabled {
		return nil, nil
	}

	combined := make(map[string]string, len(data)+2)
	for k, v := range data {
		combined[k] = v
	}
	combined["_title"] = title
	combined["_body"] = body
	linkURL := c.buildLink(data)
	if linkURL != "" {
		combined["_url"] = linkURL
	}

	var fcmOpts *messaging.WebpushFCMOptions
	if linkURL != "" {
		fcmOpts = &messaging.WebpushFCMOptions{Link: linkURL}
	}

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Data:   combined,
		Webpush: &messaging.WebpushConfig{
			Headers:    map[string]string{"Urgency": "high", "TTL": "86400"},
			FCMOptions: fcmOpts,
		},
		Android: &messaging.AndroidConfig{Priority: "high"},
		APNS:    &messaging.APNSConfig{Headers: map[string]string{"apns-priority": "10"}},
	}

	return c.messaging.SendEachForMulticast(ctx, msg)
}
