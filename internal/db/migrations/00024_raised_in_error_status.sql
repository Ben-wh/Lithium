-- Adds the 'raised_in_error' status, for tickets that shouldn't have been
-- raised (duplicates, mistakes, tests). SQLite can't alter a CHECK
-- constraint, so the tickets table is rebuilt. Foreign keys must be off so
-- dropping the old table doesn't cascade-delete comments, attachments,
-- notifications and links; that pragma only works outside a transaction,
-- hence NO TRANSACTION and the explicit BEGIN/COMMIT.

-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys = OFF;
BEGIN;
CREATE TABLE tickets_new (
    id            INTEGER PRIMARY KEY,
    subject       TEXT    NOT NULL,
    description   TEXT    NOT NULL,
    status        TEXT    NOT NULL DEFAULT 'unassigned'
                          CHECK (status IN ('unassigned', 'in_progress', 'pending', 'resolved', 'closed', 'raised_in_error')),
    priority      TEXT    NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    requester_id  INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_by    INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    assignee_id   INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    category_id   INTEGER REFERENCES ticket_categories (id) ON DELETE SET NULL,
    parent_id     INTEGER REFERENCES tickets (id) ON DELETE SET NULL,
    custom_fields TEXT    NOT NULL DEFAULT '[]',
    template_name TEXT    NOT NULL DEFAULT ''
);
INSERT INTO tickets_new (id, subject, description, status, priority, requester_id, created_by, assignee_id,
                         created_at, updated_at, category_id, parent_id, custom_fields, template_name)
    SELECT id, subject, description, status, priority, requester_id, created_by, assignee_id,
           created_at, updated_at, category_id, parent_id, custom_fields, template_name
    FROM tickets;
DROP TABLE tickets;
ALTER TABLE tickets_new RENAME TO tickets;
CREATE INDEX tickets_requester_idx ON tickets (requester_id, created_at);
CREATE INDEX tickets_assignee_idx  ON tickets (assignee_id, status);
CREATE INDEX tickets_status_idx    ON tickets (status, created_at);
CREATE INDEX tickets_category_idx  ON tickets (category_id);
CREATE INDEX tickets_parent_idx    ON tickets (parent_id);
COMMIT;
PRAGMA foreign_keys = ON;

-- +goose Down
-- Tickets raised in error go back to Closed, the nearest older status.
PRAGMA foreign_keys = OFF;
BEGIN;
CREATE TABLE tickets_old (
    id            INTEGER PRIMARY KEY,
    subject       TEXT    NOT NULL,
    description   TEXT    NOT NULL,
    status        TEXT    NOT NULL DEFAULT 'unassigned'
                          CHECK (status IN ('unassigned', 'in_progress', 'pending', 'resolved', 'closed')),
    priority      TEXT    NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    requester_id  INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_by    INTEGER NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    assignee_id   INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    category_id   INTEGER REFERENCES ticket_categories (id) ON DELETE SET NULL,
    parent_id     INTEGER REFERENCES tickets (id) ON DELETE SET NULL,
    custom_fields TEXT    NOT NULL DEFAULT '[]',
    template_name TEXT    NOT NULL DEFAULT ''
);
INSERT INTO tickets_old (id, subject, description, status, priority, requester_id, created_by, assignee_id,
                         created_at, updated_at, category_id, parent_id, custom_fields, template_name)
    SELECT id, subject, description,
           CASE status WHEN 'raised_in_error' THEN 'closed' ELSE status END,
           priority, requester_id, created_by, assignee_id,
           created_at, updated_at, category_id, parent_id, custom_fields, template_name
    FROM tickets;
DROP TABLE tickets;
ALTER TABLE tickets_old RENAME TO tickets;
CREATE INDEX tickets_requester_idx ON tickets (requester_id, created_at);
CREATE INDEX tickets_assignee_idx  ON tickets (assignee_id, status);
CREATE INDEX tickets_status_idx    ON tickets (status, created_at);
CREATE INDEX tickets_category_idx  ON tickets (category_id);
CREATE INDEX tickets_parent_idx    ON tickets (parent_id);
COMMIT;
PRAGMA foreign_keys = ON;
