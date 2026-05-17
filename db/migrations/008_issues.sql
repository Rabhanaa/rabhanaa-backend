-- +goose Up
CREATE TABLE issues (
    id          SERIAL PRIMARY KEY,
    public_id   UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    user_id     INTEGER NOT NULL REFERENCES users(id),
    title       VARCHAR(200) NOT NULL,
    description TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'open'
        CHECK (status IN ('open','replied','closed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_issues_user_id ON issues(user_id);
CREATE INDEX idx_issues_status ON issues(status);

CREATE TABLE issue_replies (
    id          SERIAL PRIMARY KEY,
    issue_id    INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    admin_id    INTEGER NOT NULL REFERENCES users(id),
    message     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_issue_replies_issue ON issue_replies(issue_id);

-- +goose Down
DROP TABLE IF EXISTS issue_replies;
DROP TABLE IF EXISTS issues;
