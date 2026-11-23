package store

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
)

type KBCollection struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ArticleCount int    `json:"articleCount"` // articles the viewer can see
}

type KBCollectionRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// KBArticle is a knowledge base article. Body is Markdown; it's left out of
// lists (Excerpt is used there instead).
type KBArticle struct {
	ID          int64            `json:"id"`
	Title       string           `json:"title"`
	Body        string           `json:"body,omitempty"`
	Excerpt     string           `json:"excerpt,omitempty"`
	Collection  *KBCollectionRef `json:"collection"`
	Visibility  string           `json:"visibility"` // staff | everyone
	Status      string           `json:"status"`     // draft | published
	Author      *UserRef         `json:"author"`
	UpdatedBy   *UserRef         `json:"updatedBy"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	PublishedAt *time.Time       `json:"publishedAt"`
}

// publicOnly limits queries to what regular users may read.
const kbPublicOnly = `a.status = 'published' AND a.visibility = 'everyone'`

func (s *Store) ListKBCollections(ctx context.Context, staff bool) ([]KBCollection, error) {
	visible := "1"
	if !staff {
		visible = kbPublicOnly
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.description,
		       (SELECT COUNT(*) FROM kb_articles a WHERE a.collection_id = c.id AND `+visible+`)
		FROM kb_collections c ORDER BY c.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KBCollection{}
	for rows.Next() {
		var c KBCollection
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.ArticleCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) KBCollectionByID(ctx context.Context, id int64) (KBCollection, error) {
	var c KBCollection
	err := s.db.QueryRowContext(ctx, `SELECT id, name, description FROM kb_collections WHERE id = ?`, id).Scan(&c.ID, &c.Name, &c.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func (s *Store) SaveKBCollection(ctx context.Context, id int64, name, description string) (KBCollection, error) {
	now := time.Now().Unix()
	if id == 0 {
		res, err := s.db.ExecContext(ctx, `INSERT INTO kb_collections (name, description, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			name, description, now, now)
		if err != nil {
			return KBCollection{}, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return KBCollection{}, err
		}
	} else {
		res, err := s.db.ExecContext(ctx, `UPDATE kb_collections SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
			name, description, now, id)
		if err != nil {
			return KBCollection{}, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return KBCollection{}, ErrNotFound
		}
	}
	return s.KBCollectionByID(ctx, id)
}

// DeleteKBCollection removes a collection; its articles become uncategorised.
func (s *Store) DeleteKBCollection(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM kb_collections WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const kbArticleCols = `a.id, a.title, a.body, a.visibility, a.status, a.created_at, a.updated_at, a.published_at,
	c.id, c.name,
	au.id, au.name, au.email, au.avatar,
	ub.id, ub.name, ub.email, ub.avatar`
const kbArticleFrom = ` FROM kb_articles a
	LEFT JOIN kb_collections c ON c.id = a.collection_id
	LEFT JOIN users au ON au.id = a.author_id
	LEFT JOIN users ub ON ub.id = a.updated_by `

func scanKBArticle(row scanner) (KBArticle, error) {
	var a KBArticle
	var created, updated int64
	var published, colID sql.NullInt64
	var colName sql.NullString
	var author, editor userRefScan
	dest := []any{&a.ID, &a.Title, &a.Body, &a.Visibility, &a.Status, &created, &updated, &published, &colID, &colName}
	dest = append(dest, author.dest()...)
	dest = append(dest, editor.dest()...)
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return a, ErrNotFound
		}
		return a, err
	}
	a.CreatedAt = time.Unix(created, 0).UTC()
	a.UpdatedAt = time.Unix(updated, 0).UTC()
	if published.Valid {
		t := time.Unix(published.Int64, 0).UTC()
		a.PublishedAt = &t
	}
	if colID.Valid {
		a.Collection = &KBCollectionRef{ID: colID.Int64, Name: colName.String}
	}
	a.Author, a.UpdatedBy = author.ref(), editor.ref()
	return a, nil
}

type KBFilter struct {
	Staff        bool   // staff see drafts and staff-only articles
	CollectionID int64  // 0 = any; -1 = no collection
	Status       string // "" = any (staff only)
	Query        string
}

// ListKBArticles returns matching articles (without bodies), most recently
// updated first.
func (s *Store) ListKBArticles(ctx context.Context, f KBFilter) ([]KBArticle, error) {
	var conds []string
	var args []any
	if !f.Staff {
		conds = append(conds, kbPublicOnly)
	} else if f.Status != "" {
		conds = append(conds, "a.status = ?")
		args = append(args, f.Status)
	}
	switch {
	case f.CollectionID > 0:
		conds = append(conds, "a.collection_id = ?")
		args = append(args, f.CollectionID)
	case f.CollectionID < 0:
		conds = append(conds, "a.collection_id IS NULL")
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		pat := likePattern(q)
		conds = append(conds, `(a.title LIKE ? ESCAPE '\' OR a.body LIKE ? ESCAPE '\')`)
		args = append(args, pat, pat)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+kbArticleCols+kbArticleFrom+where+` ORDER BY a.updated_at DESC, a.id DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KBArticle{}
	for rows.Next() {
		a, err := scanKBArticle(rows)
		if err != nil {
			return nil, err
		}
		a.Excerpt = excerpt(a.Body, 220)
		a.Body = ""
		out = append(out, a)
	}
	return out, rows.Err()
}

// excerpt is the start of an article's text with Markdown symbols and image
// links removed, for list previews.
var (
	mdImage = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	mdLink  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
)

func excerpt(body string, n int) string {
	body = mdImage.ReplaceAllString(body, "")
	body = mdLink.ReplaceAllString(body, "$1") // keep link text, drop the URL
	var b strings.Builder
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "```") || strings.HasPrefix(line, "|") || strings.HasPrefix(line, "![") {
			continue
		}
		line = strings.TrimLeft(line, "#>-*+0123456789. ")
		line = strings.TrimPrefix(strings.TrimPrefix(line, "[ ] "), "[x] ")
		line = strings.NewReplacer("**", "", "__", "", "`", "", "*", "", "_", "").Replace(line)
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(line)
		if b.Len() > n {
			break
		}
	}
	out := []rune(b.String())
	if len(out) > n {
		return strings.TrimSpace(string(out[:n])) + "…"
	}
	return string(out)
}

func (s *Store) KBArticleByID(ctx context.Context, id int64) (KBArticle, error) {
	return scanKBArticle(s.db.QueryRowContext(ctx, `SELECT `+kbArticleCols+kbArticleFrom+`WHERE a.id = ?`, id))
}

// CreateKBArticle starts a new draft.
func (s *Store) CreateKBArticle(ctx context.Context, title string, collectionID, authorID int64) (KBArticle, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO kb_articles (title, collection_id, author_id, updated_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		title, nullID(collectionID), authorID, authorID, now, now)
	if err != nil {
		return KBArticle{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return KBArticle{}, err
	}
	return s.KBArticleByID(ctx, id)
}

type KBArticleUpdate struct {
	Title        string
	Body         string
	CollectionID int64
	Visibility   string
	Status       string
}

func (s *Store) UpdateKBArticle(ctx context.Context, id, editorID int64, u KBArticleUpdate) (KBArticle, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		UPDATE kb_articles SET title = ?, body = ?, collection_id = ?, visibility = ?, status = ?, updated_by = ?, updated_at = ?,
		       published_at = CASE WHEN ? = 'published' THEN COALESCE(published_at, ?) ELSE NULL END
		WHERE id = ?`,
		u.Title, u.Body, nullID(u.CollectionID), u.Visibility, u.Status, editorID, now, u.Status, now, id)
	if err != nil {
		return KBArticle{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return KBArticle{}, ErrNotFound
	}
	return s.KBArticleByID(ctx, id)
}

// DeleteKBArticle removes an article and its images, returning the image files to delete.
func (s *Store) DeleteKBArticle(ctx context.Context, id int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT stored_name FROM kb_images WHERE article_id = ?`, id)
	if err != nil {
		return nil, err
	}
	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			rows.Close()
			return nil, err
		}
		files = append(files, f)
	}
	rows.Close()
	res, err := tx.ExecContext(ctx, `DELETE FROM kb_articles WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return files, tx.Commit()
}

type KBImage struct {
	ID          int64
	ArticleID   int64
	StoredName  string
	ContentType string
	CreatedAt   time.Time
}

func (s *Store) AddKBImage(ctx context.Context, articleID int64, storedName, contentType string, size, uploadedBy int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kb_images (article_id, stored_name, content_type, size, uploaded_by, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		articleID, storedName, contentType, size, uploadedBy, time.Now().Unix())
	return err
}

func (s *Store) KBImageByName(ctx context.Context, storedName string) (KBImage, error) {
	var img KBImage
	var created int64
	err := s.db.QueryRowContext(ctx, `SELECT id, article_id, stored_name, content_type, created_at FROM kb_images WHERE stored_name = ?`,
		storedName).Scan(&img.ID, &img.ArticleID, &img.StoredName, &img.ContentType, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return img, ErrNotFound
	}
	img.CreatedAt = time.Unix(created, 0).UTC()
	return img, err
}
