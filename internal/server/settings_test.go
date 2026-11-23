package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func TestMandatoryCategory(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")
	raise := func(body string) int {
		return postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d"`+body+`}`, nil)
	}

	// Off by default, readable by anyone signed in.
	var st store.Settings
	if code := getJSON(t, alice, ts.URL+"/api/settings", &st); code != 200 || st.RequireCategory {
		t.Fatalf("default settings: %d %+v", code, st)
	}
	if code := raise(""); code != http.StatusCreated {
		t.Fatalf("no category while optional: %d", code)
	}

	// Only admins can change settings.
	if code := do(t, agent, "PATCH", ts.URL+"/api/admin/settings", `{"requireCategory":true}`); code != http.StatusForbidden {
		t.Fatalf("agent update: %d", code)
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"requireCategory":true}`); code != 200 {
		t.Fatalf("admin update: %d", code)
	}
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if !st.RequireCategory {
		t.Fatal("setting not saved")
	}

	// With no active categories it isn't enforced (nobody would be able to raise a ticket).
	if code := raise(""); code != http.StatusCreated {
		t.Fatalf("required but no categories exist: %d", code)
	}

	var c store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware"}`, &c)
	if code := raise(""); code != http.StatusBadRequest {
		t.Fatalf("missing required category: %d", code)
	}
	if code := raise(`,"categoryId":` + itoa(c.ID)); code != http.StatusCreated {
		t.Fatalf("with category: %d", code)
	}

	// Archiving the only category lifts the requirement again.
	do(t, admin, "PATCH", ts.URL+"/api/admin/categories/"+itoa(c.ID), `{"archived":true}`)
	if code := raise(""); code != http.StatusCreated {
		t.Fatalf("all categories archived: %d", code)
	}

	// A partial update leaves other settings alone; turning it off works.
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{}`); code != 200 {
		t.Fatalf("empty update: %d", code)
	}
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if !st.RequireCategory {
		t.Fatal("empty update changed the setting")
	}
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"requireCategory":false}`)
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if st.RequireCategory {
		t.Fatal("setting not turned off")
	}
}

func TestTicketPrefix(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var st store.Settings
	getJSON(t, agent, ts.URL+"/api/settings", &st)
	if st.TicketPrefix != "TID" {
		t.Fatalf("default prefix: %q", st.TicketPrefix)
	}

	for body, want := range map[string]int{
		`{"ticketPrefix":""}`:        http.StatusBadRequest,
		`{"ticketPrefix":"TOOLONG"}`: http.StatusBadRequest,
		`{"ticketPrefix":"AB-C"}`:    http.StatusBadRequest,
		`{"ticketPrefix":"HÉLP"}`:    http.StatusBadRequest,
		`{"ticketPrefix":" help "}`:  http.StatusOK,
	} {
		if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", body); code != want {
			t.Fatalf("%s: got %d, want %d", body, code, want)
		}
	}
	getJSON(t, agent, ts.URL+"/api/settings", &st)
	if st.TicketPrefix != "HELP" {
		t.Fatalf("prefix should be trimmed and uppercased, got %q", st.TicketPrefix)
	}
	if code := do(t, agent, "PATCH", ts.URL+"/api/admin/settings", `{"ticketPrefix":"X"}`); code != http.StatusForbidden {
		t.Fatalf("agent change: %d", code)
	}

	// Any of these finds the ticket, including the old prefix.
	var tk store.Ticket
	postJSON(t, agent, ts.URL+"/api/tickets", `{"subject":"Printer","description":"d"}`, &tk)
	for _, q := range []string{"HELP-" + itoa(tk.ID), "help-" + itoa(tk.ID), "TID-" + itoa(tk.ID), "%23" + itoa(tk.ID), itoa(tk.ID)} {
		var r listResp
		getJSON(t, agent, ts.URL+"/api/tickets?q="+q, &r)
		if r.Total != 1 || r.Tickets[0].ID != tk.ID {
			t.Fatalf("search %q: %+v", q, r)
		}
	}
}

func TestBackgroundSetting(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var st store.Settings
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if st.Background != "none" {
		t.Fatalf("default background: %q", st.Background)
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"background":"sparkles"}`); code != http.StatusBadRequest {
		t.Fatalf("unknown background: %d", code)
	}
	if code := do(t, agent, "PATCH", ts.URL+"/api/admin/settings", `{"background":"dots"}`); code != http.StatusForbidden {
		t.Fatalf("agent change: %d", code)
	}
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"background":"waves"}`)
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if st.Background != "waves" {
		t.Fatalf("everyone should see the new background, got %q", st.Background)
	}
}

func TestSensitiveMode(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var st store.Settings
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if st.SensitiveMode || st.SensitiveMessage != store.DefaultSensitiveMessage {
		t.Fatalf("defaults: %v %q", st.SensitiveMode, st.SensitiveMessage)
	}
	if code := do(t, agent, "PATCH", ts.URL+"/api/admin/settings", `{"sensitiveMode":true}`); code != http.StatusForbidden {
		t.Fatalf("agent enabling: %d", code)
	}
	long := strings.Repeat("x", 121)
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"sensitiveMessage":"`+long+`"}`); code != http.StatusBadRequest {
		t.Fatalf("long message: %d", code)
	}
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"sensitiveMode":true,"sensitiveMessage":"  OFFICIAL-SENSITIVE:\n  do not share  "}`)
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if !st.SensitiveMode || st.SensitiveMessage != "OFFICIAL-SENSITIVE: do not share" {
		t.Fatalf("everyone should see the banner: %v %q", st.SensitiveMode, st.SensitiveMessage)
	}
	// A blank message falls back to the default.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"sensitiveMessage":"   "}`)
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if st.SensitiveMessage != store.DefaultSensitiveMessage {
		t.Fatalf("blank message: %q", st.SensitiveMessage)
	}
}

func TestPublicBanner(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	var b map[string]any

	// Signed out: off by default, and no message leaks while it's off.
	if code := getJSON(t, http.DefaultClient, ts.URL+"/api/public/banner", &b); code != 200 || b["sensitiveMode"] != false || b["sensitiveMessage"] != "" {
		t.Fatalf("default banner: %d %v", code, b)
	}
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"sensitiveMessage":"Secret project"}`)
	getJSON(t, http.DefaultClient, ts.URL+"/api/public/banner", &b)
	if b["sensitiveMessage"] != "" {
		t.Fatalf("message shown while sensitive mode is off: %v", b)
	}
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"sensitiveMode":true}`)
	getJSON(t, http.DefaultClient, ts.URL+"/api/public/banner", &b)
	if b["sensitiveMode"] != true || b["sensitiveMessage"] != "Secret project" || len(b) != 2 {
		t.Fatalf("banner: %v", b)
	}
}
