package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

type activityPage struct {
	Entries []store.AuditEntry
	HasMore bool
}

func activity(t *testing.T, c *http.Client, baseURL string, params url.Values) activityPage {
	t.Helper()
	var p activityPage
	if code := getJSON(t, c, baseURL+"/api/admin/activity?"+params.Encode(), &p); code != 200 {
		t.Fatalf("activity: %d", code)
	}
	return p
}

func findEntry(entries []store.AuditEntry, action string) *store.AuditEntry {
	for i := range entries {
		if entries[i].Action == action {
			return &entries[i]
		}
	}
	return nil
}

func TestActivityLog(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent Smith", "agent@example.com", "agent")

	// A failed sign-in, for an account that exists and one that doesn't.
	post(t, &http.Client{}, ts.URL+"/api/auth/login", `{"email":"agent@example.com","password":"wrong-password"}`)
	post(t, &http.Client{}, ts.URL+"/api/auth/login", `{"email":"nobody@example.com","password":"wrong-password"}`)

	// A ticket change and a settings change.
	var tk struct{ ID int64 }
	if code := postJSON(t, agent, ts.URL+"/api/tickets", `{"subject":"Printer on fire","description":"Help","priority":"high"}`, &tk); code != 201 && code != 200 {
		t.Fatalf("create ticket: %d", code)
	}
	if code := do(t, agent, "PATCH", ts.URL+"/api/tickets/"+itoa(tk.ID), `{"status":"closed","assigneeId":`+itoa(agentUser.ID)+`}`); code != 200 {
		t.Fatalf("update ticket: %d", code)
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"companyName":"Acme","requireTwoFactor":false}`); code != 200 {
		t.Fatalf("settings: %d", code)
	}
	// An agent trying an admin action is refused, and that's logged.
	if code := do(t, agent, "PATCH", ts.URL+"/api/admin/settings", `{"companyName":"Hacked"}`); code != http.StatusForbidden {
		t.Fatalf("agent settings: %d", code)
	}

	// Only admins can read it.
	if code := getJSON(t, agent, ts.URL+"/api/admin/activity", nil); code != http.StatusForbidden {
		t.Fatalf("agent read: %d", code)
	}

	all := activity(t, admin, ts.URL, nil).Entries
	if len(all) == 0 || all[0].ID < all[len(all)-1].ID {
		t.Fatal("want newest first")
	}

	if e := findEntry(all, "user.create"); e == nil || !strings.Contains(e.Target, "Agent Smith") || e.ActorEmail != "admin@example.com" {
		t.Fatalf("user.create: %+v", e)
	}

	var failedKnown, failedUnknown bool
	for _, e := range all {
		if e.Action == "auth.login" && e.Outcome == store.AuditFailure {
			if e.Summary != "Tried to sign in" {
				t.Fatalf("failed sign-in summary: %q", e.Summary)
			}
			if e.ActorEmail == "agent@example.com" {
				failedKnown = true
			}
			if e.ActorEmail == "nobody@example.com" && e.ActorID == nil {
				failedUnknown = true
			}
		}
		if strings.Contains(e.Detail, "wrong-password") || strings.Contains(e.Detail, "chosen-password") {
			t.Fatalf("a password reached the log: %+v", e)
		}
	}
	if !failedKnown || !failedUnknown {
		t.Fatalf("failed sign-ins not logged: %+v", all)
	}

	if e := findEntry(all, "ticket.create"); e == nil || e.Target != "TID-"+itoa(tk.ID) || strings.Contains(e.Detail, "Printer") {
		t.Fatalf("ticket.create: %+v", e)
	}
	if e := findEntry(all, "ticket.update"); e == nil || e.Detail != "assignee: Agent Smith (agent@example.com) · status: closed" {
		t.Fatalf("ticket.update: %+v", e)
	}

	settings := activity(t, admin, ts.URL, url.Values{"category": {"settings"}}).Entries
	var ok, refused bool
	for _, e := range settings {
		if e.Category != "settings" {
			t.Fatalf("category filter leaked %+v", e)
		}
		if e.Outcome == store.AuditSuccess && e.Detail == "company name: Acme · require two factor: off" {
			ok = true
		}
		if e.Outcome == store.AuditFailure && e.ActorEmail == "agent@example.com" {
			refused = true
		}
	}
	if !ok || !refused {
		t.Fatalf("settings entries: %+v", settings)
	}

	// Search: every word must match somewhere.
	got := activity(t, admin, ts.URL, url.Values{"q": {"agent closed"}}).Entries
	if len(got) != 1 || got[0].Action != "ticket.update" {
		t.Fatalf("search: %+v", got)
	}
	if got := activity(t, admin, ts.URL, url.Values{"q": {"100%_"}}).Entries; len(got) != 0 {
		t.Fatalf("wildcards should be literal: %+v", got)
	}
	if got := activity(t, admin, ts.URL, url.Values{"outcome": {"failure"}}).Entries; len(got) < 3 {
		t.Fatalf("failures: %+v", got)
	}

	// Paging.
	page := activity(t, admin, ts.URL, url.Values{"before": {itoa(all[2].ID)}}).Entries
	if len(page) == 0 || page[0].ID >= all[2].ID {
		t.Fatalf("paging: %+v", page)
	}
}
