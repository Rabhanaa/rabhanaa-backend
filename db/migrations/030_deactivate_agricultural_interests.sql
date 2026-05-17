-- +goose Up
UPDATE interests SET is_active = FALSE WHERE id IN (1, 2, 3, 8, 10, 11, 12);

-- +goose Down
UPDATE interests SET is_active = TRUE WHERE id IN (1, 2, 3, 8, 10, 11, 12);
