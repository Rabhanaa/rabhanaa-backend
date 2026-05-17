-- +goose Up

-- Add confirmation deadline and cancellation audit fields to orders table
ALTER TABLE orders 
ADD COLUMN confirmation_deadline TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '30 minutes'),
ADD COLUMN cancelled_at TIMESTAMPTZ;

-- Add index on confirmation_deadline for efficient cron queries
CREATE INDEX idx_orders_confirmation_deadline ON orders (confirmation_deadline) 
WHERE status IN ('seller_confirmed', 'buyer_confirmed');

-- Update existing orders that need confirmation
-- For orders in 'created' status, set deadline to 30 minutes from now
UPDATE orders 
SET confirmation_deadline = NOW() + INTERVAL '30 minutes'
WHERE status = 'created' 
  AND confirmation_deadline IS NULL;

-- For already completed/cancelled orders, set deadline to past time
UPDATE orders 
SET confirmation_deadline = created_at + INTERVAL '30 minutes'
WHERE status IN ('completed', 'cancelled')
  AND confirmation_deadline IS NULL;

-- +goose Down

-- Remove the index
DROP INDEX IF EXISTS idx_orders_confirmation_deadline;

-- Remove the added columns
ALTER TABLE orders 
DROP COLUMN IF EXISTS confirmation_deadline,
DROP COLUMN IF EXISTS cancelled_at;