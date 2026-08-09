-- +goose Up
-- The "selection window closing soon" warning had no sent-marker, so the cron
-- re-sent it on every tick while the auction sat inside the warning band.
-- Follows the same pattern as last_motivation_sent_at (033).
ALTER TABLE sell_auctions ADD COLUMN selection_warning_sent_at TIMESTAMPTZ;
ALTER TABLE buy_requests ADD COLUMN selection_warning_sent_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE sell_auctions DROP COLUMN selection_warning_sent_at;
ALTER TABLE buy_requests DROP COLUMN selection_warning_sent_at;
