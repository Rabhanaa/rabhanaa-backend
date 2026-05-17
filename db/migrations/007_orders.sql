-- +goose Up
CREATE TABLE orders (
    id                  SERIAL PRIMARY KEY,
    public_id           UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    sell_auction_id     INTEGER REFERENCES sell_auctions(id),
    buy_request_id      INTEGER REFERENCES buy_requests(id),
    seller_id           INTEGER NOT NULL REFERENCES users(id),
    buyer_id            INTEGER NOT NULL REFERENCES users(id),
    final_price         DECIMAL(12,2) NOT NULL,
    quantity            DECIMAL(10,2) NOT NULL,
    unit                VARCHAR(10) NOT NULL,
    status              VARCHAR(30) NOT NULL DEFAULT 'created'
        CHECK (status IN ('created','seller_confirmed','buyer_confirmed','completed','cancelled')),
    seller_confirmed_at TIMESTAMPTZ,
    buyer_confirmed_at  TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_order_source CHECK (
        (sell_auction_id IS NOT NULL AND buy_request_id IS NULL) OR
        (sell_auction_id IS NULL AND buy_request_id IS NOT NULL)
    )
);

CREATE INDEX idx_orders_public_id ON orders(public_id);
CREATE INDEX idx_orders_seller ON orders(seller_id);
CREATE INDEX idx_orders_buyer ON orders(buyer_id);
CREATE INDEX idx_orders_sell_auction ON orders(sell_auction_id);
CREATE INDEX idx_orders_buy_request ON orders(buy_request_id);

-- +goose Down
DROP TABLE IF EXISTS orders;
