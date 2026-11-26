package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// UserRef is the public face of a user when shown alongside other records.
type UserRef struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
}

// userRefScan collects a joined user's columns (which may all be NULL for an
// optional join) and turns them into a UserRef.
type userRefScan struct {
	id                  sql.NullInt64
	name, email, avatar sql.NullString
}

func (u *userRefScan) dest() []any { return []any{&u.id, &u.name, &u.email, &u.avatar} }

func (u *userRefScan) ref() *UserRef {
	if !u.id.Valid {
		return nil
	}
	return &UserRef{ID: u.id.Int64, Name: u.name.String, Email: u.email.String, AvatarURL: AvatarURL(u.id.Int64, u.avatar.String)}
}

type Ticket struct {
	ID          int64        `json:"id"`
	Subject     string       `json:"subject"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	Priority    string       `json:"priority"`
	Requester   UserRef      `json:"requester"`
	CreatedBy   UserRef      `json:"createdBy"`
	Assignee    *UserRef     `json:"assignee"`
	Category    *CategoryRef `json:"category"`
	Attachments []Attachment `json:"attachments"`
	// Fields are the template answers given when the ticket was raised, and
	// TemplateName the template they came from ("" for none).
	Fields       []TicketField `json:"fields"`
	TemplateName string        `json:"templateName"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

type NewTicket struct {
	Subject     string
	Description string
	Priority    string
	CategoryID  int64 // 0 = none
	RequesterID int64
	CreatedBy   int64
	Attachments []NewAttachment
	// Fields and TemplateName record a template's answers (see Ticket).
	Fields       []TicketField
	TemplateName string
}

func nullID(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: id != 0} }

func categoryRef(id sql.NullInt64, name, color sql.NullString) *CategoryRef {
	if !id.Valid {
		return nil
	}
	return &CategoryRef{ID: id.Int64, Name: name.String, Color: color.String}
}

// CreateTicket records a ticket and its attachments together, so a ticket
// never exists with half its files.
func (s *Store) CreateTicket(ctx context.Context, n NewTicket) (Ticket, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Ticket{}, err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	if n.Fields == nil {
		n.Fields = []TicketField{}
	}
	fields, err := json.Marshal(n.Fields)
	if err != nil {
		return Ticket{}, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO tickets (subject, description, priority, category_id, requester_id, created_by, created_at, updated_at, custom_fields, template_name)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Subject, n.Description, n.Priority, nullID(n.CategoryID), n.RequesterID, n.CreatedBy, now, now, string(fields), n.TemplateName)
	if err != nil {
		return Ticket{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Ticket{}, err
	}
	for _, a := range n.Attachments {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ticket_attachments (ticket_id, filename, stored_name, content_type, size, uploaded_by, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, a.Filename, a.StoredName, a.ContentType, a.Size, n.CreatedBy, now); err != nil {
			return Ticket{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Ticket{}, err
	}
	return s.TicketByID(ctx, id)
}

func (s *Store) TicketByID(ctx context.Context, id int64) (Ticket, error) {
	var t Ticket
	var created, updated int64
	var req, by, asg userRefScan
	var catID sql.NullInt64
	var catName, catColor sql.NullString
	dest := []any{&t.ID, &t.Subject, &t.Description, &t.Status, &t.Priority, &created, &updated}
	dest = append(dest, req.dest()...)
	dest = append(dest, by.dest()...)
	dest = append(dest, asg.dest()...)
	var fields string
	dest = append(dest, &catID, &catName, &catColor, &fields, &t.TemplateName)
	err := s.db.QueryRowContext(ctx, `
		SELECT t.id, t.subject, t.description, t.status, t.priority, t.created_at, t.updated_at,
		       r.id, r.name, r.email, r.avatar,
		       c.id, c.name, c.email, c.avatar,
		       a.id, a.name, a.email, a.avatar,
		       cat.id, cat.name, cat.color, t.custom_fields, t.template_name
		FROM tickets t
		JOIN users r ON r.id = t.requester_id
		JOIN users c ON c.id = t.created_by
		LEFT JOIN users a ON a.id = t.assignee_id
		LEFT JOIN ticket_categories cat ON cat.id = t.category_id
		WHERE t.id = ?`, id).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return Ticket{}, ErrNotFound
	} else if err != nil {
		return Ticket{}, err
	}
	if err := json.Unmarshal([]byte(fields), &t.Fields); err != nil || t.Fields == nil {
		t.Fields = []TicketField{}
	}
	t.CreatedAt = time.Unix(created, 0).UTC()
	t.UpdatedAt = time.Unix(updated, 0).UTC()
	t.Requester, t.CreatedBy, t.Assignee = *req.ref(), *by.ref(), asg.ref()
	t.Category = categoryRef(catID, catName, catColor)
	if t.Attachments, err = s.AttachmentsForTicket(ctx, t.ID); err != nil {
		return Ticket{}, err
	}
	return t, nil
}

// SearchUsers is a lightweight directory lookup (e.g. picking a requester).
// staffOnly limits it to admins and agents (e.g. picking an assignee).
func (s *Store) SearchUsers(ctx context.Context, q string, limit int, staffOnly bool) ([]UserRef, error) {
	q = strings.TrimSpace(q)
	pat := likePattern(q)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, email, avatar FROM users
		WHERE (? = '' OR name LIKE ? ESCAPE '\' OR email LIKE ? ESCAPE '\')
		  AND (NOT ? OR role IN ('admin', 'agent'))
		ORDER BY name COLLATE NOCASE, id
		LIMIT ?`, q, pat, pat, staffOnly, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserRef{}
	for rows.Next() {
		var u userRefScan
		if err := rows.Scan(u.dest()...); err != nil {
			return nil, err
		}
		out = append(out, *u.ref())
	}
	return out, rows.Err()
}

// TicketSummary is a ticket as shown in lists (no description).
type TicketSummary struct {
	ID        int64        `json:"id"`
	Subject   string       `json:"subject"`
	Status    string       `json:"status"`
	Priority  string       `json:"priority"`
	Requester UserRef      `json:"requester"`
	Assignee  *UserRef     `json:"assignee"`
	Category  *CategoryRef `json:"category"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

type TicketFilter struct {
	RequesterID int64    // 0 = anyone
	Statuses    []string // empty = any
	Priority    string   // "" = any
	AssigneeID  int64    // 0 = anyone
	Unassigned  bool     // only tickets with no assignee
	CategoryID  int64    // 0 = any
	// Scope limits results to what a group-restricted agent may see: tickets
	// they raised or are assigned, uncategorised ones, and ScopeCategories.
	Scoped          bool
	ScopeUserID     int64
	ScopeCategories []int64
	Query           string // matches subject, requester, or a ticket reference ("TID-123", "#123", "123")
	Sort            string // a key of ticketSorts; "" = updated
	Descending      bool   // only used with Sort; the default (updated) is newest first
	Limit           int
	Offset          int
}

// ticketSorts maps a sort key to its ORDER BY expression, in ascending order.
// Ties are broken by most recently updated.
var ticketSorts = map[string]string{
	"updated": "t.updated_at",
	"created": "t.created_at",
	// Ascending is least urgent first; descending puts urgent at the top.
	"priority": "CASE t.priority WHEN 'low' THEN 0 WHEN 'normal' THEN 1 WHEN 'high' THEN 2 ELSE 3 END",
	// The order tickets move through.
	"status": "CASE t.status WHEN 'unassigned' THEN 0 WHEN 'in_progress' THEN 1 WHEN 'pending' THEN 2 " +
		"WHEN 'resolved' THEN 3 WHEN 'closed' THEN 4 ELSE 5 END",
	"subject": "t.subject COLLATE NOCASE",
	// Unassigned tickets go last either way.
	"assignee": "a.name IS NULL, a.name COLLATE NOCASE",
}

// ValidTicketSort reports whether key can be used as TicketFilter.Sort.
func ValidTicketSort(key string) bool { _, ok := ticketSorts[key]; return ok }

func (f TicketFilter) orderBy() string {
	expr, ok := ticketSorts[f.Sort]
	if !ok {
		return "ORDER BY t.updated_at DESC, t.id DESC"
	}
	dir := " ASC"
	if f.Descending {
		dir = " DESC"
	}
	// The direction applies to the last term, so "assignee" keeps unassigned last.
	return "ORDER BY " + expr + dir + ", t.updated_at DESC, t.id DESC"
}

func (f TicketFilter) where() (string, []any) {
	conds, args := f.conds()
	if conds == "" {
		return "", nil
	}
	return "WHERE " + conds, args
}

// conds returns the filter's conditions joined with AND ("" if none).
func (f TicketFilter) conds() (string, []any) {
	var conds []string
	var args []any
	if f.RequesterID != 0 {
		conds = append(conds, "t.requester_id = ?")
		args = append(args, f.RequesterID)
	}
	if len(f.Statuses) > 0 {
		conds = append(conds, "t.status IN (?"+strings.Repeat(", ?", len(f.Statuses)-1)+")")
		for _, s := range f.Statuses {
			args = append(args, s)
		}
	}
	if f.Priority != "" {
		conds = append(conds, "t.priority = ?")
		args = append(args, f.Priority)
	}
	if f.Unassigned {
		conds = append(conds, "t.assignee_id IS NULL")
	} else if f.AssigneeID != 0 {
		conds = append(conds, "t.assignee_id = ?")
		args = append(args, f.AssigneeID)
	}
	if f.CategoryID != 0 {
		conds = append(conds, "t.category_id = ?")
		args = append(args, f.CategoryID)
	}
	if f.Scoped {
		cond := "(t.requester_id = ? OR t.assignee_id = ? OR t.category_id IS NULL"
		args = append(args, f.ScopeUserID, f.ScopeUserID)
		if n := len(f.ScopeCategories); n > 0 {
			cond += " OR t.category_id IN (?" + strings.Repeat(", ?", n-1) + ")"
			for _, c := range f.ScopeCategories {
				args = append(args, c)
			}
		}
		conds = append(conds, cond+")")
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		pat := likePattern(q)
		cond := "(t.subject LIKE ? ESCAPE '\\' OR r.name LIKE ? ESCAPE '\\' OR r.email LIKE ? ESCAPE '\\'"
		args = append(args, pat, pat, pat)
		if id, ok := parseTicketRef(q); ok {
			cond += " OR t.id = ?"
			args = append(args, id)
		}
		conds = append(conds, cond+")")
	}
	return strings.Join(conds, " AND "), args
}

// ListTickets returns one page of matching tickets, most recently updated
// first, and the total number of matches.
func (s *Store) ListTickets(ctx context.Context, f TicketFilter) ([]TicketSummary, int, error) {
	where, args := f.where()
	const from = `FROM tickets t
		JOIN users r ON r.id = t.requester_id
		LEFT JOIN users a ON a.id = t.assignee_id
		LEFT JOIN ticket_categories cat ON cat.id = t.category_id `

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+from+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.subject, t.status, t.priority, t.created_at, t.updated_at,
		       r.id, r.name, r.email, r.avatar, a.id, a.name, a.email, a.avatar, cat.id, cat.name, cat.color `+from+where+`
		`+f.orderBy()+`
		LIMIT ? OFFSET ?`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []TicketSummary{}
	for rows.Next() {
		var t TicketSummary
		var created, updated int64
		var req, asg userRefScan
		var catID sql.NullInt64
		var catName, catColor sql.NullString
		dest := []any{&t.ID, &t.Subject, &t.Status, &t.Priority, &created, &updated}
		dest = append(dest, req.dest()...)
		dest = append(dest, asg.dest()...)
		dest = append(dest, &catID, &catName, &catColor)
		if err := rows.Scan(dest...); err != nil {
			return nil, 0, err
		}
		t.CreatedAt = time.Unix(created, 0).UTC()
		t.UpdatedAt = time.Unix(updated, 0).UTC()
		t.Requester, t.Assignee = *req.ref(), asg.ref()
		t.Category = categoryRef(catID, catName, catColor)
		out = append(out, t)
	}
	return out, total, rows.Err()
}

type TicketStats struct {
	Unassigned       int `json:"unassigned"`
	InProgress       int `json:"inProgress"`
	Pending          int `json:"pending"`
	AssignedToMe     int `json:"assignedToMe"`     // active (not resolved/closed) and assigned to the viewer
	ResolvedThisWeek int `json:"resolvedThisWeek"` // resolved/closed, last changed in the past 7 days
}

// TicketStats counts tickets for the dashboard, limited to what the filter
// allows the viewer to see (only RequesterID and the scope fields are used).
func (s *Store) TicketStats(ctx context.Context, viewerID int64, f TicketFilter) (TicketStats, error) {
	var st TicketStats
	weekAgo := time.Now().Add(-7 * 24 * time.Hour).Unix()
	where, args := TicketFilter{RequesterID: f.RequesterID, Scoped: f.Scoped, ScopeUserID: f.ScopeUserID, ScopeCategories: f.ScopeCategories}.where()
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(t.status = 'unassigned'), 0),
		       COALESCE(SUM(t.status = 'in_progress'), 0),
		       COALESCE(SUM(t.status = 'pending'), 0),
		       COALESCE(SUM(t.assignee_id = ? AND t.status NOT IN `+doneStatuses+`), 0),
		       COALESCE(SUM(t.status IN ('resolved', 'closed') AND t.updated_at >= ?), 0)
		FROM tickets t `+where,
		append([]any{viewerID, weekAgo}, args...)...).Scan(&st.Unassigned, &st.InProgress, &st.Pending, &st.AssignedToMe, &st.ResolvedThisWeek)
	return st, err
}

// parseTicketRef reads a ticket number from "123", "#123" or "<PREFIX>-123".
// Any prefix is accepted, so references keep working after it's changed.
func parseTicketRef(q string) (int64, bool) {
	q = strings.TrimPrefix(strings.TrimSpace(q), "#")
	if i := strings.LastIndex(q, "-"); i > 0 {
		prefix := q[:i]
		for _, r := range prefix {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
				return 0, false
			}
		}
		q = q[i+1:]
	}
	id, err := strconv.ParseInt(q, 10, 64)
	return id, err == nil && id > 0
}
