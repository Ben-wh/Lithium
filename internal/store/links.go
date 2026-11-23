package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

var (
	ErrLinkSelf  = errors.New("a ticket can't be linked to itself")
	ErrLinkCycle = errors.New("that link would make a loop")
)

// TicketLink is a linked ticket as shown in link lists and the incident tree.
type TicketLink struct {
	ID          int64 `json:"id"`
	ParentID    int64 `json:"parentId,omitempty"`
	RequesterID int64 `json:"-"`
	// Restricted marks a ticket in an incident tree the viewer isn't allowed
	// to see; only its position is shown.
	Restricted bool         `json:"restricted,omitempty"`
	Subject    string       `json:"subject"`
	Status     string       `json:"status"`
	Priority   string       `json:"priority"`
	Assignee   *UserRef     `json:"assignee"`
	Category   *CategoryRef `json:"category"`
}

// TicketLinks is a ticket's place in its tree.
type TicketLinks struct {
	Parent   *TicketLink  `json:"parent"`
	Children []TicketLink `json:"children"`
	// IncidentRootID is the top of the tree when that ticket is urgent and the
	// tree has more than one ticket; 0 otherwise.
	IncidentRootID int64 `json:"incidentRootId"`
}

const linkCols = `t.id, COALESCE(t.parent_id, 0), t.requester_id, t.subject, t.status, t.priority,
	a.id, a.name, a.email, a.avatar, cat.id, cat.name, cat.color`
const linkFrom = ` FROM tickets t
	LEFT JOIN users a ON a.id = t.assignee_id
	LEFT JOIN ticket_categories cat ON cat.id = t.category_id `

func scanLink(row scanner) (TicketLink, error) {
	var l TicketLink
	var asg userRefScan
	var catID sql.NullInt64
	var catName, catColor sql.NullString
	dest := []any{&l.ID, &l.ParentID, &l.RequesterID, &l.Subject, &l.Status, &l.Priority}
	dest = append(dest, asg.dest()...)
	dest = append(dest, &catID, &catName, &catColor)
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return l, ErrNotFound
		}
		return l, err
	}
	l.Assignee = asg.ref()
	l.Category = categoryRef(catID, catName, catColor)
	return l, nil
}

func (s *Store) queryLinks(ctx context.Context, where string, args ...any) ([]TicketLink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+linkCols+linkFrom+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TicketLink{}
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// rootOf walks up from a ticket to the top of its tree.
func (s *Store) rootOf(ctx context.Context, id int64) (int64, error) {
	for depth := 0; depth < 1000; depth++ {
		var parent sql.NullInt64
		if err := s.db.QueryRowContext(ctx, `SELECT parent_id FROM tickets WHERE id = ?`, id).Scan(&parent); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, ErrNotFound
			}
			return 0, err
		}
		if !parent.Valid {
			return id, nil
		}
		id = parent.Int64
	}
	return 0, ErrLinkCycle // can't happen: links are checked for loops
}

// Links returns a ticket's parent, children and incident root.
func (s *Store) Links(ctx context.Context, id int64) (TicketLinks, error) {
	var out TicketLinks
	var parentID sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT parent_id FROM tickets WHERE id = ?`, id).Scan(&parentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, ErrNotFound
		}
		return out, err
	}
	if parentID.Valid {
		p, err := scanLink(s.db.QueryRowContext(ctx, `SELECT `+linkCols+linkFrom+`WHERE t.id = ?`, parentID.Int64))
		if err != nil {
			return out, err
		}
		out.Parent = &p
	}
	var err error
	if out.Children, err = s.queryLinks(ctx, `WHERE t.parent_id = ? ORDER BY t.id`, id); err != nil {
		return out, err
	}
	if out.Parent == nil && len(out.Children) == 0 {
		return out, nil
	}
	root, err := s.rootOf(ctx, id)
	if err != nil {
		return out, err
	}
	var priority string
	if err := s.db.QueryRowContext(ctx, `SELECT priority FROM tickets WHERE id = ?`, root).Scan(&priority); err != nil {
		return out, err
	}
	if priority == "urgent" {
		out.IncidentRootID = root
	}
	return out, nil
}

// Tree returns every ticket in the tree containing id, root first (each with
// its ParentID), capped at maxNodes.
func (s *Store) Tree(ctx context.Context, id int64, maxNodes int) ([]TicketLink, error) {
	root, err := s.rootOf(ctx, id)
	if err != nil {
		return nil, err
	}
	nodes, err := s.queryLinks(ctx, `WHERE t.id = ?`, root)
	if err != nil {
		return nil, err
	}
	nodes[0].ParentID = 0
	frontier := []int64{root}
	for len(frontier) > 0 && len(nodes) < maxNodes {
		var next []int64
		for _, pid := range frontier {
			kids, err := s.queryLinks(ctx, `WHERE t.parent_id = ? ORDER BY t.id`, pid)
			if err != nil {
				return nil, err
			}
			for _, k := range kids {
				if len(nodes) >= maxNodes {
					break
				}
				nodes = append(nodes, k)
				next = append(next, k.ID)
			}
		}
		frontier = next
	}
	return nodes, nil
}

// SetParent links a ticket under parentID (0 unlinks it). It refuses links to
// itself or that would create a loop, and records a staff-only Lithium update
// on each ticket involved.
func (s *Store) SetParent(ctx context.Context, id, parentID, actorID int64) error {
	if id == parentID {
		return ErrLinkSelf
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var current sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT parent_id FROM tickets WHERE id = ?`, id).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if current.Int64 == parentID {
		return nil
	}
	// The new parent must exist and must not be this ticket's descendant.
	for at := parentID; at != 0; {
		var up sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT parent_id FROM tickets WHERE id = ?`, at).Scan(&up); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if up.Int64 == id {
			return ErrLinkCycle
		}
		at = up.Int64
	}

	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, `UPDATE tickets SET parent_id = ?, updated_at = ? WHERE id = ?`, nullID(parentID), now, id); err != nil {
		return err
	}
	event := `INSERT INTO ticket_comments (ticket_id, kind, author_id, event, old_value, new_value, internal, created_at)
		VALUES (?, 'event', ?, ?, ?, ?, 1, ?)`
	idStr := func(n int64) string {
		if n == 0 {
			return ""
		}
		return strconv.FormatInt(n, 10)
	}
	if _, err := tx.ExecContext(ctx, event, id, actorID, "parent", idStr(current.Int64), idStr(parentID), now); err != nil {
		return err
	}
	if current.Valid {
		if _, err := tx.ExecContext(ctx, event, current.Int64, actorID, "child", idStr(id), "", now); err != nil {
			return err
		}
	}
	if parentID != 0 {
		if _, err := tx.ExecContext(ctx, event, parentID, actorID, "child", "", idStr(id), now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
