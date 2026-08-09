-- name: CreateBuyRequest :one
INSERT INTO buy_requests (owner_id, region_id, interest_id, title, description, image_url, unit, quantity, buy_all_from_one, end_time, owner_name, region_name, interest_name)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetBuyRequestByID :one
SELECT * FROM buy_requests WHERE id = $1;

-- name: GetBuyRequestByPublicID :one
SELECT * FROM buy_requests WHERE public_id = $1;

-- name: GetBuyRequestByPublicIDForUpdate :one
SELECT * FROM buy_requests WHERE public_id = $1 FOR UPDATE;

-- name: ListActiveBuyRequests :many
SELECT br.* FROM buy_requests br
WHERE br.status = 'active' AND br.end_time > NOW()
  AND (@exclude_owner_id::int IS NULL OR br.owner_id != @exclude_owner_id::int)
  AND (@exclude_offered_requests::int[] IS NULL OR br.id NOT IN (
    SELECT so.buy_request_id FROM supply_offers so WHERE so.supplier_id = @user_id::int
  ))
ORDER BY
  CASE WHEN br.interest_id = ANY(@user_interest_ids::integer[]) THEN 0 ELSE 1 END,
  br.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountActiveBuyRequests :one
SELECT COUNT(*) FROM buy_requests
WHERE status = 'active' AND end_time > NOW()
  AND (@exclude_owner_id::int IS NULL OR owner_id != @exclude_owner_id::int)
  AND (@exclude_offered_requests::int[] IS NULL OR id NOT IN (
    SELECT so.buy_request_id FROM supply_offers so WHERE so.supplier_id = @user_id::int
  ));

-- name: SearchBuyRequests :many
SELECT br.* FROM buy_requests br
WHERE br.status = 'active' AND br.end_time > NOW()
  AND br.title ILIKE '%' || @search_term::text || '%'
  AND (@exclude_owner_id::int IS NULL OR br.owner_id != @exclude_owner_id::int)
  AND (@exclude_offered_requests::int[] IS NULL OR br.id NOT IN (
    SELECT so.buy_request_id FROM supply_offers so WHERE so.supplier_id = @user_id::int
  ))
ORDER BY
  CASE WHEN br.interest_id = ANY(@user_interest_ids::integer[]) THEN 0 ELSE 1 END,
  br.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountSearchBuyRequests :one
SELECT COUNT(*) FROM buy_requests
WHERE status = 'active' AND end_time > NOW()
  AND title ILIKE '%' || @search_term::text || '%'
  AND (@exclude_owner_id::int IS NULL OR owner_id != @exclude_owner_id::int)
  AND (@exclude_offered_requests::int[] IS NULL OR id NOT IN (
    SELECT so.buy_request_id FROM supply_offers so WHERE so.supplier_id = @user_id::int
  ));

-- name: ListBuyRequestsByOwner :many
SELECT * FROM buy_requests WHERE owner_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: CountBuyRequestsByOwner :one
SELECT COUNT(*) FROM buy_requests WHERE owner_id = $1;

-- name: UpdateBuyRequestStatus :exec
UPDATE buy_requests SET status = $2, updated_at = NOW() WHERE id = $1;

-- name: IncrementBuyRequestOfferCount :exec
UPDATE buy_requests SET offer_count = offer_count + 1, updated_at = NOW() WHERE id = $1;

-- name: IncrementBuyRequestAcceptedOfferCount :exec
UPDATE buy_requests SET accepted_offer_count = accepted_offer_count + 1, updated_at = NOW() WHERE id = $1;

-- name: UpdateBuyRequestFulfilledQuantity :exec
UPDATE buy_requests SET fulfilled_quantity = $2, updated_at = NOW() WHERE id = $1;

-- name: CancelBuyRequest :exec
UPDATE buy_requests SET status = 'cancelled', updated_at = NOW()
WHERE id = $1 AND status = 'active';

-- name: GetExpiredActiveBuyRequests :many
SELECT * FROM buy_requests WHERE status = 'active' AND end_time <= NOW();

-- name: GetExpiredPendingSelectionBuyRequests :many
SELECT * FROM buy_requests
WHERE status = 'pending_selection'
  AND end_time + (sqlc.arg(window_hours)::int * INTERVAL '1 hour') <= NOW();

-- name: GetSoonExpiringSelectionBuyRequests :many
-- Fires once, during the final hour before the selection deadline.
SELECT * FROM buy_requests
WHERE status = 'pending_selection'
  AND selection_warning_sent_at IS NULL
  AND NOW() >= end_time + (sqlc.arg(window_hours)::int * INTERVAL '1 hour') - INTERVAL '1 hour'
  AND NOW() <  end_time + (sqlc.arg(window_hours)::int * INTERVAL '1 hour');

-- name: MarkBuyRequestSelectionWarned :exec
UPDATE buy_requests SET selection_warning_sent_at = NOW() WHERE id = $1;

-- name: CountMonthlyBuyCancellations :one
SELECT COUNT(*) FROM buy_requests
WHERE owner_id = $1 AND status = 'cancelled'
  AND updated_at >= date_trunc('month', NOW());

-- name: RevertBuyRequestToPendingSelection :exec
UPDATE buy_requests 
SET status = 'pending_selection', accepted_offer_count = 0, fulfilled_quantity = 0, updated_at = NOW()
WHERE id = $1 AND status IN ('fulfilled', 'partially_fulfilled');

-- name: GetUnnotifiedActiveBuyRequests :many
SELECT * FROM buy_requests
WHERE status = 'active' AND notified_at IS NULL;

-- name: MarkBuyRequestNotified :exec
UPDATE buy_requests SET notified_at = NOW() WHERE id = $1;

-- name: GetMotivatableActiveBuyRequests :many
SELECT * FROM buy_requests
WHERE status = 'active'
  AND end_time > NOW()
  AND end_time <= NOW() + INTERVAL '30 minutes'
  AND (last_motivation_sent_at IS NULL OR last_motivation_sent_at < NOW() - INTERVAL '20 minutes');

-- name: MarkBuyRequestMotivated :exec
UPDATE buy_requests SET last_motivation_sent_at = NOW() WHERE id = $1;
