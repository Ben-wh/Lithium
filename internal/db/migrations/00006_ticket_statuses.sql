-- Replaces the 'open' status with 'unassigned' and 'in_progress'. SQLite can't
-- alter a CHECK constraint, so the table is rebuilt. Nothing references
-- tickets yet, so no foreign key juggling is needed.

-- +goose Up
CREATE TABLE tickets_new (
    id           INTEGER PRIMARY KEY,
    subject      TEXT    NOT NULL,
    description  TEXT    NOT NULL,
    status       TEXT    NOT NULL DEFAULT 'unassigned'
                         CHECK (status IN ('unassigned', 'in_progress', 'pending', 'resolved', 'closed')),
    priority     TEXT    NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    requester_id INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_by   INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    assignee_id  INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);
INSERT INTO tickets_new
    SELECT id, subject, description,
           CASE WHEN status = 'open' AND assignee_id IS NULL THEN 'unassigned'
                WHEN status = 'open' THEN 'in_progress'
                ELSE status END,
           priority, requester_id, created_by, assignee_id, created_at, updated_at
    FROM tickets;
DROP TABLE tickets;
ALTER TABLE tickets_new RENAME TO tickets;
CREATE INDEX tickets_requester_idx ON tickets (requester_id, created_at);
CREATE INDEX tickets_assignee_idx  ON tickets (assignee_id, status);
CREATE INDEX tickets_status_idx    ON tickets (status, created_at);

-- +goose Down
CREATE TABLE tickets_old (
    id           INTEGER PRIMARY KEY,
    subject      TEXT    NOT NULL,
    description  TEXT    NOT NULL,
    status       TEXT    NOT NULL DEFAULT 'open'   CHECK (status IN ('open', 'pending', 'resolved', 'closed')),
    priority     TEXT    NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    requester_id INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_by   INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    assignee_id  INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);
INSERT INTO tickets_old
    SELECT id, subject, description,
           CASE WHEN status IN ('unassigned', 'in_progress') THEN 'open' ELSE status END,
           priority, requester_id, created_by, assignee_id, created_at, updated_at
    FROM tickets;
DROP TABLE tickets;
ALTER TABLE tickets_old RENAME TO tickets;
CREATE INDEX tickets_requester_idx ON tickets (requester_id, created_at);
CREATE INDEX tickets_assignee_idx  ON tickets (assignee_id, status);
CREATE INDEX tickets_status_idx    ON tickets (status, created_at);
