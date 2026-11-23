-- +goose Up
CREATE TABLE password_reset_requests (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status      TEXT    NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'dismissed')),
    ip          TEXT    NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    resolved_at INTEGER,
    resolved_by INTEGER REFERENCES users (id) ON DELETE SET NULL
);

-- At most one open request per user; repeat requests are ignored.
CREATE UNIQUE INDEX password_reset_requests_one_pending
    ON password_reset_requests (user_id) WHERE status = 'pending';
CREATE INDEX password_reset_requests_status_idx
    ON password_reset_requests (status, created_at);

-- +goose Down
DROP TABLE password_reset_requests;
