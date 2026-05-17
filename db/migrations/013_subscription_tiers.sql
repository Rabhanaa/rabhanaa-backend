-- +goose Up

-- Subscription tiers with limits and pricing
CREATE TABLE subscription_tiers (
    tier_name VARCHAR(20) PRIMARY KEY,
    price_egp DECIMAL(10,2) NOT NULL DEFAULT 0.00,
    price_usd DECIMAL(10,2) NOT NULL DEFAULT 0.00,
    
    -- Auction limits (NULL = unlimited)
    max_sell_auctions_per_month INTEGER,
    max_buy_requests_per_month INTEGER,
    
    -- Bidding/Offer limits
    max_bids_per_month INTEGER,
    max_offers_per_month INTEGER,
    
    -- Feature flags
    can_create_sell_auctions BOOLEAN NOT NULL DEFAULT FALSE,
    can_create_buy_requests BOOLEAN NOT NULL DEFAULT FALSE,
    can_place_bids BOOLEAN NOT NULL DEFAULT FALSE,
    can_make_offers BOOLEAN NOT NULL DEFAULT FALSE,
    priority_support BOOLEAN NOT NULL DEFAULT FALSE,
    
    -- Tier display info
    display_name_ar VARCHAR(100) NOT NULL,
    display_name_en VARCHAR(100) NOT NULL,
    description_ar TEXT,
    description_en TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default tiers
INSERT INTO subscription_tiers (
    tier_name, price_egp, price_usd,
    max_sell_auctions_per_month, max_buy_requests_per_month,
    max_bids_per_month, max_offers_per_month,
    can_create_sell_auctions, can_create_buy_requests, 
    can_place_bids, can_make_offers, priority_support,
    display_name_ar, display_name_en
) VALUES 
    -- Free: Browse only, cannot create or bid
    ('free', 0.00, 0.00, 
     0, 0, 0, 0,
     FALSE, FALSE, FALSE, FALSE, FALSE,
     'مجاني', 'Free'),
     
    -- Basic: Limited creation and bidding
    ('basic', 99.00, 2.99,
     5, 5, 20, 20,
     TRUE, TRUE, TRUE, TRUE, FALSE,
     'أساسي', 'Basic'),
     
    -- Premium: Higher limits
    ('premium', 299.00, 7.99,
     20, 20, 100, 100,
     TRUE, TRUE, TRUE, TRUE, TRUE,
     'بريميوم', 'Premium'),
     
    -- Zico: Unlimited everything
    ('zico', 999.00, 24.99,
     NULL, NULL, NULL, NULL,
     TRUE, TRUE, TRUE, TRUE, TRUE,
     'زيكو غير محدود', 'Zico Unlimited');

CREATE INDEX idx_subscription_tiers_price ON subscription_tiers(price_egp);

-- +goose Down
DROP TABLE IF EXISTS subscription_tiers;
