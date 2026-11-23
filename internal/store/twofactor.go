package store

import (
	"context"
	"time"
)

// SetTOTPSecret stores a new, not yet confirmed authenticator secret.
// It does nothing once two-factor is enabled.
func (s *Store) SetTOTPSecret(ctx context.Context, userID int64, secret string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET totp_secret = ?, updated_at = ? WHERE id = ? AND totp_enabled = 0`,
		secret, time.Now().Unix(), userID)
	return err
}

// EnableTOTP turns two-factor on after the user proved their app works with
// a code for step, and replaces their recovery codes.
func (s *Store) EnableTOTP(ctx context.Context, userID, step int64, codeHashes []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET totp_enabled = 1, totp_last_step = ?, updated_at = ? WHERE id = ? AND totp_secret != ''`,
		step, time.Now().Unix(), userID); err != nil {
		return err
	}
	if err := replaceRecoveryCodes(ctx, tx, userID, codeHashes); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceRecoveryCodes swaps a user's recovery codes for new ones.
func (s *Store) ReplaceRecoveryCodes(ctx context.Context, userID int64, codeHashes []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := replaceRecoveryCodes(ctx, tx, userID, codeHashes); err != nil {
		return err
	}
	return tx.Commit()
}

func replaceRecoveryCodes(ctx context.Context, tx execer, userID int64, codeHashes []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, h := range codeHashes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, userID, h); err != nil {
			return err
		}
	}
	return nil
}

// UseTOTPStep records that a code for step was accepted. It returns false if
// that step (or a later one) was already used, so each code works once.
func (s *Store) UseTOTPStep(ctx context.Context, userID, step int64) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET totp_last_step = ? WHERE id = ? AND totp_last_step < ?`, step, userID, step)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// UseRecoveryCode spends an unused recovery code; false if there's no such code.
func (s *Store) UseRecoveryCode(ctx context.Context, userID int64, codeHash string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE recovery_codes SET used_at = ? WHERE user_id = ? AND code_hash = ? AND used_at IS NULL`,
		time.Now().Unix(), userID, codeHash)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// RecoveryCodesLeft counts a user's unused recovery codes.
func (s *Store) RecoveryCodesLeft(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used_at IS NULL`, userID).Scan(&n)
	return n, err
}

// ResetTwoFactor turns two-factor off and forgets the user's secret and
// recovery codes, then signs them out everywhere except keepSession.
func (s *Store) ResetTwoFactor(ctx context.Context, userID int64, keepSession string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET totp_enabled = 0, totp_secret = '', totp_last_step = 0, updated_at = ? WHERE id = ?`,
		time.Now().Unix(), userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, keepSession); err != nil {
		return err
	}
	return tx.Commit()
}

// CompleteMFA marks a pending session as fully signed in and sets its expiry.
func (s *Store) CompleteMFA(ctx context.Context, tokenHash string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET mfa_pending = 0, mfa_attempts = 0, expires_at = ? WHERE token_hash = ?`,
		expires.Unix(), tokenHash)
	return err
}

// FailMFA counts a wrong code on a pending session and ends the session after
// max attempts, so the password has to be entered (and rate limited) again.
// It reports whether the session was ended.
func (s *Store) FailMFA(ctx context.Context, tokenHash string, max int) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`UPDATE sessions SET mfa_attempts = mfa_attempts + 1 WHERE token_hash = ? RETURNING mfa_attempts`,
		tokenHash).Scan(&n); err != nil {
		return false, err
	}
	if n < max {
		return false, nil
	}
	return true, s.DeleteSession(ctx, tokenHash)
}
