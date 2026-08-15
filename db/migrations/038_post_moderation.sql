-- +goose Up

-- Client request #18: posts wait for an admin to approve them before going live,
-- and a live post can be taken down if there is a problem.
--
-- The two tables have different status vocabularies (sell has winner_selected,
-- buy has fulfilled/partially_fulfilled), so each constraint is recreated with
-- its own list plus the three shared moderation states.
ALTER TABLE sell_auctions DROP CONSTRAINT sell_auctions_status_check;
ALTER TABLE sell_auctions ADD CONSTRAINT sell_auctions_status_check
    CHECK (status IN (
        'active','cancelled','expired','pending_selection','winner_selected',
        'pending_approval','rejected','suspended'
    ));

ALTER TABLE buy_requests DROP CONSTRAINT buy_requests_status_check;
ALTER TABLE buy_requests ADD CONSTRAINT buy_requests_status_check
    CHECK (status IN (
        'active','cancelled','expired','pending_selection','fulfilled','partially_fulfilled',
        'pending_approval','rejected','suspended'
    ));

-- Why the post was rejected or suspended, and who did it. Mirrors the user
-- lifecycle columns added in 032.
ALTER TABLE sell_auctions
    ADD COLUMN moderation_reason     TEXT,
    ADD COLUMN moderated_by_admin_id INTEGER REFERENCES users(id),
    ADD COLUMN moderated_at          TIMESTAMPTZ;

ALTER TABLE buy_requests
    ADD COLUMN moderation_reason     TEXT,
    ADD COLUMN moderated_by_admin_id INTEGER REFERENCES users(id),
    ADD COLUMN moderated_at          TIMESTAMPTZ;

CREATE INDEX idx_sell_auctions_pending_approval ON sell_auctions (created_at)
    WHERE status = 'pending_approval';
CREATE INDEX idx_buy_requests_pending_approval ON buy_requests (created_at)
    WHERE status = 'pending_approval';

-- Deliberately no backfill: existing posts stay active. Moving live posts into
-- the review queue would empty the marketplace the moment this deploys.

-- +goose Down

-- Rows in one of the new statuses would violate the restored constraint, so
-- retire them first. pending_approval never went live, and rejected/suspended
-- were already withdrawn, so cancelled is the honest equivalent.
UPDATE sell_auctions SET status = 'cancelled'
    WHERE status IN ('pending_approval','rejected','suspended');
UPDATE buy_requests SET status = 'cancelled'
    WHERE status IN ('pending_approval','rejected','suspended');

DROP INDEX IF EXISTS idx_sell_auctions_pending_approval;
DROP INDEX IF EXISTS idx_buy_requests_pending_approval;

ALTER TABLE sell_auctions
    DROP COLUMN moderation_reason,
    DROP COLUMN moderated_by_admin_id,
    DROP COLUMN moderated_at;

ALTER TABLE buy_requests
    DROP COLUMN moderation_reason,
    DROP COLUMN moderated_by_admin_id,
    DROP COLUMN moderated_at;

ALTER TABLE sell_auctions DROP CONSTRAINT sell_auctions_status_check;
ALTER TABLE sell_auctions ADD CONSTRAINT sell_auctions_status_check
    CHECK (status IN ('active','cancelled','expired','pending_selection','winner_selected'));

ALTER TABLE buy_requests DROP CONSTRAINT buy_requests_status_check;
ALTER TABLE buy_requests ADD CONSTRAINT buy_requests_status_check
    CHECK (status IN ('active','cancelled','expired','pending_selection','fulfilled','partially_fulfilled'));
