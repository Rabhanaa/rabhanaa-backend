-- +goose Up

-- Removes the admin-typed carrier directory that 041 created. #14 was rebuilt
-- around carrier accounts (042), so these two tables have no reader left: the
-- /shipping-companies endpoints are gone and so is the panel on the create-post
-- form.
--
-- Held back deliberately. 042 leaves them in place so it can be applied to a
-- live database before the new backend deploys, and rolled back without loss.
-- Run this only once carrier accounts are confirmed working in production —
-- dropping a table is the one migration that a code rollback cannot undo.
--
-- If an admin populated the directory before this ships, export it first:
--   \copy (SELECT sc.name, sc.phone, sc.notes, r.name_ar
--          FROM shipping_companies sc
--          JOIN shipping_company_regions scr ON scr.shipping_company_id = sc.id
--          JOIN regions r ON r.id = scr.region_id
--          ORDER BY sc.name) TO 'carriers.csv' CSV HEADER
-- Those carriers then need real accounts; there is no automatic path, because a
-- directory row has no email and no password.

DROP TABLE IF EXISTS shipping_company_regions;
DROP TABLE IF EXISTS shipping_companies;

-- +goose Down

-- Recreated exactly as 041 defined them, empty. The rows themselves are gone;
-- see the export note above.
CREATE TABLE shipping_companies (
    id         SERIAL PRIMARY KEY,
    public_id  UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    name       VARCHAR(150) NOT NULL,
    phone      VARCHAR(20) NOT NULL,
    logo_url   VARCHAR(500),
    notes      TEXT,
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shipping_companies_active ON shipping_companies (is_active);

CREATE TABLE shipping_company_regions (
    shipping_company_id INTEGER NOT NULL REFERENCES shipping_companies(id) ON DELETE CASCADE,
    region_id           INTEGER NOT NULL REFERENCES regions(id),
    PRIMARY KEY (shipping_company_id, region_id)
);

CREATE INDEX idx_shipping_company_regions_region ON shipping_company_regions (region_id);
