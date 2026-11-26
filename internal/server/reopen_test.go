package server

import (
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func TestReopenTicket(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, _ := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var tk store.Ticket
	if code := postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Printer","description":"Jammed"}`, &tk); code != http.StatusCreated {
		t.Fatalf("create: %d", code)
	}
	reopen := func(c *http.Client) (int, store.Ticket) {
		var got store.Ticket
		code := postJSON(t, c, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/reopen", `{}`, &got)
		return code, got
	}

	// Can't reopen something that's still open.
	if code, _ := reopen(alice); code != http.StatusConflict {
		t.Fatalf("reopen open ticket: %d", code)
	}

	// Resolved and unassigned: the requester reopens it to Unassigned.
	do(t, agent, "PATCH", ts.URL+"/api/tickets/"+itoa(tk.ID), `{"status":"resolved"}`)
	if code, _ := reopen(bob); code != http.StatusNotFound {
		t.Fatalf("someone else's ticket: %d", code)
	}
	code, got := reopen(alice)
	if code != 200 || got.Status != "unassigned" {
		t.Fatalf("requester reopen: %d %q", code, got.Status)
	}

	// Closed with an assignee: staff reopen it back to In progress.
	do(t, agent, "PATCH", ts.URL+"/api/tickets/"+itoa(tk.ID), `{"assigneeId":`+itoa(agentUser.ID)+`,"status":"closed"}`)
	code, got = reopen(admin)
	if code != 200 || got.Status != "in_progress" {
		t.Fatalf("admin reopen: %d %q", code, got.Status)
	}

	// It's in the conversation as a status change.
	var comments []store.Comment
	getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/comments", &comments)
	reopened := 0
	for _, c := range comments {
		if c.Event == "status" && (c.From == "resolved" || c.From == "closed") {
			reopened++
		}
	}
	if reopened != 2 {
		t.Fatalf("want 2 reopen updates in the conversation, got %d: %+v", reopened, comments)
	}
}

// Raised in error is a finished status: it leaves the queue, isn't a
// resolution in Reports, and can be reopened.
func TestRaisedInError(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")

	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Oops","description":"Wrong form"}`, &tk)
	if code := do(t, admin, "PATCH", ts.URL+"/api/tickets/"+itoa(tk.ID), `{"status":"raised_in_error"}`); code != 200 {
		t.Fatalf("set raised_in_error: %d", code)
	}

	var list struct{ Total int }
	getJSON(t, admin, ts.URL+"/api/tickets?status=raised_in_error", &list)
	if list.Total != 1 {
		t.Fatalf("filter by raised_in_error: %d", list.Total)
	}
	var rep store.Report
	getJSON(t, admin, ts.URL+"/api/reports?days=7", &rep)
	if rep.Totals.Open != 0 || rep.Totals.Resolved != 0 {
		t.Fatalf("report counts it as open or resolved: %+v", rep.Totals)
	}

	var got store.Ticket
	if code := postJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/reopen", `{}`, &got); code != 200 || got.Status != "unassigned" {
		t.Fatalf("reopen: %d %q", code, got.Status)
	}
}

// Admins can stop requesters reopening their tickets; staff still can.
func TestRequesterReopenSetting(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var st store.Settings
	if getJSON(t, alice, ts.URL+"/api/settings", &st); !st.AllowRequesterReopen {
		t.Fatal("reopening should be allowed by default")
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"allowRequesterReopen":false}`); code != 200 {
		t.Fatalf("turn off: %d", code)
	}

	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d"}`, &tk)
	do(t, agent, "PATCH", ts.URL+"/api/tickets/"+itoa(tk.ID), `{"status":"closed"}`)
	url := ts.URL + "/api/tickets/" + itoa(tk.ID) + "/reopen"
	if code := postJSON(t, alice, url, `{}`, nil); code != http.StatusForbidden {
		t.Fatalf("requester reopen while off: %d", code)
	}
	if code := postJSON(t, agent, url, `{}`, nil); code != 200 {
		t.Fatalf("agent reopen while off: %d", code)
	}

	do(t, agent, "PATCH", ts.URL+"/api/tickets/"+itoa(tk.ID), `{"status":"closed"}`)
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"allowRequesterReopen":true}`)
	if code := postJSON(t, alice, url, `{}`, nil); code != 200 {
		t.Fatalf("requester reopen while on: %d", code)
	}
}
