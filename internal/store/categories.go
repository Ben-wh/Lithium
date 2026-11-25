package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrCategoryInUse = errors.New("category is used by tickets")

type Category struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Archived    bool   `json:"archived"`
	TicketCount int    `json:"ticketCount"`
	// TemplateID is the ticket template used for this category, or nil.
	TemplateID *int64    `json:"templateId"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CategoryRef is a category as shown on a ticket.
type CategoryRef struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

const categoryCols = `c.id, c.name, c.description, c.color, c.archived,
	(SELECT COUNT(*) FROM tickets t WHERE t.category_id = c.id), c.template_id, c.created_at`

func scanCategory(row scanner) (Category, error) {
	var c Category
	var created int64
	var tpl sql.NullInt64
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.Color, &c.Archived, &c.TicketCount, &tpl, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	if tpl.Valid {
		c.TemplateID = &tpl.Int64
	}
	c.CreatedAt = time.Unix(created, 0).UTC()
	return c, err
}

// ListCategories returns categories sorted by name. Archived ones are only
// included when includeArchived is set.
func (s *Store) ListCategories(ctx context.Context, includeArchived bool) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+categoryCols+` FROM ticket_categories c
		WHERE ? OR c.archived = 0
		ORDER BY c.archived, c.name COLLATE NOCASE`, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CategoryByID(ctx context.Context, id int64) (Category, error) {
	return scanCategory(s.db.QueryRowContext(ctx, `SELECT `+categoryCols+` FROM ticket_categories c WHERE c.id = ?`, id))
}

func (s *Store) CreateCategory(ctx context.Context, name, description, color string) (Category, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO ticket_categories (name, description, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		name, description, color, now, now)
	if err != nil {
		return Category{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Category{}, err
	}
	return s.CategoryByID(ctx, id)
}

func (s *Store) UpdateCategory(ctx context.Context, id int64, name, description, color string, archived bool) (Category, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE ticket_categories SET name = ?, description = ?, color = ?, archived = ?, updated_at = ? WHERE id = ?`,
		name, description, color, archived, time.Now().Unix(), id)
	if err != nil {
		return Category{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Category{}, ErrNotFound
	}
	return s.CategoryByID(ctx, id)
}

// DeleteCategory removes a category that no ticket uses.
func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var inUse bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM tickets WHERE category_id = ?)`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrCategoryInUse
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM ticket_categories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}
