package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/lithium/lithium/internal/config"
	"github.com/lithium/lithium/internal/mail"
	"github.com/lithium/lithium/internal/store"
)

// Where the active email setup comes from.
const (
	emailFromEnv      = "environment" // LITHIUM_SMTP_* variables; can't be changed in the app
	emailFromSettings = "settings"    // saved by an admin in Settings → Email
)

// mailSetup is the email setup in use right now.
type mailSetup struct {
	Mail    config.Mail
	BaseURL string // for links in emails; "" when unknown
	Source  string // emailFromEnv, emailFromSettings, or "" when email is off
}

func (m mailSetup) on() bool { return m.Source != "" }

// mailSetup works out the email setup in use. Environment variables win, so
// a deployment that sets them keeps full control; otherwise whatever an admin
// saved in settings is used. Read fresh each time, so saved changes apply
// straight away without a restart.
func (s *Server) mailSetup(ctx context.Context) (mailSetup, error) {
	if s.cfg.Mail.Enabled() {
		return mailSetup{Mail: s.cfg.Mail, BaseURL: s.cfg.BaseURL, Source: emailFromEnv}, nil
	}
	c, err := s.store.EmailConfig(ctx)
	if err != nil {
		return mailSetup{BaseURL: s.cfg.BaseURL}, err
	}
	m := mailSetup{BaseURL: s.cfg.BaseURL}
	if m.BaseURL == "" {
		m.BaseURL = c.BaseURL
	}
	if c.Host != "" {
		m.Source = emailFromSettings
		m.Mail = config.Mail{Host: c.Host, Port: c.Port, Username: c.Username, Password: c.Password, From: c.From, TLS: c.TLS}
	}
	return m, nil
}

// senderFor returns what sends through m. Tests swap in a fake via s.mailer.
func (s *Server) senderFor(m config.Mail) mail.Sender {
	if s.mailer != nil {
		return s.mailer
	}
	return mail.NewSMTP(m)
}

// OutboxSetup gives the background outbox the current sender and from
// address, or ok=false while email is off.
func (s *Server) OutboxSetup(ctx context.Context) (sender mail.Sender, from string, ok bool) {
	m, err := s.mailSetup(ctx)
	if err != nil {
		s.log.Error("load email setup", "err", err)
		return nil, "", false
	}
	if !m.on() {
		return nil, "", false
	}
	return s.senderFor(m.Mail), m.Mail.From, true
}

// emailSettings is the email setup as shown to admins. The password is never
// included, only whether there is one.
type emailSettings struct {
	Source         string `json:"source"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	HasPassword    bool   `json:"hasPassword"`
	From           string `json:"from"`
	TLS            string `json:"tls"`
	BaseURL        string `json:"baseUrl"`
	BaseURLFromEnv bool   `json:"baseUrlFromEnv"`
}

func (s *Server) emailSettings(ctx context.Context) (emailSettings, error) {
	m, err := s.mailSetup(ctx)
	if err != nil {
		return emailSettings{}, err
	}
	out := emailSettings{
		Source: m.Source, BaseURL: m.BaseURL, BaseURLFromEnv: s.cfg.BaseURL != "",
		Host: m.Mail.Host, Port: m.Mail.Port, Username: m.Mail.Username,
		HasPassword: m.Mail.Password != "", From: m.Mail.From, TLS: m.Mail.TLS,
	}
	if !m.on() {
		out.Port, out.TLS = 587, "starttls" // sensible starting point for the form
	}
	return out, nil
}

func (s *Server) handleGetEmailSettings(w http.ResponseWriter, r *http.Request) {
	out, err := s.emailSettings(r.Context())
	if err != nil {
		s.log.Error("load email settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleSaveEmailSettings sets up email from Settings → Email. Not
// allowed while the environment sets it, since that would silently be ignored.
func (s *Server) handleSaveEmailSettings(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Mail.Enabled() {
		writeError(w, http.StatusConflict, "email is set by the server's LITHIUM_SMTP_* environment variables, so change it there")
		return
	}
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		// Password: absent (null) keeps the saved one; "" removes it.
		Password *string `json:"password"`
		From     string  `json:"from"`
		TLS      string  `json:"tls"`
		BaseURL  string  `json:"baseUrl"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	saved, err := s.store.EmailConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	c := store.EmailConfig{
		Host:     strings.TrimSpace(req.Host),
		Port:     req.Port,
		Username: strings.TrimSpace(req.Username),
		Password: saved.Password,
		From:     strings.TrimSpace(req.From),
		TLS:      strings.ToLower(strings.TrimSpace(req.TLS)),
		BaseURL:  strings.TrimRight(strings.TrimSpace(req.BaseURL), "/"),
	}
	if req.Password != nil {
		c.Password = *req.Password
	}
	if c.Username == "" {
		c.Password = ""
	}
	m := config.Mail{Host: c.Host, Port: c.Port, Username: c.Username, Password: c.Password, From: c.From, TLS: c.TLS}
	if err := m.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if c.Username != "" && c.Password == "" {
		writeError(w, http.StatusBadRequest, "enter the SMTP password, or leave the username blank if the server doesn't need one")
		return
	}
	if c.Username != "" && c.TLS == "none" {
		writeError(w, http.StatusBadRequest, "a username and password can't be sent without encryption; choose STARTTLS or TLS")
		return
	}
	if len(c.Host) > 255 || len(c.Username) > 255 || len(c.Password) > 255 || len(c.From) > 255 || len(c.BaseURL) > 255 {
		writeError(w, http.StatusBadRequest, "a value is too long")
		return
	}
	if c.BaseURL != "" && s.cfg.BaseURL == "" {
		if err := config.ValidateBaseURL(c.BaseURL); err != nil {
			writeError(w, http.StatusBadRequest, "the Lithium address "+err.Error())
			return
		}
	}

	if err := s.store.SaveEmailConfig(r.Context(), c); err != nil {
		s.log.Error("save email settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("email settings saved", "by", userFrom(r.Context()).ID, "smtp", c.Host, "port", c.Port, "tls", c.TLS, "from", c.From)
	s.handleGetEmailSettings(w, r)
}

// handleDeleteEmailSettings turns email off by removing the saved setup.
func (s *Server) handleDeleteEmailSettings(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Mail.Enabled() {
		writeError(w, http.StatusConflict, "email is set by the server's LITHIUM_SMTP_* environment variables, so change it there")
		return
	}
	if err := s.store.SaveEmailConfig(r.Context(), store.EmailConfig{}); err != nil {
		s.log.Error("clear email settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("email settings removed", "by", userFrom(r.Context()).ID)
	s.handleGetEmailSettings(w, r)
}
