package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// IsUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// AdminUser is a user as shown in the admin user list.
type AdminUser struct {
	User
	PendingReset  bool `json:"pendingReset"`
	TicketCount   int  `json:"ticketCount"`   // tickets they raised (deleted with the account)
	AssignedCount int  `json:"assignedCount"` // active tickets assigned to them
}

func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

// ListUsers returns users whose name or email contains q (all users when q is empty).
func (s *Store) ListUsers(ctx context.Context, q string) ([]AdminUser, error) {
	q = strings.TrimSpace(q)
	pat := likePattern(q)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+userCols+`,
		       EXISTS (SELECT 1 FROM password_reset_requests p WHERE p.user_id = u.id AND p.status = 'pending'),
		       (SELECT COUNT(*) FROM tickets t WHERE t.requester_id = u.id),
		       (SELECT COUNT(*) FROM tickets t WHERE t.assignee_id = u.id AND t.status NOT IN `+doneStatuses+`)
		FROM users u
		WHERE ? = '' OR u.name LIKE ? ESCAPE '\' OR u.email LIKE ? ESCAPE '\'
		ORDER BY u.name COLLATE NOCASE, u.id
		LIMIT 500`, q, pat, pat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []AdminUser{}
	for rows.Next() {
		var u AdminUser
		var created int64
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &created, &u.MustChangePassword, &u.Avatar, &u.TwoFactorEnabled, &u.TOTPSecret, &u.TOTPLastStep, &u.EmailNotifications, &u.EmailEvents, &u.PendingReset, &u.TicketCount, &u.AssignedCount); err != nil {
			return nil, err
		}
		u.AvatarURL = AvatarURL(u.ID, u.Avatar)
		u.CreatedAt = time.Unix(created, 0).UTC()
		users = append(users, u)
	}
	return users, rows.Err()
}

// SetPassword sets a user's password, signs them out of every session except
// keepSession (so someone changing their own password stays signed in), and
// marks any pending reset request as completed by resolvedBy.
// mustChange forces the user to choose a new password at their next request.
func (s *Store) SetPassword(ctx context.Context, userID int64, passwordHash string, mustChange bool, resolvedBy int64, keepSession string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, must_change_password = ?, updated_at = ? WHERE id = ?`,
		passwordHash, mustChange, now, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, keepSession); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE password_reset_requests SET status = 'completed', resolved_at = ?, resolved_by = ?
		WHERE user_id = ? AND status = 'pending'`, now, resolvedBy, userID); err != nil {
		return err
	}
	return tx.Commit()
}

type ResetRequestUser struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ResetRequest struct {
	ID         int64            `json:"id"`
	Status     string           `json:"status"`
	IP         string           `json:"ip"`
	CreatedAt  time.Time        `json:"createdAt"`
	ResolvedAt *time.Time       `json:"resolvedAt,omitempty"`
	User       ResetRequestUser `json:"user"`
}

// CreateResetRequest opens a reset request for a user. It's a no-op if one
// is already pending.
func (s *Store) CreateResetRequest(ctx context.Context, userID int64, ip string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO password_reset_requests (user_id, ip, created_at) VALUES (?, ?, ?)
		ON CONFLICT DO NOTHING`, userID, ip, time.Now().Unix())
	return err
}

// ListResetRequests returns requests with the given status, newest first.
func (s *Store) ListResetRequests(ctx context.Context, status string) ([]ResetRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.status, p.ip, p.created_at, p.resolved_at, u.id, u.name, u.email
		FROM password_reset_requests p JOIN users u ON u.id = p.user_id
		WHERE p.status = ?
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT 200`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ResetRequest{}
	for rows.Next() {
		var r ResetRequest
		var created int64
		var resolved sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Status, &r.IP, &created, &resolved, &r.User.ID, &r.User.Name, &r.User.Email); err != nil {
			return nil, err
		}
		r.CreatedAt = time.Unix(created, 0).UTC()
		if resolved.Valid {
			t := time.Unix(resolved.Int64, 0).UTC()
			r.ResolvedAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DismissResetRequest closes a pending request without changing the password.
func (s *Store) DismissResetRequest(ctx context.Context, id, adminID int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE password_reset_requests SET status = 'dismissed', resolved_at = ?, resolved_by = ?
		WHERE id = ? AND status = 'pending'`, time.Now().Unix(), adminID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
