package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// EmailBatchWindow is how long a notification email waits before sending,
// so that more events on the same ticket can join it as extra lines.
var EmailBatchWindow = 2 * time.Minute

// Kinds of event people can choose to get emails about.
const (
	EmailReplies  = "replies"  // replies, and internal notes for staff
	EmailMentions = "mentions" // @mentions of them or their groups
	EmailAssigned = "assigned" // a ticket being assigned to them
	EmailUpdates  = "updates"  // status, priority, category and other changes
)

// AllEmailEvents is every event kind, in display order (and the default).
var AllEmailEvents = []string{EmailReplies, EmailMentions, EmailAssigned, EmailUpdates}

// emailEventFor maps a notification kind to the event someone opts into.
var emailEventFor = map[string]string{
	"reply": EmailReplies, "note": EmailReplies, "mention": EmailMentions,
	"assigned": EmailAssigned, "update": EmailUpdates,
}

// EmailEvents is a set of event kinds, stored comma-separated.
type EmailEvents []string

func (e *EmailEvents) Scan(v any) error {
	var s string
	switch v := v.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	case nil:
	default:
		return fmt.Errorf("email events: unexpected %T", v)
	}
	out := EmailEvents{}
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); slices.Contains(AllEmailEvents, part) {
			out = append(out, part)
		}
	}
	*e = out
	return nil
}

func (e EmailEvents) Value() (driver.Value, error) { return strings.Join(e, ","), nil }

// EmailContent is a notification email. Line is the event ("Agnes
// replied."); Subject and Rest (the ticket, link and footer) are shared by
// every event merged into the same email.
type EmailContent struct {
	Subject string
	Line    string
	Rest    string
}

// OutboxEmail is an email waiting to be sent.
type OutboxEmail struct {
	ID       int64
	To       string
	Subject  string
	Body     string
	Attempts int
}

// enqueueEmails queues the email for each user who wants this kind of
// notification by email. If one about the same ticket is still waiting for
// its batch window, the event is added to that instead.
func enqueueEmails(ctx context.Context, tx *sql.Tx, userIDs []int64, ticketID int64, kind string, e EmailContent, now time.Time) error {
	event := emailEventFor[kind]
	for _, uid := range userIDs {
		var addr string
		var on bool
		var events EmailEvents
		err := tx.QueryRowContext(ctx, `SELECT email, email_notifications, email_events FROM users WHERE id = ?`, uid).
			Scan(&addr, &on, &events)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		} else if err != nil {
			return err
		}
		if !on || !slices.Contains(events, event) {
			continue
		}

		// Join an email that hasn't been sent or tried yet. Newer subject
		// and footer win, so a settings change (like sensitive mode) applies.
		res, err := tx.ExecContext(ctx, `
			UPDATE email_outbox SET lines = lines || char(10) || ?, subject = ?, body = ?, to_addr = ?
			WHERE id = (
				SELECT id FROM email_outbox
				WHERE user_id = ? AND ticket_id = ? AND sent_at IS NULL AND attempts = 0 AND next_attempt_at > ?
				ORDER BY id DESC LIMIT 1)`,
			e.Line, e.Subject, e.Rest, addr, uid, ticketID, now.Unix())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO email_outbox (user_id, ticket_id, to_addr, subject, lines, body, next_attempt_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uid, ticketID, addr, e.Subject, e.Line, e.Rest, now.Add(EmailBatchWindow).Unix(), now.Unix()); err != nil {
			return err
		}
	}
	return nil
}

// DueEmails returns up to limit emails that are ready to be (re)tried, oldest first.
func (s *Store) DueEmails(ctx context.Context, now time.Time, limit int) ([]OutboxEmail, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, to_addr, subject, lines, body, attempts FROM email_outbox
		WHERE next_attempt_at IS NOT NULL AND next_attempt_at <= ?
		ORDER BY next_attempt_at, id LIMIT ?`, now.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OutboxEmail
	for rows.Next() {
		var e OutboxEmail
		var lines, rest string
		if err := rows.Scan(&e.ID, &e.To, &e.Subject, &lines, &rest, &e.Attempts); err != nil {
			return nil, err
		}
		e.Body = rest
		if lines != "" {
			e.Body = lines + "\n\n" + rest
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// MarkEmailSent records a successful send.
func (s *Store) MarkEmailSent(ctx context.Context, id int64, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE email_outbox SET attempts = attempts + 1, sent_at = ?, next_attempt_at = NULL, last_error = ''
		WHERE id = ?`, now.Unix(), id)
	return err
}

// MarkEmailFailed records a failed attempt. A zero retryAt means give up.
func (s *Store) MarkEmailFailed(ctx context.Context, id int64, errText string, retryAt time.Time) error {
	var next any
	if !retryAt.IsZero() {
		next = retryAt.Unix()
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE email_outbox SET attempts = attempts + 1, last_error = ?, next_attempt_at = ?
		WHERE id = ?`, errText, next, id)
	return err
}

// PruneEmails deletes sent and abandoned emails created before cutoff.
func (s *Store) PruneEmails(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM email_outbox WHERE next_attempt_at IS NULL AND created_at < ?`, cutoff.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// EmailPreferences changes; nil fields are left alone.
type EmailPreferences struct {
	Enabled *bool
	Events  *EmailEvents
}

// SetEmailPreferences changes a user's notification email choices.
func (s *Store) SetEmailPreferences(ctx context.Context, userID int64, p EmailPreferences) error {
	if p.Enabled != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET email_notifications = ? WHERE id = ?`, *p.Enabled, userID); err != nil {
			return err
		}
	}
	if p.Events != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET email_events = ? WHERE id = ?`, *p.Events, userID); err != nil {
			return err
		}
	}
	return nil
}
