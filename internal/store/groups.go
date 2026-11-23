package store

import (
	"context"
	"time"
)

type Group struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Members     []UserRef     `json:"members"`
	Categories  []CategoryRef `json:"categories"`
	CreatedAt   time.Time     `json:"createdAt"`
}

// ListGroups returns every group with its members and categories, by name.
func (s *Store) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, created_at FROM agent_groups ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	groups := []Group{}
	byID := map[int64]int{}
	for rows.Next() {
		var g Group
		var created int64
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &created); err != nil {
			rows.Close()
			return nil, err
		}
		g.CreatedAt = time.Unix(created, 0).UTC()
		g.Members, g.Categories = []UserRef{}, []CategoryRef{}
		byID[g.ID] = len(groups)
		groups = append(groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	mrows, err := s.db.QueryContext(ctx, `
		SELECT m.group_id, u.id, u.name, u.email, u.avatar
		FROM agent_group_members m JOIN users u ON u.id = m.user_id
		ORDER BY u.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	for mrows.Next() {
		var gid int64
		var u userRefScan
		if err := mrows.Scan(append([]any{&gid}, u.dest()...)...); err != nil {
			mrows.Close()
			return nil, err
		}
		if i, ok := byID[gid]; ok {
			groups[i].Members = append(groups[i].Members, *u.ref())
		}
	}
	mrows.Close()
	if err := mrows.Err(); err != nil {
		return nil, err
	}

	crows, err := s.db.QueryContext(ctx, `
		SELECT gc.group_id, c.id, c.name, c.color
		FROM agent_group_categories gc JOIN ticket_categories c ON c.id = gc.category_id
		ORDER BY c.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	for crows.Next() {
		var gid int64
		var c CategoryRef
		if err := crows.Scan(&gid, &c.ID, &c.Name, &c.Color); err != nil {
			return nil, err
		}
		if i, ok := byID[gid]; ok {
			groups[i].Categories = append(groups[i].Categories, c)
		}
	}
	return groups, crows.Err()
}

func (s *Store) GroupByID(ctx context.Context, id int64) (Group, error) {
	groups, err := s.ListGroups(ctx)
	if err != nil {
		return Group{}, err
	}
	for _, g := range groups {
		if g.ID == id {
			return g, nil
		}
	}
	return Group{}, ErrNotFound
}

// SaveGroup creates a group (id 0) or replaces an existing one's details,
// members and categories.
func (s *Store) SaveGroup(ctx context.Context, id int64, name, description string, memberIDs, categoryIDs []int64) (Group, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Group{}, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	if id == 0 {
		res, err := tx.ExecContext(ctx, `INSERT INTO agent_groups (name, description, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			name, description, now, now)
		if err != nil {
			return Group{}, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return Group{}, err
		}
	} else {
		res, err := tx.ExecContext(ctx, `UPDATE agent_groups SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
			name, description, now, id)
		if err != nil {
			return Group{}, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return Group{}, ErrNotFound
		}
	}
	for _, q := range []string{`DELETE FROM agent_group_members WHERE group_id = ?`, `DELETE FROM agent_group_categories WHERE group_id = ?`} {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return Group{}, err
		}
	}
	for _, uid := range memberIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO agent_group_members (group_id, user_id) VALUES (?, ?)`, id, uid); err != nil {
			return Group{}, err
		}
	}
	for _, cid := range categoryIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO agent_group_categories (group_id, category_id) VALUES (?, ?)`, id, cid); err != nil {
			return Group{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Group{}, err
	}
	return s.GroupByID(ctx, id)
}

func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM agent_groups WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GroupCategoryIDs returns the categories covered by every group a user is in.
func (s *Store) GroupCategoryIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT gc.category_id
		FROM agent_group_members m JOIN agent_group_categories gc ON gc.group_id = m.group_id
		WHERE m.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
