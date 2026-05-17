-- +goose Up
CREATE TABLE sell_bids (
    id          SERIAL PRIMARY KEY,
    public_id   UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    auction_id  INTEGER NOT NULL REFERENCES sell_auctions(id) ON DELETE CASCADE,
    bidder_id   INTEGER NOT NULL REFERENCES users(id),
    amount      DECIMAL(12,2) NOT NULL CHECK (amount > 0),
    is_selected BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(auction_id, bidder_id)
);

CREATE INDEX idx_sell_bids_auction ON sell_bids(auction_id);
CREATE INDEX idx_sell_bids_bidder ON sell_bids(bidder_id);
CREATE INDEX idx_sell_bids_public_id ON sell_bids(public_id);

-- +goose Down
DROP TABLE IF EXISTS sell_bids;
