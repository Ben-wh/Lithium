-- +goose Up
-- Workspace-wide settings as key/value pairs. Missing keys mean "use the default".
CREATE TABLE app_settings (
    key        TEXT    PRIMARY KEY,
    value      TEXT    NOT NULL,
    updated_at INTEGER NOT NULL
);

-- +goose Down
DROP TABLE app_settings;
