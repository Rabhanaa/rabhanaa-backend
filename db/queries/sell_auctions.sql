-- name: CreateSellAuction :one
INSERT INTO sell_auctions (owner_id, region_id, interest_id, title, description, image_url, unit, quantity, unit_price, buy_all_from_one, end_time, owner_name, region_name, interest_name)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: GetSellAuctionByID :one
SELECT * FROM sell_auctions WHERE id = $1;

-- name: GetSellAuctionByPublicID :one
SELECT * FROM sell_auctions WHERE public_id = $1;

-- name: GetSellAuctionByPublicIDForUpdate :one
SELECT * FROM sell_auctions WHERE public_id = $1 FOR UPDATE;

-- name: ListActiveSellAuctions :many
SELECT sa.* FROM sell_auctions sa
WHERE sa.status = 'active' AND sa.end_time > NOW()
  AND (@exclude_owner_id::int IS NULL OR sa.owner_id != @exclude_owner_id::int)
  AND (@exclude_bidded_auctions::int[] IS NULL OR sa.id NOT IN (
    SELECT sb.auction_id FROM sell_bids sb WHERE sb.bidder_id = @user_id::int
  ))
ORDER BY
  CASE WHEN sa.interest_id = ANY(@user_interest_ids::integer[]) THEN 0 ELSE 1 END,
  sa.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountActiveSellAuctions :one
SELECT COUNT(*) FROM sell_auctions
WHERE status = 'active' AND end_time > NOW()
  AND (@exclude_owner_id::int IS NULL OR owner_id != @exclude_owner_id::int)
  AND (@exclude_bidded_auctions::int[] IS NULL OR id NOT IN (
    SELECT sb.auction_id FROM sell_bids sb WHERE sb.bidder_id = @user_id::int
  ));

-- name: SearchSellAuctions :many
SELECT sa.* FROM sell_auctions sa
WHERE sa.status = 'active' AND sa.end_time > NOW()
  AND sa.title ILIKE '%' || @search_term::text || '%'
  AND (@exclude_owner_id::int IS NULL OR sa.owner_id != @exclude_owner_id::int)
  AND (@exclude_bidded_auctions::int[] IS NULL OR sa.id NOT IN (
    SELECT sb.auction_id FROM sell_bids sb WHERE sb.bidder_id = @user_id::int
  ))
ORDER BY
  CASE WHEN sa.interest_id = ANY(@user_interest_ids::integer[]) THEN 0 ELSE 1 END,
  sa.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountSearchSellAuctions :one
SELECT COUNT(*) FROM sell_auctions
WHERE status = 'active' AND end_time > NOW()
  AND title ILIKE '%' || @search_term::text || '%'
  AND (@exclude_owner_id::int IS NULL OR owner_id != @exclude_owner_id::int)
  AND (@exclude_bidded_auctions::int[] IS NULL OR id NOT IN (
    SELECT sb.auction_id FROM sell_bids sb WHERE sb.bidder_id = @user_id::int
  ));

-- name: ListSellAuctionsByOwner :many
SELECT * FROM sell_auctions WHERE owner_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountSellAuctionsByOwner :one
SELECT COUNT(*) FROM sell_auctions WHERE owner_id = $1;

-- name: UpdateSellAuctionStatus :exec
UPDATE sell_auctions SET status = $2, updated_at = NOW() WHERE id = $1;

-- name: IncrementSellAuctionBidCount :exec
UPDATE sell_auctions SET bid_count = bid_count + 1, updated_at = NOW() WHERE id = $1;

-- name: SelectSellWinner :exec
UPDATE sell_auctions
SET selected_bid_id = $2, winner_id = $3, final_price = $4, status = 'winner_selected', updated_at = NOW()
WHERE id = $1;

-- name: CancelSellAuction :exec
UPDATE sell_auctions SET status = 'cancelled', updated_at = NOW()
WHERE id = $1 AND status = 'active';

-- name: GetExpiredActiveSellAuctions :many
SELECT * FROM sell_auctions WHERE status = 'active' AND end_time <= NOW();

-- name: GetExpiredPendingSelectionSellAuctions :many
SELECT * FROM sell_auctions
WHERE status = 'pending_selection'
  AND end_time + (sqlc.arg(window_hours)::int * INTERVAL '1 hour') <= NOW();

-- name: GetSoonExpiringSelectionSellAuctions :many
-- Fires once, during the final hour before the selection deadline.
SELECT * FROM sell_auctions
WHERE status = 'pending_selection'
  AND selection_warning_sent_at IS NULL
  AND NOW() >= end_time + (sqlc.arg(window_hours)::int * INTERVAL '1 hour') - INTERVAL '1 hour'
  AND NOW() <  end_time + (sqlc.arg(window_hours)::int * INTERVAL '1 hour');

-- name: MarkSellAuctionSelectionWarned :exec
UPDATE sell_auctions SET selection_warning_sent_at = NOW() WHERE id = $1;

-- name: CountMonthlySellCancellations :one
SELECT COUNT(*) FROM sell_auctions
WHERE owner_id = $1 AND status = 'cancelled'
  AND updated_at >= date_trunc('month', NOW());

-- name: RevertSellAuctionToPendingSelection :exec
UPDATE sell_auctions 
SET status = 'pending_selection', selected_bid_id = NULL, winner_id = NULL, final_price = NULL, updated_at = NOW()
WHERE id = $1 AND status = 'winner_selected';

-- name: GetUnnotifiedActiveSellAuctions :many
SELECT * FROM sell_auctions
WHERE status = 'active' AND notified_at IS NULL;

-- name: MarkSellAuctionNotified :exec
UPDATE sell_auctions SET notified_at = NOW() WHERE id = $1;

-- name: GetMotivatableActiveSellAuctions :many
SELECT * FROM sell_auctions
WHERE status = 'active'
  AND end_time > NOW()
  AND end_time <= NOW() + INTERVAL '30 minutes'
  AND (last_motivation_sent_at IS NULL OR last_motivation_sent_at < NOW() - INTERVAL '20 minutes');

-- name: MarkSellAuctionMotivated :exec
UPDATE sell_auctions SET last_motivation_sent_at = NOW() WHERE id = $1;
