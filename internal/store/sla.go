package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrDuplicateRule: an SLA rule for that category and priority already exists.
var ErrDuplicateRule = errors.New("a rule for that category and priority already exists")

// SLARule sets targets for matching tickets. A nil Category or Priority
// means "any"; at least one target is set.
type SLARule struct {
	ID                   int64        `json:"id"`
	Category             *CategoryRef `json:"category"`
	Priority             string       `json:"priority"` // "" = any
	FirstResponseMinutes *int         `json:"firstResponseMinutes"`
	ResolutionMinutes    *int         `json:"resolutionMinutes"`
}

// specificity orders rules for matching: category and priority, then
// category only, then priority only, then the catch-all.
func (r SLARule) specificity() int {
	n := 0
	if r.Category != nil {
		n += 2
	}
	if r.Priority != "" {
		n++
	}
	return n
}

// SLARuleInput is a rule as an admin saves it.
type SLARuleInput struct {
	CategoryID           int64 // 0 = any
	Priority             string
	FirstResponseMinutes *int
	ResolutionMinutes    *int
}

// SLARules returns every rule, most specific first.
func (s *Store) SLARules(ctx context.Context) ([]SLARule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.priority, r.first_response_minutes, r.resolution_minutes, c.id, c.name, c.color
		FROM sla_rules r LEFT JOIN ticket_categories c ON c.id = r.category_id
		ORDER BY (r.category_id IS NOT NULL) * 2 + (r.priority IS NOT NULL) DESC,
		         c.name COLLATE NOCASE,
		         CASE r.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SLARule{}
	for rows.Next() {
		var r SLARule
		var prio sql.NullString
		var first, res sql.NullInt64
		var catID sql.NullInt64
		var catName, catColor sql.NullString
		if err := rows.Scan(&r.ID, &prio, &first, &res, &catID, &catName, &catColor); err != nil {
			return nil, err
		}
		r.Priority = prio.String
		if first.Valid {
			n := int(first.Int64)
			r.FirstResponseMinutes = &n
		}
		if res.Valid {
			n := int(res.Int64)
			r.ResolutionMinutes = &n
		}
		r.Category = categoryRef(catID, catName, catColor)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateSLARule(ctx context.Context, in SLARuleInput) (int64, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO sla_rules (category_id, priority, first_response_minutes, resolution_minutes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		nullID(in.CategoryID), nullString(in.Priority), in.FirstResponseMinutes, in.ResolutionMinutes, now, now)
	if isUniqueErr(err) {
		return 0, ErrDuplicateRule
	} else if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateSLARule(ctx context.Context, id int64, in SLARuleInput) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sla_rules SET category_id = ?, priority = ?, first_response_minutes = ?, resolution_minutes = ?, updated_at = ?
		WHERE id = ?`,
		nullID(in.CategoryID), nullString(in.Priority), in.FirstResponseMinutes, in.ResolutionMinutes, time.Now().Unix(), id)
	if isUniqueErr(err) {
		return ErrDuplicateRule
	} else if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteSLARule(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sla_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func isUniqueErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// MatchSLARule returns the most specific rule for a ticket's category (0 =
// none) and priority, or nil. rules must be most specific first, as
// SLARules returns them.
func MatchSLARule(rules []SLARule, categoryID int64, priority string) *SLARule {
	for i, r := range rules {
		if r.Category != nil && r.Category.ID != categoryID {
			continue
		}
		if r.Priority != "" && r.Priority != priority {
			continue
		}
		return &rules[i]
	}
	return nil
}

// --- The clock ---

// SLAClock is what a ticket stores to work out its SLA. Time only counts
// while the ticket is being worked: it pauses while Pending or finished.
type SLAClock struct {
	CreatedAt            time.Time
	PausedSeconds        int64
	PausedSince          *time.Time // nil while running
	FirstResponseSeconds *int64     // on this clock; nil until staff reply
	Alerts               int
}

// Alert bits stored in tickets.sla_alerts.
const (
	AlertFirstResponseWarning = 1 << iota
	AlertFirstResponseBreach
	AlertResolutionWarning
	AlertResolutionBreach
)

// SLAWarningShare is how far through a target the warning goes out.
const SLAWarningShare = 0.75

// slaPaused reports whether the clock stops in this status.
func slaPaused(status string) bool { return status == "pending" || IsDone(status) }

// Elapsed is the time counted against the SLA as of now.
func (c SLAClock) Elapsed(now time.Time) time.Duration {
	end := now
	if c.PausedSince != nil {
		end = *c.PausedSince
	}
	d := end.Sub(c.CreatedAt) - time.Duration(c.PausedSeconds)*time.Second
	if d < 0 {
		return 0
	}
	return d
}

// SLATarget is how one target stands.
type SLATarget struct {
	TargetMinutes int `json:"targetMinutes"`
	// State is "running", "paused" (Pending), "overdue" (still open, past the
	// target), "met" or "missed" (finished in or past the target).
	State          string `json:"state"`
	ElapsedSeconds int64  `json:"elapsedSeconds"`
	// DueAt is when a running target falls due.
	DueAt *time.Time `json:"dueAt,omitempty"`
}

// SLAStatus is a ticket's SLA, for staff.
type SLAStatus struct {
	RuleID        int64      `json:"ruleId"`
	FirstResponse *SLATarget `json:"firstResponse,omitempty"`
	Resolution    *SLATarget `json:"resolution,omitempty"`
}

// ComputeSLA works out a ticket's SLA, or nil if no rule applies or it
// doesn't apply (raised in error).
func ComputeSLA(rules []SLARule, categoryID int64, priority, status string, c SLAClock, now time.Time) *SLAStatus {
	if status == "raised_in_error" {
		return nil
	}
	r := MatchSLARule(rules, categoryID, priority)
	if r == nil {
		return nil
	}
	elapsed := c.Elapsed(now)
	paused := slaPaused(status)
	done := IsDone(status)

	running := func(minutes int) *SLATarget {
		target := time.Duration(minutes) * time.Minute
		t := &SLATarget{TargetMinutes: minutes, ElapsedSeconds: int64(elapsed / time.Second)}
		switch {
		case elapsed > target:
			t.State = "overdue"
		case paused:
			t.State = "paused"
		default:
			t.State = "running"
			due := now.Add(target - elapsed)
			t.DueAt = &due
		}
		return t
	}
	finished := func(minutes int, took time.Duration) *SLATarget {
		t := &SLATarget{TargetMinutes: minutes, ElapsedSeconds: int64(took / time.Second), State: "met"}
		if took > time.Duration(minutes)*time.Minute {
			t.State = "missed"
		}
		return t
	}

	st := &SLAStatus{RuleID: r.ID}
	if m := r.FirstResponseMinutes; m != nil {
		switch {
		case c.FirstResponseSeconds != nil:
			st.FirstResponse = finished(*m, time.Duration(*c.FirstResponseSeconds)*time.Second)
		case done:
			// Finished without a reply: nothing to measure.
		default:
			st.FirstResponse = running(*m)
		}
	}
	if m := r.ResolutionMinutes; m != nil {
		if done {
			st.Resolution = finished(*m, elapsed)
		} else {
			st.Resolution = running(*m)
		}
	}
	if st.FirstResponse == nil && st.Resolution == nil {
		return nil
	}
	return st
}

// slaClockCols are the columns scanned by slaClockScan, from tickets t.
const slaClockCols = `t.sla_paused_seconds, t.sla_paused_since, t.first_response_seconds, t.sla_alerts`

type slaClockScan struct {
	paused int64
	since  sql.NullInt64
	first  sql.NullInt64
	alerts int
}

func (s *slaClockScan) dest() []any { return []any{&s.paused, &s.since, &s.first, &s.alerts} }

func (s *slaClockScan) clock(created time.Time) SLAClock {
	c := SLAClock{CreatedAt: created, PausedSeconds: s.paused, Alerts: s.alerts}
	if s.since.Valid {
		t := time.Unix(s.since.Int64, 0).UTC()
		c.PausedSince = &t
	}
	if s.first.Valid {
		n := s.first.Int64
		c.FirstResponseSeconds = &n
	}
	return c
}

// --- Alerts ---

// SLACandidate is an open ticket the alert check looks at.
type SLACandidate struct {
	ID         int64
	CategoryID int64
	Priority   string
	Status     string
	Clock      SLAClock
}

// SLACandidates returns open tickets that still have alerts to send.
func (s *Store) SLACandidates(ctx context.Context) ([]SLACandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, COALESCE(t.category_id, 0), t.priority, t.status, t.created_at, `+slaClockCols+`
		FROM tickets t
		WHERE t.status NOT IN `+doneStatuses+` AND t.sla_alerts != 15`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SLACandidate
	for rows.Next() {
		var c SLACandidate
		var created int64
		var cl slaClockScan
		if err := rows.Scan(append([]any{&c.ID, &c.CategoryID, &c.Priority, &c.Status, &created}, cl.dest()...)...); err != nil {
			return nil, err
		}
		c.Clock = cl.clock(time.Unix(created, 0).UTC())
		out = append(out, c)
	}
	return out, rows.Err()
}

// AddSLAAlerts records that alerts went out (bits are only ever added).
func (s *Store) AddSLAAlerts(ctx context.Context, ticketID int64, bits int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE tickets SET sla_alerts = sla_alerts | ? WHERE id = ?`, bits, ticketID)
	return err
}

// AdminIDs returns every admin, who get SLA alerts.
func (s *Store) AdminIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM users WHERE role = 'admin' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
