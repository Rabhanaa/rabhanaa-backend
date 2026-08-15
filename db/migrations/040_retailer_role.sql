-- +goose Up

-- Client request #7. Unlike the roles added in 036, this one carries behaviour:
-- a retailer's feed is limited to supply-side sellers, they cannot publish sell
-- posts, and their buy requests reach supply-side merchants only.
-- Explicit id so existing users keep their job_id.
INSERT INTO jobs (id, key, name_ar, name_en, is_active) VALUES
    (8, 'retailer', 'تاجر تجزئة', 'Retailer', true)
ON CONFLICT (id) DO NOTHING;

SELECT setval(pg_get_serial_sequence('jobs', 'id'), (SELECT MAX(id) FROM jobs));

-- The stable role identifier, cached on the user exactly like job_name in 022.
-- The frontend must branch on this rather than job_name, which is Arabic display
-- text, or job_id, which is a magic number. Caching keeps /auth/me — called on
-- every app load — free of an extra join.
ALTER TABLE users ADD COLUMN job_key VARCHAR(50) NOT NULL DEFAULT '';

UPDATE users u SET job_key = COALESCE(j.key, '')
FROM jobs j WHERE u.job_id = j.id;

-- +goose Down

ALTER TABLE users DROP COLUMN IF EXISTS job_key;

-- Same guard as 036/037: leave the role in place if anyone registered under it,
-- rather than aborting on the foreign key or reassigning those merchants.
DELETE FROM jobs
WHERE id = 8
  AND id NOT IN (SELECT job_id FROM users WHERE job_id IS NOT NULL);

SELECT setval(pg_get_serial_sequence('jobs', 'id'), (SELECT MAX(id) FROM jobs));
