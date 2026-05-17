-- name: ListInterests :many
SELECT * FROM interests WHERE is_active = TRUE ORDER BY name_ar;

-- name: GetInterestByID :one
SELECT * FROM interests WHERE id = $1;
