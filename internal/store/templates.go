package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Field types a template can use.
const (
	FieldText     = "text"     // one line
	FieldTextarea = "textarea" // several lines
	FieldNumber   = "number"
	FieldSelect   = "select" // one of Options
	FieldCheckbox = "checkbox"
	FieldDate     = "date" // YYYY-MM-DD
)

// How a template treats the ticket's main description box.
const (
	DescriptionRequired = "required"
	DescriptionOptional = "optional"
	DescriptionHidden   = "hidden"
)

// TemplateField is one extra question on a template.
type TemplateField struct {
	// ID is stable across edits, so answers line up with their question.
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Help     string   `json:"help"`
	Options  []string `json:"options,omitempty"`
}

// TicketTemplate shapes the form for tickets in its categories.
type TicketTemplate struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	Intro           string          `json:"intro"`
	DescriptionMode string          `json:"descriptionMode"`
	DefaultPriority string          `json:"defaultPriority"`
	Fields          []TemplateField `json:"fields"`
	// CategoryCount is how many categories use it.
	CategoryCount int       `json:"categoryCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// TicketField is one answer on a ticket, with its question copied in.
type TicketField struct {
	Label string `json:"label"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

const templateCols = `t.id, t.name, t.intro, t.description_mode, t.default_priority, t.fields,
	(SELECT COUNT(*) FROM ticket_categories c WHERE c.template_id = t.id), t.created_at, t.updated_at`

func scanTemplate(row scanner) (TicketTemplate, error) {
	var t TicketTemplate
	var fields string
	var created, updated int64
	err := row.Scan(&t.ID, &t.Name, &t.Intro, &t.DescriptionMode, &t.DefaultPriority, &fields, &t.CategoryCount, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return TicketTemplate{}, ErrNotFound
	} else if err != nil {
		return TicketTemplate{}, err
	}
	if err := json.Unmarshal([]byte(fields), &t.Fields); err != nil {
		return TicketTemplate{}, err
	}
	if t.Fields == nil {
		t.Fields = []TemplateField{}
	}
	t.CreatedAt, t.UpdatedAt = time.Unix(created, 0).UTC(), time.Unix(updated, 0).UTC()
	return t, nil
}

// ListTemplates returns every template, by name.
func (s *Store) ListTemplates(ctx context.Context) ([]TicketTemplate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+templateCols+` FROM ticket_templates t ORDER BY t.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TicketTemplate{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) TemplateByID(ctx context.Context, id int64) (TicketTemplate, error) {
	return scanTemplate(s.db.QueryRowContext(ctx, `SELECT `+templateCols+` FROM ticket_templates t WHERE t.id = ?`, id))
}

// SaveTemplate creates the template (ID 0) or replaces an existing one.
func (s *Store) SaveTemplate(ctx context.Context, t TicketTemplate) (TicketTemplate, error) {
	fields, err := json.Marshal(t.Fields)
	if err != nil {
		return TicketTemplate{}, err
	}
	now := time.Now().Unix()
	if t.ID == 0 {
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO ticket_templates (name, intro, description_mode, default_priority, fields, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			t.Name, t.Intro, t.DescriptionMode, t.DefaultPriority, string(fields), now, now)
		if err != nil {
			return TicketTemplate{}, err
		}
		if t.ID, err = res.LastInsertId(); err != nil {
			return TicketTemplate{}, err
		}
	} else {
		res, err := s.db.ExecContext(ctx, `
			UPDATE ticket_templates SET name = ?, intro = ?, description_mode = ?, default_priority = ?, fields = ?, updated_at = ?
			WHERE id = ?`,
			t.Name, t.Intro, t.DescriptionMode, t.DefaultPriority, string(fields), now, t.ID)
		if err != nil {
			return TicketTemplate{}, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return TicketTemplate{}, ErrNotFound
		}
	}
	return s.TemplateByID(ctx, t.ID)
}

// DeleteTemplate removes a template and unlinks it from its categories.
// Tickets keep what was filled in.
func (s *Store) DeleteTemplate(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE ticket_categories SET template_id = NULL WHERE template_id = ?`, id); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM ticket_templates WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// SetCategoryTemplate links a category to a template (0 unlinks it).
func (s *Store) SetCategoryTemplate(ctx context.Context, categoryID, templateID int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE ticket_categories SET template_id = ?, updated_at = ? WHERE id = ?`,
		nullID(templateID), time.Now().Unix(), categoryID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
