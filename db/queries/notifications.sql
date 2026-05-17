-- name: CreateNotification :one
INSERT INTO notifications (user_id, title, body, event_type, data)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListNotificationsByUser :many
SELECT * FROM notifications WHERE user_id = $1
ORDER BY created_at DESC LIMIT $2;

-- name: MarkNotificationRead :exec
UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET is_read = TRUE WHERE user_id = $1 AND is_read = FALSE;

-- name: DeleteOldNotifications :exec
DELETE FROM notifications n1
WHERE n1.user_id = $1
AND n1.id NOT IN (
    SELECT n2.id FROM notifications n2
    WHERE n2.user_id = $1
    ORDER BY n2.created_at DESC
    LIMIT @keep_count::integer
);

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE;
