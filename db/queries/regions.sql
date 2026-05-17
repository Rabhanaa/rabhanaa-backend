-- name: ListRegions :many
SELECT * FROM regions WHERE is_active = TRUE ORDER BY name_ar;

-- name: GetRegionByID :one
SELECT * FROM regions WHERE id = $1;
