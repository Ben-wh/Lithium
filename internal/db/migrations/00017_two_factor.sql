-- Two-factor authentication with authenticator apps (TOTP) and one-time
-- recovery codes. A session stays "pending" after the password step until a
-- code is entered.

-- +goose Up
ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0;
-- The last 30-second step a code was accepted for, so codes can't be replayed.
ALTER TABLE users ADD COLUMN totp_last_step INTEGER NOT NULL DEFAULT 0;

ALTER TABLE sessions ADD COLUMN mfa_pending INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN mfa_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN remember INTEGER NOT NULL DEFAULT 0;

CREATE TABLE recovery_codes (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT NOT NULL,
    used_at    INTEGER
);
CREATE INDEX recovery_codes_user ON recovery_codes(user_id);

-- +goose Down
DROP TABLE recovery_codes;
ALTER TABLE sessions DROP COLUMN remember;
ALTER TABLE sessions DROP COLUMN mfa_attempts;
ALTER TABLE sessions DROP COLUMN mfa_pending;
ALTER TABLE users DROP COLUMN totp_last_step;
ALTER TABLE users DROP COLUMN totp_enabled;
ALTER TABLE users DROP COLUMN totp_secret;
