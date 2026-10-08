-- +goose Up

-- The push a story is announced with. Separate from the title and summary so
-- it can be written to get a tap (shorter, emoji welcome) without changing how
-- the story reads. Empty falls back to the title and summary.
ALTER TABLE news
    ADD COLUMN notification_title VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN notification_body  VARCHAR(300) NOT NULL DEFAULT '';

-- One row per member a story was pushed to. was_pro is their plan at send
-- time, so the Pro / non-Pro split stays true after people upgrade or lapse.
CREATE TABLE news_deliveries (
    news_id   INTEGER NOT NULL REFERENCES news(id) ON DELETE CASCADE,
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    was_pro   BOOLEAN NOT NULL,
    sent_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- FCM accepted it for at least one of the member's devices. False also for
    -- members with no device registered.
    delivered BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (news_id, user_id)
);

-- One row per member who opened a story, however they got there.
CREATE TABLE news_views (
    news_id         INTEGER NOT NULL REFERENCES news(id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Opened it at least once from the push notification.
    from_push       BOOLEAN NOT NULL DEFAULT FALSE,
    -- Read it as a Pro member / was shown the upgrade prompt instead. Both
    -- can be true: a member who upgraded after hitting the prompt.
    read_as_pro     BOOLEAN NOT NULL DEFAULT FALSE,
    gated           BOOLEAN NOT NULL DEFAULT FALSE,
    read_to_end     BOOLEAN NOT NULL DEFAULT FALSE,
    upgrade_clicked BOOLEAN NOT NULL DEFAULT FALSE,
    view_count      INTEGER NOT NULL DEFAULT 1,
    first_viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_viewed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (news_id, user_id)
);

CREATE INDEX idx_news_views_news_last ON news_views (news_id, last_viewed_at DESC);

-- +goose Down

DROP TABLE IF EXISTS news_views;
DROP TABLE IF EXISTS news_deliveries;
ALTER TABLE news
    DROP COLUMN IF EXISTS notification_body,
    DROP COLUMN IF EXISTS notification_title;
