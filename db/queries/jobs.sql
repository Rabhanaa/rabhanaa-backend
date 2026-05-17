-- name: ListJobs :many
SELECT * FROM jobs WHERE is_active = TRUE ORDER BY name_ar;

-- name: GetJobByID :one
SELECT * FROM jobs WHERE id = $1;

-- name: GetJobByKey :one
SELECT * FROM jobs WHERE key = $1;
