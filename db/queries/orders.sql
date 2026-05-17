-- name: CreateOrderFromSellAuction :one
INSERT INTO orders (
    sell_auction_id, seller_id, buyer_id, final_price, quantity, unit,
    seller_name, seller_phone, seller_region,
    buyer_name, buyer_phone, buyer_region,
    source_public_id, status, seller_confirmed_at, confirmation_deadline
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9,
    $10, $11, $12,
    $13, 'seller_confirmed', NOW(), NOW() + INTERVAL '30 minutes'
) RETURNING *;

-- name: CreateOrderFromBuyRequest :one
INSERT INTO orders (
    buy_request_id, seller_id, buyer_id, final_price, quantity, unit,
    seller_name, seller_phone, seller_region,
    buyer_name, buyer_phone, buyer_region,
    source_public_id, status, buyer_confirmed_at, confirmation_deadline
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9,
    $10, $11, $12,
    $13, 'buyer_confirmed', NOW(), NOW() + INTERVAL '30 minutes'
) RETURNING *;

-- name: GetOrderByID :one
SELECT * FROM orders WHERE id = $1;

-- name: GetOrderByPublicID :one
SELECT * FROM orders WHERE public_id = $1;

-- name: ListOrdersByUser :many
SELECT * FROM orders WHERE seller_id = $1 OR buyer_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountOrdersByUser :one
SELECT COUNT(*) FROM orders WHERE seller_id = $1 OR buyer_id = $1;

-- name: ListOrdersBySellAuction :many
SELECT * FROM orders WHERE sell_auction_id = $1;

-- name: ListOrdersByBuyRequest :many
SELECT * FROM orders WHERE buy_request_id = $1;

-- name: ConfirmOrderAsSeller :exec
UPDATE orders 
SET status = 'completed', seller_confirmed_at = NOW(), completed_at = NOW(), updated_at = NOW()
WHERE id = $1 AND status = 'buyer_confirmed' AND confirmation_deadline > NOW();

-- name: ConfirmOrderAsBuyer :exec
UPDATE orders 
SET status = 'completed', buyer_confirmed_at = NOW(), completed_at = NOW(), updated_at = NOW()
WHERE id = $1 AND status = 'seller_confirmed' AND confirmation_deadline > NOW();

-- name: CompleteOrder :exec
UPDATE orders SET status = 'completed', completed_at = NOW(), updated_at = NOW() WHERE id = $1;

-- name: CheckOrderExistsForSellAuction :one
SELECT EXISTS(SELECT 1 FROM orders WHERE sell_auction_id = $1);

-- name: CheckOrderExistsForBuyRequestAndSupplier :one
SELECT EXISTS(SELECT 1 FROM orders WHERE buy_request_id = $1 AND seller_id = $2);

-- name: GetOrdersPendingConfirmation :many
SELECT * FROM orders 
WHERE confirmation_deadline < NOW() 
  AND status IN ('seller_confirmed', 'buyer_confirmed')
ORDER BY confirmation_deadline ASC;

-- name: CancelOrder :exec
UPDATE orders 
SET status = 'cancelled', cancelled_at = NOW(), updated_at = NOW()
WHERE id = $1 AND status IN ('seller_confirmed', 'buyer_confirmed');
