-- name: AnalyticsCountUsersByDay :many
SELECT date_trunc('day', created_at)::date AS bucket, COUNT(*)::bigint AS value
FROM users
WHERE created_at >= @from_time AND created_at < @to_time
GROUP BY 1 ORDER BY 1;

-- name: AnalyticsCountFailedLoginsByDay :many
SELECT date_trunc('day', login_at)::date AS bucket, COUNT(*)::bigint AS value
FROM login_history
WHERE success = false AND login_at >= @from_time AND login_at < @to_time
GROUP BY 1 ORDER BY 1;

-- name: AnalyticsCountActiveSessions :one
SELECT COUNT(*)::bigint AS count FROM user_sessions WHERE expires_at > now();

-- name: AnalyticsProfileCompletionRatio :one
SELECT
  COUNT(*) FILTER (WHERE status IN ('pending_review','active'))::bigint AS completed,
  COUNT(*)::bigint AS total
FROM users;

-- name: AnalyticsCountBidsByDay :many
SELECT bucket::date, SUM(value)::bigint AS value
FROM (
  SELECT date_trunc('day', sb.created_at) AS bucket, 1 AS value FROM sell_bids sb
   WHERE sb.created_at >= @from_time AND sb.created_at < @to_time
  UNION ALL
  SELECT date_trunc('day', so.created_at), 1 FROM supply_offers so
   WHERE so.created_at >= @from_time AND so.created_at < @to_time
) x GROUP BY bucket ORDER BY bucket;

-- name: AnalyticsCountClosedAuctionsByDay :many
SELECT date_trunc('day', end_time)::date AS bucket, COUNT(*)::bigint AS value
FROM sell_auctions
WHERE status IN ('expired','winner_selected') AND end_time >= @from_time AND end_time < @to_time
GROUP BY 1 ORDER BY 1;

-- name: AnalyticsOrdersGMVByDay :many
SELECT date_trunc('day', created_at)::date AS bucket,
       COUNT(*)::bigint AS orders,
       COALESCE(SUM(final_price * quantity), 0)::numeric AS gmv
FROM orders
WHERE created_at >= @from_time AND created_at < @to_time
GROUP BY 1 ORDER BY 1;

-- name: AnalyticsCountIssuesByDay :many
SELECT date_trunc('day', created_at)::date AS bucket, COUNT(*)::bigint AS value
FROM issues
WHERE created_at >= @from_time AND created_at < @to_time
GROUP BY 1 ORDER BY 1;

-- name: AnalyticsIssuesByStatus :many
SELECT status, COUNT(*)::bigint AS count
FROM issues
WHERE created_at >= @from_time AND created_at < @to_time
GROUP BY status ORDER BY count DESC;

-- name: AnalyticsActiveSubscriptionsByTier :many
SELECT t.tier_name, t.display_name_en, COUNT(us.id)::bigint AS count
FROM subscription_tiers t
LEFT JOIN user_subscriptions us ON us.tier_name = t.tier_name AND us.is_active = true
GROUP BY t.tier_name, t.display_name_en ORDER BY t.tier_name;

-- name: AnalyticsInactiveSubscriptionsCount :one
SELECT COUNT(*)::bigint AS count FROM user_subscriptions WHERE is_active = false;

-- name: AnalyticsCountBuyRequestsByDay :many
SELECT date_trunc('day', created_at)::date AS bucket, COUNT(*)::bigint AS value
FROM buy_requests
WHERE created_at >= @from_time AND created_at < @to_time
GROUP BY 1 ORDER BY 1;

-- name: AnalyticsUsersByStatus :many
SELECT status, COUNT(*)::bigint AS count FROM users GROUP BY status ORDER BY count DESC;

-- name: AnalyticsUsersBySource :many
SELECT signup_source AS source, COUNT(*)::bigint AS count
FROM users
WHERE created_at >= @from_time AND created_at < @to_time
GROUP BY signup_source
ORDER BY count DESC;

-- name: AnalyticsUsersBySourceByDay :many
SELECT date_trunc('day', created_at)::date AS bucket,
       signup_source AS source,
       COUNT(*)::bigint AS value
FROM users
WHERE created_at >= @from_time AND created_at < @to_time
GROUP BY bucket, signup_source
ORDER BY bucket, signup_source;

-- name: AnalyticsOverviewUsers :one
SELECT
  COUNT(*)::bigint AS total,
  COUNT(*) FILTER (WHERE created_at >= @from_time AND created_at < @to_time)::bigint AS new_in_range
FROM users;

-- name: AnalyticsOverviewOrders :one
SELECT
  COUNT(*)::bigint AS count,
  COALESCE(SUM(final_price * quantity), 0)::numeric AS gmv
FROM orders
WHERE created_at >= @from_time AND created_at < @to_time;

-- name: AnalyticsCountOpenIssues :one
SELECT COUNT(*)::bigint AS count FROM issues WHERE status = 'open';

-- name: AnalyticsUsersCountByInterest :many
SELECT
  i.id,
  i.name_ar,
  i.name_en,
  COUNT(ui.user_id)::bigint AS count
FROM interests i
LEFT JOIN user_interests ui ON ui.interest_id = i.id
WHERE i.is_active = TRUE
GROUP BY i.id, i.name_ar, i.name_en
ORDER BY count DESC, i.name_ar;

-- name: AnalyticsUsersByInterest :many
SELECT
  u.public_id,
  u.name,
  u.phone,
  u.status,
  u.created_at
FROM users u
JOIN user_interests ui ON ui.user_id = u.id
WHERE ui.interest_id = $1
ORDER BY u.created_at DESC;
