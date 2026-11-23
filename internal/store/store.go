// Package store holds the database queries.
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-"`
	Avatar       string    `json:"-"`
	AvatarURL    string    `json:"avatarUrl"`
	CreatedAt    time.Time `json:"createdAt"`
	// MustChangePassword is set when someone else chose the password (new
	// accounts, admin resets). The user must pick their own before doing anything else.
	MustChangePassword bool `json:"mustChangePassword"`
	// TwoFactorEnabled is true once the user has confirmed an authenticator app.
	TwoFactorEnabled bool   `json:"twoFactorEnabled"`
	TOTPSecret       string `json:"-"`
	TOTPLastStep     int64  `json:"-"`
	// TwoFactor says what this session still has to do: "verify" (enter a
	// code after the password), "setup" (2FA is mandatory but not set up),
	// or "" when nothing. Set by the server, not stored.
	TwoFactor string `json:"twoFactor"`
}

const userCols = `u.id, u.email, u.name, u.role, u.password_hash, u.created_at, u.must_change_password, u.avatar, u.totp_enabled, u.totp_secret, u.totp_last_step`

type scanner interface{ Scan(dest ...any) error }

func scanUser(row scanner) (User, error) {
	var u User
	var created int64
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &created, &u.MustChangePassword, &u.Avatar, &u.TwoFactorEnabled, &u.TOTPSecret, &u.TOTPLastStep)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	u.AvatarURL = AvatarURL(u.ID, u.Avatar)
	u.CreatedAt = time.Unix(created, 0).UTC()
	return u, err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, email, name, passwordHash, role string, mustChangePassword bool) (User, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, name, password_hash, role, must_change_password, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		email, name, passwordHash, role, mustChangePassword, now, now)
	if err != nil {
		return User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users u WHERE u.id = ?`, id))
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users u WHERE u.id = ?`, id))
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users u WHERE u.email = ?`, email))
}

type NewSession struct {
	TokenHash string
	UserID    int64
	ExpiresAt time.Time
	UserAgent string
	IP        string
	// MFAPending sessions only allow entering a two-factor code.
	MFAPending bool
	Remember   bool
}

func (s *Store) CreateSession(ctx context.Context, n NewSession) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at, user_agent, ip, mfa_pending, remember) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		n.TokenHash, n.UserID, time.Now().Unix(), n.ExpiresAt.Unix(), n.UserAgent, n.IP, n.MFAPending, n.Remember)
	return err
}

// Session is a signed-in browser.
type Session struct {
	User       User
	MFAPending bool // password entered, two-factor code not yet
	Remember   bool
}

// SessionByToken returns a non-expired session and its user.
func (s *Store) SessionByToken(ctx context.Context, tokenHash string) (Session, error) {
	var sess Session
	row := s.db.QueryRowContext(ctx,
		`SELECT `+userCols+`, s.mfa_pending, s.remember FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`,
		tokenHash, time.Now().Unix())
	var created int64
	u := &sess.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &created, &u.MustChangePassword, &u.Avatar,
		&u.TwoFactorEnabled, &u.TOTPSecret, &u.TOTPLastStep, &sess.MFAPending, &sess.Remember)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	} else if err != nil {
		return Session{}, err
	}
	u.AvatarURL = AvatarURL(u.ID, u.Avatar)
	u.CreatedAt = time.Unix(created, 0).UTC()
	return sess, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
