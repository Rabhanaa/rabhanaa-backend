-- Platform commission (#13). See migration 045 for the shape and the reasoning.

-- Accrual. Charges are derived from orders rather than written by the
-- confirmation handler: ConfirmOrderAsSeller/AsBuyer are conditional updates
-- declared :exec, so the service cannot tell whether the transition actually
-- applied, and accruing there would bill orders that never completed. Running
-- this every minute is idempotent because commission_charges.order_id is UNIQUE.
-- name: ListCompletedOrdersWithoutCharge :many
SELECT o.id, o.seller_id, o.final_price, o.quantity, o.completed_at
FROM orders o
JOIN users s ON s.id = o.seller_id
LEFT JOIN commission_charges c ON c.order_id = o.id
WHERE o.status = 'completed'
  AND o.completed_at IS NOT NULL
  AND c.id IS NULL
  -- Production is almost entirely seeded; billing the seeder would bury every
  -- real debt under fiction.
  AND s.email <> @seed_email::text
ORDER BY o.completed_at
LIMIT $1;

-- name: CreateCommissionCharge :one
INSERT INTO commission_charges (
    order_id, seller_id, deal_value, rate_percent, amount, order_completed_at
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (order_id) DO NOTHING
RETURNING *;

-- Invoicing.
-- name: ListSellersWithUninvoicedCharges :many
SELECT seller_id, SUM(amount)::DECIMAL(14,2) AS total, COUNT(*) AS charge_count
FROM commission_charges
WHERE invoice_id IS NULL
  AND order_completed_at >= @period_start::timestamptz
  AND order_completed_at <  @period_end::timestamptz
GROUP BY seller_id
ORDER BY seller_id;

-- name: CreateCommissionInvoice :one
INSERT INTO commission_invoices (
    seller_id, period_start, period_end, total_amount, due_at
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (seller_id, period_start) DO NOTHING
RETURNING *;

-- name: AttachChargesToInvoice :execrows
UPDATE commission_charges
SET invoice_id = @invoice_id::int
WHERE invoice_id IS NULL
  AND seller_id = @seller_id::int
  AND order_completed_at >= @period_start::timestamptz
  AND order_completed_at <  @period_end::timestamptz;

-- Seller-facing.
-- name: GetSellerCommissionSummary :one
SELECT
    COALESCE((SELECT SUM(total_amount) FROM commission_invoices
              WHERE seller_id = @seller_id::int AND status = 'unpaid'), 0)::DECIMAL(14,2) AS outstanding,
    COALESCE((SELECT SUM(amount) FROM commission_charges
              WHERE seller_id = @seller_id::int AND invoice_id IS NULL), 0)::DECIMAL(14,2) AS accruing,
    (SELECT COUNT(*) FROM commission_invoices
     WHERE seller_id = @seller_id::int AND status = 'unpaid' AND due_at < NOW()) AS overdue_count;

-- name: ListInvoicesBySeller :many
SELECT * FROM commission_invoices
WHERE seller_id = $1
ORDER BY period_start DESC
LIMIT $2 OFFSET $3;

-- name: ListChargesBySeller :many
SELECT * FROM commission_charges
WHERE seller_id = $1
ORDER BY order_completed_at DESC
LIMIT $2 OFFSET $3;

-- name: ListChargesByInvoice :many
SELECT * FROM commission_charges
WHERE invoice_id = $1
ORDER BY order_completed_at;

-- Admin. The worklist is one row per seller who owes something, overdue first.
-- name: ListSellerBalances :many
SELECT
    u.id AS seller_id, u.public_id, u.name, u.phone, u.email, u.status,
    SUM(i.total_amount)::DECIMAL(14,2) AS outstanding,
    COUNT(*) AS unpaid_invoices,
    MIN(i.due_at)::timestamptz AS earliest_due_at,
    BOOL_OR(i.due_at < NOW()) AS is_overdue
FROM commission_invoices i
JOIN users u ON u.id = i.seller_id
WHERE i.status = 'unpaid'
  AND (@overdue_only::bool = false OR i.due_at < NOW())
GROUP BY u.id, u.public_id, u.name, u.phone, u.email, u.status
ORDER BY BOOL_OR(i.due_at < NOW()) DESC, MIN(i.due_at)
LIMIT $1 OFFSET $2;

-- name: CountSellerBalances :one
SELECT COUNT(DISTINCT seller_id) FROM commission_invoices
WHERE status = 'unpaid'
  AND (@overdue_only::bool = false OR due_at < NOW());

-- name: GetCommissionTotals :one
SELECT
    COALESCE(SUM(total_amount) FILTER (WHERE status = 'unpaid'), 0)::DECIMAL(14,2) AS total_outstanding,
    COALESCE(SUM(total_amount) FILTER (WHERE status = 'unpaid' AND due_at < NOW()), 0)::DECIMAL(14,2) AS total_overdue,
    COALESCE(SUM(total_amount) FILTER (WHERE status = 'paid'), 0)::DECIMAL(14,2) AS total_collected
FROM commission_invoices;

-- name: GetInvoiceByPublicID :one
SELECT * FROM commission_invoices WHERE public_id = $1;

-- name: ListInvoicesForAdmin :many
SELECT i.*, u.name AS seller_name, u.phone AS seller_phone, u.public_id AS seller_public_id
FROM commission_invoices i
JOIN users u ON u.id = i.seller_id
WHERE (@status_filter::text = '' OR i.status = @status_filter::text)
ORDER BY i.issued_at DESC
LIMIT $1 OFFSET $2;

-- Recording a payment is admin-only and one-way: an invoice is unpaid, paid or
-- waived. The status guard makes a double-submit a no-op rather than a second
-- payment record.
-- name: MarkInvoicePaid :execrows
UPDATE commission_invoices
SET status = 'paid', paid_at = NOW(), payment_method = $2, payment_reference = $3,
    payment_note = $4, recorded_by_admin_id = $5, updated_at = NOW()
WHERE id = $1 AND status = 'unpaid';

-- name: WaiveInvoice :execrows
UPDATE commission_invoices
SET status = 'waived', waived_reason = $2, recorded_by_admin_id = $3, updated_at = NOW()
WHERE id = $1 AND status = 'unpaid';

-- Payment reminders. Candidates are unpaid invoices that are due and either
-- never reminded or last reminded longer ago than the configured interval. The
-- interval is a parameter rather than a literal so the admin setting is the only
-- place the cadence is defined.
-- name: ListInvoicesNeedingReminder :many
SELECT i.*, u.name AS seller_name
FROM commission_invoices i
JOIN users u ON u.id = i.seller_id
WHERE i.status = 'unpaid'
  AND i.due_at <= NOW()
  -- The cutoff is computed by the caller from the configured cadence: doing the
  -- interval arithmetic here would put the same rule in two places.
  AND (i.last_reminder_at IS NULL OR i.last_reminder_at < @remind_before::timestamptz)
ORDER BY i.due_at
LIMIT $1;

-- name: MarkInvoiceReminded :exec
UPDATE commission_invoices
SET last_reminder_at = NOW(),
    reminder_count   = reminder_count + 1,
    updated_at       = NOW()
WHERE id = $1;
