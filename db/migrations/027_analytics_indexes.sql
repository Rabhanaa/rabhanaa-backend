-- +goose Up
-- +goose NO TRANSACTION
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at         ON users (created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_status             ON users (status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_login_history_login      ON login_history (login_at) WHERE success = false;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_sessions_expires    ON user_sessions (expires_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sell_bids_created_at     ON sell_bids (created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_supply_offers_created_at ON supply_offers (created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sell_auctions_status_end ON sell_auctions (status, end_time);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_orders_created_at        ON orders (created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issues_created_at        ON issues (created_at);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issues_status_idx        ON issues (status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_subs_active_tier    ON user_subscriptions (tier_name) WHERE is_active = true;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_buy_requests_created_at  ON buy_requests (created_at);

-- +goose Down
-- +goose NO TRANSACTION
DROP INDEX CONCURRENTLY IF EXISTS idx_users_created_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_users_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_login_history_login;
DROP INDEX CONCURRENTLY IF EXISTS idx_user_sessions_expires;
DROP INDEX CONCURRENTLY IF EXISTS idx_sell_bids_created_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_supply_offers_created_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_sell_auctions_status_end;
DROP INDEX CONCURRENTLY IF EXISTS idx_orders_created_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_issues_created_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_issues_status_idx;
DROP INDEX CONCURRENTLY IF EXISTS idx_user_subs_active_tier;
DROP INDEX CONCURRENTLY IF EXISTS idx_buy_requests_created_at;
