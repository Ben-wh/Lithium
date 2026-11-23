-- +goose Up
CREATE TABLE tickets (
    id           INTEGER PRIMARY KEY,
    subject      TEXT    NOT NULL,
    description  TEXT    NOT NULL,
    status       TEXT    NOT NULL DEFAULT 'open'   CHECK (status IN ('open', 'pending', 'resolved', 'closed')),
    priority     TEXT    NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    requester_id INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT, -- who the ticket is for
    created_by   INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT, -- who raised it (may be an agent on their behalf)
    assignee_id  INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX tickets_requester_idx ON tickets (requester_id, created_at);
CREATE INDEX tickets_assignee_idx  ON tickets (assignee_id, status);
CREATE INDEX tickets_status_idx    ON tickets (status, created_at);

-- +goose Down
DROP TABLE tickets;
