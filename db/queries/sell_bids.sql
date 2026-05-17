-- name: CreateSellBid :one
INSERT INTO sell_bids (auction_id, bidder_id, amount, bidder_region_name, bidder_fake_name, auction_title, auction_unit_price, auction_quantity, auction_unit)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING *;

-- name: GetSellBidByID :one
SELECT * FROM sell_bids WHERE id = $1;

-- name: GetSellBidByPublicID :one
SELECT * FROM sell_bids WHERE public_id = $1;

-- name: GetSellBidByAuctionAndBidder :one
SELECT * FROM sell_bids WHERE auction_id = $1 AND bidder_id = $2;

-- name: ListSellBidsByAuction :many
SELECT 
    sb.*,
    sb.bidder_fake_name as bidder_name,
    sb.bidder_region_name as bidder_region
FROM sell_bids sb 
WHERE sb.auction_id = $1
ORDER BY sb.amount DESC, sb.created_at ASC;

-- name: ListSellBidsByBidder :many
SELECT sb.*, sa.title as auction_title, sa.public_id as auction_public_id,
       sa.status as auction_status, sa.end_time as auction_end_time
FROM sell_bids sb
JOIN sell_auctions sa ON sa.id = sb.auction_id
WHERE sb.bidder_id = $1
ORDER BY sb.created_at DESC LIMIT $2 OFFSET $3;

-- name: CountSellBidsByAuction :one
SELECT COUNT(*) FROM sell_bids WHERE auction_id = $1;

-- name: CountActiveSellBidsByBidder :one
SELECT COUNT(*) FROM sell_bids sb
JOIN sell_auctions sa ON sa.id = sb.auction_id
WHERE sb.bidder_id = $1 AND sa.status = 'active';

-- name: CountActiveSellBidsByUser :one
SELECT COUNT(*) FROM sell_bids sb
JOIN sell_auctions sa ON sa.id = sb.auction_id
WHERE sb.bidder_id = $1 AND sa.status = 'active';

-- name: SelectSellBid :exec
UPDATE sell_bids SET is_selected = TRUE WHERE id = $1;

-- name: MarkNotChosenSellBids :exec
UPDATE sell_bids SET is_not_chosen = TRUE
WHERE auction_id = $1 AND is_selected = FALSE;

-- name: GetLosingBidderIDsForAuction :many
SELECT DISTINCT bidder_id
FROM sell_bids
WHERE auction_id = $1 AND id != $2;
