package store

import (
	"context"
	"strconv"
	"time"
)

// EmailConfig is the outgoing email (SMTP) setup an admin saved in
// settings. It's only used when the server's environment doesn't set SMTP.
// Email is off while Host is empty.
type EmailConfig struct {
	Host     string
	Port     int
	Username string
	// Password is stored as entered (SMTP needs it in full), so it's never
	// sent back to the browser.
	Password string
	From     string
	TLS      string // "starttls", "tls" or "none"
	// BaseURL is where people reach Lithium, for links in emails.
	BaseURL string
}

const (
	keyEmailHost     = "email.smtp_host"
	keyEmailPort     = "email.smtp_port"
	keyEmailUsername = "email.smtp_username"
	keyEmailPassword = "email.smtp_password"
	keyEmailFrom     = "email.smtp_from"
	keyEmailTLS      = "email.smtp_tls"
	keyEmailBaseURL  = "email.base_url"
)

var emailKeys = []string{keyEmailHost, keyEmailPort, keyEmailUsername, keyEmailPassword, keyEmailFrom, keyEmailTLS, keyEmailBaseURL}

// EmailConfig returns the saved email setup (zero value when none).
func (s *Store) EmailConfig(ctx context.Context) (EmailConfig, error) {
	var c EmailConfig
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM app_settings WHERE key LIKE 'email.%'`)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return c, err
		}
		switch key {
		case keyEmailHost:
			c.Host = value
		case keyEmailPort:
			c.Port, _ = strconv.Atoi(value)
		case keyEmailUsername:
			c.Username = value
		case keyEmailPassword:
			c.Password = value
		case keyEmailFrom:
			c.From = value
		case keyEmailTLS:
			c.TLS = value
		case keyEmailBaseURL:
			c.BaseURL = value
		}
	}
	return c, rows.Err()
}

// SaveEmailConfig replaces the saved email setup. A zero EmailConfig turns
// email off.
func (s *Store) SaveEmailConfig(ctx context.Context, c EmailConfig) error {
	values := map[string]string{
		keyEmailHost:     c.Host,
		keyEmailPort:     strconv.Itoa(c.Port),
		keyEmailUsername: c.Username,
		keyEmailPassword: c.Password,
		keyEmailFrom:     c.From,
		keyEmailTLS:      c.TLS,
		keyEmailBaseURL:  c.BaseURL,
	}
	if c.Port == 0 {
		values[keyEmailPort] = ""
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	for _, k := range emailKeys {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			k, values[k], now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
