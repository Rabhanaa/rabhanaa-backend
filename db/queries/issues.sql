-- name: CreateIssue :one
INSERT INTO issues (user_id, title, description, category, priority)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIssueByID :one
SELECT * FROM issues WHERE id = $1;

-- name: GetIssueByPublicID :one
SELECT * FROM issues WHERE public_id = $1;

-- name: ListIssuesByUser :many
SELECT * FROM issues WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountOpenIssuesByUser :one
SELECT COUNT(*) FROM issues WHERE user_id = $1 AND status = 'open';

-- name: ListAllIssues :many
SELECT * FROM issues ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: UpdateIssueStatus :exec
UPDATE issues SET status = $2, updated_at = NOW() WHERE id = $1;

-- name: CloseIssueIfOpen :one
UPDATE issues SET status = 'closed', updated_at = NOW() WHERE public_id = $1 AND status <> 'closed' RETURNING id;

-- name: CreateIssueReply :one
INSERT INTO issue_replies (issue_id, admin_id, message) VALUES ($1, $2, $3) RETURNING *;

-- name: ListIssueReplies :many
SELECT * FROM issue_replies WHERE issue_id = $1 ORDER BY created_at ASC;

-- name: GetIssueAdminDetail :one
SELECT
    i.id, i.public_id, i.title, i.description, i.status, i.category, i.priority, i.created_at, i.updated_at,
    u.name AS user_name,
    u.email AS user_email,
    u.phone AS user_phone,
    u.region_name AS user_region
FROM issues i
JOIN users u ON u.id = i.user_id
WHERE i.public_id = $1;
