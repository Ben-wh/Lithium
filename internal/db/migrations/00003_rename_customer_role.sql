-- Renames the 'customer' role to 'user'. SQLite can't alter a CHECK
-- constraint, so the users table is rebuilt (the documented 12-step
-- procedure). Foreign keys must be off so dropping the old table doesn't
-- cascade-delete sessions and reset requests; that pragma only works outside
-- a transaction, hence NO TRANSACTION and the explicit BEGIN/COMMIT.

-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys = OFF;
BEGIN;
CREATE TABLE users_new (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    name          TEXT    NOT NULL,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'agent' CHECK (role IN ('admin', 'agent', 'user')),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);
INSERT INTO users_new (id, email, name, password_hash, role, created_at, updated_at)
    SELECT id, email, name, password_hash,
           CASE role WHEN 'customer' THEN 'user' ELSE role END,
           created_at, updated_at
    FROM users;
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;
COMMIT;
PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;
BEGIN;
CREATE TABLE users_old (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    name          TEXT    NOT NULL,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'agent' CHECK (role IN ('admin', 'agent', 'customer')),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);
INSERT INTO users_old (id, email, name, password_hash, role, created_at, updated_at)
    SELECT id, email, name, password_hash,
           CASE role WHEN 'user' THEN 'customer' ELSE role END,
           created_at, updated_at
    FROM users;
DROP TABLE users;
ALTER TABLE users_old RENAME TO users;
COMMIT;
PRAGMA foreign_keys = ON;
