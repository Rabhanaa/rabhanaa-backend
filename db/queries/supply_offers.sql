-- name: CreateSupplyOffer :one
INSERT INTO supply_offers (buy_request_id, supplier_id, price_per_unit, offered_quantity, supplier_region_name, supplier_fake_name, request_title, request_quantity, request_unit)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING *;

-- name: GetSupplyOfferByID :one
SELECT * FROM supply_offers WHERE id = $1;

-- name: GetSupplyOfferByPublicID :one
SELECT * FROM supply_offers WHERE public_id = $1;

-- name: GetSupplyOfferByRequestAndSupplier :one
SELECT * FROM supply_offers WHERE buy_request_id = $1 AND supplier_id = $2;

-- name: ListSupplyOffersByRequest :many
SELECT 
    so.*,
    so.supplier_fake_name as supplier_name,
    so.supplier_region_name as supplier_region
FROM supply_offers so 
WHERE so.buy_request_id = $1
ORDER BY so.price_per_unit ASC, so.created_at ASC;

-- name: ListSupplyOffersBySupplier :many
SELECT so.*, br.title as request_title, br.public_id as request_public_id,
       br.status as request_status, br.end_time as request_end_time
FROM supply_offers so
JOIN buy_requests br ON br.id = so.buy_request_id
WHERE so.supplier_id = $1
ORDER BY so.created_at DESC LIMIT $2 OFFSET $3;

-- name: CountSupplyOffersByRequest :one
SELECT COUNT(*) FROM supply_offers WHERE buy_request_id = $1;

-- name: CountActiveSupplyOffersBySupplier :one
SELECT COUNT(*) FROM supply_offers so
JOIN buy_requests br ON br.id = so.buy_request_id
WHERE so.supplier_id = $1 AND br.status = 'active';

-- name: CountActiveSupplyOffersByUser :one
SELECT COUNT(*) FROM supply_offers so
JOIN buy_requests br ON br.id = so.buy_request_id
WHERE so.supplier_id = $1 AND br.status = 'active';

-- name: AcceptSupplyOffer :exec
UPDATE supply_offers SET is_accepted = TRUE, accepted_at = NOW() WHERE id = $1;

-- name: SumAcceptedQuantityByRequest :one
SELECT COALESCE(SUM(offered_quantity), 0)::DECIMAL(10,2) FROM supply_offers
WHERE buy_request_id = $1 AND is_accepted = TRUE;

-- name: ListAcceptedOffersByRequest :many
SELECT * FROM supply_offers WHERE buy_request_id = $1 AND is_accepted = TRUE;

-- name: MarkNotChosenSupplyOffers :exec
UPDATE supply_offers SET is_not_chosen = TRUE
WHERE buy_request_id = $1 AND is_accepted = FALSE;

-- name: GetUnacceptedSupplierIDsForRequest :many
SELECT DISTINCT supplier_id
FROM supply_offers
WHERE buy_request_id = $1
  AND is_accepted = FALSE;
