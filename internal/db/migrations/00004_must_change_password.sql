-- +goose Up
ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0;

-- Every existing account was set up by an admin (or the bootstrap env vars),
-- so nobody has chosen their own password yet.
UPDATE users SET must_change_password = 1;

-- +goose Down
ALTER TABLE users DROP COLUMN must_change_password;
