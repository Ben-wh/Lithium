package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/lithium/lithium/internal/config"
)

// Sender delivers one message. Tests swap in a fake.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// SMTP sends through the server in the config, opening a connection per
// message. Volumes are small, and this keeps failures independent.
type SMTP struct {
	cfg config.Mail
}

func NewSMTP(cfg config.Mail) *SMTP { return &SMTP{cfg: cfg} }

func (s *SMTP) Send(ctx context.Context, m Message) error {
	msg, err := m.Bytes(time.Now())
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	tlsConfig := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}

	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	if s.cfg.TLS == "tls" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("connect to %s: %w", addr, err)
	}
	// One deadline for the whole conversation, so a stuck server can't hang the worker.
	deadline := time.Now().Add(60 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()

	if s.cfg.TLS == "starttls" {
		// Required, not opportunistic: never send credentials or mail in the clear by accident.
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("the mail server doesn't support STARTTLS; set LITHIUM_SMTP_TLS if that's expected")
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if s.cfg.Username != "" {
		// PlainAuth refuses to send the password over an unencrypted
		// connection (except to localhost), which is what we want.
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("sign in to the mail server: %w", err)
		}
	}
	if err := c.Mail(m.From); err != nil {
		return err
	}
	if err := c.Rcpt(m.To); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
