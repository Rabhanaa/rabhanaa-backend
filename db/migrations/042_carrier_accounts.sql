-- +goose Up

-- Client request #14, rebuilt. The first pass (041) read the request as a
-- directory: rows an admin types in, shown to a merchant before publishing. The
-- client meant the other reading the analysis flagged as unanswered — carriers
-- register their own accounts, see the jobs they can serve, quote a price, and
-- the merchant accepts or rejects it.
--
-- Additive only. 041's tables are deliberately left in place and unused;
-- 043 removes them as a separate, later step, so this migration can be applied
-- to a live database before the new backend is deployed and rolled back without
-- losing anything.

-- The role. Same shape as 040: explicit id so existing users keep their job_id,
-- and the sequence advanced afterwards because 010 seeded this table with
-- literal ids and never moved it.
INSERT INTO jobs (id, key, name_ar, name_en, is_active) VALUES
    (9, 'shipping_company', 'شركة شحن', 'Shipping Company', true)
ON CONFLICT (id) DO NOTHING;

SELECT setval(pg_get_serial_sequence('jobs', 'id'), (SELECT MAX(id) FROM jobs));

-- Carriers are users: name, phone, email and status already live on users, and
-- reusing them means login, suspension, notifications and the admin review
-- queue all work with no new machinery. Only the carrier-specific extras need
-- storage of their own.
CREATE TABLE carrier_profiles (
    user_id    INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    logo_url   VARCHAR(500),
    notes      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Which governorates a carrier actually serves. A carrier with no rows here
-- sees no jobs at all, which is the predictable reading — registration requires
-- at least one, so it cannot happen by accident.
CREATE TABLE carrier_regions (
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    region_id INTEGER NOT NULL REFERENCES regions(id),
    PRIMARY KEY (user_id, region_id)
);

-- Every carrier-facing lookup starts from a governorate.
CREATE INDEX idx_carrier_regions_region ON carrier_regions (region_id);

-- A carrier's price for moving one job. Modelled on supply_offers (006): one
-- row per carrier per job, a status the owner drives, and an index that serves
-- the cheapest-first listing.
--
-- Three nullable targets rather than three tables: which one is used depends on
-- the carrier_quote_stage setting, and a single table keeps "my quotes" and the
-- accept path from forking into three near-identical copies.
CREATE TABLE shipping_quotes (
    id              SERIAL PRIMARY KEY,
    public_id       UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    carrier_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sell_auction_id INTEGER REFERENCES sell_auctions(id) ON DELETE CASCADE,
    buy_request_id  INTEGER REFERENCES buy_requests(id) ON DELETE CASCADE,
    order_id        INTEGER REFERENCES orders(id) ON DELETE CASCADE,
    price           DECIMAL(12,2) NOT NULL CHECK (price > 0),
    notes           TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'rejected', 'withdrawn')),
    accepted_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Exactly one target. Without this a row could quote nothing, or two jobs
    -- at once, and every read would need to guess which column to trust.
    CONSTRAINT shipping_quotes_one_target
        CHECK (num_nonnulls(sell_auction_id, buy_request_id, order_id) = 1)
);

-- One live quote per carrier per job. Partial indexes because two of the three
-- target columns are NULL on any given row, and a plain UNIQUE would treat
-- those NULLs as distinct and let a carrier quote the same job repeatedly.
CREATE UNIQUE INDEX idx_shipping_quotes_carrier_auction
    ON shipping_quotes (carrier_id, sell_auction_id) WHERE sell_auction_id IS NOT NULL;
CREATE UNIQUE INDEX idx_shipping_quotes_carrier_request
    ON shipping_quotes (carrier_id, buy_request_id) WHERE buy_request_id IS NOT NULL;
CREATE UNIQUE INDEX idx_shipping_quotes_carrier_order
    ON shipping_quotes (carrier_id, order_id) WHERE order_id IS NOT NULL;

-- The merchant's view of a job's quotes: cheapest first.
CREATE INDEX idx_shipping_quotes_auction_price
    ON shipping_quotes (sell_auction_id, price ASC) WHERE sell_auction_id IS NOT NULL;
CREATE INDEX idx_shipping_quotes_request_price
    ON shipping_quotes (buy_request_id, price ASC) WHERE buy_request_id IS NOT NULL;
CREATE INDEX idx_shipping_quotes_order_price
    ON shipping_quotes (order_id, price ASC) WHERE order_id IS NOT NULL;

-- The carrier's own list.
CREATE INDEX idx_shipping_quotes_carrier ON shipping_quotes (carrier_id, created_at DESC);

-- The accepted carrier, recorded on the deal. Nullable: most orders have no
-- carrier, and in post-stage mode the accepted quote is copied here when the
-- deal closes so the order is always the record of who moved the goods.
ALTER TABLE orders
    ADD COLUMN carrier_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN shipping_price DECIMAL(12,2);

-- Carrier scoping needs region ids, and orders only denormalize region *names*.
-- Two ids rather than a join back to the source post: goods move between the
-- two parties, so both ends decide which carriers are relevant, and this table
-- already denormalizes the rest of the same data for the same reason.
ALTER TABLE orders
    ADD COLUMN seller_region_id INTEGER REFERENCES regions(id),
    ADD COLUMN buyer_region_id  INTEGER REFERENCES regions(id);

UPDATE orders o SET seller_region_id = s.region_id FROM users s WHERE s.id = o.seller_id;
UPDATE orders o SET buyer_region_id  = b.region_id FROM users b WHERE b.id = o.buyer_id;

CREATE INDEX idx_orders_seller_region ON orders (seller_region_id);
CREATE INDEX idx_orders_buyer_region  ON orders (buyer_region_id);

-- Settings an admin changes without a redeploy. Every flag until now has been an
-- env var read once at boot, which is right for infrastructure and wrong for a
-- policy the client wants to flip themselves.
--
-- The rule to keep: env vars for deploy-time infrastructure, this table for
-- behaviour the client owns. Keys are whitelisted in the handler — this is not a
-- general-purpose key/value store.
CREATE TABLE app_settings (
    key                 VARCHAR(64) PRIMARY KEY,
    value               TEXT NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_admin_id INTEGER REFERENCES users(id) ON DELETE SET NULL
);

-- Where carriers quote: 'order' (after a winner exists), 'post' (while the post
-- is live) or 'both'. Defaults to 'order' because before a winner is picked
-- there is no buyer, so there is no destination and a transport price is a guess.
INSERT INTO app_settings (key, value) VALUES ('carrier_quote_stage', 'order')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS app_settings;

DROP INDEX IF EXISTS idx_orders_buyer_region;
DROP INDEX IF EXISTS idx_orders_seller_region;
ALTER TABLE orders
    DROP COLUMN IF EXISTS buyer_region_id,
    DROP COLUMN IF EXISTS seller_region_id,
    DROP COLUMN IF EXISTS shipping_price,
    DROP COLUMN IF EXISTS carrier_id;

DROP TABLE IF EXISTS shipping_quotes;

DROP INDEX IF EXISTS idx_carrier_regions_region;
DROP TABLE IF EXISTS carrier_regions;
DROP TABLE IF EXISTS carrier_profiles;

-- Same guard as 036/037/040: leave the role in place if anyone registered under
-- it, rather than aborting on the foreign key or silently reassigning a carrier
-- to some other trade.
DELETE FROM jobs
WHERE id = 9
  AND id NOT IN (SELECT job_id FROM users WHERE job_id IS NOT NULL);

SELECT setval(pg_get_serial_sequence('jobs', 'id'), (SELECT MAX(id) FROM jobs));
