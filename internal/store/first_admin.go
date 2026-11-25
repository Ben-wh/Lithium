package store

import (
	"context"
	"strconv"
	"time"
)

// keyFirstAdminTwoFactorSkip holds the ID of the admin created at first
// startup while they may still skip mandatory two-factor setup. It's a
// one-time offer: removed when they skip or when they set two-factor up.
const keyFirstAdminTwoFactorSkip = "setup.first_admin_2fa_skip"

// AllowFirstAdminTwoFactorSkip lets the admin created at first startup skip
// two-factor setup once, so a test install can be tried without an
// authenticator app.
func (s *Store) AllowFirstAdminTwoFactorSkip(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		keyFirstAdminTwoFactorSkip, strconv.FormatInt(userID, 10), time.Now().Unix())
	return err
}

// CanSkipTwoFactor reports whether this user is the first admin and still
// has the one-time offer to skip two-factor setup.
func (s *Store) CanSkipTwoFactor(ctx context.Context, userID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM app_settings WHERE key = ? AND value = ?`,
		keyFirstAdminTwoFactorSkip, strconv.FormatInt(userID, 10)).Scan(&n)
	return n > 0, err
}

// SkipFirstAdminTwoFactor uses up the one-time offer and turns mandatory
// two-factor off. It reports false (changing nothing) when this user doesn't
// have the offer, including when it was already used.
func (s *Store) SkipFirstAdminTwoFactor(ctx context.Context, userID int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM app_settings WHERE key = ? AND value = ?`,
		keyFirstAdminTwoFactorSkip, strconv.FormatInt(userID, 10))
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, 'false', ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		keyRequireTwoFactor, time.Now().Unix()); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// EndFirstAdminTwoFactorSkip withdraws the offer from this user (they set
// two-factor up instead), so it can't come back after a later 2FA reset.
func (s *Store) EndFirstAdminTwoFactorSkip(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM app_settings WHERE key = ? AND value = ?`,
		keyFirstAdminTwoFactorSkip, strconv.FormatInt(userID, 10))
	return err
}
