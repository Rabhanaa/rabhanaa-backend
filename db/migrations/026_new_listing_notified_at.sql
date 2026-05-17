-- +goose Up
ALTER TABLE sell_auctions ADD COLUMN notified_at TIMESTAMPTZ;
ALTER TABLE buy_requests ADD COLUMN notified_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE sell_auctions DROP COLUMN notified_at;
ALTER TABLE buy_requests DROP COLUMN notified_at;
