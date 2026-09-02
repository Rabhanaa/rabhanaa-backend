-- +goose Up

-- Client request #13: the platform takes 1.5% of every sale from the seller.
--
-- This is a ledger and a collection workflow, not a payment system. No money
-- moves through the platform: the app computes what is owed, shows it to the
-- seller and the admin, and records what the admin says was paid over Vodafone
-- Cash or InstaPay.
--
-- Additive only — nothing existing is altered, so this can be applied to a live
-- database before the new backend is deployed.

-- One invoice per seller per week, created only when that seller actually has
-- uninvoiced charges. Created before commission_charges because charges
-- reference it.
CREATE TABLE commission_invoices (
    id                   SERIAL PRIMARY KEY,
    public_id            UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    seller_id            INTEGER NOT NULL REFERENCES users(id),

    -- The week this invoice bills, [period_start, period_end).
    period_start         TIMESTAMPTZ NOT NULL,
    period_end           TIMESTAMPTZ NOT NULL,

    -- Sum of the attached charges, never re-derived from deal values: summing
    -- rounded charges and rounding a summed base give different answers.
    total_amount         DECIMAL(14,2) NOT NULL CHECK (total_amount >= 0),

    -- 'waived' is the pressure valve. A disputed or mistaken invoice is written
    -- off rather than deleted, so the financial history stays intact.
    status               TEXT NOT NULL DEFAULT 'unpaid'
        CHECK (status IN ('unpaid', 'paid', 'waived')),

    issued_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- issued_at + commission_grace_days at the time of issue. Stored rather than
    -- computed so that changing the grace setting cannot silently move the due
    -- date of an invoice a seller has already been told about.
    due_at               TIMESTAMPTZ NOT NULL,

    paid_at              TIMESTAMPTZ,
    payment_method       VARCHAR(30),
    payment_reference    VARCHAR(120),
    payment_note         TEXT,
    waived_reason        TEXT,
    recorded_by_admin_id INTEGER REFERENCES users(id),

    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Makes a double-run of the weekly job harmless.
    CONSTRAINT commission_invoices_seller_period_key UNIQUE (seller_id, period_start)
);

-- The collection worklist is "unpaid, oldest first"; overdue is derived from
-- due_at rather than stored, so it cannot go stale when the grace setting moves.
CREATE INDEX idx_commission_invoices_unpaid ON commission_invoices (due_at)
    WHERE status = 'unpaid';
CREATE INDEX idx_commission_invoices_seller ON commission_invoices (seller_id, issued_at DESC);

-- One row per completed order. Written by the cron as soon as it sees an order
-- reach 'completed', which is what gives the seller a running total between
-- invoices.
CREATE TABLE commission_charges (
    id                 SERIAL PRIMARY KEY,
    public_id          UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),

    -- UNIQUE is what makes accrual safe to re-run every minute.
    order_id           INTEGER NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    seller_id          INTEGER NOT NULL REFERENCES users(id),

    -- final_price is per unit (order/service/service.go builds the total the
    -- same way), so the base is price x quantity — the deal value, not the unit
    -- price. Shipping is excluded: it is the carrier's money, not the seller's.
    deal_value         DECIMAL(14,2) NOT NULL CHECK (deal_value >= 0),

    -- Snapshotted per charge. Changing commission_rate_percent in settings must
    -- never rewrite what a seller has already been told they owe.
    rate_percent       DECIMAL(5,3) NOT NULL CHECK (rate_percent >= 0 AND rate_percent <= 100),
    amount             DECIMAL(14,2) NOT NULL CHECK (amount >= 0),

    -- Which week the charge belongs to. Copied from the order rather than joined
    -- so an invoice period is a plain range scan on this table.
    order_completed_at TIMESTAMPTZ NOT NULL,

    invoice_id         INTEGER REFERENCES commission_invoices(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The invoicing job's central query: uninvoiced charges in a period.
CREATE INDEX idx_commission_charges_uninvoiced ON commission_charges (seller_id, order_completed_at)
    WHERE invoice_id IS NULL;
CREATE INDEX idx_commission_charges_invoice ON commission_charges (invoice_id);

-- +goose Down

DROP TABLE IF EXISTS commission_charges;
DROP TABLE IF EXISTS commission_invoices;
