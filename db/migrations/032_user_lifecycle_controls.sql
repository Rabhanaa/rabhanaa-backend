-- +goose Up

-- Extend status CHECK to include 'banned'.
-- Drop the existing constraint dynamically (name may vary by PG version;
-- default for inline column CHECK is 'users_status_check').
-- +goose StatementBegin
DO $$
DECLARE v_name text;
BEGIN
    SELECT conname INTO v_name
    FROM pg_constraint
    WHERE conrelid = 'users'::regclass AND contype = 'c' AND conname LIKE '%status%';
    IF v_name IS NOT NULL THEN
        EXECUTE 'ALTER TABLE users DROP CONSTRAINT ' || quote_ident(v_name);
    END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE users ADD CONSTRAINT users_status_check
    CHECK (status IN ('pending_documents','pending_review','active','rejected','suspended','banned'));

-- Add lifecycle tracking columns
ALTER TABLE users
    ADD COLUMN suspended_until            TIMESTAMPTZ NULL,
    ADD COLUMN suspension_reason          TEXT        NULL,
    ADD COLUMN banned_at                  TIMESTAMPTZ NULL,
    ADD COLUMN banned_reason              TEXT        NULL,
    ADD COLUMN status_changed_by_admin_id INTEGER     NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN status_changed_at          TIMESTAMPTZ NULL;

CREATE INDEX idx_users_suspended_until ON users(suspended_until) WHERE suspended_until IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_users_suspended_until;

ALTER TABLE users
    DROP COLUMN IF EXISTS status_changed_at,
    DROP COLUMN IF EXISTS status_changed_by_admin_id,
    DROP COLUMN IF EXISTS banned_reason,
    DROP COLUMN IF EXISTS banned_at,
    DROP COLUMN IF EXISTS suspension_reason,
    DROP COLUMN IF EXISTS suspended_until;

ALTER TABLE users DROP CONSTRAINT users_status_check;
ALTER TABLE users ADD CONSTRAINT users_status_check
    CHECK (status IN ('pending_documents','pending_review','active','rejected','suspended'));
