package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Person is someone in the User book: a user (or staff member who has raised
// tickets) with counts of the tickets the viewer is allowed to see.
type Person struct {
	UserRef
	Role         string     `json:"role"`
	OpenTickets  int        `json:"openTickets"`
	TotalTickets int        `json:"totalTickets"`
	LastTicketAt *time.Time `json:"lastTicketAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// personQuery builds the User book query. scope holds the viewer's ticket
// visibility rules (RequesterID is ignored), so counts only include tickets
// they may see.
func personQuery(scope TicketFilter, extraWhere string, extraArgs ...any) (string, []any) {
	scope.RequesterID = 0
	join := "t.requester_id = u.id"
	conds, args := scope.conds()
	if conds != "" {
		join += " AND " + conds
	}
	q := `
		SELECT u.id, u.name, u.email, u.avatar, u.role, u.created_at,
		       COUNT(t.id),
		       COALESCE(SUM(t.status NOT IN ` + doneStatuses + `), 0),
		       MAX(t.created_at)
		FROM users u
		LEFT JOIN tickets t ON ` + join + `
		WHERE (u.role = 'user' OR EXISTS (SELECT 1 FROM tickets x WHERE x.requester_id = u.id)) ` + extraWhere + `
		GROUP BY u.id`
	return q, append(args, extraArgs...)
}

func scanPerson(row scanner) (Person, error) {
	var p Person
	var u userRefScan
	var created int64
	var last sql.NullInt64
	dest := append(u.dest(), &p.Role, &created, &p.TotalTickets, &p.OpenTickets, &last)
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return p, ErrNotFound
		}
		return p, err
	}
	p.UserRef = *u.ref()
	p.CreatedAt = time.Unix(created, 0).UTC()
	if last.Valid {
		t := time.Unix(last.Int64, 0).UTC()
		p.LastTicketAt = &t
	}
	return p, nil
}

// ListPeople returns one page of the User book, most recently active first.
func (s *Store) ListPeople(ctx context.Context, scope TicketFilter, query string, limit, offset int) ([]Person, int, error) {
	query = strings.TrimSpace(query)
	pat := likePattern(query)
	match := `AND (? = '' OR u.name LIKE ? ESCAPE '\' OR u.email LIKE ? ESCAPE '\')`

	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users u
		WHERE (u.role = 'user' OR EXISTS (SELECT 1 FROM tickets x WHERE x.requester_id = u.id)) `+match,
		query, pat, pat).Scan(&total); err != nil {
		return nil, 0, err
	}

	q, args := personQuery(scope, match, query, pat, pat)
	rows, err := s.db.QueryContext(ctx, q+`
		ORDER BY MAX(t.created_at) IS NULL, MAX(t.created_at) DESC, u.name COLLATE NOCASE
		LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Person{}
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// PersonByID returns one User book entry.
func (s *Store) PersonByID(ctx context.Context, scope TicketFilter, id int64) (Person, error) {
	q, args := personQuery(scope, `AND u.id = ?`, id)
	return scanPerson(s.db.QueryRowContext(ctx, q, args...))
}
