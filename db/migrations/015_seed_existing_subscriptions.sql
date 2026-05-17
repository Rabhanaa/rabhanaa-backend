-- +goose Up

-- Give all existing active users the Zico tier (unlimited grandfather)
INSERT INTO user_subscriptions (user_id, tier_name, started_at, expires_at, is_active, is_primary)
SELECT id, 'zico', NOW(), NULL, TRUE, TRUE
FROM users 
WHERE status = 'active'
ON CONFLICT (user_id, tier_name) DO NOTHING;

-- Give pending users free tier (browse only until approved)
INSERT INTO user_subscriptions (user_id, tier_name, started_at, expires_at, is_active, is_primary)
SELECT id, 'free', NOW(), NULL, TRUE, TRUE
FROM users 
WHERE status IN ('pending_documents', 'pending_review', 'rejected')
ON CONFLICT (user_id, tier_name) DO NOTHING;

-- +goose Down
DELETE FROM user_subscriptions WHERE tier_name IN ('zico', 'free');
