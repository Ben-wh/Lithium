-- +goose Up
-- Activity log for admins: who did what, when and from where. Names are
-- copied in (not joined) so entries stay readable after a user or ticket is
-- deleted, and actor_id has no foreign key for the same reason.
CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY,
    created_at  INTEGER NOT NULL,
    actor_id    INTEGER,
    actor_name  TEXT NOT NULL DEFAULT '',
    actor_email TEXT NOT NULL DEFAULT '',
    action      TEXT NOT NULL,             -- e.g. "ticket.update"
    category    TEXT NOT NULL,             -- auth, tickets, users, settings, knowledge, profile, reports, system
    summary     TEXT NOT NULL,             -- e.g. "Updated a ticket"
    target      TEXT NOT NULL DEFAULT '',  -- e.g. "TID-42" or "Alice Smith"
    detail      TEXT NOT NULL DEFAULT '',  -- e.g. "status: closed"
    outcome     TEXT NOT NULL,             -- success or failure
    ip          TEXT NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX audit_log_created_idx ON audit_log (created_at);
CREATE INDEX audit_log_category_idx ON audit_log (category, id);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_id, id);

-- +goose Down
DROP TABLE audit_log;
