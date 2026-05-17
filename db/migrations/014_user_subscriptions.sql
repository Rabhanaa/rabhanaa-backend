-- +goose Up

-- Junction table: users can have multiple subscriptions/packages
CREATE TABLE user_subscriptions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier_name VARCHAR(20) NOT NULL REFERENCES subscription_tiers(tier_name),
    
    -- Subscription period
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    
    -- Usage tracking (for monthly limits)
    auctions_created_this_month INTEGER NOT NULL DEFAULT 0,
    requests_created_this_month INTEGER NOT NULL DEFAULT 0,
    bids_placed_this_month INTEGER NOT NULL DEFAULT 0,
    offers_made_this_month INTEGER NOT NULL DEFAULT 0,
    month_reset_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Status
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(user_id, tier_name)
);

-- CRITICAL INDEX for single-query JOIN performance
CREATE INDEX idx_user_subscriptions_lookup 
ON user_subscriptions(user_id, is_active, expires_at, tier_name);

-- Additional indexes
CREATE INDEX idx_user_subscriptions_user ON user_subscriptions(user_id);
CREATE INDEX idx_user_subscriptions_active ON user_subscriptions(user_id, is_active);

-- +goose Down
DROP TABLE IF EXISTS user_subscriptions;
