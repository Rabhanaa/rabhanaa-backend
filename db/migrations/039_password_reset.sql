-- +goose Up

-- Client request #3: password reset by emailed code.
--
-- A dedicated table rather than reusing users.otp_hash: reset codes need
-- single-use consumption and per-code attempt counting, and keeping them
-- separate leaves the reset path independent of the OTP login path.
CREATE TABLE password_reset_codes (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash   VARCHAR(64) NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    attempts    INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Serves both the throttle (how many codes has this user asked for recently)
-- and the lookup for their most recent live code.
CREATE INDEX idx_password_reset_codes_user_created
    ON password_reset_codes (user_id, created_at DESC);

-- Revocation marker. middleware.Auth only checks the JWT signature and never
-- reads user_sessions, so flipping is_current revokes nothing — with a 365-day
-- token a stolen credential would outlive a password reset by a year. Tokens
-- carry IssuedAt, so anything minted before this timestamp is stale.
ALTER TABLE users ADD COLUMN password_changed_at TIMESTAMPTZ;

-- +goose Down

ALTER TABLE users DROP COLUMN password_changed_at;
DROP INDEX IF EXISTS idx_password_reset_codes_user_created;
DROP TABLE IF EXISTS password_reset_codes;
