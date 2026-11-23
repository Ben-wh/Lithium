package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type CommentAuthor struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	AvatarURL string `json:"avatarUrl"`
}

// Comment is one entry in a ticket's conversation: a reply someone wrote
// (Kind "comment"), or an automatic update from Lithium when the ticket
// changed (Kind "event"; Author is whoever made the change).
type Comment struct {
	ID        int64          `json:"id"`
	Kind      string         `json:"kind"`
	Author    *CommentAuthor `json:"author"` // nil if the user was deleted
	Body      string         `json:"body,omitempty"`
	Internal  bool           `json:"internal"`        // staff-only note, hidden from the requester
	Event     string         `json:"event,omitempty"` // status | priority | category | assignee
	From      string         `json:"from"`
	To        string         `json:"to"`
	CreatedAt time.Time      `json:"createdAt"`
}

const commentQuery = `
	SELECT c.id, c.kind, c.body, c.internal, c.event, c.old_value, c.new_value, c.created_at, u.id, u.name, u.role, u.avatar
	FROM ticket_comments c LEFT JOIN users u ON u.id = c.author_id `

func scanComment(row scanner) (Comment, error) {
	var c Comment
	var created int64
	var uID sql.NullInt64
	var uName, uRole, uAvatar sql.NullString
	err := row.Scan(&c.ID, &c.Kind, &c.Body, &c.Internal, &c.Event, &c.From, &c.To, &created, &uID, &uName, &uRole, &uAvatar)
	if errors.Is(err, sql.ErrNoRows) {
		return Comment{}, ErrNotFound
	}
	c.CreatedAt = time.Unix(created, 0).UTC()
	if uID.Valid {
		c.Author = &CommentAuthor{ID: uID.Int64, Name: uName.String, Role: uRole.String, AvatarURL: AvatarURL(uID.Int64, uAvatar.String)}
	}
	return c, err
}

// ListComments returns a ticket's conversation, oldest first. Internal notes
// are only included for staff.
func (s *Store) ListComments(ctx context.Context, ticketID int64, includeInternal bool) ([]Comment, error) {
	rows, err := s.db.QueryContext(ctx, commentQuery+`WHERE c.ticket_id = ? AND (? OR c.internal = 0) ORDER BY c.id`, ticketID, includeInternal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddComment records a reply (or a staff-only internal note). Public replies
// mark the ticket as updated; internal notes don't, so the requester can't
// tell from the ticket list that something hidden happened.
func (s *Store) AddComment(ctx context.Context, ticketID, authorID int64, body string, internal bool) (Comment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Comment{}, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO ticket_comments (ticket_id, kind, author_id, body, internal, created_at) VALUES (?, 'comment', ?, ?, ?, ?)`,
		ticketID, authorID, body, internal, now)
	if err != nil {
		return Comment{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Comment{}, err
	}
	if !internal {
		if _, err := tx.ExecContext(ctx, `UPDATE tickets SET updated_at = ? WHERE id = ?`, now, ticketID); err != nil {
			return Comment{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Comment{}, err
	}
	return scanComment(s.db.QueryRowContext(ctx, commentQuery+`WHERE c.id = ?`, id))
}

// TicketUpdate holds staff changes to a ticket. Nil/false fields are left alone.
type TicketUpdate struct {
	Status      *string
	Priority    *string
	SetCategory bool
	CategoryID  int64 // 0 = no category
	SetAssignee bool
	AssigneeID  int64 // 0 = unassigned
}

// TicketChange is one field that changed in an update.
type TicketChange struct{ Field, From, To string }

// UpdateTicket applies changes and records a Lithium update for each thing
// that actually changed, all in one transaction. Assigning someone to an
// unassigned ticket moves it to in progress, and removing the assignee from
// an in-progress ticket moves it back, unless a status was given explicitly.
// It returns what changed.
func (s *Store) UpdateTicket(ctx context.Context, id, actorID int64, u TicketUpdate) ([]TicketChange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var status, priority string
	var catID, asgID sql.NullInt64
	var catName, asgName sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT t.status, t.priority, t.category_id, cat.name, t.assignee_id, a.name
		FROM tickets t
		LEFT JOIN ticket_categories cat ON cat.id = t.category_id
		LEFT JOIN users a ON a.id = t.assignee_id
		WHERE t.id = ?`, id).Scan(&status, &priority, &catID, &catName, &asgID, &asgName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	nameOf := func(table string, id int64) (string, error) {
		if id == 0 {
			return "", nil
		}
		var name string
		err := tx.QueryRowContext(ctx, `SELECT name FROM `+table+` WHERE id = ?`, id).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return name, err
	}

	var events []TicketChange
	newStatus, newPriority := status, priority
	newCat, newAsg := catID.Int64, asgID.Int64

	if u.SetAssignee && u.AssigneeID != asgID.Int64 {
		name, err := nameOf("users", u.AssigneeID)
		if err != nil {
			return nil, err
		}
		events = append(events, TicketChange{"assignee", asgName.String, name})
		newAsg = u.AssigneeID
		if u.Status == nil {
			if newAsg != 0 && status == "unassigned" {
				newStatus = "in_progress"
			} else if newAsg == 0 && status == "in_progress" {
				newStatus = "unassigned"
			}
		}
	}
	if u.Status != nil {
		newStatus = *u.Status
	}
	if newStatus != status {
		events = append(events, TicketChange{"status", status, newStatus})
	}
	if u.Priority != nil && *u.Priority != priority {
		newPriority = *u.Priority
		events = append(events, TicketChange{"priority", priority, newPriority})
	}
	if u.SetCategory && u.CategoryID != catID.Int64 {
		name, err := nameOf("ticket_categories", u.CategoryID)
		if err != nil {
			return nil, err
		}
		events = append(events, TicketChange{"category", catName.String, name})
		newCat = u.CategoryID
	}
	if len(events) == 0 {
		return nil, nil
	}

	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, `
		UPDATE tickets SET status = ?, priority = ?, category_id = ?, assignee_id = ?, updated_at = ? WHERE id = ?`,
		newStatus, newPriority, nullID(newCat), nullID(newAsg), now, id); err != nil {
		return nil, err
	}
	for _, e := range events {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ticket_comments (ticket_id, kind, author_id, event, old_value, new_value, created_at)
			VALUES (?, 'event', ?, ?, ?, ?, ?)`, id, actorID, e.Field, e.From, e.To, now); err != nil {
			return nil, err
		}
	}
	return events, tx.Commit()
}
