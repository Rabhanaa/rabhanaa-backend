-- +goose Up

-- News articles written by admins, usually drafted by AI from a source link.
-- Readable by Pro members only; publishing can push a notification to them.
CREATE TABLE news (
    id                  SERIAL PRIMARY KEY,
    public_id           UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    title               VARCHAR(200) NOT NULL,
    -- Doubles as the push notification body, hence the short limit.
    summary             VARCHAR(300) NOT NULL DEFAULT '',
    -- Sanitised on every write; never stored as the admin or AI sent it.
    body_html           TEXT NOT NULL DEFAULT '',
    cover_image_url     TEXT,
    source_url          TEXT,
    source_name         VARCHAR(200),
    -- The text extracted from source_url, kept so the article can be
    -- regenerated (or written by another provider) without fetching the page again.
    source_text         TEXT,
    status              VARCHAR(20) NOT NULL DEFAULT 'draft'
                            CHECK (status IN ('draft', 'published')),
    ai_provider         VARCHAR(20),
    ai_model            VARCHAR(128),
    created_by_admin_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    published_at        TIMESTAMPTZ,
    -- Set once, when the notification goes out, so neither a second publish
    -- nor an edit after publishing can notify members twice.
    notified_at         TIMESTAMPTZ,
    notified_count      INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_news_published ON news (published_at DESC) WHERE status = 'published';
CREATE INDEX idx_news_status_updated ON news (status, updated_at DESC);

-- Which interests a story is about. Drives the "Pro members by interest"
-- notification mode.
CREATE TABLE news_interests (
    news_id     INTEGER NOT NULL REFERENCES news(id) ON DELETE CASCADE,
    interest_id INTEGER NOT NULL REFERENCES interests(id),
    PRIMARY KEY (news_id, interest_id)
);

CREATE INDEX idx_news_interests_interest ON news_interests (interest_id);

-- When each member last opened the news list, for the unread badge on the
-- profile. News pushes stay out of the in-app notification list, so this is
-- the only way a member without push enabled finds out about new stories.
CREATE TABLE news_seen (
    user_id      INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down

DROP TABLE IF EXISTS news_seen;
DROP TABLE IF EXISTS news_interests;
DROP TABLE IF EXISTS news;
