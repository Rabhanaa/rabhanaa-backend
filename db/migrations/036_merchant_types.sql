-- +goose Up

-- Client request #1: the merchant types offered at registration were only
-- trader / importer / processor / company. Adds the missing supply-side roles.
-- Explicit ids so existing users keep their job_id.
INSERT INTO jobs (id, key, name_ar, name_en, is_active) VALUES
    (5, 'wholesaler',  'تاجر جملة', 'Wholesaler',  true),
    (6, 'distributor', 'موزع',      'Distributor', true),
    (7, 'supplier',    'مورد',      'Supplier',    true)
ON CONFLICT (id) DO NOTHING;

-- Migration 010 seeded these tables with explicit ids and never advanced the
-- sequences, so they still pointed at 1 while rows 1..N existed — any insert
-- that let the sequence assign an id would fail on the primary key.
SELECT setval(pg_get_serial_sequence('jobs', 'id'), (SELECT MAX(id) FROM jobs));
SELECT setval(pg_get_serial_sequence('regions', 'id'), (SELECT MAX(id) FROM regions));
SELECT setval(pg_get_serial_sequence('interests', 'id'), (SELECT MAX(id) FROM interests));

-- Whether this merchant supplies retailers. Collected at registration for the
-- supply-side roles above; drives no permissions yet.
ALTER TABLE users ADD COLUMN supplies_to_retail BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down

ALTER TABLE users DROP COLUMN supplies_to_retail;

-- Only removes roles nobody registered under. Deleting one that is in use would
-- violate users.job_id and abort the whole rollback, so an in-use role is left
-- in place rather than reassigning merchants to something they did not choose.
DELETE FROM jobs
WHERE id IN (5, 6, 7)
  AND id NOT IN (SELECT job_id FROM users WHERE job_id IS NOT NULL);
