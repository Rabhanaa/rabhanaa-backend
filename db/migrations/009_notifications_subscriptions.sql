-- +goose Up

CREATE TABLE device_tokens (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       VARCHAR(500) NOT NULL,
    platform    VARCHAR(20) NOT NULL DEFAULT 'web',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, token)
);

CREATE INDEX idx_device_tokens_user ON device_tokens(user_id);

CREATE TABLE notifications (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       VARCHAR(200) NOT NULL,
    body        TEXT NOT NULL,
    event_type  VARCHAR(50) NOT NULL,
    data        JSONB,
    is_read     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user ON notifications(user_id, created_at DESC);

CREATE TABLE notification_preferences (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    push_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE subscriptions (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER UNIQUE NOT NULL REFERENCES users(id),
    tier        VARCHAR(20) NOT NULL DEFAULT 'free'
        CHECK (tier IN ('free','basic','premium')),
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_user ON subscriptions(user_id);

CREATE TABLE referral_codes (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER UNIQUE NOT NULL REFERENCES users(id),
    code            VARCHAR(20) UNIQUE NOT NULL,
    discount_percent DECIMAL(5,2) NOT NULL DEFAULT 50.00,
    usage_count     INTEGER NOT NULL DEFAULT 0,
    max_uses        INTEGER,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_referral_codes_code ON referral_codes(code);

CREATE TABLE referral_usages (
    id              SERIAL PRIMARY KEY,
    referral_code_id INTEGER NOT NULL REFERENCES referral_codes(id),
    referred_user_id INTEGER NOT NULL REFERENCES users(id),
    discount_applied DECIMAL(12,2),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(referral_code_id, referred_user_id)
);

CREATE TABLE error_logs (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER REFERENCES users(id),
    error_type  VARCHAR(100),
    message     TEXT,
    stack_trace TEXT,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS error_logs;
DROP TABLE IF EXISTS referral_usages;
DROP TABLE IF EXISTS referral_codes;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS device_tokens;
