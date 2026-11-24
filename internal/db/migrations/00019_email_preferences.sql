-- +goose Up
-- Whether someone wants notification emails (when the admin has email on).
ALTER TABLE users ADD COLUMN email_notifications INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE users DROP COLUMN email_notifications;
