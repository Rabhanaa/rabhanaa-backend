-- +goose Up
-- +goose StatementBegin

-- supplies_to_retail (036) has been collected at registration but never read:
-- retailer feeds filtered on job_key alone, so a merchant who left the box
-- unchecked was still fully visible. The sell-auction queries now honour it.
--
-- That flips the default for everyone who registered before the question
-- existed, or who registered as a supply-side merchant and simply left the box
-- alone: they would all disappear from every retailer feed at once. So opt in
-- the merchants who were never meaningfully asked. Enforcement then applies
-- only to answers actually given.
--
-- The affected ids are recorded rather than derived, because after the UPDATE
-- there is no way to tell a backfilled row from one that was always true — the
-- down migration would otherwise have to opt everyone out.
CREATE TABLE supplies_to_retail_backfill (
    user_id    INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO supplies_to_retail_backfill (user_id)
SELECT u.id
FROM users u
JOIN jobs j ON j.id = u.job_id
WHERE u.supplies_to_retail = FALSE
  -- Kept in step with SupplySideRoles in auction/service/sell_auction_service.go
  -- and SUPPLY_SIDE_ROLES in the frontend RegisterPage.
  AND j.key IN ('importer', 'wholesaler', 'distributor', 'processor', 'supplier');

UPDATE users
SET supplies_to_retail = TRUE,
    updated_at         = NOW()
WHERE id IN (SELECT user_id FROM supplies_to_retail_backfill);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE users
SET supplies_to_retail = FALSE,
    updated_at         = NOW()
WHERE id IN (SELECT user_id FROM supplies_to_retail_backfill);

DROP TABLE supplies_to_retail_backfill;

-- +goose StatementEnd
