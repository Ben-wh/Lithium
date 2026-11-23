-- +goose Up
CREATE TABLE ticket_categories (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT    NOT NULL DEFAULT '',
    archived    INTEGER NOT NULL DEFAULT 0, -- hidden from new tickets, kept on old ones
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- Categories in use can't be deleted (the app archives instead); SET NULL is
-- only a safety net.
ALTER TABLE tickets ADD COLUMN category_id INTEGER REFERENCES ticket_categories (id) ON DELETE SET NULL;
CREATE INDEX tickets_category_idx ON tickets (category_id);

-- +goose Down
DROP INDEX tickets_category_idx;
ALTER TABLE tickets DROP COLUMN category_id;
DROP TABLE ticket_categories;
