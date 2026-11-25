-- +goose Up
-- Ticket templates: extra fields (and description rules) that admins attach
-- to categories, so tickets in that category ask for the right details.
CREATE TABLE ticket_templates (
    id               INTEGER PRIMARY KEY,
    name             TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    intro            TEXT    NOT NULL DEFAULT '',       -- shown above the form
    description_mode TEXT    NOT NULL DEFAULT 'required', -- required, optional or hidden
    default_priority TEXT    NOT NULL DEFAULT '',       -- '' keeps the usual default
    fields           TEXT    NOT NULL DEFAULT '[]',     -- JSON array of field definitions
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL
);

-- No foreign key: deleting a template unlinks its categories in the same
-- transaction (see store.DeleteTemplate), and it keeps the column droppable.
ALTER TABLE ticket_categories ADD COLUMN template_id INTEGER;

-- What was filled in, copied with the field labels when the ticket was
-- raised, so editing or deleting the template never changes old tickets.
ALTER TABLE tickets ADD COLUMN custom_fields TEXT NOT NULL DEFAULT '[]';
ALTER TABLE tickets ADD COLUMN template_name TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE tickets DROP COLUMN template_name;
ALTER TABLE tickets DROP COLUMN custom_fields;
ALTER TABLE ticket_categories DROP COLUMN template_id;
DROP TABLE ticket_templates;
