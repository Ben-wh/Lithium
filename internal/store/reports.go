package store

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DayCount struct {
	Date     string `json:"date"` // YYYY-MM-DD in the viewer's timezone
	Raised   int    `json:"raised"`
	Resolved int    `json:"resolved"`
}

type NamedCount struct {
	Key       string `json:"key"` // status / priority value, or an id as text ("" = none)
	Name      string `json:"name"`
	Color     string `json:"color,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	Count     int    `json:"count"`
}

type Report struct {
	Days   int        `json:"days"`
	Series []DayCount `json:"series"`
	Totals struct {
		Raised   int `json:"raised"`
		Resolved int `json:"resolved"`
		Open     int `json:"open"` // active right now (any age)
		// Medians in hours over the period; nil when there's no data.
		MedianFirstResponseHours *float64 `json:"medianFirstResponseHours"`
		MedianResolutionHours    *float64 `json:"medianResolutionHours"`
	} `json:"totals"`
	ByStatus   []NamedCount `json:"byStatus"`   // all tickets, current status
	ByPriority []NamedCount `json:"byPriority"` // active tickets
	ByCategory []NamedCount `json:"byCategory"` // active tickets
	ByAssignee []NamedCount `json:"byAssignee"` // active tickets
}

var reportStatuses = []string{"unassigned", "in_progress", "pending", "resolved", "closed"}
var reportPriorities = []string{"urgent", "high", "normal", "low"}

func isActive(status string) bool { return status != "resolved" && status != "closed" }

func median(xs []float64) *float64 {
	if len(xs) == 0 {
		return nil
	}
	sort.Float64s(xs)
	m := xs[len(xs)/2]
	if len(xs)%2 == 0 {
		m = (xs[len(xs)/2-1] + xs[len(xs)/2]) / 2
	}
	return &m
}

// BuildReport computes the Reports page figures over the last `days` days,
// limited to the tickets the scope allows, with days bucketed in loc.
func (s *Store) BuildReport(ctx context.Context, scope TicketFilter, days int, loc *time.Location) (Report, error) {
	var r Report
	r.Days = days
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(days - 1))
	from := start.Unix()

	scope.RequesterID, scope.Query = 0, ""
	conds, args := scope.conds()
	where := ""
	if conds != "" {
		where = " AND " + conds
	}

	// Empty day buckets for the whole period.
	idx := map[string]int{}
	for d := 0; d < days; d++ {
		date := start.AddDate(0, 0, d).Format("2006-01-02")
		idx[date] = d
		r.Series = append(r.Series, DayCount{Date: date})
	}
	day := func(unix int64) (int, bool) {
		i, ok := idx[time.Unix(unix, 0).In(loc).Format("2006-01-02")]
		return i, ok
	}

	// Every ticket in scope: current breakdowns, and those raised in the period.
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.status, t.priority, t.created_at,
		       cat.id, cat.name, cat.color, a.id, a.name, a.avatar
		FROM tickets t
		LEFT JOIN ticket_categories cat ON cat.id = t.category_id
		LEFT JOIN users a ON a.id = t.assignee_id
		WHERE 1 = 1`+where, args...)
	if err != nil {
		return r, err
	}
	statusCount := map[string]int{}
	priorityCount := map[string]int{}
	categories := map[string]*NamedCount{}
	assignees := map[string]*NamedCount{}
	for rows.Next() {
		var status, priority string
		var created int64
		var catID, aID sql.NullInt64
		var catName, catColor, aName, aAvatar sql.NullString
		if err := rows.Scan(&status, &priority, &created, &catID, &catName, &catColor, &aID, &aName, &aAvatar); err != nil {
			rows.Close()
			return r, err
		}
		statusCount[status]++
		if created >= from {
			if i, ok := day(created); ok {
				r.Series[i].Raised++
				r.Totals.Raised++
			}
		}
		if !isActive(status) {
			continue
		}
		r.Totals.Open++
		priorityCount[priority]++

		ck, cn, cc := "", "No category", ""
		if catID.Valid {
			ck, cn, cc = itoa64(catID.Int64), catName.String, catColor.String
		}
		if categories[ck] == nil {
			categories[ck] = &NamedCount{Key: ck, Name: cn, Color: cc}
		}
		categories[ck].Count++

		ak, an, av := "", "Unassigned", ""
		if aID.Valid {
			ak, an, av = itoa64(aID.Int64), aName.String, AvatarURL(aID.Int64, aAvatar.String)
		}
		if assignees[ak] == nil {
			assignees[ak] = &NamedCount{Key: ak, Name: an, AvatarURL: av}
		}
		assignees[ak].Count++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return r, err
	}
	for _, st := range reportStatuses {
		r.ByStatus = append(r.ByStatus, NamedCount{Key: st, Count: statusCount[st]})
	}
	for _, p := range reportPriorities {
		r.ByPriority = append(r.ByPriority, NamedCount{Key: p, Count: priorityCount[p]})
	}
	r.ByCategory = sortedCounts(categories)
	r.ByAssignee = sortedCounts(assignees)

	// Resolutions in the period: a status change into resolved/closed from an
	// active status. Time to resolve runs from when the ticket was raised.
	rows, err = s.db.QueryContext(ctx, `
		SELECT c.created_at, t.created_at
		FROM ticket_comments c JOIN tickets t ON t.id = c.ticket_id
		WHERE c.kind = 'event' AND c.event = 'status'
		  AND c.new_value IN ('resolved', 'closed') AND c.old_value NOT IN ('resolved', 'closed')
		  AND c.created_at >= ?`+where, append([]any{from}, args...)...)
	if err != nil {
		return r, err
	}
	var resolveHours []float64
	for rows.Next() {
		var at, created int64
		if err := rows.Scan(&at, &created); err != nil {
			rows.Close()
			return r, err
		}
		if i, ok := day(at); ok {
			r.Series[i].Resolved++
			r.Totals.Resolved++
		}
		resolveHours = append(resolveHours, float64(at-created)/3600)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return r, err
	}
	r.Totals.MedianResolutionHours = median(resolveHours)

	// First response: the first public reply by staff (other than the
	// requester) on tickets raised in the period.
	rows, err = s.db.QueryContext(ctx, `
		SELECT MIN(c.created_at) - t.created_at
		FROM ticket_comments c
		JOIN tickets t ON t.id = c.ticket_id
		JOIN users u ON u.id = c.author_id
		WHERE c.kind = 'comment' AND c.internal = 0 AND u.role IN ('admin', 'agent')
		  AND c.author_id != t.requester_id AND t.created_at >= ?`+where+`
		GROUP BY t.id`, append([]any{from}, args...)...)
	if err != nil {
		return r, err
	}
	var firstHours []float64
	for rows.Next() {
		var secs int64
		if err := rows.Scan(&secs); err != nil {
			rows.Close()
			return r, err
		}
		firstHours = append(firstHours, float64(secs)/3600)
	}
	rows.Close()
	r.Totals.MedianFirstResponseHours = median(firstHours)
	return r, rows.Err()
}

// sortedCounts orders by count (desc), keeping "none" ("" key) last.
func sortedCounts(m map[string]*NamedCount) []NamedCount {
	out := make([]NamedCount, 0, len(m))
	for _, c := range m {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Key == "") != (out[j].Key == "") {
			return out[j].Key == ""
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// ExportTicket is one row of a ticket export.
type ExportTicket struct {
	ID             int64     `json:"id"`
	Subject        string    `json:"subject"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"`
	Priority       string    `json:"priority"`
	Category       string    `json:"category"`
	Requester      string    `json:"requester"`
	RequesterEmail string    `json:"requesterEmail"`
	Assignee       string    `json:"assignee"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ExportTickets returns tickets raised between from and to (inclusive of
// from, exclusive of to) that the filter allows, oldest first.
func (s *Store) ExportTickets(ctx context.Context, f TicketFilter, from, to time.Time, withDescription bool) ([]ExportTicket, error) {
	f.Query = ""
	conds, args := f.conds()
	where := "WHERE t.created_at >= ? AND t.created_at < ?"
	args = append([]any{from.Unix(), to.Unix()}, args...)
	if conds != "" {
		where += " AND " + conds
	}
	desc := "''"
	if withDescription {
		desc = "t.description"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.subject, `+desc+`, t.status, t.priority, COALESCE(cat.name, ''),
		       r.name, r.email, COALESCE(a.name, ''), t.created_at, t.updated_at
		FROM tickets t
		JOIN users r ON r.id = t.requester_id
		LEFT JOIN users a ON a.id = t.assignee_id
		LEFT JOIN ticket_categories cat ON cat.id = t.category_id
		`+where+`
		ORDER BY t.created_at, t.id
		LIMIT 100000`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExportTicket{}
	for rows.Next() {
		var e ExportTicket
		var created, updated int64
		if err := rows.Scan(&e.ID, &e.Subject, &e.Description, &e.Status, &e.Priority, &e.Category,
			&e.Requester, &e.RequesterEmail, &e.Assignee, &created, &updated); err != nil {
			return nil, err
		}
		e.CreatedAt = time.Unix(created, 0).UTC()
		e.UpdatedAt = time.Unix(updated, 0).UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }
