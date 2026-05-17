-- name: CreateLoginHistory :exec
INSERT INTO login_history (user_id, device_info, ip_address, success)
VALUES ($1, $2, $3, $4);

-- name: ListLoginHistoryByUser :many
SELECT * FROM login_history WHERE user_id = $1
ORDER BY login_at DESC LIMIT $2 OFFSET $3;
