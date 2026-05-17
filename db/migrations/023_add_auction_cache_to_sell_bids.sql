-- +goose Up
-- Cache auction details in sell_bids for efficient queries
ALTER TABLE sell_bids 
    ADD COLUMN auction_title VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN auction_unit_price DECIMAL(12,2) NOT NULL DEFAULT 0,
    ADD COLUMN auction_quantity DECIMAL(10,2) NOT NULL DEFAULT 0,
    ADD COLUMN auction_unit VARCHAR(10) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE sell_bids 
    DROP COLUMN IF EXISTS auction_title,
    DROP COLUMN IF EXISTS auction_unit_price,
    DROP COLUMN IF EXISTS auction_quantity,
    DROP COLUMN IF EXISTS auction_unit;
