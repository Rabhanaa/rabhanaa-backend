-- +goose Up
-- Add cached name columns to users table
ALTER TABLE users 
    ADD COLUMN region_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN job_name VARCHAR(100) NOT NULL DEFAULT '';

-- Create index for faster lookups
CREATE INDEX idx_users_region_job_names ON users(region_name, job_name);

-- Backfill existing users
UPDATE users u
SET 
    region_name = COALESCE(r.name_ar, ''),
    job_name = COALESCE(j.name_ar, '')
FROM regions r, jobs j
WHERE u.region_id = r.id
    AND u.job_id = j.id;

-- +goose Down
DROP INDEX IF EXISTS idx_users_region_job_names;
ALTER TABLE users 
    DROP COLUMN IF EXISTS job_name,
    DROP COLUMN IF EXISTS region_name;
