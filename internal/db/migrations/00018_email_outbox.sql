-- +goose Up
-- Emails waiting to be sent (and recently sent ones). Rows are added in the
-- same transaction as the in-app notification, and a background worker
-- sends them, so a slow or broken mail server never holds up a request.
CREATE TABLE email_outbox (
    id              INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- deleted users get nothing
    to_addr         TEXT    NOT NULL,
    subject         TEXT    NOT NULL,
    body            TEXT    NOT NULL, -- plain text
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at INTEGER,          -- NULL once sent or given up on
    last_error      TEXT    NOT NULL DEFAULT '',
    sent_at         INTEGER,
    created_at      INTEGER NOT NULL
);
CREATE INDEX email_outbox_due_idx ON email_outbox (next_attempt_at) WHERE next_attempt_at IS NOT NULL;

-- +goose Down
DROP TABLE email_outbox;
