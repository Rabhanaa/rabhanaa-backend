-- +goose Up
CREATE TABLE sell_auctions (
    id              SERIAL PRIMARY KEY,
    public_id       UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    owner_id        INTEGER NOT NULL REFERENCES users(id),
    region_id       INTEGER NOT NULL REFERENCES regions(id),
    interest_id     INTEGER NOT NULL REFERENCES interests(id),
    title           VARCHAR(200) NOT NULL,
    description     TEXT,
    image_url       VARCHAR(500) NOT NULL,
    unit            VARCHAR(10) NOT NULL CHECK (unit IN ('kg','ton','piece','box')),
    quantity        DECIMAL(10,2) NOT NULL CHECK (quantity > 0),
    unit_price      DECIMAL(12,2) NOT NULL CHECK (unit_price > 0),
    buy_all_from_one BOOLEAN NOT NULL DEFAULT TRUE,
    bid_count       INTEGER NOT NULL DEFAULT 0,
    end_time        TIMESTAMPTZ NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','cancelled','expired','pending_selection','winner_selected')),
    selected_bid_id INTEGER,
    winner_id       INTEGER REFERENCES users(id),
    final_price     DECIMAL(12,2),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sell_auctions_public_id ON sell_auctions(public_id);
CREATE INDEX idx_sell_auctions_owner_id ON sell_auctions(owner_id);
CREATE INDEX idx_sell_auctions_status_end ON sell_auctions(status, end_time);
CREATE INDEX idx_sell_auctions_region ON sell_auctions(region_id);
CREATE INDEX idx_sell_auctions_interest ON sell_auctions(interest_id);
CREATE INDEX idx_sell_auctions_created ON sell_auctions(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS sell_auctions;
