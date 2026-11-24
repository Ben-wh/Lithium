package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lithium/lithium/internal/config"
	"github.com/lithium/lithium/internal/mail"
	"github.com/lithium/lithium/internal/store"
)

func queuedEmails(t *testing.T, st *store.Store) map[string][]store.OutboxEmail {
	t.Helper()
	due, err := st.DueEmails(context.Background(), time.Now().Add(store.EmailBatchWindow+time.Minute), 100)
	if err != nil {
		t.Fatal(err)
	}
	byTo := map[string][]store.OutboxEmail{}
	for _, e := range due {
		byTo[e.To] = append(byTo[e.To], e)
	}
	return byTo
}

func TestNotificationEmails(t *testing.T) {
	ts, st := newTestServerWithConfig(t, config.Config{
		DataDir: t.TempDir(),
		BaseURL: "https://help.example.com",
		Mail:    config.Mail{Host: "smtp.example.com", Port: 587, From: "help@example.com", TLS: "starttls"},
	})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent Smith", "agent@example.com", "agent")

	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Printer on fire","description":"d"}`, &tk)
	ticketURL := ts.URL + "/api/tickets/" + itoa(tk.ID)
	do(t, admin, "PATCH", ticketURL, `{"assigneeId":`+itoa(agentUser.ID)+`}`)

	// One email per in-app notification, to the same people.
	q := queuedEmails(t, st)
	if len(q["agent@example.com"]) != 1 || len(q["alice@example.com"]) != 1 || len(q["admin@example.com"]) != 0 {
		t.Fatalf("after assign: %+v", q)
	}
	e := q["agent@example.com"][0]
	ref := "TID-" + itoa(tk.ID)
	if e.Subject != "["+ref+"] Printer on fire" ||
		!strings.Contains(e.Body, "Admin assigned this to you.") ||
		!strings.Contains(e.Body, "https://help.example.com/tickets/"+itoa(tk.ID)) {
		t.Fatalf("assigned email: %+v", e)
	}

	// Reply text is never put in an email, and internal notes never email
	// the requester. Events while an email waits are merged into it.
	postJSON(t, agent, ticketURL+"/comments", `{"body":"secret-reply-text"}`, nil)
	postJSON(t, agent, ticketURL+"/comments", `{"body":"secret-note-text","internal":true}`, nil)
	q = queuedEmails(t, st)
	if a := q["alice@example.com"]; len(a) != 1 || a[0].Body != "Admin updated the assignee and status.\nAgent Smith replied.\n\n"+
		ref+": Printer on fire\n\nView the ticket: https://help.example.com/tickets/"+itoa(tk.ID)+
		"\n\n-- \nYou're getting this because you're involved in this ticket in Lithium.\n" {
		t.Fatalf("alice should have one merged email with the update and the reply only: %+v", a)
	}
	for _, list := range q {
		for _, e := range list {
			if strings.Contains(e.Body, "secret-") || strings.Contains(e.Subject, "secret-") {
				t.Fatalf("comment text leaked into email: %+v", e)
			}
		}
	}

	// Sensitive mode leaves the ticket subject out.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"sensitiveMode":true}`)
	postJSON(t, agent, ticketURL+"/comments", `{"body":"more"}`, nil)
	q = queuedEmails(t, st)
	last := q["alice@example.com"][len(q["alice@example.com"])-1]
	if last.Subject != "["+ref+"] New activity" || strings.Contains(last.Body, "Printer") {
		t.Fatalf("sensitive email: %+v", last)
	}
}

func TestNoEmailsWithoutSMTP(t *testing.T) {
	ts, st := newTestServerWithConfig(t, config.Config{DataDir: t.TempDir()})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"S","description":"d"}`, &tk)
	postJSON(t, admin, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/comments", `{"body":"hi"}`, nil)
	if q := queuedEmails(t, st); len(q) != 0 {
		t.Fatalf("emails queued with email off: %+v", q)
	}
	if n := notifications(t, alice, ts.URL); n.Unread == 0 {
		t.Fatal("in-app notifications should still work")
	}
}

func TestEmailPreferences(t *testing.T) {
	ts, st := newTestServerWithConfig(t, config.Config{
		DataDir: t.TempDir(),
		Mail:    config.Mail{Host: "smtp.example.com", Port: 587, From: "help@example.com", TLS: "starttls"},
	})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, _ := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")

	var settings store.Settings
	getJSON(t, alice, ts.URL+"/api/settings", &settings)
	if !settings.EmailConfigured || !settings.EmailNotifications {
		t.Fatalf("settings: %+v", settings)
	}
	if !me(t, alice, ts.URL).EmailNotifications {
		t.Fatal("emails should be on for new users")
	}

	var a, b store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"A","description":"d"}`, &a)
	postJSON(t, bob, ts.URL+"/api/tickets", `{"subject":"B","description":"d"}`, &b)
	reply := func() {
		postJSON(t, admin, ts.URL+"/api/tickets/"+itoa(a.ID)+"/comments", `{"body":"hi"}`, nil)
		postJSON(t, admin, ts.URL+"/api/tickets/"+itoa(b.ID)+"/comments", `{"body":"hi"}`, nil)
	}

	// Alice opts out: Bob still gets his, and Alice still gets in-app notifications.
	if code := do(t, alice, "PUT", ts.URL+"/api/profile/email-notifications", `{"enabled":false}`); code != 200 {
		t.Fatalf("opt out: %d", code)
	}
	if me(t, alice, ts.URL).EmailNotifications {
		t.Fatal("opt-out not saved")
	}
	if code := do(t, alice, "PUT", ts.URL+"/api/profile/email-notifications", `{}`); code != http.StatusBadRequest {
		t.Fatalf("missing enabled: %d", code)
	}
	reply()
	q := queuedEmails(t, st)
	if len(q["alice@example.com"]) != 0 || len(q["bob@example.com"]) != 1 {
		t.Fatalf("after opt-out: %+v", q)
	}
	if n := notifications(t, alice, ts.URL); n.Unread == 0 {
		t.Fatal("in-app notifications should still arrive")
	}

	// An admin turning emails off stops them for everyone; only admins can.
	if code := do(t, bob, "PATCH", ts.URL+"/api/admin/settings", `{"emailNotifications":false}`); code != http.StatusForbidden {
		t.Fatalf("non-admin: %d", code)
	}
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"emailNotifications":false}`)
	reply()
	if q := queuedEmails(t, st); len(q["bob@example.com"]) != 1 {
		t.Fatalf("emails queued while turned off: %+v", q)
	}
}

func TestEmailConfiguredFlag(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	var settings store.Settings
	getJSON(t, admin, ts.URL+"/api/settings", &settings)
	if settings.EmailConfigured {
		t.Fatal("emailConfigured should be false without SMTP")
	}
}

func TestEmailBatchesPerTicket(t *testing.T) {
	ts, st := newTestServerWithConfig(t, config.Config{
		DataDir: t.TempDir(),
		Mail:    config.Mail{Host: "smtp.example.com", Port: 587, From: "help@example.com", TLS: "starttls"},
	})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	var a, b store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"A","description":"d"}`, &a)
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"B","description":"d"}`, &b)

	// Not sent straight away: it waits so later events can join.
	postJSON(t, admin, ts.URL+"/api/tickets/"+itoa(a.ID)+"/comments", `{"body":"1"}`, nil)
	if due, _ := st.DueEmails(context.Background(), time.Now(), 10); len(due) != 0 {
		t.Fatalf("sent before the batch window: %+v", due)
	}
	// More on ticket A joins the same email; ticket B gets its own.
	postJSON(t, admin, ts.URL+"/api/tickets/"+itoa(a.ID)+"/comments", `{"body":"2"}`, nil)
	postJSON(t, admin, ts.URL+"/api/tickets/"+itoa(b.ID)+"/comments", `{"body":"3"}`, nil)
	q := queuedEmails(t, st)["alice@example.com"]
	if len(q) != 2 || strings.Count(q[0].Body, "Admin replied.") != 2 || strings.Count(q[1].Body, "Admin replied.") != 1 {
		t.Fatalf("batching: %+v", q)
	}

	// Once an email has been sent, the next event starts a new one.
	ctx := context.Background()
	for _, e := range q {
		st.MarkEmailSent(ctx, e.ID, time.Now())
	}
	postJSON(t, admin, ts.URL+"/api/tickets/"+itoa(a.ID)+"/comments", `{"body":"4"}`, nil)
	if q := queuedEmails(t, st)["alice@example.com"]; len(q) != 1 || strings.Count(q[0].Body, "Admin replied.") != 1 {
		t.Fatalf("after sending: %+v", q)
	}
}

func TestEmailEventChoices(t *testing.T) {
	ts, st := newTestServerWithConfig(t, config.Config{
		DataDir: t.TempDir(),
		Mail:    config.Mail{Host: "smtp.example.com", Port: 587, From: "help@example.com", TLS: "starttls"},
	})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")
	if got := me(t, agent, ts.URL).EmailEvents; strings.Join(got, ",") != "replies,mentions,assigned,updates" {
		t.Fatalf("default events: %v", got)
	}
	if code := do(t, agent, "PUT", ts.URL+"/api/profile/email-notifications", `{"events":["spam"]}`); code != http.StatusBadRequest {
		t.Fatalf("unknown event: %d", code)
	}
	// Only assignments and mentions; order and repeats are tidied.
	if code := do(t, agent, "PUT", ts.URL+"/api/profile/email-notifications", `{"events":["mentions","assigned","mentions"]}`); code != 200 {
		t.Fatalf("set events: %d", code)
	}
	if u := me(t, agent, ts.URL); strings.Join(u.EmailEvents, ",") != "mentions,assigned" || !u.EmailNotifications {
		t.Fatalf("events: %+v", u)
	}

	var tk store.Ticket
	postJSON(t, admin, ts.URL+"/api/tickets", `{"subject":"S","description":"d"}`, &tk)
	ticketURL := ts.URL + "/api/tickets/" + itoa(tk.ID)
	do(t, admin, "PATCH", ticketURL, `{"assigneeId":`+itoa(agentUser.ID)+`}`)
	do(t, admin, "PATCH", ticketURL, `{"priority":"high"}`)            // an update: not wanted
	postJSON(t, admin, ticketURL+"/comments", `{"body":"hello"}`, nil) // a reply: not wanted
	postJSON(t, admin, ticketURL+"/comments", `{"body":"@[Agent](`+itoa(agentUser.ID)+`) look"}`, nil)
	q := queuedEmails(t, st)["agent@example.com"]
	if len(q) != 1 || q[0].Body[:strings.Index(q[0].Body, "\n\n")] != "Admin assigned this to you.\nAdmin mentioned you." {
		t.Fatalf("agent emails: %+v", q)
	}
}

func TestSendTestEmail(t *testing.T) {
	ts, srv := newTestServerWithMailer(t, config.Config{
		DataDir: t.TempDir(),
		Mail:    config.Mail{Host: "smtp.example.com", Port: 587, From: "help@example.com", TLS: "starttls"},
	})
	f := &fakeMailer{}
	srv.mailer = f
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	if code := postJSON(t, agent, ts.URL+"/api/admin/email/test", `{}`, nil); code != http.StatusForbidden {
		t.Fatalf("non-admin: %d", code)
	}
	var res map[string]string
	if code := postJSON(t, admin, ts.URL+"/api/admin/email/test", `{}`, &res); code != 200 || res["sentTo"] != "admin@example.com" {
		t.Fatalf("send: %d %v", code, res)
	}
	if len(f.sent) != 1 || f.sent[0].To != "admin@example.com" || f.sent[0].Subject != "Test email from Lithium" {
		t.Fatalf("sent: %+v", f.sent)
	}

	// A failure is reported, with the reason.
	f.fail = errors.New("535 authentication failed")
	resp := post(t, admin, ts.URL+"/api/admin/email/test", `{}`)
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("failure: %d", resp.StatusCode)
	}

	// Without SMTP there's nothing to test.
	plain := newTestServer(t)
	admin2 := signIn(t, plain.URL, "admin@example.com", "correct-horse")
	if code := postJSON(t, admin2, plain.URL+"/api/admin/email/test", `{}`, nil); code != http.StatusConflict {
		t.Fatalf("no smtp: %d", code)
	}
}

type fakeMailer struct {
	sent []mail.Message
	fail error
}

func (f *fakeMailer) Send(_ context.Context, m mail.Message) error {
	if f.fail != nil {
		return f.fail
	}
	f.sent = append(f.sent, m)
	return nil
}
