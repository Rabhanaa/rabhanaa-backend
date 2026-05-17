-- +goose Up
ALTER TABLE sell_auctions ADD COLUMN last_motivation_sent_at TIMESTAMPTZ;
ALTER TABLE buy_requests ADD COLUMN last_motivation_sent_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE sell_auctions DROP COLUMN last_motivation_sent_at;
ALTER TABLE buy_requests DROP COLUMN last_motivation_sent_at;
