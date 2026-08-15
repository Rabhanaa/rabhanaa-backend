-- name: GetUserWithSubscription :one
-- Get user info with their highest active subscription in one query
SELECT
    u.id as user_id,
    u.status as user_status,
    u.suspended_until as user_suspended_until,
    u.suspension_reason as user_suspension_reason,
    u.banned_reason as user_banned_reason,
    u.password_changed_at as user_password_changed_at,
    u.public_id,
    us.id as subscription_id,
    us.tier_name,
    us.is_active as sub_is_active,
    us.expires_at as sub_expires_at,
    us.auctions_created_this_month,
    us.requests_created_this_month,
    us.bids_placed_this_month,
    us.offers_made_this_month,
    us.month_reset_at,
    st.can_create_sell_auctions,
    st.can_create_buy_requests,
    st.can_place_bids,
    st.can_make_offers,
    st.max_sell_auctions_per_month,
    st.max_buy_requests_per_month,
    st.max_bids_per_month,
    st.max_offers_per_month,
    st.price_egp,
    st.display_name_ar
FROM users u
LEFT JOIN user_subscriptions us ON u.id = us.user_id 
    AND us.is_active = TRUE
    AND (us.expires_at IS NULL OR us.expires_at > NOW())
LEFT JOIN subscription_tiers st ON us.tier_name = st.tier_name
WHERE u.id = $1
ORDER BY us.is_primary DESC NULLS LAST, st.price_egp DESC NULLS LAST
LIMIT 1;

-- name: GetSubscriptionTiers :many
SELECT * FROM subscription_tiers ORDER BY price_egp ASC;

-- name: GetUserSubscriptions :many
SELECT us.*, st.* 
FROM user_subscriptions us
JOIN subscription_tiers st ON us.tier_name = st.tier_name
WHERE us.user_id = $1 AND us.is_active = TRUE
ORDER BY st.price_egp DESC;

-- name: CreateUserSubscription :one
INSERT INTO user_subscriptions (
    user_id, tier_name, started_at, expires_at, is_active, is_primary
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: IncrementAuctionCount :exec
UPDATE user_subscriptions 
SET auctions_created_this_month = auctions_created_this_month + 1,
    updated_at = NOW()
WHERE id = $1;

-- name: IncrementRequestCount :exec
UPDATE user_subscriptions 
SET requests_created_this_month = requests_created_this_month + 1,
    updated_at = NOW()
WHERE id = $1;

-- name: IncrementBidCount :exec
UPDATE user_subscriptions 
SET bids_placed_this_month = bids_placed_this_month + 1,
    updated_at = NOW()
WHERE id = $1;

-- name: IncrementOfferCount :exec
UPDATE user_subscriptions 
SET offers_made_this_month = offers_made_this_month + 1,
    updated_at = NOW()
WHERE id = $1;

-- name: ResetMonthlyCounts :exec
UPDATE user_subscriptions 
SET auctions_created_this_month = 0,
    requests_created_this_month = 0,
    bids_placed_this_month = 0,
    offers_made_this_month = 0,
    month_reset_at = NOW(),
    updated_at = NOW()
WHERE month_reset_at < DATE_TRUNC('month', NOW());

-- name: GetAllUserSubscriptions :many
-- Admin: lists ALL subscriptions for a user, active or not.
SELECT us.*, st.*
FROM user_subscriptions us
JOIN subscription_tiers st ON us.tier_name = st.tier_name
WHERE us.user_id = $1
ORDER BY us.is_active DESC, st.price_egp DESC;

-- name: GetUserSubscriptionByID :one
SELECT * FROM user_subscriptions WHERE id = $1;

-- name: GetUserSubscriptionByUserAndTier :one
SELECT * FROM user_subscriptions
WHERE user_id = $1 AND tier_name = $2;

-- name: UpdateUserSubscription :one
-- NOTE: clearing expires_at to NULL is not supported by COALESCE(narg, expires_at).
--       For perpetual subscriptions, send a far-future date from the frontend.
UPDATE user_subscriptions
SET started_at = COALESCE(sqlc.narg('started_at'),  started_at),
    expires_at = COALESCE(sqlc.narg('expires_at'),  expires_at),
    is_active  = COALESCE(sqlc.narg('is_active'),   is_active),
    is_primary = COALESCE(sqlc.narg('is_primary'),  is_primary),
    updated_at = NOW()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ReactivateUserSubscription :one
UPDATE user_subscriptions
SET is_active  = TRUE,
    started_at = $3,
    expires_at = $4,
    is_primary = $5,
    updated_at = NOW()
WHERE user_id = $1 AND tier_name = $2
RETURNING *;

-- name: DeactivateAllUserSubscriptions :exec
UPDATE user_subscriptions
SET is_active  = FALSE,
    is_primary = FALSE,
    updated_at = NOW()
WHERE user_id = $1 AND is_active = TRUE;
