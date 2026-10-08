-- name: ListInterests :many
SELECT * FROM interests WHERE is_active = TRUE ORDER BY name_ar;

-- name: GetInterestByID :one
SELECT * FROM interests WHERE id = $1;

-- name: CountActiveInterestsByIDs :one
SELECT COUNT(*) FROM interests WHERE id = ANY(@ids::int[]) AND is_active = TRUE;

-- name: ListActiveInterestsByIDs :many
SELECT * FROM interests WHERE id = ANY(@ids::int[]) AND is_active = TRUE ORDER BY name_ar;
