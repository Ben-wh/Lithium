-- +goose Up
CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    name          TEXT    NOT NULL,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'agent' CHECK (role IN ('admin', 'agent', 'customer')),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

CREATE TABLE sessions (
    token_hash   TEXT    PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    user_agent   TEXT    NOT NULL DEFAULT '',
    ip           TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
DROP TABLE users;
