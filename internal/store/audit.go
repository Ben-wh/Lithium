package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// AuditEntry is one line of the admin activity log.
type AuditEntry struct {
	ID         int64     `json:"id"`
	At         time.Time `json:"at"`
	ActorID    *int64    `json:"actorId"`
	ActorName  string    `json:"actorName"`
	ActorEmail string    `json:"actorEmail"`
	Action     string    `json:"action"`
	Category   string    `json:"category"`
	Summary    string    `json:"summary"`
	Target     string    `json:"target"`
	Detail     string    `json:"detail"`
	Outcome    string    `json:"outcome"` // AuditSuccess or AuditFailure
	IP         string    `json:"ip"`
	UserAgent  string    `json:"userAgent"`
}

const (
	AuditSuccess = "success"
	AuditFailure = "failure"
)

// AddAudit records an entry. At defaults to now.
func (s *Store) AddAudit(ctx context.Context, e AuditEntry) error {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	var actor sql.NullInt64
	if e.ActorID != nil {
		actor = sql.NullInt64{Int64: *e.ActorID, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (created_at, actor_id, actor_name, actor_email, action, category, summary, target, detail, outcome, ip, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.At.Unix(), actor, e.ActorName, e.ActorEmail, e.Action, e.Category, e.Summary,
		e.Target, e.Detail, e.Outcome, e.IP, e.UserAgent)
	return err
}

// AuditQuery filters the activity log. Zero values mean "any".
type AuditQuery struct {
	// Search matches every word against who, what, target, detail and IP.
	Search   string
	Category string
	Outcome  string
	ActorID  int64
	Since    time.Time
	Until    time.Time
	// BeforeID pages backwards: only entries older than this one.
	BeforeID int64
	Limit    int
}

// AuditLog returns matching entries, newest first.
func (s *Store) AuditLog(ctx context.Context, q AuditQuery) ([]AuditEntry, error) {
	var where []string
	var args []any
	for _, word := range strings.Fields(q.Search) {
		like := "%" + escapeLike(strings.ToLower(word)) + "%"
		where = append(where, `(lower(actor_name) LIKE ? ESCAPE '\' OR lower(actor_email) LIKE ? ESCAPE '\'
			OR lower(summary) LIKE ? ESCAPE '\' OR lower(target) LIKE ? ESCAPE '\'
			OR lower(detail) LIKE ? ESCAPE '\' OR ip LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like, like, like, like)
	}
	if q.Category != "" {
		where, args = append(where, "category = ?"), append(args, q.Category)
	}
	if q.Outcome != "" {
		where, args = append(where, "outcome = ?"), append(args, q.Outcome)
	}
	if q.ActorID != 0 {
		where, args = append(where, "actor_id = ?"), append(args, q.ActorID)
	}
	if !q.Since.IsZero() {
		where, args = append(where, "created_at >= ?"), append(args, q.Since.Unix())
	}
	if !q.Until.IsZero() {
		where, args = append(where, "created_at < ?"), append(args, q.Until.Unix())
	}
	if q.BeforeID > 0 {
		where, args = append(where, "id < ?"), append(args, q.BeforeID)
	}
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	query := `SELECT id, created_at, actor_id, actor_name, actor_email, action, category, summary, target, detail, outcome, ip, user_agent FROM audit_log`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, q.Limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		var at int64
		var actor sql.NullInt64
		if err := rows.Scan(&e.ID, &at, &actor, &e.ActorName, &e.ActorEmail, &e.Action, &e.Category,
			&e.Summary, &e.Target, &e.Detail, &e.Outcome, &e.IP, &e.UserAgent); err != nil {
			return nil, err
		}
		e.At = time.Unix(at, 0).UTC()
		if actor.Valid {
			id := actor.Int64
			e.ActorID = &id
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PruneAudit deletes entries older than before.
func (s *Store) PruneAudit(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM audit_log WHERE created_at < ?`, before.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
