-- name: ListAppSettings :many
SELECT * FROM app_settings ORDER BY key;

-- name: GetAppSetting :one
SELECT * FROM app_settings WHERE key = $1;

-- name: UpsertAppSetting :one
-- Keys are whitelisted in the handler; this is not a general-purpose store.
INSERT INTO app_settings (key, value, updated_by_admin_id)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_by_admin_id = EXCLUDED.updated_by_admin_id,
    updated_at = NOW()
RETURNING *;
