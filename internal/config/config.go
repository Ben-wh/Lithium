// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Addr          string // listen address, e.g. ":8080"
	DataDir       string // directory for the database and attachments
	DBPath        string // SQLite database file
	SecureCookies bool   // set the Secure flag on cookies (enable behind HTTPS)
	TrustProxy    bool   // trust X-Forwarded-For for the client IP
	AdminEmail    string // bootstrap admin, created only when no users exist
	AdminPassword string
	AdminName     string
	// BaseURL is where people reach Lithium, e.g. "https://help.example.com",
	// used for links in emails. No trailing slash.
	BaseURL string
	Mail    Mail
}

// Mail is the outgoing email (SMTP) setup. Email is off unless Host is set.
type Mail struct {
	Host     string
	Port     int
	Username string // optional; no authentication when empty
	Password string
	From     string // e.g. "help@example.com"; the display name comes from branding
	// TLS is "starttls" (upgrade a plain connection, usually port 587),
	// "tls" (TLS from the start, usually port 465) or "none" (only for a
	// local test server such as Mailpit).
	TLS string
}

// Enabled reports whether outgoing email is configured.
func (m Mail) Enabled() bool { return m.Host != "" }

func Load() Config {
	dataDir := env("LITHIUM_DATA_DIR", "./data")
	return Config{
		Addr:          env("LITHIUM_ADDR", ":8080"),
		DataDir:       dataDir,
		DBPath:        env("LITHIUM_DB_PATH", filepath.Join(dataDir, "lithium.db")),
		SecureCookies: envBool("LITHIUM_SECURE_COOKIES", false),
		TrustProxy:    envBool("LITHIUM_TRUST_PROXY", false),
		AdminEmail:    os.Getenv("LITHIUM_ADMIN_EMAIL"),
		AdminPassword: os.Getenv("LITHIUM_ADMIN_PASSWORD"),
		AdminName:     env("LITHIUM_ADMIN_NAME", "Administrator"),
		BaseURL:       strings.TrimRight(os.Getenv("LITHIUM_BASE_URL"), "/"),
		Mail:          loadMail(),
	}
}

func loadMail() Mail {
	m := Mail{
		Host:     os.Getenv("LITHIUM_SMTP_HOST"),
		Username: os.Getenv("LITHIUM_SMTP_USERNAME"),
		Password: os.Getenv("LITHIUM_SMTP_PASSWORD"),
		From:     os.Getenv("LITHIUM_SMTP_FROM"),
		TLS:      strings.ToLower(os.Getenv("LITHIUM_SMTP_TLS")),
	}
	m.Port, _ = strconv.Atoi(os.Getenv("LITHIUM_SMTP_PORT"))
	if m.Port == 0 {
		m.Port = 587
		if m.TLS == "tls" {
			m.Port = 465
		}
	}
	if m.TLS == "" {
		m.TLS = "starttls"
		if m.Port == 465 {
			m.TLS = "tls"
		}
	}
	return m
}

// Validate reports configuration mistakes that should stop the server
// starting, rather than surfacing later as emails that never arrive.
func (c Config) Validate() error {
	if !c.Mail.Enabled() {
		return nil
	}
	if err := c.Mail.Validate(); err != nil {
		return fmt.Errorf("LITHIUM_SMTP_* settings: %w", err)
	}
	if c.BaseURL != "" {
		if err := ValidateBaseURL(c.BaseURL); err != nil {
			return fmt.Errorf("LITHIUM_BASE_URL: %w", err)
		}
	}
	return nil
}

// Validate checks an SMTP setup, whether it came from environment variables
// or from an admin in settings. Messages name the setting, not the variable.
func (m Mail) Validate() error {
	if strings.TrimSpace(m.Host) == "" || strings.ContainsAny(m.Host, " /:@") {
		return errors.New("the SMTP server must be a host name like smtp.example.com")
	}
	if m.Port < 1 || m.Port > 65535 {
		return errors.New("the SMTP port must be between 1 and 65535")
	}
	if m.From == "" {
		return errors.New("the from address is required")
	}
	if a, err := mail.ParseAddress(m.From); err != nil || a.Name != "" {
		return fmt.Errorf("the from address must be a plain email address like help@example.com, not %q", m.From)
	}
	switch m.TLS {
	case "starttls", "tls", "none":
	default:
		return fmt.Errorf("encryption must be starttls, tls or none, not %q", m.TLS)
	}
	return nil
}

// ValidateBaseURL checks the address emails link to.
func ValidateBaseURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("must be a full URL like https://help.example.com, not %q", s)
	}
	return nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return def
	}
	return v
}
