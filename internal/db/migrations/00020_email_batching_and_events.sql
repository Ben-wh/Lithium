-- +goose Up
-- Notification emails about the same ticket are merged into one email while
-- it waits to be sent: each event adds a line, and body holds the rest.
ALTER TABLE email_outbox ADD COLUMN ticket_id INTEGER REFERENCES tickets (id) ON DELETE CASCADE;
ALTER TABLE email_outbox ADD COLUMN lines TEXT NOT NULL DEFAULT '';
CREATE INDEX email_outbox_collect_idx ON email_outbox (user_id, ticket_id) WHERE sent_at IS NULL;

-- Which kinds of event someone wants emails about (comma-separated:
-- replies, mentions, assigned, updates).
ALTER TABLE users ADD COLUMN email_events TEXT NOT NULL DEFAULT 'replies,mentions,assigned,updates';

-- +goose Down
ALTER TABLE users DROP COLUMN email_events;
DROP INDEX email_outbox_collect_idx;
ALTER TABLE email_outbox DROP COLUMN lines;
ALTER TABLE email_outbox DROP COLUMN ticket_id;
