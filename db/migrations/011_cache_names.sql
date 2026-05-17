-- +goose Up
-- Add cached name columns to sell_auctions
ALTER TABLE sell_auctions 
    ADD COLUMN owner_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN region_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN interest_name VARCHAR(100) NOT NULL DEFAULT '';

-- Add cached name columns to buy_requests
ALTER TABLE buy_requests 
    ADD COLUMN owner_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN region_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN interest_name VARCHAR(100) NOT NULL DEFAULT '';

-- Backfill existing sell_auctions rows
UPDATE sell_auctions sa
SET 
    owner_name = u.name,
    region_name = r.name_ar,
    interest_name = i.name_ar
FROM users u, regions r, interests i
WHERE sa.owner_id = u.id
    AND sa.region_id = r.id
    AND sa.interest_id = i.id;

-- Backfill existing buy_requests rows
UPDATE buy_requests br
SET 
    owner_name = u.name,
    region_name = r.name_ar,
    interest_name = i.name_ar
FROM users u, regions r, interests i
WHERE br.owner_id = u.id
    AND br.region_id = r.id
    AND br.interest_id = i.id;

-- +goose Down
-- Remove cached name columns from sell_auctions
ALTER TABLE sell_auctions 
    DROP COLUMN IF EXISTS owner_name,
    DROP COLUMN IF EXISTS region_name,
    DROP COLUMN IF EXISTS interest_name;

-- Remove cached name columns from buy_requests
ALTER TABLE buy_requests 
    DROP COLUMN IF EXISTS owner_name,
    DROP COLUMN IF EXISTS region_name,
    DROP COLUMN IF EXISTS interest_name;
