-- +goose Up
-- Agent groups: when the feature is on, agents only see tickets in the
-- categories their groups cover (admins always see everything).
CREATE TABLE agent_groups (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT    NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);
CREATE TABLE agent_group_members (
    group_id INTEGER NOT NULL REFERENCES agent_groups (id) ON DELETE CASCADE,
    user_id  INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX agent_group_members_user_idx ON agent_group_members (user_id);
CREATE TABLE agent_group_categories (
    group_id    INTEGER NOT NULL REFERENCES agent_groups (id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES ticket_categories (id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, category_id)
);

-- +goose Down
DROP TABLE agent_group_categories;
DROP TABLE agent_group_members;
DROP TABLE agent_groups;
