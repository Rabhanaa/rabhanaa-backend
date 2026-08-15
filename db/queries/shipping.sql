-- name: ListShippingCompaniesByRegion :many
-- What a merchant sees while creating a post: active carriers that actually
-- serve the post's governorate.
SELECT sc.* FROM shipping_companies sc
JOIN shipping_company_regions scr ON scr.shipping_company_id = sc.id
WHERE sc.is_active = TRUE AND scr.region_id = $1
ORDER BY sc.name;

-- name: ListAllShippingCompanies :many
-- Admin view: everything, including deactivated carriers.
SELECT * FROM shipping_companies ORDER BY is_active DESC, name;

-- name: GetShippingCompanyByPublicID :one
SELECT * FROM shipping_companies WHERE public_id = $1;

-- name: CreateShippingCompany :one
INSERT INTO shipping_companies (name, phone, logo_url, notes)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateShippingCompany :one
UPDATE shipping_companies
SET name = $2, phone = $3, logo_url = $4, notes = $5, is_active = $6, updated_at = NOW()
WHERE public_id = $1
RETURNING *;

-- name: DeactivateShippingCompany :exec
-- Soft delete: a merchant may already have written the number down, and the
-- admin's own history should not lose the record.
UPDATE shipping_companies SET is_active = FALSE, updated_at = NOW() WHERE public_id = $1;

-- name: SetShippingCompanyRegions :exec
-- Coverage is replaced wholesale rather than added and removed one at a time —
-- it is a checkbox list in practice. Paired with ClearShippingCompanyRegions.
INSERT INTO shipping_company_regions (shipping_company_id, region_id)
SELECT $1, unnest(@region_ids::int[])
ON CONFLICT DO NOTHING;

-- name: ClearShippingCompanyRegions :exec
DELETE FROM shipping_company_regions WHERE shipping_company_id = $1;

-- name: ListShippingCompanyRegions :many
SELECT r.id, r.name_ar
FROM shipping_company_regions scr
JOIN regions r ON r.id = scr.region_id
WHERE scr.shipping_company_id = $1
ORDER BY r.id;
