// Package model holds the wire types for platform commission (#13).
//
// Every money value crosses the wire as a string. These are amounts a seller is
// asked to pay: a float64 in JSON would round in ways neither side controls, and
// the resulting arguments would be about money.
package model

import "time"

// Invoice is one week's bill for one seller.
type Invoice struct {
	PublicID    string     `json:"public_id"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	TotalAmount string     `json:"total_amount"`
	Status      string     `json:"status"`
	IssuedAt    time.Time  `json:"issued_at"`
	DueAt       time.Time  `json:"due_at"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	// Derived from due_at rather than stored, so it stays correct when the grace
	// period setting changes.
	IsOverdue bool `json:"is_overdue"`

	PaymentMethod    string `json:"payment_method,omitempty"`
	PaymentReference string `json:"payment_reference,omitempty"`
	WaivedReason     string `json:"waived_reason,omitempty"`
}

// Charge is the commission on a single completed sale.
type Charge struct {
	PublicID    string    `json:"public_id"`
	DealValue   string    `json:"deal_value"`
	RatePercent string    `json:"rate_percent"`
	Amount      string    `json:"amount"`
	CompletedAt time.Time `json:"completed_at"`
	Invoiced    bool      `json:"invoiced"`
}

// SellerSummary is what a seller sees on their own screen.
type SellerSummary struct {
	// Outstanding is issued and unpaid. Accruing is this period's sales, not yet
	// billed — shown separately so nobody reads it as due today.
	Outstanding  string    `json:"outstanding"`
	Accruing     string    `json:"accruing"`
	OverdueCount int64     `json:"overdue_count"`
	RatePercent  string    `json:"rate_percent"`
	Invoices     []Invoice `json:"invoices"`
}

// SellerBalance is one row of the admin collection worklist.
type SellerBalance struct {
	SellerPublicID string     `json:"seller_public_id"`
	Name           string     `json:"name"`
	Phone          string     `json:"phone"`
	Email          string     `json:"email"`
	AccountStatus  string     `json:"account_status"`
	Outstanding    string     `json:"outstanding"`
	UnpaidInvoices int64      `json:"unpaid_invoices"`
	EarliestDueAt  *time.Time `json:"earliest_due_at,omitempty"`
	IsOverdue      bool       `json:"is_overdue"`
	DaysOverdue    int        `json:"days_overdue"`
}

type SellerBalancesResponse struct {
	Sellers []SellerBalance `json:"sellers"`
	Total   int64           `json:"total"`
	Page    int32           `json:"page"`
	Totals  Totals          `json:"totals"`
}

// Totals is the header figure on the admin screen.
type Totals struct {
	TotalOutstanding string `json:"total_outstanding"`
	TotalOverdue     string `json:"total_overdue"`
	TotalCollected   string `json:"total_collected"`
}

// SellerDetail is one seller's full history, for the admin who is about to phone
// them.
type SellerDetail struct {
	SellerPublicID string    `json:"seller_public_id"`
	Name           string    `json:"name"`
	Phone          string    `json:"phone"`
	Email          string    `json:"email"`
	AccountStatus  string    `json:"account_status"`
	Outstanding    string    `json:"outstanding"`
	Invoices       []Invoice `json:"invoices"`
	Charges        []Charge  `json:"charges"`
}

// PayRequest records a payment the admin collected off-platform.
type PayRequest struct {
	Method    string `json:"method" binding:"required,oneof=vodafone_cash instapay bank_transfer cash other"`
	Reference string `json:"reference" binding:"max=120"`
	Note      string `json:"note" binding:"max=500"`
}

type WaiveRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=500"`
}
