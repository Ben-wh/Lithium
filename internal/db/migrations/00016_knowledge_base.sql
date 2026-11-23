-- +goose Up
CREATE TABLE kb_collections (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT    NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE kb_articles (
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
CREATE INDEX kb_articles_collection_idx ON kb_articles (collection_id);

-- Images belong to an article and go with it. Files live under <data dir>/kb-images/.
CREATE TABLE kb_images (
    id           INTEGER PRIMARY KEY,
    article_id   INTEGER NOT NULL REFERENCES kb_articles (id) ON DELETE CASCADE,
    stored_name  TEXT    NOT NULL UNIQUE,
    content_type TEXT    NOT NULL,
    size         INTEGER NOT NULL,
    uploaded_by  INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at   INTEGER NOT NULL
);
CREATE INDEX kb_images_article_idx ON kb_images (article_id);

-- +goose Down
DROP TABLE kb_images;
DROP TABLE kb_articles;
DROP TABLE kb_collections;
