-- name: CreateUser :one
INSERT INTO users (email, phone, password_hash, name, job_id, region_id, status, signup_source, supplies_to_retail)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: EnsureSeedUser :one
INSERT INTO users (
    email, name, status,
    email_verified, email_verified_at,
    phone, phone_verified,
    region_id, job_id,
    latitude, longitude
) VALUES (
    $1, $2, 'active',
    TRUE, NOW(),
    $3, TRUE,
    $4, $5,
    $6, $7
)
ON CONFLICT (email) DO UPDATE SET
    status = 'active',
    email_verified = TRUE,
    phone_verified = TRUE,
    updated_at = NOW()
RETURNING *;

-- name: GetUserByPublicID :one
SELECT * FROM users WHERE public_id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateUserStatus :exec
UPDATE users SET status = $2, updated_at = NOW() WHERE id = $1;

-- name: UpdateUserStatusWithReason :exec
UPDATE users SET status = $2, rejection_reason = $3, updated_at = NOW() WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1;

-- name: UpdateUserOTP :exec
UPDATE users SET otp_hash = $2, otp_expires_at = $3, updated_at = NOW() WHERE id = $1;

-- name: ClearUserOTP :exec
UPDATE users SET otp_hash = NULL, otp_expires_at = NULL, updated_at = NOW() WHERE id = $1;

-- name: UpdateUserFCMToken :exec
UPDATE users SET fcm_token = $2, updated_at = NOW() WHERE id = $1;

-- name: UpdateUserLocation :exec
UPDATE users SET latitude = $2, longitude = $3, updated_at = NOW() WHERE id = $1;

-- name: ListUsersByStatus :many
SELECT * FROM users WHERE status = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountUsersByStatus :one
SELECT COUNT(*) FROM users WHERE status = $1;

-- name: GetUserInterests :many
SELECT i.id, i.name_ar, i.name_en FROM interests i
JOIN user_interests ui ON ui.interest_id = i.id
WHERE ui.user_id = $1;

-- name: AddUserInterest :exec
INSERT INTO user_interests (user_id, interest_id) VALUES ($1, $2)
ON CONFLICT (user_id, interest_id) DO NOTHING;

-- name: DeleteUserInterests :exec
DELETE FROM user_interests WHERE user_id = $1;

-- name: GetUserInterestIDs :many
SELECT interest_id FROM user_interests WHERE user_id = $1;

-- name: UpdateUserProfile :exec
UPDATE users
SET job_id = $2, region_id = $3, updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserProfileWithNames :exec
UPDATE users
SET 
    job_id = $2,
    region_id = $3,
    job_name = COALESCE((SELECT j.name_ar FROM jobs j WHERE j.id = $2), ''),
    job_key = COALESCE((SELECT j.key FROM jobs j WHERE j.id = $2), ''),
    region_name = COALESCE((SELECT r.name_ar FROM regions r WHERE r.id = $3), ''),
    updated_at = NOW()
WHERE users.id = $1;

-- name: UpdateUserCachedNames :exec
UPDATE users
SET 
    job_name = COALESCE((SELECT j.name_ar FROM jobs j WHERE j.id = $2), ''),
    job_key = COALESCE((SELECT j.key FROM jobs j WHERE j.id = $2), ''),
    region_name = COALESCE((SELECT r.name_ar FROM regions r WHERE r.id = $3), ''),
    updated_at = NOW()
WHERE users.id = $1;

-- name: GetUserWithRegion :one
SELECT
    u.*,
    COALESCE(r.name_ar, '') as region_name
FROM users u
LEFT JOIN regions r ON r.id = u.region_id
WHERE u.id = $1;

-- name: GetUserWithRegionAndJob :one
SELECT
    u.*,
    COALESCE(r.name_ar, '') as region_name,
    COALESCE(j.name_ar, '') as job_name
FROM users u
LEFT JOIN regions r ON r.id = u.region_id
LEFT JOIN jobs j ON j.id = u.job_id
WHERE u.id = $1;

-- name: GetUserWithRegionAndJobByPublicID :one
SELECT
    u.*,
    COALESCE(r.name_ar, '') as region_name,
    COALESCE(j.name_ar, '') as job_name
FROM users u
LEFT JOIN regions r ON r.id = u.region_id
LEFT JOIN jobs j ON j.id = u.job_id
WHERE u.public_id = $1;

-- name: UpdateUserInterestsCount :exec
UPDATE users SET interests_count = $2, updated_at = NOW() WHERE id = $1;

-- name: GetUserStatusData :one
SELECT id, status, latitude, longitude, interests_count
FROM users WHERE id = $1;

-- name: HasActiveSubscription :one
SELECT EXISTS(
    SELECT 1 FROM user_subscriptions
    WHERE user_id = $1
      AND is_active = TRUE
      AND (expires_at IS NULL OR expires_at > NOW())
) AS has_active;

-- name: GetUserSubscriptionStatus :one
SELECT
    us.id,
    us.user_id,
    us.tier_name,
    us.started_at,
    us.expires_at,
    us.auctions_created_this_month,
    us.requests_created_this_month,
    us.bids_placed_this_month,
    us.offers_made_this_month,
    us.month_reset_at,
    us.is_active,
    us.is_primary,
    us.created_at,
    us.updated_at,
    st.max_sell_auctions_per_month,
    st.max_buy_requests_per_month,
    st.max_bids_per_month,
    st.max_offers_per_month,
    st.can_create_sell_auctions,
    st.can_create_buy_requests,
    st.can_place_bids,
    st.can_make_offers
FROM user_subscriptions us
JOIN subscription_tiers st ON st.tier_name = us.tier_name
WHERE us.user_id = $1 AND us.is_active = TRUE
ORDER BY us.is_primary DESC
LIMIT 1;

-- name: GetActiveUsersByInterest :many
SELECT u.id FROM users u
JOIN user_interests ui ON ui.user_id = u.id
WHERE ui.interest_id = @interest_id::int
  AND u.status = 'active'
  AND u.id != @exclude_user_id::int
  AND (@filter_region_id::int = 0 OR u.region_id = @filter_region_id::int)
  -- Nobody should be told about a post their feed hides: retailers cannot fill a
  -- buy request, and they only see sell posts from supply-side merchants.
  AND (@exclude_retailers::bool = false OR u.job_key <> 'retailer')
  -- Carriers never trade, so a new-listing notification has nothing in it for
  -- them. Belt and braces: they also pick no interests, so the join above should
  -- already exclude them — but that is a data accident, not a rule, and one
  -- stray interest row would start spamming them.
  AND u.job_key <> 'shipping_company';


-- name: ListAllUsersAnyStatus :many
SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CountAllUsersAnyStatus :one
SELECT COUNT(*) FROM users;

-- name: SearchUsers :many
SELECT * FROM users
WHERE (@query::text = '' OR
       email ILIKE '%' || @query::text || '%' OR
       phone ILIKE '%' || @query::text || '%' OR
       name  ILIKE '%' || @query::text || '%')
  AND (@status::text = '' OR status = @status::text)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: SearchUsersCount :one
SELECT COUNT(*) FROM users
WHERE (@query::text = '' OR
       email ILIKE '%' || @query::text || '%' OR
       phone ILIKE '%' || @query::text || '%' OR
       name  ILIKE '%' || @query::text || '%')
  AND (@status::text = '' OR status = @status::text);

-- name: SuspendUser :execrows
UPDATE users
SET status = 'suspended',
    suspension_reason = $2,
    suspended_until = $3,
    status_changed_by_admin_id = $4,
    status_changed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND status NOT IN ('banned');

-- name: UnsuspendUser :execrows
UPDATE users
SET status = 'active',
    suspension_reason = NULL,
    suspended_until = NULL,
    status_changed_by_admin_id = $2,
    status_changed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND status = 'suspended';

-- name: LazyRestoreExpiredSuspension :execrows
UPDATE users
SET status = 'active',
    suspension_reason = NULL,
    suspended_until = NULL,
    status_changed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND status = 'suspended'
  AND suspended_until IS NOT NULL
  AND suspended_until <= NOW();

-- name: BanUser :execrows
UPDATE users
SET status = 'banned',
    banned_at = NOW(),
    banned_reason = $2,
    suspension_reason = NULL,
    suspended_until = NULL,
    status_changed_by_admin_id = $3,
    status_changed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND status <> 'banned';

-- name: UnbanUser :execrows
UPDATE users
SET status = 'active',
    banned_at = NULL,
    banned_reason = NULL,
    status_changed_by_admin_id = $2,
    status_changed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND status = 'banned';

-- name: GetUserStatusByID :one
SELECT status, suspension_reason, banned_reason FROM users WHERE id = $1;

