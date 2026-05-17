-- +goose Up
-- Add cached user data columns to orders table
ALTER TABLE orders
    ADD COLUMN seller_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN seller_phone VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN seller_region VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN buyer_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN buyer_phone VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN buyer_region VARCHAR(100) NOT NULL DEFAULT '';

-- Remove defaults after adding columns (they were just for migration)
-- Note: We keep the defaults for safety, but new orders MUST provide these values

-- +goose Down
-- Remove cached user data columns from orders table
ALTER TABLE orders
    DROP COLUMN IF EXISTS seller_name,
    DROP COLUMN IF EXISTS seller_phone,
    DROP COLUMN IF EXISTS seller_region,
    DROP COLUMN IF EXISTS buyer_name,
    DROP COLUMN IF EXISTS buyer_phone,
    DROP COLUMN IF EXISTS buyer_region;
