// Package service implements the platform's commission on completed sales (#13).
//
// This is a ledger and a collection workflow, not a payment system. No money
// moves through the platform: charges accrue as sales complete, a weekly job
// bundles them into invoices, and an admin records what a seller paid over
// Vodafone Cash or InstaPay. Blocking a non-payer reuses the existing account
// suspension and stays an admin decision.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	commissionModel "rabhana/commission/model"
	"rabhana/db/sqlc"
	notifModel "rabhana/notification/model"
	"rabhana/pkg/errs"
	settingsSvc "rabhana/settings/service"
)

// How many orders one accrual tick will charge. The cron runs every minute, so a
// backlog drains quickly; the cap only stops a single tick from holding the
// database for an unbounded time after a long outage.
const accrualBatchSize = 500

// Invoices are dated in Cairo time — the server runs UTC, and "which week did
// this sale fall in" has to match what the client would say.
const billingTimezone = "Africa/Cairo"

type NotificationSender interface {
	Send(ctx context.Context, userID int32, event notifModel.EventType, data map[string]string)
}

type Service struct {
	queries  *sqlc.Queries
	pool     *pgxpool.Pool
	settings *settingsSvc.Service
	notifier NotificationSender

	// Sales by the seeder are not real and must never be billed. Passed in
	// rather than redeclared so there is one definition of who the seeder is.
	seedUserEmail string
}

func NewService(queries *sqlc.Queries, pool *pgxpool.Pool, settings *settingsSvc.Service, notifier NotificationSender, seedUserEmail string) *Service {
	return &Service{queries: queries, pool: pool, settings: settings, notifier: notifier, seedUserEmail: seedUserEmail}
}

// ComputeCharge returns the deal value and the commission owed on it.
//
// Exported because this is the arithmetic that must not be wrong, and a pure
// function is the only part of this feature that can be tested exhaustively.
// final_price is per unit, so the base is price x quantity — the deal value.
// Shipping is excluded: that is the carrier's money, not the seller's revenue.
func ComputeCharge(unitPrice, quantity, ratePercent decimal.Decimal) (dealValue, amount decimal.Decimal) {
	dealValue = unitPrice.Mul(quantity).Round(2)
	amount = dealValue.Mul(ratePercent).Div(decimal.NewFromInt(100)).Round(2)
	return dealValue, amount
}

// ---------------------------------------------------------------- accrual

// AccrueCharges writes a commission row for every completed order that does not
// have one yet, and returns how many it wrote.
//
// Deliberately derived from the orders table rather than hooked into the
// confirmation handler: ConfirmOrderAsSeller/AsBuyer are conditional updates
// declared :exec, so the caller cannot tell whether the transition actually
// applied. Billing on "the call returned no error" would charge for orders that
// were never completed. Reading the table instead is idempotent (order_id is
// UNIQUE), self-healing after downtime, and auditable.
func (s *Service) AccrueCharges(ctx context.Context) (int, error) {
	orders, err := s.queries.ListCompletedOrdersWithoutCharge(ctx, sqlc.ListCompletedOrdersWithoutChargeParams{
		SeedEmail: s.seedUserEmail,
		Limit:     accrualBatchSize,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list uncharged orders: %w", err)
	}
	if len(orders) == 0 {
		return 0, nil
	}

	// Read once per batch, then snapshot onto each row: an admin changing the
	// rate mid-batch must not split one tick across two rates.
	rate := s.settings.CommissionRate()

	written := 0
	for _, o := range orders {
		dealValue, amount := ComputeCharge(numericToDecimal(o.FinalPrice), numericToDecimal(o.Quantity), rate)

		_, err := s.queries.CreateCommissionCharge(ctx, sqlc.CreateCommissionChargeParams{
			OrderID:          o.ID,
			SellerID:         o.SellerID,
			DealValue:        mustNumeric(dealValue),
			RatePercent:      mustNumeric(rate),
			Amount:           mustNumeric(amount),
			OrderCompletedAt: o.CompletedAt,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING — another tick got there first.
			continue
		}
		if err != nil {
			// One bad order must not stop the rest of the batch; the next tick
			// retries it because no charge row was written.
			slog.Error("failed to create commission charge", "order_id", o.ID, "error", err)
			continue
		}
		written++
	}

	if written > 0 {
		slog.Info("commission charges accrued", "count", written)
	}
	return written, nil
}

// ---------------------------------------------------------------- invoicing

// IssueWeeklyInvoices bundles the previous week's charges into one invoice per
// seller. It is a no-op on every day except the configured closing weekday, so
// the cron can call it every minute.
func (s *Service) IssueWeeklyInvoices(ctx context.Context, now time.Time) (int, error) {
	loc, err := time.LoadLocation(billingTimezone)
	if err != nil {
		// Alpine images need tzdata; the Dockerfile installs it. Falling back to
		// UTC would silently shift every period boundary by a couple of hours.
		return 0, fmt.Errorf("failed to load %s: %w", billingTimezone, err)
	}

	local := now.In(loc)
	if local.Weekday() != s.settings.CommissionWeekCloseDay() {
		return 0, nil
	}

	// The week that just ended: [today 00:00 - 7d, today 00:00) in Cairo.
	periodEnd := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	periodStart := periodEnd.AddDate(0, 0, -7)
	dueAt := now.AddDate(0, 0, s.settings.CommissionGraceDays())

	sellers, err := s.queries.ListSellersWithUninvoicedCharges(ctx, sqlc.ListSellersWithUninvoicedChargesParams{
		PeriodStart: pgtype.Timestamptz{Time: periodStart, Valid: true},
		PeriodEnd:   pgtype.Timestamptz{Time: periodEnd, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list sellers with uninvoiced charges: %w", err)
	}

	issued := 0
	for _, seller := range sellers {
		invoice, err := s.issueOne(ctx, seller, periodStart, periodEnd, dueAt)
		if err != nil {
			slog.Error("failed to issue commission invoice", "seller_id", seller.SellerID, "error", err)
			continue
		}
		if invoice == nil {
			continue // already issued for this period
		}
		issued++

		s.notifier.Send(ctx, seller.SellerID, notifModel.EventCommissionInvoiceIssued, map[string]string{
			"invoice_id": invoice.PublicID.String(),
			"amount":     numericString(invoice.TotalAmount),
		})
	}

	if issued > 0 {
		slog.Info("commission invoices issued", "count", issued, "period_start", periodStart)
	}
	return issued, nil
}

// issueOne creates the invoice and attaches its charges in one transaction.
//
// This is the only place in the codebase that needs one. If the invoice were
// committed and the attach then failed, those charges would stay unattached and
// be billed again next week — the seller pays twice for the same sale, and the
// error is invisible until they complain.
func (s *Service) issueOne(ctx context.Context, seller sqlc.ListSellersWithUninvoicedChargesRow, periodStart, periodEnd time.Time, dueAt time.Time) (*sqlc.CommissionInvoice, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	qtx := s.queries.WithTx(tx)

	invoice, err := qtx.CreateCommissionInvoice(ctx, sqlc.CreateCommissionInvoiceParams{
		SellerID:    seller.SellerID,
		PeriodStart: pgtype.Timestamptz{Time: periodStart, Valid: true},
		PeriodEnd:   pgtype.Timestamptz{Time: periodEnd, Valid: true},
		TotalAmount: seller.Total,
		DueAt:       pgtype.Timestamptz{Time: dueAt, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // UNIQUE(seller_id, period_start) — already issued
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create invoice: %w", err)
	}

	attached, err := qtx.AttachChargesToInvoice(ctx, sqlc.AttachChargesToInvoiceParams{
		InvoiceID:   invoice.ID,
		SellerID:    seller.SellerID,
		PeriodStart: pgtype.Timestamptz{Time: periodStart, Valid: true},
		PeriodEnd:   pgtype.Timestamptz{Time: periodEnd, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach charges: %w", err)
	}
	if attached != seller.ChargeCount {
		// The set moved between the two statements. Roll back rather than issue
		// an invoice whose total does not match the sales behind it.
		return nil, fmt.Errorf("charge count mismatch: expected %d, attached %d", seller.ChargeCount, attached)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit invoice: %w", err)
	}
	return &invoice, nil
}

// ---------------------------------------------------------------- seller side

func (s *Service) GetSellerSummary(ctx context.Context, sellerID int32, limit, offset int32) (*commissionModel.SellerSummary, error) {
	totals, err := s.queries.GetSellerCommissionSummary(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to load commission summary: %w", err)
	}

	rows, err := s.queries.ListInvoicesBySeller(ctx, sqlc.ListInvoicesBySellerParams{
		SellerID: sellerID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list invoices: %w", err)
	}

	out := &commissionModel.SellerSummary{
		Outstanding:  numericString(totals.Outstanding),
		Accruing:     numericString(totals.Accruing),
		OverdueCount: totals.OverdueCount,
		RatePercent:  s.settings.CommissionRate().String(),
		Invoices:     make([]commissionModel.Invoice, 0, len(rows)),
	}
	for _, r := range rows {
		out.Invoices = append(out.Invoices, toInvoice(r))
	}
	return out, nil
}

func (s *Service) ListSellerCharges(ctx context.Context, sellerID int32, limit, offset int32) ([]commissionModel.Charge, error) {
	rows, err := s.queries.ListChargesBySeller(ctx, sqlc.ListChargesBySellerParams{
		SellerID: sellerID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list charges: %w", err)
	}
	out := make([]commissionModel.Charge, 0, len(rows))
	for _, r := range rows {
		out = append(out, toCharge(r))
	}
	return out, nil
}

// ---------------------------------------------------------------- admin side

func (s *Service) ListBalances(ctx context.Context, overdueOnly bool, limit, offset int32) (*commissionModel.SellerBalancesResponse, error) {
	rows, err := s.queries.ListSellerBalances(ctx, sqlc.ListSellerBalancesParams{
		OverdueOnly: overdueOnly, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list balances: %w", err)
	}
	total, err := s.queries.CountSellerBalances(ctx, overdueOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to count balances: %w", err)
	}
	totals, err := s.queries.GetCommissionTotals(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load totals: %w", err)
	}

	out := &commissionModel.SellerBalancesResponse{
		Sellers: make([]commissionModel.SellerBalance, 0, len(rows)),
		Total:   total,
		Totals: commissionModel.Totals{
			TotalOutstanding: numericString(totals.TotalOutstanding),
			TotalOverdue:     numericString(totals.TotalOverdue),
			TotalCollected:   numericString(totals.TotalCollected),
		},
	}
	now := time.Now()
	for _, r := range rows {
		b := commissionModel.SellerBalance{
			SellerPublicID: r.PublicID.String(),
			Name:           r.Name,
			Phone:          r.Phone.String,
			Email:          r.Email,
			AccountStatus:  r.Status,
			Outstanding:    numericString(r.Outstanding),
			UnpaidInvoices: r.UnpaidInvoices,
			IsOverdue:      r.IsOverdue,
		}
		if r.EarliestDueAt.Valid {
			due := r.EarliestDueAt.Time
			b.EarliestDueAt = &due
			if r.IsOverdue {
				b.DaysOverdue = int(now.Sub(due).Hours() / 24)
			}
		}
		out.Sellers = append(out.Sellers, b)
	}
	return out, nil
}

// MarkPaid records a payment the admin collected off-platform. It deliberately
// does not reactivate a suspended account: unblocking stays a separate, explicit
// admin decision.
func (s *Service) MarkPaid(ctx context.Context, invoicePublicID uuid.UUID, adminID int32, req commissionModel.PayRequest) error {
	invoice, err := s.queries.GetInvoiceByPublicID(ctx, pgtype.UUID{Bytes: invoicePublicID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrInvoiceNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to load invoice: %w", err)
	}

	rows, err := s.queries.MarkInvoicePaid(ctx, sqlc.MarkInvoicePaidParams{
		ID:                invoice.ID,
		PaymentMethod:     pgtype.Text{String: req.Method, Valid: true},
		PaymentReference:  pgtype.Text{String: req.Reference, Valid: req.Reference != ""},
		PaymentNote:       pgtype.Text{String: req.Note, Valid: req.Note != ""},
		RecordedByAdminID: pgtype.Int4{Int32: adminID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to mark invoice paid: %w", err)
	}
	if rows == 0 {
		// The status guard rejected it — already paid or waived.
		return errs.ErrInvoiceNotPayable
	}

	slog.Info("commission invoice paid", "invoice_id", invoice.ID, "method", req.Method, "admin_id", adminID)
	return nil
}

func (s *Service) Waive(ctx context.Context, invoicePublicID uuid.UUID, adminID int32, reason string) error {
	invoice, err := s.queries.GetInvoiceByPublicID(ctx, pgtype.UUID{Bytes: invoicePublicID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrInvoiceNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to load invoice: %w", err)
	}

	rows, err := s.queries.WaiveInvoice(ctx, sqlc.WaiveInvoiceParams{
		ID:                invoice.ID,
		WaivedReason:      pgtype.Text{String: reason, Valid: true},
		RecordedByAdminID: pgtype.Int4{Int32: adminID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to waive invoice: %w", err)
	}
	if rows == 0 {
		return errs.ErrInvoiceNotPayable
	}

	slog.Info("commission invoice waived", "invoice_id", invoice.ID, "admin_id", adminID, "reason", reason)
	return nil
}

// ---------------------------------------------------------------- conversions

func numericToDecimal(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid || n.Int == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(new(big.Int).Set(n.Int), n.Exp)
}

// mustNumeric converts through the fixed-point string form rather than the
// coefficient, which is what the rest of the codebase does and what pgx parses
// most predictably.
func mustNumeric(d decimal.Decimal) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(d.StringFixed(2)); err != nil {
		slog.Error("failed to convert decimal for storage", "value", d.String(), "error", err)
		return pgtype.Numeric{Valid: false}
	}
	return n
}

func numericString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0.00"
	}
	return numericToDecimal(n).StringFixed(2)
}

func toInvoice(r sqlc.CommissionInvoice) commissionModel.Invoice {
	inv := commissionModel.Invoice{
		PublicID:         r.PublicID.String(),
		PeriodStart:      r.PeriodStart.Time,
		PeriodEnd:        r.PeriodEnd.Time,
		TotalAmount:      numericString(r.TotalAmount),
		Status:           r.Status,
		IssuedAt:         r.IssuedAt.Time,
		DueAt:            r.DueAt.Time,
		IsOverdue:        r.Status == "unpaid" && r.DueAt.Valid && r.DueAt.Time.Before(time.Now()),
		PaymentMethod:    r.PaymentMethod.String,
		PaymentReference: r.PaymentReference.String,
		WaivedReason:     r.WaivedReason.String,
	}
	if r.PaidAt.Valid {
		paid := r.PaidAt.Time
		inv.PaidAt = &paid
	}
	return inv
}

func toCharge(r sqlc.CommissionCharge) commissionModel.Charge {
	return commissionModel.Charge{
		PublicID:    r.PublicID.String(),
		DealValue:   numericString(r.DealValue),
		RatePercent: numericToDecimal(r.RatePercent).String(),
		Amount:      numericString(r.Amount),
		CompletedAt: r.OrderCompletedAt.Time,
		Invoiced:    r.InvoiceID.Valid,
	}
}

// GetSellerDetail is what an admin reads before phoning a seller: the unpaid
// invoices to settle and the sales behind them.
func (s *Service) GetSellerDetail(ctx context.Context, sellerPublicID uuid.UUID) (*commissionModel.SellerDetail, error) {
	user, err := s.queries.GetUserByPublicID(ctx, pgtype.UUID{Bytes: sellerPublicID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load seller: %w", err)
	}

	totals, err := s.queries.GetSellerCommissionSummary(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load seller totals: %w", err)
	}

	invoiceRows, err := s.queries.ListInvoicesBySeller(ctx, sqlc.ListInvoicesBySellerParams{
		SellerID: user.ID, Limit: 100, Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list seller invoices: %w", err)
	}

	chargeRows, err := s.queries.ListChargesBySeller(ctx, sqlc.ListChargesBySellerParams{
		SellerID: user.ID, Limit: 100, Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list seller charges: %w", err)
	}

	out := &commissionModel.SellerDetail{
		SellerPublicID: user.PublicID.String(),
		Name:           user.Name,
		Phone:          user.Phone.String,
		Email:          user.Email,
		AccountStatus:  user.Status,
		Outstanding:    numericString(totals.Outstanding),
		Invoices:       make([]commissionModel.Invoice, 0, len(invoiceRows)),
		Charges:        make([]commissionModel.Charge, 0, len(chargeRows)),
	}
	for _, r := range invoiceRows {
		out.Invoices = append(out.Invoices, toInvoice(r))
	}
	for _, r := range chargeRows {
		out.Charges = append(out.Charges, toCharge(r))
	}
	return out, nil
}
