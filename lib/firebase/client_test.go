package firebase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// buildLink is what ends up in the push payload as _url, and
// resolveNotificationLink on the frontend returns _url before it consults its
// own switch. That makes this function the deciding side for push, so a
// disagreement with the frontend does not surface as a conflict — it silently
// overrides it, and only for push, while in-app taps keep following the
// frontend. These cases pin the pairs that have already diverged once.
func TestBuildLink(t *testing.T) {
	const base = "https://rabhanaa.com"

	tests := []struct {
		name string
		data map[string]string
		want string
	}{
		{
			// A carrier is not a party to the order it just won. Sending it to
			// the deal returns NOT_ORDER_PARTICIPANT and a blank screen, so the
			// verdict has to land on the carrier's own quote list — matching
			// resolveNotificationLink.
			name: "accepted verdict goes to the carrier's quotes, not the order",
			data: map[string]string{"type": "shipping_quote_accepted", "order_id": "ord-1"},
			want: base + "/carrier/quotes",
		},
		{
			name: "rejected verdict goes to the carrier's quotes",
			data: map[string]string{"type": "shipping_quote_rejected", "order_id": "ord-1"},
			want: base + "/carrier/quotes",
		},
		{
			// The merchant owns the deal and is the only one told a quote
			// arrived, so this one does open it.
			name: "new quote goes to the merchant's order",
			data: map[string]string{"type": "shipping_quote_received", "order_id": "ord-1"},
			want: base + "/orders/ord-1",
		},
		{
			name: "new quote falls back to the sell post when quoting at post stage",
			data: map[string]string{"type": "shipping_quote_received", "auction_id": "auc-1"},
			want: base + "/auctions/sell/auc-1",
		},
		{
			name: "new quote falls back to the buy request",
			data: map[string]string{"type": "shipping_quote_received", "request_id": "req-1"},
			want: base + "/auctions/buy/req-1",
		},
		{
			name: "moderation verdict opens the post",
			data: map[string]string{"type": "post_approved", "auction_id": "auc-1"},
			want: base + "/auctions/sell/auc-1",
		},
		{
			name: "unknown event carries no link",
			data: map[string]string{"type": "something_new", "order_id": "ord-1"},
			want: "",
		},
		{
			name: "missing id carries no link rather than a broken one",
			data: map[string]string{"type": "shipping_quote_received"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{baseURL: base}
			assert.Equal(t, tt.want, c.buildLink(tt.data))
		})
	}
}

// FCM only accepts an HTTPS link, so a local or misconfigured base URL must
// yield no link at all rather than one the push service will reject.
func TestBuildLinkRequiresHTTPS(t *testing.T) {
	data := map[string]string{"type": "shipping_quote_accepted", "order_id": "ord-1"}

	for _, base := range []string{"", "http://localhost:5173"} {
		c := &Client{baseURL: base}
		assert.Empty(t, c.buildLink(data), "base %q", base)
	}
}
