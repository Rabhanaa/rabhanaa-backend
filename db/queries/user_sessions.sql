-- name: CreateSession :one
INSERT INTO user_sessions (user_id, token_hash, device_info, ip_address, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM user_sessions WHERE token_hash = $1 AND is_current = TRUE;

-- name: InvalidateUserSessions :exec
UPDATE user_sessions SET is_current = FALSE WHERE user_id = $1 AND is_current = TRUE;

-- name: InvalidateSession :exec
UPDATE user_sessions SET is_current = FALSE WHERE id = $1;

-- name: UpdateSessionLastUsed :exec
UPDATE user_sessions SET last_used_at = NOW() WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM user_sessions WHERE expires_at < NOW();
