-- +goose Up
ALTER TABLE users
    ADD COLUMN signup_source VARCHAR(32) NOT NULL DEFAULT 'direct';

-- +goose Down
ALTER TABLE users DROP COLUMN signup_source;
