-- Carrier-facing job feeds.
--
-- Every one of these deliberately omits the money: final_price, unit_price and
-- total_price never leave the database on a carrier request. Transport is priced
-- on weight, distance and goods type, so a carrier has no need for the value of
-- the cargo — and handing a third party the market's pricing is not recoverable
-- once done.

-- name: ListQuotableOrdersForCarrier :many
-- Deals that still need moving: both parties have a stake, no carrier chosen
-- yet, and at least one end sits in a governorate this carrier serves.
SELECT o.public_id, o.quantity, o.unit, o.status, o.created_at,
       o.seller_region, o.buyer_region,
       sa.title AS sell_title, br.title AS buy_title,
       si.name_ar AS sell_interest, bi.name_ar AS buy_interest,
       EXISTS (
           SELECT 1 FROM shipping_quotes q
           WHERE q.order_id = o.id AND q.carrier_id = @carrier_id::int
             AND q.status <> 'withdrawn'
       ) AS already_quoted
FROM orders o
LEFT JOIN sell_auctions sa ON sa.id = o.sell_auction_id
LEFT JOIN buy_requests br ON br.id = o.buy_request_id
LEFT JOIN interests si ON si.id = sa.interest_id
LEFT JOIN interests bi ON bi.id = br.interest_id
WHERE o.carrier_id IS NULL
  AND o.status IN ('created', 'seller_confirmed', 'buyer_confirmed', 'completed')
  AND (o.seller_region_id = ANY(@region_ids::int[]) OR o.buyer_region_id = ANY(@region_ids::int[]))
ORDER BY o.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountQuotableOrdersForCarrier :one
SELECT COUNT(*) FROM orders o
WHERE o.carrier_id IS NULL
  AND o.status IN ('created', 'seller_confirmed', 'buyer_confirmed', 'completed')
  AND (o.seller_region_id = ANY(@region_ids::int[]) OR o.buyer_region_id = ANY(@region_ids::int[]));

-- name: ListQuotableSellAuctionsForCarrier :many
-- Post-stage mode. The destination does not exist yet — no winner means no
-- buyer — so a quote here is indicative on the origin governorate alone.
SELECT sa.public_id, sa.title, sa.quantity, sa.unit, sa.end_time, sa.created_at,
       sa.region_name, i.name_ar AS interest_name,
       EXISTS (
           SELECT 1 FROM shipping_quotes q
           WHERE q.sell_auction_id = sa.id AND q.carrier_id = @carrier_id::int
             AND q.status <> 'withdrawn'
       ) AS already_quoted
FROM sell_auctions sa
JOIN interests i ON i.id = sa.interest_id
WHERE sa.status = 'active' AND sa.end_time > NOW()
  AND sa.region_id = ANY(@region_ids::int[])
ORDER BY sa.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountQuotableSellAuctionsForCarrier :one
SELECT COUNT(*) FROM sell_auctions sa
WHERE sa.status = 'active' AND sa.end_time > NOW()
  AND sa.region_id = ANY(@region_ids::int[]);

-- name: ListQuotableBuyRequestsForCarrier :many
SELECT br.public_id, br.title, br.quantity, br.unit, br.end_time, br.created_at,
       br.region_name, i.name_ar AS interest_name,
       EXISTS (
           SELECT 1 FROM shipping_quotes q
           WHERE q.buy_request_id = br.id AND q.carrier_id = @carrier_id::int
             AND q.status <> 'withdrawn'
       ) AS already_quoted
FROM buy_requests br
JOIN interests i ON i.id = br.interest_id
WHERE br.status = 'active' AND br.end_time > NOW()
  AND br.region_id = ANY(@region_ids::int[])
ORDER BY br.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountQuotableBuyRequestsForCarrier :one
SELECT COUNT(*) FROM buy_requests br
WHERE br.status = 'active' AND br.end_time > NOW()
  AND br.region_id = ANY(@region_ids::int[]);

-- Quotes.

-- name: CreateShippingQuote :one
INSERT INTO shipping_quotes (carrier_id, sell_auction_id, buy_request_id, order_id, price, notes)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: GetShippingQuoteByPublicID :one
SELECT * FROM shipping_quotes WHERE public_id = $1;

-- name: GetShippingQuoteByPublicIDForUpdate :one
SELECT * FROM shipping_quotes WHERE public_id = $1 FOR UPDATE;

-- name: ListShippingQuotesByCarrier :many
-- The carrier's own list, so their own prices are theirs to see. The job's title
-- comes from whichever target the quote points at.
SELECT q.*,
       COALESCE(sa.title, br.title, osa.title, obr.title, '') AS job_title,
       COALESCE(sa.region_name, br.region_name, o.seller_region, '') AS job_region,
       o.public_id AS order_public_id,
       sa.public_id AS sell_auction_public_id,
       br.public_id AS buy_request_public_id
FROM shipping_quotes q
LEFT JOIN sell_auctions sa ON sa.id = q.sell_auction_id
LEFT JOIN buy_requests br ON br.id = q.buy_request_id
LEFT JOIN orders o ON o.id = q.order_id
LEFT JOIN sell_auctions osa ON osa.id = o.sell_auction_id
LEFT JOIN buy_requests obr ON obr.id = o.buy_request_id
WHERE q.carrier_id = $1
ORDER BY q.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountShippingQuotesByCarrier :one
SELECT COUNT(*) FROM shipping_quotes WHERE carrier_id = $1;

-- name: WithdrawShippingQuote :one
-- Only a quote nobody has answered yet: withdrawing an accepted one would strip
-- a carrier off a deal the merchant has already committed to.
UPDATE shipping_quotes
SET status = 'withdrawn', updated_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- Merchant-facing.

-- name: ListShippingQuotesForOrder :many
SELECT q.*, u.name AS carrier_name, u.phone AS carrier_phone,
       cp.logo_url AS carrier_logo, cp.notes AS carrier_notes
FROM shipping_quotes q
JOIN users u ON u.id = q.carrier_id
LEFT JOIN carrier_profiles cp ON cp.user_id = q.carrier_id
WHERE q.order_id = $1 AND q.status <> 'withdrawn'
ORDER BY q.price ASC, q.created_at ASC;

-- name: ListShippingQuotesForSellAuction :many
SELECT q.*, u.name AS carrier_name, u.phone AS carrier_phone,
       cp.logo_url AS carrier_logo, cp.notes AS carrier_notes
FROM shipping_quotes q
JOIN users u ON u.id = q.carrier_id
LEFT JOIN carrier_profiles cp ON cp.user_id = q.carrier_id
WHERE q.sell_auction_id = $1 AND q.status <> 'withdrawn'
ORDER BY q.price ASC, q.created_at ASC;

-- name: ListShippingQuotesForBuyRequest :many
SELECT q.*, u.name AS carrier_name, u.phone AS carrier_phone,
       cp.logo_url AS carrier_logo, cp.notes AS carrier_notes
FROM shipping_quotes q
JOIN users u ON u.id = q.carrier_id
LEFT JOIN carrier_profiles cp ON cp.user_id = q.carrier_id
WHERE q.buy_request_id = $1 AND q.status <> 'withdrawn'
ORDER BY q.price ASC, q.created_at ASC;

-- name: AcceptShippingQuote :one
UPDATE shipping_quotes
SET status = 'accepted', accepted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: RejectShippingQuote :one
UPDATE shipping_quotes
SET status = 'rejected', updated_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: RejectSiblingShippingQuotes :many
-- Accepting one answers the rest. Returns the losers so each can be told, which
-- is the difference between a carrier knowing where it stands and being ignored.
UPDATE shipping_quotes
SET status = 'rejected', updated_at = NOW()
WHERE id <> @accepted_id::int
  AND status = 'pending'
  AND (
      (@order_id::int > 0 AND order_id = @order_id::int)
      OR (@sell_auction_id::int > 0 AND sell_auction_id = @sell_auction_id::int)
      OR (@buy_request_id::int > 0 AND buy_request_id = @buy_request_id::int)
  )
RETURNING id, carrier_id, public_id;

-- name: AttachCarrierToOrder :one
UPDATE orders
SET carrier_id = $2, shipping_price = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetAcceptedQuoteForOrder :one
SELECT q.*, u.name AS carrier_name, u.phone AS carrier_phone
FROM shipping_quotes q
JOIN users u ON u.id = q.carrier_id
WHERE q.order_id = $1 AND q.status = 'accepted';

-- name: GetAcceptedQuoteForPost :one
-- Post-stage acceptance happens before an order exists, so when the deal closes
-- the winning carrier has to be carried onto it — otherwise the order looks
-- unshipped and the carrier feed would offer it again.
SELECT * FROM shipping_quotes
WHERE status = 'accepted'
  AND ((@sell_auction_id::int > 0 AND sell_auction_id = @sell_auction_id::int)
    OR (@buy_request_id::int > 0 AND buy_request_id = @buy_request_id::int))
LIMIT 1;
