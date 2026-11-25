package server

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/lithium/lithium/internal/config"
	"github.com/lithium/lithium/internal/store"
)

func TestLockdown(t *testing.T) {
	ts, srv := newTestServerWithMailer(t, config.Config{DataDir: t.TempDir()})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	other, _ := newUser(t, ts.URL, admin, "Other Admin", "other@example.com", "admin")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	// A second session for the same admin: lockdown keeps only the one that started it.
	adminElsewhere := signIn(t, ts.URL, "admin@example.com", "correct-horse")

	// Every safeguard is checked on the server, not just in the app.
	for name, body := range map[string]string{
		"no phrase":      `{"password":"correct-horse"}`,
		"wrong phrase":   `{"confirm":"lockdown","password":"correct-horse"}`,
		"wrong password": `{"confirm":"LOCKDOWN","password":"nope"}`,
	} {
		if code := postJSON(t, admin, ts.URL+"/api/admin/lockdown", body, nil); code != http.StatusBadRequest {
			t.Fatalf("%s: got %d", name, code)
		}
	}
	if code := postJSON(t, alice, ts.URL+"/api/admin/lockdown", `{"confirm":"LOCKDOWN","password":"chosen-password"}`, nil); code != http.StatusForbidden {
		t.Fatalf("non-admin: got %d", code)
	}
	var banner struct{ Lockdown bool }
	getJSON(t, &http.Client{}, ts.URL+"/api/public/banner", &banner)
	if banner.Lockdown {
		t.Fatal("lockdown on before it was started")
	}

	var l store.Lockdown
	if code := postJSON(t, admin, ts.URL+"/api/admin/lockdown", `{"confirm":"LOCKDOWN","password":"correct-horse","reason":"suspected breach"}`, &l); code != 200 {
		t.Fatalf("start: got %d", code)
	}
	if !l.Active || l.ByName != "Admin" || l.Reason != "suspected breach" {
		t.Fatalf("state: %+v", l)
	}
	if code := postJSON(t, admin, ts.URL+"/api/admin/lockdown", `{"confirm":"LOCKDOWN","password":"correct-horse"}`, nil); code != http.StatusConflict {
		t.Fatalf("second start: got %d", code)
	}

	// Everyone else is signed out, including the admin's other session.
	for name, c := range map[string]*http.Client{"other admin": other, "user": alice, "admin elsewhere": adminElsewhere} {
		if code := get(t, c, ts.URL+"/api/auth/me").StatusCode; code != http.StatusUnauthorized {
			t.Fatalf("%s still signed in: %d", name, code)
		}
	}
	// The admin who started it carries on.
	if code := get(t, admin, ts.URL+"/api/tickets").StatusCode; code != 200 {
		t.Fatalf("admin: %d", code)
	}

	// Nobody else can sign in, with the same answer for right and wrong passwords.
	srv.limiter = newIPLimiter() // the test has used up its sign-in attempts
	for _, body := range []string{
		`{"email":"other@example.com","password":"chosen-password"}`,
		`{"email":"other@example.com","password":"wrong"}`,
		`{"email":"nobody@example.com","password":"wrong"}`,
		`{"email":"admin@example.com","password":"wrong"}`,
	} {
		if code := post(t, &http.Client{}, ts.URL+"/api/auth/login", body).StatusCode; code != http.StatusLocked {
			t.Fatalf("login %s: got %d", body, code)
		}
	}
	// ...but the admin who started it can.
	signIn(t, ts.URL, "admin@example.com", "correct-horse")

	getJSON(t, &http.Client{}, ts.URL+"/api/public/banner", &banner)
	if !banner.Lockdown {
		t.Fatal("public banner should show lockdown")
	}

	// It's all in the activity log.
	var page activityPage
	getJSON(t, admin, ts.URL+"/api/admin/activity?"+url.Values{"q": {"lockdown"}}.Encode(), &page)
	var started, blocked, failedStart bool
	for _, e := range page.Entries {
		switch {
		case e.Action == "security.lockdown_start" && e.Outcome == store.AuditSuccess:
			started = e.Detail == "reason: suspected breach · 3 other sessions signed out"
		case e.Action == "security.lockdown_start" && e.Outcome == store.AuditFailure:
			failedStart = true
		case e.Action == "auth.login" && e.Detail == "blocked: lockdown mode":
			blocked = true
		}
	}
	if !started || !blocked || !failedStart {
		t.Fatalf("activity: started=%v blocked=%v failedStart=%v %+v", started, blocked, failedStart, page.Entries)
	}

	// Ending it lets everyone back in.
	srv.limiter = newIPLimiter()
	if code := postJSON(t, admin, ts.URL+"/api/admin/lockdown/end", `{}`, nil); code != 200 {
		t.Fatalf("end: got %d", code)
	}
	signIn(t, ts.URL, "other@example.com", "chosen-password")
	if code := postJSON(t, admin, ts.URL+"/api/admin/lockdown/end", `{}`, nil); code != http.StatusConflict {
		t.Fatalf("end twice: got %d", code)
	}
}
