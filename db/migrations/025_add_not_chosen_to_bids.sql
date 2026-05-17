-- +goose Up
ALTER TABLE sell_bids ADD COLUMN is_not_chosen BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE supply_offers ADD COLUMN is_not_chosen BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE sell_bids DROP COLUMN IF EXISTS is_not_chosen;
ALTER TABLE supply_offers DROP COLUMN IF EXISTS is_not_chosen;
