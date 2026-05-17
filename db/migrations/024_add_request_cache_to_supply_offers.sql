-- +goose Up
-- Cache request details in supply_offers for efficient queries
ALTER TABLE supply_offers 
    ADD COLUMN request_title VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN request_quantity DECIMAL(10,2) NOT NULL DEFAULT 0,
    ADD COLUMN request_unit VARCHAR(10) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE supply_offers 
    DROP COLUMN IF EXISTS request_title,
    DROP COLUMN IF EXISTS request_quantity,
    DROP COLUMN IF EXISTS request_unit;
