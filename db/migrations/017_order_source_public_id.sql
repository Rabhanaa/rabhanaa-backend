-- +goose Up
-- Add source_public_id to orders table to track auction/request public ID
ALTER TABLE orders
    ADD COLUMN source_public_id UUID;

-- +goose Down
-- Remove source_public_id from orders table
ALTER TABLE orders
    DROP COLUMN IF EXISTS source_public_id;
