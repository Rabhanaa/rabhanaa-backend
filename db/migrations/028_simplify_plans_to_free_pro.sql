-- +goose Up

-- Add the new pro tier (unlimited, all features enabled)
INSERT INTO subscription_tiers (
    tier_name, price_egp, price_usd,
    max_sell_auctions_per_month, max_buy_requests_per_month,
    max_bids_per_month, max_offers_per_month,
    can_create_sell_auctions, can_create_buy_requests,
    can_place_bids, can_make_offers, priority_support,
    display_name_ar, display_name_en,
    description_ar, description_en
) VALUES (
    'pro', 0.00, 0.00,
    NULL, NULL, NULL, NULL,
    TRUE, TRUE, TRUE, TRUE, TRUE,
    'برو', 'Pro',
    'وصول غير محدود لجميع الميزات', 'Unlimited access to all features'
);

-- Migrate all users on basic/premium/zico to pro
-- Reuse existing pro row if user already has one, otherwise insert
INSERT INTO user_subscriptions (user_id, tier_name, started_at, expires_at, is_active, is_primary)
SELECT DISTINCT user_id, 'pro', NOW(), NULL::TIMESTAMPTZ, TRUE, TRUE
FROM user_subscriptions
WHERE tier_name IN ('basic', 'premium', 'zico') AND is_active = TRUE
ON CONFLICT (user_id, tier_name) DO UPDATE
    SET is_active = TRUE, is_primary = TRUE, started_at = NOW(), expires_at = NULL::TIMESTAMPTZ;

-- Also give pro to any active users that have no active subscription at all
INSERT INTO user_subscriptions (user_id, tier_name, started_at, expires_at, is_active, is_primary)
SELECT u.id, 'pro', NOW(), NULL::TIMESTAMPTZ, TRUE, TRUE
FROM users u
WHERE u.status = 'active'
  AND NOT EXISTS (
      SELECT 1 FROM user_subscriptions us
      WHERE us.user_id = u.id AND us.is_active = TRUE AND us.tier_name = 'pro'
  )
ON CONFLICT (user_id, tier_name) DO UPDATE
    SET is_active = TRUE, is_primary = TRUE, started_at = NOW(), expires_at = NULL::TIMESTAMPTZ;

-- Deactivate old tier subscriptions for users now on pro
UPDATE user_subscriptions
SET is_active = FALSE, is_primary = FALSE
WHERE tier_name IN ('basic', 'premium', 'zico');

-- Remove the old tiers
DELETE FROM subscription_tiers WHERE tier_name IN ('basic', 'premium', 'zico');

-- +goose Down

-- Re-add old tiers
INSERT INTO subscription_tiers (
    tier_name, price_egp, price_usd,
    max_sell_auctions_per_month, max_buy_requests_per_month,
    max_bids_per_month, max_offers_per_month,
    can_create_sell_auctions, can_create_buy_requests,
    can_place_bids, can_make_offers, priority_support,
    display_name_ar, display_name_en
) VALUES
    ('basic', 99.00, 2.99, 5, 5, 20, 20, TRUE, TRUE, TRUE, TRUE, FALSE, 'أساسي', 'Basic'),
    ('premium', 299.00, 7.99, 20, 20, 100, 100, TRUE, TRUE, TRUE, TRUE, TRUE, 'بريميوم', 'Premium'),
    ('zico', 999.00, 24.99, NULL, NULL, NULL, NULL, TRUE, TRUE, TRUE, TRUE, TRUE, 'زيكو غير محدود', 'Zico Unlimited');

-- Restore zico subscriptions for users currently on pro (best effort)
INSERT INTO user_subscriptions (user_id, tier_name, started_at, expires_at, is_active, is_primary)
SELECT user_id, 'zico', started_at, COALESCE(expires_at, NULL::TIMESTAMPTZ), TRUE, TRUE
FROM user_subscriptions
WHERE tier_name = 'pro' AND is_active = TRUE
ON CONFLICT (user_id, tier_name) DO UPDATE
    SET is_active = TRUE, is_primary = TRUE;

-- Deactivate pro subscriptions
UPDATE user_subscriptions SET is_active = FALSE, is_primary = FALSE WHERE tier_name = 'pro';

-- Remove pro tier
DELETE FROM subscription_tiers WHERE tier_name = 'pro';
