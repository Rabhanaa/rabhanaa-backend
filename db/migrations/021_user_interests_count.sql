-- +goose Up

-- Add interests_count column to users table for fast onboarding status checks
ALTER TABLE users ADD COLUMN interests_count INTEGER NOT NULL DEFAULT 0;

-- Index for faster queries on interests_count
CREATE INDEX idx_users_interests_count ON users(interests_count);

-- +goose Down

DROP INDEX IF EXISTS idx_users_interests_count;
ALTER TABLE users DROP COLUMN IF EXISTS interests_count;
