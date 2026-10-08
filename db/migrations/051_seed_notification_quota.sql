-- +goose Up

-- How many demo (seed) post notifications each member has had today. Demo
-- posts may notify a member only a few times a day, hours apart, in daytime
-- (see CronService.allowSeedNotification). One row per member; the count
-- resets when the Cairo date changes.
CREATE TABLE seed_notification_quota (
    user_id      INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    day          DATE NOT NULL,
    sent_today   INTEGER NOT NULL,
    last_sent_at TIMESTAMPTZ NOT NULL
);

-- +goose Down

DROP TABLE IF EXISTS seed_notification_quota;
