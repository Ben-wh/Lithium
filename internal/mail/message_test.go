package mail

import (
	"mime"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestMessageHeaders(t *testing.T) {
	m := Message{
		FromName: "Acme\r\nBcc: victim@example.com",
		From:     "help@acme.example",
		To:       "alice@example.com",
		Subject:  "[TID-1] Café printer\r\nBcc: victim@example.com",
		Body:     "Line one\nLine two é\n",
	}
	raw, err := m.Bytes(time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if got := msg.Header.Get("Bcc"); got != "" {
		t.Fatalf("header injection: Bcc %q", got)
	}
	subject, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if subject != "[TID-1] Café printer Bcc: victim@example.com" {
		t.Fatalf("subject: %q", subject)
	}
	from, err := mail.ParseAddress(msg.Header.Get("From"))
	if err != nil || from.Address != "help@acme.example" || from.Name != "Acme Bcc: victim@example.com" {
		t.Fatalf("from: %+v %v", from, err)
	}
	if !strings.HasSuffix(msg.Header.Get("Message-ID"), "@acme.example>") || msg.Header.Get("Auto-Submitted") != "auto-generated" {
		t.Fatalf("headers: %v", msg.Header)
	}
	if !strings.Contains(string(raw), "Line one\r\nLine two =C3=A9") {
		t.Fatalf("body not CRLF quoted-printable:\n%s", raw)
	}
}

func TestMessageRejectsBadRecipient(t *testing.T) {
	if _, err := (Message{From: "a@b.c", To: "not an address"}).Bytes(time.Now()); err == nil {
		t.Fatal("expected an error")
	}
}
