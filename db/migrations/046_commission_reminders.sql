-- +goose Up

-- Payment reminders (#13). The invoice-issued notification is sent once; a
-- seller who misses it hears nothing more until an admin phones them. These two
-- columns are what make a repeating reminder safe: without a record of the last
-- one, a cron that ticks every minute would notify on every tick.
ALTER TABLE commission_invoices
    ADD COLUMN last_reminder_at TIMESTAMPTZ,
    ADD COLUMN reminder_count   INTEGER NOT NULL DEFAULT 0;

-- The reminder sweep looks for unpaid invoices that are due and not reminded
-- recently. Partial index because paid and waived invoices are never candidates.
CREATE INDEX idx_commission_invoices_reminder ON commission_invoices (due_at, last_reminder_at)
    WHERE status = 'unpaid';

-- +goose Down

DROP INDEX IF EXISTS idx_commission_invoices_reminder;
ALTER TABLE commission_invoices
    DROP COLUMN IF EXISTS last_reminder_at,
    DROP COLUMN IF EXISTS reminder_count;
