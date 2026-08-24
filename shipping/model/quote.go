// Package model holds the wire types for carrier accounts and shipping quotes
// (#14).
//
// The carrier-facing job types deliberately carry no price field of any kind.
// That is the point rather than an omission: transport is priced on weight,
// distance and goods type, so a carrier has no need for the value of the cargo,
// and handing a third party the market's pricing cannot be undone.
package model

// CarrierJob is one thing a carrier could move. Both post-stage and order-stage
// jobs collapse into this shape so the carrier's list is one list.
type CarrierJob struct {
	// Kind is "order", "sell_auction" or "buy_request" — which of the three the
	// PublicID refers to, and therefore what the quote will point at.
	Kind         string  `json:"kind"`
	PublicID     string  `json:"public_id"`
	Title        string  `json:"title"`
	InterestName string  `json:"interest_name"`
	Quantity     string  `json:"quantity"`
	Unit         string  `json:"unit"`
	FromRegion   string  `json:"from_region"`
	ToRegion     *string `json:"to_region,omitempty"`
	Deadline     *string `json:"deadline,omitempty"`
	CreatedAt    string  `json:"created_at"`
	// AlreadyQuoted keeps the carrier from re-entering a price the database
	// would reject on the unique index.
	AlreadyQuoted bool `json:"already_quoted"`
}

type CarrierJobsResponse struct {
	Jobs  []CarrierJob `json:"jobs"`
	Total int64        `json:"total"`
	// Stage echoes the admin setting so the client can label the list honestly:
	// a price on a live post is indicative, a price on an order is firm.
	Stage string `json:"stage"`
}

type CreateQuoteRequest struct {
	Price float64 `json:"price" binding:"required,gt=0"`
	Notes *string `json:"notes"`
}

// Quote is the carrier's view of its own offer.
type Quote struct {
	PublicID  string  `json:"public_id"`
	JobKind   string  `json:"job_kind"`
	JobID     string  `json:"job_public_id"`
	JobTitle  string  `json:"job_title"`
	JobRegion string  `json:"job_region"`
	Price     string  `json:"price"`
	Notes     *string `json:"notes,omitempty"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

type QuotesResponse struct {
	Quotes []Quote `json:"quotes"`
	Total  int64   `json:"total"`
}

// MerchantQuote is the merchant's view of a carrier's offer.
//
// Phone is withheld until the merchant accepts. Before that the merchant is
// comparing prices, not arranging a pickup, and publishing every bidder's
// number would let a merchant collect the whole carrier directory by posting
// once and accepting nothing.
type MerchantQuote struct {
	PublicID     string  `json:"public_id"`
	CarrierName  string  `json:"carrier_name"`
	CarrierPhone *string `json:"carrier_phone,omitempty"`
	CarrierLogo  *string `json:"carrier_logo,omitempty"`
	CarrierNotes *string `json:"carrier_notes,omitempty"`
	Price        string  `json:"price"`
	Notes        *string `json:"notes,omitempty"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
}

type MerchantQuotesResponse struct {
	Quotes []MerchantQuote `json:"quotes"`
	// Stage tells the merchant screen whether to render this section at all.
	Stage string `json:"stage"`
}

// CarrierProfile is what a carrier edits about itself.
type CarrierProfile struct {
	Name      string   `json:"name"`
	Phone     string   `json:"phone"`
	LogoURL   *string  `json:"logo_url,omitempty"`
	Notes     *string  `json:"notes,omitempty"`
	RegionIDs []int32  `json:"region_ids"`
	Regions   []string `json:"regions"`
}

type UpdateCarrierProfileRequest struct {
	RegionIDs []int32 `json:"region_ids" binding:"required,min=1"`
	LogoURL   *string `json:"logo_url"`
	Notes     *string `json:"notes"`
}
