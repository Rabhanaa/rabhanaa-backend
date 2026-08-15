-- +goose Up

-- Client request #14: a curated directory of transport companies, shown to a
-- merchant before they publish a post so they know who can move the goods.
--
-- Carriers are records, not users — there is no login, no quoting and no link
-- to an order. An admin maintains the list; merchants read it and call the
-- number.
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

-- Which governorates a carrier actually serves. A carrier with no rows here
-- appears for nobody, which is the predictable reading — the admin form
-- requires at least one so it cannot happen by accident.
CREATE TABLE shipping_company_regions (
    shipping_company_id INTEGER NOT NULL REFERENCES shipping_companies(id) ON DELETE CASCADE,
    region_id           INTEGER NOT NULL REFERENCES regions(id),
    PRIMARY KEY (shipping_company_id, region_id)
);

-- Every merchant-facing lookup starts from the region.
CREATE INDEX idx_shipping_company_regions_region ON shipping_company_regions (region_id);

-- +goose Down

DROP TABLE IF EXISTS shipping_company_regions;
DROP TABLE IF EXISTS shipping_companies;
