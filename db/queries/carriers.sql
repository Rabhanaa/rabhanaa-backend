-- name: UpsertCarrierProfile :one
-- Registration and profile edits share one path: a carrier always has exactly
-- one profile row, so there is nothing to distinguish create from update.
INSERT INTO carrier_profiles (user_id, logo_url, notes)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
SET logo_url = EXCLUDED.logo_url,
    notes = EXCLUDED.notes,
    updated_at = NOW()
RETURNING *;

-- name: GetCarrierProfile :one
SELECT * FROM carrier_profiles WHERE user_id = $1;

-- name: ReplaceCarrierRegions :exec
-- Coverage is replaced wholesale rather than diffed: the edit form always sends
-- the complete set, and a partial update would leave stale governorates behind.
DELETE FROM carrier_regions WHERE user_id = $1;

-- name: AddCarrierRegion :exec
INSERT INTO carrier_regions (user_id, region_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListCarrierRegions :many
SELECT cr.region_id, r.name_ar, r.name_en
FROM carrier_regions cr
JOIN regions r ON r.id = cr.region_id
WHERE cr.user_id = $1
ORDER BY r.name_ar;

-- name: ListCarrierRegionIDs :many
SELECT region_id FROM carrier_regions WHERE user_id = $1;

-- name: CountCarrierRegions :one
SELECT COUNT(*) FROM carrier_regions WHERE user_id = $1;
