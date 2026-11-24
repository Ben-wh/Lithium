// Package mail sends Lithium's outgoing email: it builds messages, talks to
// the SMTP server, and works through the email outbox in the background.
package mail

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

// Message is one plain-text email to one recipient.
type Message struct {
	FromName string // display name, e.g. the company name
	From     string // bare address
	To       string // bare address
	Subject  string
	Body     string
}

// singleLine removes line breaks, so user-controlled text (ticket subjects,
// company names) can't add headers.
func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Bytes renders the message as RFC 5322 text with CRLF line endings.
func (m Message) Bytes(now time.Time) ([]byte, error) {
	from := mail.Address{Name: singleLine(m.FromName), Address: m.From}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return nil, fmt.Errorf("recipient %q: %w", m.To, err)
	}
	domain := "lithium.local"
	if at := strings.LastIndexByte(m.From, '@'); at >= 0 {
		domain = m.From[at+1:]
	}
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}

	var b bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", from.String())
	header("To", to.String())
	header("Subject", mime.QEncoding.Encode("utf-8", singleLine(m.Subject)))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", fmt.Sprintf("<%s@%s>", hex.EncodeToString(id), domain))
	header("MIME-Version", "1.0")
	header("Content-Type", `text/plain; charset="utf-8"`)
	header("Content-Transfer-Encoding", "quoted-printable")
	// Ask mail servers and clients not to send out-of-office replies back.
	header("Auto-Submitted", "auto-generated")
	header("X-Auto-Response-Suppress", "All")
	b.WriteString("\r\n")

	qp := quotedprintable.NewWriter(&b)
	body := strings.ReplaceAll(strings.ReplaceAll(m.Body, "\r\n", "\n"), "\n", "\r\n")
	if _, err := qp.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
