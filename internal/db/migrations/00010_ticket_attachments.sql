-- +goose Up
-- Files live on disk under <data dir>/attachments/<stored_name>; this table
-- records what they are and which ticket they belong to.
CREATE TABLE ticket_attachments (
    id           INTEGER PRIMARY KEY,
    ticket_id    INTEGER NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    filename     TEXT    NOT NULL, -- original name, for display only
    stored_name  TEXT    NOT NULL UNIQUE,
    content_type TEXT    NOT NULL,
    size         INTEGER NOT NULL,
    uploaded_by  INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at   INTEGER NOT NULL
);
CREATE INDEX ticket_attachments_ticket_idx ON ticket_attachments (ticket_id);

-- +goose Down
DROP TABLE ticket_attachments;
