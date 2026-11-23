-- Tickets can be linked into a tree: each ticket has at most one parent.
-- An urgent ticket at the top of a tree is shown as an incident tree.

-- +goose Up
ALTER TABLE tickets ADD COLUMN parent_id INTEGER REFERENCES tickets (id) ON DELETE SET NULL;
CREATE INDEX tickets_parent_idx ON tickets (parent_id);

-- Allow Lithium updates about links ('parent' on the child, 'child' on the
-- parent). SQLite can't alter a CHECK constraint, so the table is rebuilt.
-- Nothing references ticket_comments, so dropping it can't cascade.
CREATE TABLE ticket_comments_new (
    id         INTEGER PRIMARY KEY,
    ticket_id  INTEGER NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('comment', 'event')),
    author_id  INTEGER REFERENCES users (id) ON DELETE SET NULL,
    body       TEXT    NOT NULL DEFAULT '',
    event      TEXT    NOT NULL DEFAULT '' CHECK (event IN ('', 'status', 'priority', 'category', 'assignee', 'parent', 'child')),
    old_value  TEXT    NOT NULL DEFAULT '',
    new_value  TEXT    NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    internal   INTEGER NOT NULL DEFAULT 0
);
INSERT INTO ticket_comments_new (id, ticket_id, kind, author_id, body, event, old_value, new_value, created_at, internal)
    SELECT id, ticket_id, kind, author_id, body, event, old_value, new_value, created_at, internal FROM ticket_comments;
DROP TABLE ticket_comments;
ALTER TABLE ticket_comments_new RENAME TO ticket_comments;
CREATE INDEX ticket_comments_ticket_idx ON ticket_comments (ticket_id, id);

-- +goose Down
DELETE FROM ticket_comments WHERE event IN ('parent', 'child');
CREATE TABLE ticket_comments_old (
    id         INTEGER PRIMARY KEY,
    ticket_id  INTEGER NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('comment', 'event')),
    author_id  INTEGER REFERENCES users (id) ON DELETE SET NULL,
    body       TEXT    NOT NULL DEFAULT '',
    event      TEXT    NOT NULL DEFAULT '' CHECK (event IN ('', 'status', 'priority', 'category', 'assignee')),
    old_value  TEXT    NOT NULL DEFAULT '',
    new_value  TEXT    NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    internal   INTEGER NOT NULL DEFAULT 0
);
INSERT INTO ticket_comments_old SELECT id, ticket_id, kind, author_id, body, event, old_value, new_value, created_at, internal FROM ticket_comments;
DROP TABLE ticket_comments;
ALTER TABLE ticket_comments_old RENAME TO ticket_comments;
CREATE INDEX ticket_comments_ticket_idx ON ticket_comments (ticket_id, id);
DROP INDEX tickets_parent_idx;
ALTER TABLE tickets DROP COLUMN parent_id;
