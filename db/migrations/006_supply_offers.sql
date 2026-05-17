-- +goose Up
CREATE TABLE supply_offers (
    id               SERIAL PRIMARY KEY,
    public_id        UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    buy_request_id   INTEGER NOT NULL REFERENCES buy_requests(id) ON DELETE CASCADE,
    supplier_id      INTEGER NOT NULL REFERENCES users(id),
    price_per_unit   DECIMAL(12,2) NOT NULL CHECK (price_per_unit > 0),
    offered_quantity DECIMAL(10,2) NOT NULL CHECK (offered_quantity > 0),
    is_accepted      BOOLEAN NOT NULL DEFAULT FALSE,
    accepted_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(buy_request_id, supplier_id)
);

CREATE INDEX idx_supply_offers_request ON supply_offers(buy_request_id);
CREATE INDEX idx_supply_offers_supplier ON supply_offers(supplier_id);
CREATE INDEX idx_supply_offers_public_id ON supply_offers(public_id);
CREATE INDEX idx_supply_offers_price ON supply_offers(buy_request_id, price_per_unit ASC);

-- +goose Down
DROP TABLE IF EXISTS supply_offers;
