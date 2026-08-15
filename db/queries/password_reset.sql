-- name: CreatePasswordResetCode :one
INSERT INTO password_reset_codes (user_id, code_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetLivePasswordResetCode :one
-- The user's most recent code that is still usable: not consumed, not expired,
-- and not yet burned through its attempt allowance.
SELECT * FROM password_reset_codes
WHERE user_id = @user_id::int
  AND consumed_at IS NULL
  AND expires_at > NOW()
  AND attempts < @max_attempts::int
ORDER BY created_at DESC
LIMIT 1;

-- name: IncrementPasswordResetAttempts :exec
UPDATE password_reset_codes SET attempts = attempts + 1 WHERE id = $1;

-- name: ConsumePasswordResetCode :exec
UPDATE password_reset_codes SET consumed_at = NOW() WHERE id = $1;

-- name: InvalidatePasswordResetCodes :exec
-- After a successful reset, retire every other outstanding code for the user.
UPDATE password_reset_codes SET consumed_at = NOW()
WHERE user_id = $1 AND consumed_at IS NULL;

-- name: CountRecentPasswordResetCodes :one
-- Throttle input: how many codes this user has requested since a cutoff.
SELECT COUNT(*) FROM password_reset_codes
WHERE user_id = @user_id::int AND created_at > @since::timestamptz;

-- name: MarkPasswordChanged :exec
-- Revocation marker — tokens issued before this are rejected downstream.
UPDATE users SET password_changed_at = NOW(), updated_at = NOW() WHERE id = $1;
