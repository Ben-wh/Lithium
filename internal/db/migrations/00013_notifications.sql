-- +goose Up
-- Staff-only notes in a ticket's conversation, hidden from the requester.
ALTER TABLE ticket_comments ADD COLUMN internal INTEGER NOT NULL DEFAULT 0;

-- In-app notifications about tickets someone is involved in.
CREATE TABLE notifications (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,   -- who it's for
    ticket_id  INTEGER NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    actor_id   INTEGER REFERENCES users (id) ON DELETE SET NULL,           -- who caused it
    kind       TEXT    NOT NULL CHECK (kind IN ('reply', 'note', 'mention', 'update', 'assigned')),
    summary    TEXT    NOT NULL, -- e.g. "changed the status to Resolved" (actor shown separately)
    read_at    INTEGER,
    created_at INTEGER NOT NULL
);
CREATE INDEX notifications_user_idx ON notifications (user_id, id);
CREATE INDEX notifications_unread_idx ON notifications (user_id, read_at);

-- +goose Down
DROP TABLE notifications;
ALTER TABLE ticket_comments DROP COLUMN internal;
