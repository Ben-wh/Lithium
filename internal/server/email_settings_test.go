package server

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/config"
)

func getEmailSettings(t *testing.T, c *http.Client, baseURL string) emailSettings {
	t.Helper()
	var es emailSettings
	if code := getJSON(t, c, baseURL+"/api/admin/email", &es); code != 200 {
		t.Fatalf("get email settings: %d", code)
	}
	return es
}

func TestEmailSetUpInSettings(t *testing.T) {
	ts, srv := newTestServerWithMailer(t, config.Config{DataDir: t.TempDir()})
	f := &fakeMailer{}
	srv.mailer = f
	ctx := context.Background()
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	// Not set up yet: the page offers the form, and nothing can be sent.
	if es := getEmailSettings(t, admin, ts.URL); es.Source != "" || es.Port != 587 || es.TLS != "starttls" {
		t.Fatalf("before setup: %+v", es)
	}
	if code := getJSON(t, agent, ts.URL+"/api/admin/email", nil); code != http.StatusForbidden {
		t.Fatalf("non-admin read: %d", code)
	}
	if code := do(t, agent, "PUT", ts.URL+"/api/admin/email", `{}`); code != http.StatusForbidden {
		t.Fatalf("non-admin save: %d", code)
	}

	for name, body := range map[string]string{
		"no host":         `{"port":587,"from":"help@example.com","tls":"starttls"}`,
		"bad port":        `{"host":"smtp.example.com","port":0,"from":"help@example.com","tls":"starttls"}`,
		"bad from":        `{"host":"smtp.example.com","port":587,"from":"nope","tls":"starttls"}`,
		"bad tls":         `{"host":"smtp.example.com","port":587,"from":"help@example.com","tls":"ssl"}`,
		"login, no pass":  `{"host":"smtp.example.com","port":587,"username":"me","from":"help@example.com","tls":"starttls"}`,
		"login, no tls":   `{"host":"smtp.example.com","port":25,"username":"me","password":"pw","from":"help@example.com","tls":"none"}`,
		"bad lithium url": `{"host":"smtp.example.com","port":587,"from":"help@example.com","tls":"starttls","baseUrl":"help.example.com"}`,
	} {
		if code := do(t, admin, "PUT", ts.URL+"/api/admin/email", body); code != http.StatusBadRequest {
			t.Fatalf("%s: got %d", name, code)
		}
	}

	// Save a real setup. The password is never sent back.
	resp := putRaw(t, admin, ts.URL+"/api/admin/email",
		`{"host":"smtp.example.com","port":587,"username":"me","password":"s3cret!","from":"help@example.com","tls":"starttls","baseUrl":"https://help.example.com/"}`)
	if resp.status != 200 || strings.Contains(resp.body, "s3cret") {
		t.Fatalf("save: %d %s", resp.status, resp.body)
	}
	es := getEmailSettings(t, admin, ts.URL)
	if es.Source != emailFromSettings || !es.HasPassword || es.Host != "smtp.example.com" || es.BaseURL != "https://help.example.com" {
		t.Fatalf("after save: %+v", es)
	}
	var settings struct{ EmailConfigured bool }
	getJSON(t, admin, ts.URL+"/api/settings", &settings)
	if !settings.EmailConfigured {
		t.Fatal("settings should report email as configured")
	}

	// Saving again without a password keeps the saved one.
	if code := do(t, admin, "PUT", ts.URL+"/api/admin/email",
		`{"host":"smtp2.example.com","port":465,"username":"me","from":"help@example.com","tls":"tls"}`); code != 200 {
		t.Fatalf("resave: %d", code)
	}
	if c, _ := srv.store.EmailConfig(ctx); c.Password != "s3cret!" || c.Host != "smtp2.example.com" {
		t.Fatalf("password not kept: %+v", c)
	}

	// It's used straight away, with no restart: test email and the outbox.
	if code := postJSON(t, admin, ts.URL+"/api/admin/email/test", `{}`, nil); code != 200 {
		t.Fatalf("test email: %d", code)
	}
	if len(f.sent) != 1 || f.sent[0].From != "help@example.com" {
		t.Fatalf("sent: %+v", f.sent)
	}
	if _, from, ok := srv.OutboxSetup(ctx); !ok || from != "help@example.com" {
		t.Fatalf("outbox setup: %q %v", from, ok)
	}

	// Removing it turns email off.
	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/email", ``); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if es := getEmailSettings(t, admin, ts.URL); es.Source != "" || es.HasPassword {
		t.Fatalf("after delete: %+v", es)
	}
	if _, _, ok := srv.OutboxSetup(ctx); ok {
		t.Fatal("outbox should be off")
	}
}

func TestEmailFromEnvironmentCantBeChanged(t *testing.T) {
	ts, _ := newTestServerWithConfig(t, config.Config{
		DataDir: t.TempDir(),
		Mail:    config.Mail{Host: "smtp.env.example", Port: 587, Password: "env-pass", From: "env@example.com", TLS: "starttls"},
	})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	if es := getEmailSettings(t, admin, ts.URL); es.Source != emailFromEnv || es.Host != "smtp.env.example" || !es.HasPassword {
		t.Fatalf("env setup: %+v", es)
	}
	if code := do(t, admin, "PUT", ts.URL+"/api/admin/email",
		`{"host":"smtp.example.com","port":587,"from":"help@example.com","tls":"starttls"}`); code != http.StatusConflict {
		t.Fatalf("save over env: %d", code)
	}
	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/email", ``); code != http.StatusConflict {
		t.Fatalf("delete over env: %d", code)
	}
}

type rawResponse struct {
	status int
	body   string
}

func putRaw(t *testing.T, c *http.Client, url, body string) rawResponse {
	t.Helper()
	req, _ := http.NewRequest("PUT", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return rawResponse{resp.StatusCode, string(b)}
}
