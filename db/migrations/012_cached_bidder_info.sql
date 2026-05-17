-- +goose Up
-- Add cached region and fake name columns to sell_bids for efficient queries
ALTER TABLE sell_bids 
    ADD COLUMN bidder_region_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN bidder_fake_name VARCHAR(100) NOT NULL DEFAULT '';

-- Add cached region and fake name columns to supply_offers for efficient queries
ALTER TABLE supply_offers 
    ADD COLUMN supplier_region_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN supplier_fake_name VARCHAR(100) NOT NULL DEFAULT '';

-- Create index for efficient lookups by fake name
CREATE INDEX idx_sell_bids_fake_name ON sell_bids(auction_id, bidder_fake_name);
CREATE INDEX idx_supply_offers_fake_name ON supply_offers(buy_request_id, supplier_fake_name);

-- +goose Down
-- Remove cached columns from sell_bids
ALTER TABLE sell_bids 
    DROP COLUMN IF EXISTS bidder_region_name,
    DROP COLUMN IF EXISTS bidder_fake_name;

-- Remove cached columns from supply_offers
ALTER TABLE supply_offers 
    DROP COLUMN IF EXISTS supplier_region_name,
    DROP COLUMN IF EXISTS supplier_fake_name;

-- Drop indexes
DROP INDEX IF EXISTS idx_sell_bids_fake_name;
DROP INDEX IF EXISTS idx_supply_offers_fake_name;
