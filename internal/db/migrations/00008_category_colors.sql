-- +goose Up
-- Stored as a palette name (not hex) so the actual shades can be tuned per theme in the UI.
ALTER TABLE ticket_categories ADD COLUMN color TEXT NOT NULL DEFAULT 'gray'
    CHECK (color IN ('gray', 'red', 'orange', 'amber', 'green', 'teal', 'blue', 'indigo', 'purple', 'pink'));

-- +goose Down
ALTER TABLE ticket_categories DROP COLUMN color;
