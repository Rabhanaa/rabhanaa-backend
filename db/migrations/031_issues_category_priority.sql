-- +goose Up
-- +goose StatementBegin
ALTER TABLE issues
  ADD COLUMN category VARCHAR(20) NOT NULL DEFAULT 'support'
    CHECK (category IN ('inquiry','support','problem','suggestion')),
  ADD COLUMN priority VARCHAR(10) NOT NULL DEFAULT 'normal'
    CHECK (priority IN ('low','normal','high','urgent'));

CREATE INDEX idx_issues_category ON issues(category);
CREATE INDEX idx_issues_priority ON issues(priority);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_issues_category;
DROP INDEX IF EXISTS idx_issues_priority;
ALTER TABLE issues DROP COLUMN category;
ALTER TABLE issues DROP COLUMN priority;
-- +goose StatementEnd
