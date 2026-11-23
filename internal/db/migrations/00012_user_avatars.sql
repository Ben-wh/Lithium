-- +goose Up
-- '' = initials, 'preset:<1-10>' = a built-in avatar, 'custom:<file>' = an
-- uploaded picture stored under <data dir>/avatars/.
ALTER TABLE users ADD COLUMN avatar TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN avatar;
