-- +goose Up
CREATE TABLE buy_requests (
    id                   SERIAL PRIMARY KEY,
    public_id            UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    owner_id             INTEGER NOT NULL REFERENCES users(id),
    region_id            INTEGER NOT NULL REFERENCES regions(id),
    interest_id          INTEGER NOT NULL REFERENCES interests(id),
    title                VARCHAR(200) NOT NULL,
    description          TEXT,
    image_url            VARCHAR(500) NOT NULL,
    unit                 VARCHAR(10) NOT NULL CHECK (unit IN ('kg','ton','piece','box')),
    quantity             DECIMAL(10,2) NOT NULL CHECK (quantity > 0),
    buy_all_from_one     BOOLEAN NOT NULL DEFAULT TRUE,
    offer_count          INTEGER NOT NULL DEFAULT 0,
    accepted_offer_count INTEGER NOT NULL DEFAULT 0,
    fulfilled_quantity   DECIMAL(10,2) NOT NULL DEFAULT 0,
    end_time             TIMESTAMPTZ NOT NULL,
    status               VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','cancelled','expired','pending_selection','fulfilled','partially_fulfilled')),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_buy_requests_public_id ON buy_requests(public_id);
CREATE INDEX idx_buy_requests_owner_id ON buy_requests(owner_id);
CREATE INDEX idx_buy_requests_status_end ON buy_requests(status, end_time);
CREATE INDEX idx_buy_requests_region ON buy_requests(region_id);
CREATE INDEX idx_buy_requests_interest ON buy_requests(interest_id);
CREATE INDEX idx_buy_requests_created ON buy_requests(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS buy_requests;
