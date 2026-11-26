-- SLA targets: rules set a time to first response and/or to resolution for
-- tickets in a category, of a priority, or both. Each ticket keeps the
-- numbers needed to run its clock, which pauses while it's Pending (or
-- finished) and carries on if it's reopened.

-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys = OFF;
BEGIN;

CREATE TABLE sla_rules (
    id                     INTEGER PRIMARY KEY,
    category_id            INTEGER REFERENCES ticket_categories (id) ON DELETE CASCADE, -- NULL = any category
    priority               TEXT CHECK (priority IN ('low', 'normal', 'high', 'urgent')), -- NULL = any priority
    first_response_minutes INTEGER CHECK (first_response_minutes > 0),                  -- NULL = no target
    resolution_minutes     INTEGER CHECK (resolution_minutes > 0),
    created_at             INTEGER NOT NULL,
    updated_at             INTEGER NOT NULL,
    CHECK (first_response_minutes IS NOT NULL OR resolution_minutes IS NOT NULL)
);
-- One rule per combination (NULLs would otherwise count as all different).
CREATE UNIQUE INDEX sla_rules_match_idx ON sla_rules (COALESCE(category_id, 0), COALESCE(priority, ''));

-- The SLA clock: time spent paused so far, when the current pause started
-- (NULL while running), how long the first staff reply took on that clock,
-- and which alerts have gone out (bits: 1 first-response warning,
-- 2 first-response breach, 4 resolution warning, 8 resolution breach).
ALTER TABLE tickets ADD COLUMN sla_paused_seconds     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tickets ADD COLUMN sla_paused_since       INTEGER;
ALTER TABLE tickets ADD COLUMN first_response_seconds INTEGER;
ALTER TABLE tickets ADD COLUMN sla_alerts             INTEGER NOT NULL DEFAULT 0;

-- Existing tickets: their pause history isn't known, so tickets that are
-- paused now count as paused since their last update, and first responses
-- are measured on the wall clock.
UPDATE tickets SET sla_paused_since = updated_at
    WHERE status IN ('pending', 'resolved', 'closed', 'raised_in_error');
UPDATE tickets SET first_response_seconds = (
    SELECT MAX(0, MIN(c.created_at) - tickets.created_at)
    FROM ticket_comments c JOIN users u ON u.id = c.author_id
    WHERE c.ticket_id = tickets.id AND c.kind = 'comment' AND c.internal = 0
      AND u.role IN ('admin', 'agent') AND c.author_id != tickets.requester_id);

-- SLA alerts are a new kind of notification. SQLite can't alter a CHECK
-- constraint, so the table is rebuilt (nothing references it).
CREATE TABLE notifications_new (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    ticket_id  INTEGER NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    actor_id   INTEGER REFERENCES users (id) ON DELETE SET NULL,
    kind       TEXT    NOT NULL CHECK (kind IN ('reply', 'note', 'mention', 'update', 'assigned', 'sla')),
    summary    TEXT    NOT NULL,
    read_at    INTEGER,
    created_at INTEGER NOT NULL
);
INSERT INTO notifications_new SELECT id, user_id, ticket_id, actor_id, kind, summary, read_at, created_at FROM notifications;
DROP TABLE notifications;
ALTER TABLE notifications_new RENAME TO notifications;
CREATE INDEX notifications_user_idx ON notifications (user_id, id);
CREATE INDEX notifications_unread_idx ON notifications (user_id, read_at);

COMMIT;
PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;
BEGIN;
DELETE FROM notifications WHERE kind = 'sla';
CREATE TABLE notifications_old (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    ticket_id  INTEGER NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    actor_id   INTEGER REFERENCES users (id) ON DELETE SET NULL,
    kind       TEXT    NOT NULL CHECK (kind IN ('reply', 'note', 'mention', 'update', 'assigned')),
    summary    TEXT    NOT NULL,
    read_at    INTEGER,
    created_at INTEGER NOT NULL
);
INSERT INTO notifications_old SELECT id, user_id, ticket_id, actor_id, kind, summary, read_at, created_at FROM notifications;
DROP TABLE notifications;
ALTER TABLE notifications_old RENAME TO notifications;
CREATE INDEX notifications_user_idx ON notifications (user_id, id);
CREATE INDEX notifications_unread_idx ON notifications (user_id, read_at);
ALTER TABLE tickets DROP COLUMN sla_alerts;
ALTER TABLE tickets DROP COLUMN first_response_seconds;
ALTER TABLE tickets DROP COLUMN sla_paused_since;
ALTER TABLE tickets DROP COLUMN sla_paused_seconds;
DROP TABLE sla_rules;
COMMIT;
PRAGMA foreign_keys = ON;
