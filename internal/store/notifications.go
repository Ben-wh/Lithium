package store

import (
	"context"
	"database/sql"
	"time"
)

type NotificationTicket struct {
	ID      int64  `json:"id"`
	Subject string `json:"subject"`
}

type Notification struct {
	ID        int64              `json:"id"`
	Kind      string             `json:"kind"` // reply | note | mention | update | assigned
	Summary   string             `json:"summary"`
	Actor     *UserRef           `json:"actor"` // nil if their account was deleted
	Ticket    NotificationTicket `json:"ticket"`
	Read      bool               `json:"read"`
	CreatedAt time.Time          `json:"createdAt"`
}

// Notify records the same notification for each recipient and, when email
// is non-nil, queues an email to each of them in the same transaction.
func (s *Store) Notify(ctx context.Context, recipients []int64, ticketID, actorID int64, kind, summary string, email *EmailContent) error {
	if len(recipients) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	nowT := time.Now()
	now := nowT.Unix()
	for _, uid := range recipients {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO notifications (user_id, ticket_id, actor_id, kind, summary, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			uid, ticketID, nullID(actorID), kind, summary, now); err != nil {
			return err
		}
	}
	if email != nil {
		if err := enqueueEmails(ctx, tx, recipients, ticketID, kind, *email, nowT); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListNotifications returns a user's most recent notifications, newest first.
func (s *Store) ListNotifications(ctx context.Context, userID int64, limit int) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT n.id, n.kind, n.summary, n.read_at, n.created_at, t.id, t.subject,
		       a.id, a.name, a.email, a.avatar
		FROM notifications n
		JOIN tickets t ON t.id = n.ticket_id
		LEFT JOIN users a ON a.id = n.actor_id
		WHERE n.user_id = ?
		ORDER BY n.id DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		var n Notification
		var readAt sql.NullInt64
		var created int64
		var actor userRefScan
		dest := append([]any{&n.ID, &n.Kind, &n.Summary, &readAt, &created, &n.Ticket.ID, &n.Ticket.Subject}, actor.dest()...)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		n.Read = readAt.Valid
		n.CreatedAt = time.Unix(created, 0).UTC()
		n.Actor = actor.ref()
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) UnreadNotificationCount(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

// MarkNotificationsRead marks a user's notifications read: all of them, those
// about one ticket (ticketID != 0), or specific ones (ids).
func (s *Store) MarkNotificationsRead(ctx context.Context, userID, ticketID int64, ids []int64) error {
	now := time.Now().Unix()
	switch {
	case ticketID != 0:
		_, err := s.db.ExecContext(ctx, `UPDATE notifications SET read_at = ? WHERE user_id = ? AND ticket_id = ? AND read_at IS NULL`, now, userID, ticketID)
		return err
	case len(ids) > 0:
		for _, id := range ids {
			if _, err := s.db.ExecContext(ctx, `UPDATE notifications SET read_at = ? WHERE user_id = ? AND id = ? AND read_at IS NULL`, now, userID, id); err != nil {
				return err
			}
		}
		return nil
	default:
		_, err := s.db.ExecContext(ctx, `UPDATE notifications SET read_at = ? WHERE user_id = ? AND read_at IS NULL`, now, userID)
		return err
	}
}
