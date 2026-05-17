-- +goose Up

CREATE TABLE regions (
    id          SERIAL PRIMARY KEY,
    name_ar     VARCHAR(100) NOT NULL,
    name_en     VARCHAR(100),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE interests (
    id          SERIAL PRIMARY KEY,
    name_ar     VARCHAR(100) NOT NULL,
    name_en     VARCHAR(100),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE jobs (
    id          SERIAL PRIMARY KEY,
    key         VARCHAR(50) UNIQUE NOT NULL,
    name_ar     VARCHAR(100) NOT NULL,
    name_en     VARCHAR(100),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE users ADD CONSTRAINT fk_users_job_id FOREIGN KEY (job_id) REFERENCES jobs(id);
ALTER TABLE users ADD CONSTRAINT fk_users_region_id FOREIGN KEY (region_id) REFERENCES regions(id);
ALTER TABLE user_interests ADD CONSTRAINT fk_user_interests_interest_id FOREIGN KEY (interest_id) REFERENCES interests(id);

-- +goose Down
ALTER TABLE user_interests DROP CONSTRAINT IF EXISTS fk_user_interests_interest_id;
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_region_id;
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_job_id;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS interests;
DROP TABLE IF EXISTS regions;
