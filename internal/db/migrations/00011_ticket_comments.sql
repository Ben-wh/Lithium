-- +goose Up
-- The ticket's conversation: people's replies ('comment') and Lithium's
-- automatic updates when a ticket changes ('event').
CREATE TABLE ticket_comments (
    id         INTEGER PRIMARY KEY,
    ticket_id  INTEGER NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('comment', 'event')),
    author_id  INTEGER REFERENCES users (id) ON DELETE SET NULL, -- who wrote it, or who made the change
    body       TEXT    NOT NULL DEFAULT '',
    event      TEXT    NOT NULL DEFAULT '' CHECK (event IN ('', 'status', 'priority', 'category', 'assignee')),
    old_value  TEXT    NOT NULL DEFAULT '', -- status/priority keys; category/assignee names at the time
    new_value  TEXT    NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);
CREATE INDEX ticket_comments_ticket_idx ON ticket_comments (ticket_id, id);

-- +goose Down
DROP TABLE ticket_comments;
