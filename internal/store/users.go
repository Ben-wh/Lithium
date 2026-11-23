package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// unassignAll removes userID as assignee from every ticket, sending
// in-progress ones back to unassigned, and records a Lithium update on each.
func unassignAll(ctx context.Context, tx execer, userID, actorID int64) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT t.id, t.status, u.name FROM tickets t JOIN users u ON u.id = t.assignee_id
		WHERE t.assignee_id = ?`, userID)
	if err != nil {
		return err
	}
	type assigned struct {
		id           int64
		status, name string
	}
	var list []assigned
	for rows.Next() {
		var a assigned
		if err := rows.Scan(&a.id, &a.status, &a.name); err != nil {
			rows.Close()
			return err
		}
		list = append(list, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	now := time.Now().Unix()
	event := `INSERT INTO ticket_comments (ticket_id, kind, author_id, event, old_value, new_value, created_at)
		VALUES (?, 'event', ?, ?, ?, ?, ?)`
	for _, a := range list {
		status := a.status
		if status == "in_progress" {
			status = "unassigned"
		}
		if _, err := tx.ExecContext(ctx, `UPDATE tickets SET assignee_id = NULL, status = ?, updated_at = ? WHERE id = ?`,
			status, now, a.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, event, a.id, actorID, "assignee", a.name, "", now); err != nil {
			return err
		}
		if status != a.status {
			if _, err := tx.ExecContext(ctx, event, a.id, actorID, "status", a.status, status, now); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetUserRole changes a user's role. Moving someone out of a staff role
// (admin/agent) unassigns them from their tickets, since only staff can be
// assigned.
func (s *Store) SetUserRole(ctx context.Context, userID int64, role string, actorID int64) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	var current string
	err = tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, userID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	} else if err != nil {
		return User{}, err
	}
	if current == role {
		return s.UserByID(ctx, userID)
	}
	if role == "user" {
		if err := unassignAll(ctx, tx, userID, actorID); err != nil {
			return User{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET role = ?, updated_at = ? WHERE id = ?`, role, time.Now().Unix(), userID); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return s.UserByID(ctx, userID)
}

// DeletedUser reports what deleting an account removed, so the caller can
// clean up files on disk.
type DeletedUser struct {
	TicketsDeleted  int
	AttachmentFiles []string // stored names under the attachments dir
	AvatarFile      string   // custom avatar under the avatars dir, if any
}

// DeleteUser removes an account and every ticket they raised (with its
// replies and attachments). Tickets they were assigned are unassigned; tickets
// they raised on someone else's behalf stay with that person; their replies
// elsewhere remain, without an author.
func (s *Store) DeleteUser(ctx context.Context, userID, actorID int64) (DeletedUser, error) {
	var out DeletedUser
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	var avatar string
	err = tx.QueryRowContext(ctx, `SELECT avatar FROM users WHERE id = ?`, userID).Scan(&avatar)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	} else if err != nil {
		return out, err
	}
	out.AvatarFile = CustomAvatarFile(avatar)

	// Files belonging to the tickets about to go.
	rows, err := tx.QueryContext(ctx, `
		SELECT a.stored_name FROM ticket_attachments a JOIN tickets t ON t.id = a.ticket_id
		WHERE t.requester_id = ?`, userID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return out, err
		}
		out.AttachmentFiles = append(out.AttachmentFiles, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	if err := unassignAll(ctx, tx, userID, actorID); err != nil {
		return out, err
	}
	// Tickets they raised for other people now count as raised by those people.
	if _, err := tx.ExecContext(ctx, `
		UPDATE ticket_attachments SET uploaded_by = (SELECT requester_id FROM tickets WHERE tickets.id = ticket_id)
		WHERE uploaded_by = ? AND ticket_id IN (SELECT id FROM tickets WHERE requester_id != ?)`, userID, userID); err != nil {
		return out, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tickets SET created_by = requester_id WHERE created_by = ? AND requester_id != ?`, userID, userID); err != nil {
		return out, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM tickets WHERE requester_id = ?`, userID)
	if err != nil {
		return out, err
	}
	n, _ := res.RowsAffected()
	out.TicketsDeleted = int(n)
	// Sessions and reset requests cascade; their replies elsewhere lose the author.
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
