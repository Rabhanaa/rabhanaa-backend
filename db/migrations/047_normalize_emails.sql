-- +goose Up

-- Emails were stored exactly as typed, against a case-sensitive unique index.
-- Two consequences, both live on production:
--
--   1. Registering a case-variant of an existing address passed the duplicate
--      check and then violated the index, surfacing to the user as an
--      unexplained "unexpected error" instead of "this address is taken".
--   2. An account stored with capitals could only sign in by reproducing that
--      capitalisation, and its password reset silently did nothing — that
--      endpoint reports success whether or not the address was found, so the
--      user waits for an email that was never sent.
--
-- The code now normalises on register, login and both reset paths. This aligns
-- the rows already stored; without it those accounts would match nothing at all
-- once lookups are normalised, turning a partial problem into a total lockout.
-- Run before deploying that code.
UPDATE users
SET email      = LOWER(email),
    updated_at = NOW()
WHERE email <> LOWER(email);

-- Guard against the case this makes possible: if two accounts differed only by
-- capitalisation, the update above would collide with the unique index and this
-- migration would fail loudly rather than merge two people's accounts. Verified
-- clean on production before writing this, but the check belongs here for every
-- other environment.
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_key ON users (LOWER(email));

-- +goose Down

DROP INDEX IF EXISTS users_email_lower_key;

-- Lowercasing is not meaningfully reversible, and deliberately not reversed:
-- capitalisation carries no information in an email address, and restoring it
-- would recreate the lockout this migration exists to fix.
