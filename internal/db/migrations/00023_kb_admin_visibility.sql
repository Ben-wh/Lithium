-- Adds 'admins' as a knowledge base article visibility, for articles only
-- admins may read. SQLite can't alter a CHECK constraint, so kb_articles is
-- rebuilt. Foreign keys must be off so dropping the old table doesn't
-- cascade-delete kb_images; that pragma only works outside a transaction,
-- hence NO TRANSACTION and the explicit BEGIN/COMMIT.

-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys = OFF;
BEGIN;
CREATE TABLE kb_articles_new (
    id            INTEGER PRIMARY KEY,
    title         TEXT    NOT NULL,
    body          TEXT    NOT NULL DEFAULT '', -- Markdown
    collection_id INTEGER REFERENCES kb_collections (id) ON DELETE SET NULL,
    visibility    TEXT    NOT NULL DEFAULT 'staff' CHECK (visibility IN ('admins', 'staff', 'everyone')),
    status        TEXT    NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    author_id     INTEGER REFERENCES users (id) ON DELETE SET NULL,
    updated_by    INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    published_at  INTEGER
);
INSERT INTO kb_articles_new SELECT id, title, body, collection_id, visibility, status, author_id, updated_by,
    created_at, updated_at, published_at FROM kb_articles;
DROP TABLE kb_articles;
ALTER TABLE kb_articles_new RENAME TO kb_articles;
CREATE INDEX kb_articles_collection_idx ON kb_articles (collection_id);
COMMIT;
PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;
BEGIN;
CREATE TABLE kb_articles_old (
    id            INTEGER PRIMARY KEY,
    title         TEXT    NOT NULL,
    body          TEXT    NOT NULL DEFAULT '', -- Markdown
    collection_id INTEGER REFERENCES kb_collections (id) ON DELETE SET NULL,
    visibility    TEXT    NOT NULL DEFAULT 'staff' CHECK (visibility IN ('staff', 'everyone')),
    status        TEXT    NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    author_id     INTEGER REFERENCES users (id) ON DELETE SET NULL,
    updated_by    INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    published_at  INTEGER
);
-- Admin-only articles become staff-only, the closest older setting.
INSERT INTO kb_articles_old SELECT id, title, body, collection_id,
    CASE visibility WHEN 'admins' THEN 'staff' ELSE visibility END,
    status, author_id, updated_by, created_at, updated_at, published_at FROM kb_articles;
DROP TABLE kb_articles;
ALTER TABLE kb_articles_old RENAME TO kb_articles;
CREATE INDEX kb_articles_collection_idx ON kb_articles (collection_id);
COMMIT;
PRAGMA foreign_keys = ON;
