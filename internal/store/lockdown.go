package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Lockdown is the emergency state an admin can put Lithium into: everyone
// else is signed out and can't sign in until it ends.
type Lockdown struct {
	Active bool      `json:"active"`
	ByID   int64     `json:"byId"`
	ByName string    `json:"byName"`
	At     time.Time `json:"at"`
	Reason string    `json:"reason"`
}

const keyLockdown = "security.lockdown"

// ErrLockdownActive is returned when starting a lockdown that's already on.
var ErrLockdownActive = errors.New("lockdown is already active")

// Lockdown returns the current state (Active false when there's none).
func (s *Store) Lockdown(ctx context.Context) (Lockdown, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, keyLockdown).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || raw == "" {
		return Lockdown{}, nil
	}
	if err != nil {
		return Lockdown{}, err
	}
	var l Lockdown
	if err := json.Unmarshal([]byte(raw), &l); err != nil {
		return Lockdown{}, err
	}
	return l, nil
}

// StartLockdown turns lockdown on for admin `by` and, in the same
// transaction, deletes every session except keepSession (the admin's own),
// so nobody else stays signed in. It returns how many sessions were ended.
func (s *Store) StartLockdown(ctx context.Context, by User, reason, keepSession string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var raw string
	err = tx.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, keyLockdown).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if raw != "" {
		var cur Lockdown
		if json.Unmarshal([]byte(raw), &cur) == nil && cur.Active {
			return 0, ErrLockdownActive
		}
	}

	state, _ := json.Marshal(Lockdown{Active: true, ByID: by.ID, ByName: by.Name, At: time.Now().UTC(), Reason: reason})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		keyLockdown, string(state), time.Now().Unix()); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash != ?`, keepSession)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, tx.Commit()
}

// EndLockdown turns lockdown off. It reports whether one was active.
func (s *Store) EndLockdown(ctx context.Context) (bool, error) {
	cur, err := s.Lockdown(ctx)
	if err != nil {
		return false, err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM app_settings WHERE key = ?`, keyLockdown); err != nil {
		return false, err
	}
	return cur.Active, nil
}
