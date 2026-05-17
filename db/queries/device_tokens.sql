-- name: UpsertDeviceToken :exec
INSERT INTO device_tokens (user_id, token, platform)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, token) DO UPDATE SET is_active = TRUE, updated_at = NOW();

-- name: GetActiveDeviceTokensByUser :many
SELECT * FROM device_tokens WHERE user_id = $1 AND is_active = TRUE;

-- name: DeactivateDeviceToken :exec
UPDATE device_tokens SET is_active = FALSE, updated_at = NOW() WHERE token = $1;
