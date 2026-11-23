package server

import (
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func comments(t *testing.T, c *http.Client, baseURL string, ticketID int64) []store.Comment {
	t.Helper()
	var out []store.Comment
	if code := getJSON(t, c, baseURL+"/api/tickets/"+itoa(ticketID)+"/comments", &out); code != 200 {
		t.Fatalf("list comments: %d", code)
	}
	return out
}

func TestReplies(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, aliceUser := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, _ := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d"}`, &tk)
	url := ts.URL + "/api/tickets/" + itoa(tk.ID) + "/comments"

	// Back and forth between requester and staff.
	var c store.Comment
	if code := postJSON(t, agent, url, `{"body":"  Have you tried turning it off and on?  "}`, &c); code != http.StatusCreated {
		t.Fatalf("agent reply: %d", code)
	}
	if c.Kind != "comment" || c.Body != "Have you tried turning it off and on?" || c.Author == nil || c.Author.Role != "agent" {
		t.Fatalf("reply: %+v", c)
	}
	if code := postJSON(t, alice, url, `{"body":"Yes, still broken."}`, nil); code != http.StatusCreated {
		t.Fatalf("requester reply: %d", code)
	}
	got := comments(t, alice, ts.URL, tk.ID)
	if len(got) != 2 || got[1].Author.ID != aliceUser.ID {
		t.Fatalf("thread: %+v", got)
	}

	// Replying bumps the ticket's updated time (so it rises in lists).
	var after store.Ticket
	getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID), &after)
	if after.UpdatedAt.Before(tk.UpdatedAt) {
		t.Fatal("updated time went backwards")
	}

	// Other users can't read or post; empty and huge replies are rejected.
	var none []store.Comment
	if code := getJSON(t, bob, url, &none); code != http.StatusNotFound {
		t.Fatalf("other user read: %d", code)
	}
	if code := postJSON(t, bob, url, `{"body":"hi"}`, nil); code != http.StatusNotFound {
		t.Fatalf("other user post: %d", code)
	}
	if code := postJSON(t, alice, url, `{"body":"   "}`, nil); code != http.StatusBadRequest {
		t.Fatalf("empty reply: %d", code)
	}
	if code := postJSON(t, alice, ts.URL+"/api/tickets/9999/comments", `{"body":"x"}`, nil); code != http.StatusNotFound {
		t.Fatalf("missing ticket: %d", code)
	}
}

func TestStaffChangesPostLithiumUpdates(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent Smith", "agent@example.com", "agent")
	_, bobUser := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")

	var hw store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware"}`, &hw)
	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d"}`, &tk)
	url := ts.URL + "/api/tickets/" + itoa(tk.ID)

	// Only staff can change tickets.
	if code := do(t, alice, "PATCH", url, `{"status":"closed"}`); code != http.StatusForbidden {
		t.Fatalf("requester change: %d", code)
	}

	// Assigning moves an unassigned ticket to in progress, with an update for each.
	if code := do(t, agent, "PATCH", url, `{"assigneeId":`+itoa(agentUser.ID)+`}`); code != 200 {
		t.Fatalf("assign: %d", code)
	}
	ev := comments(t, alice, ts.URL, tk.ID)
	if len(ev) != 2 ||
		ev[0].Kind != "event" || ev[0].Event != "assignee" || ev[0].From != "" || ev[0].To != "Agent Smith" ||
		ev[1].Event != "status" || ev[1].From != "unassigned" || ev[1].To != "in_progress" ||
		ev[0].Author == nil || ev[0].Author.ID != agentUser.ID {
		t.Fatalf("assign events: %+v", ev)
	}

	// Category, priority and status together; unchanged values add nothing.
	if code := do(t, agent, "PATCH", url, `{"categoryId":`+itoa(hw.ID)+`,"priority":"high","status":"pending"}`); code != 200 {
		t.Fatalf("multi change: %d", code)
	}
	if code := do(t, agent, "PATCH", url, `{"priority":"high","status":"pending"}`); code != 200 {
		t.Fatalf("no-op change: %d", code)
	}
	ev = comments(t, alice, ts.URL, tk.ID)
	if len(ev) != 5 {
		t.Fatalf("expected 5 events, got %d: %+v", len(ev), ev)
	}
	byEvent := map[string]store.Comment{}
	for _, e := range ev[2:] {
		byEvent[e.Event] = e
	}
	if byEvent["category"].To != "Hardware" || byEvent["priority"].From != "normal" || byEvent["priority"].To != "high" ||
		byEvent["status"].To != "pending" {
		t.Fatalf("multi events: %+v", byEvent)
	}

	// Clearing the category records the old name; the ticket shows the change.
	do(t, agent, "PATCH", url, `{"categoryId":null}`)
	var got store.Ticket
	getJSON(t, alice, url, &got)
	if got.Category != nil || got.Status != "pending" || got.Priority != "high" || got.Assignee == nil {
		t.Fatalf("ticket after changes: %+v", got)
	}
	ev = comments(t, alice, ts.URL, tk.ID)
	if last := ev[len(ev)-1]; last.Event != "category" || last.From != "Hardware" || last.To != "" {
		t.Fatalf("clear category event: %+v", last)
	}

	// Unassigning an in-progress ticket sends it back to unassigned.
	do(t, agent, "PATCH", url, `{"status":"in_progress"}`)
	do(t, agent, "PATCH", url, `{"assigneeId":null}`)
	getJSON(t, alice, url, &got)
	if got.Assignee != nil || got.Status != "unassigned" {
		t.Fatalf("after unassign: %+v", got)
	}

	// Validation.
	for body, want := range map[string]int{
		`{"status":"open"}`:                       http.StatusBadRequest,
		`{"priority":"critical"}`:                 http.StatusBadRequest,
		`{"categoryId":9999}`:                     http.StatusBadRequest,
		`{"categoryId":"x"}`:                      http.StatusBadRequest,
		`{"assigneeId":` + itoa(bobUser.ID) + `}`: http.StatusBadRequest, // not staff
	} {
		if code := do(t, agent, "PATCH", url, body); code != want {
			t.Fatalf("%s: got %d, want %d", body, code, want)
		}
	}
	if code := do(t, agent, "PATCH", ts.URL+"/api/tickets/9999", `{"status":"closed"}`); code != http.StatusNotFound {
		t.Fatalf("missing ticket: %d", code)
	}

	// The staff directory filter only offers admins and agents.
	var staff []store.UserRef
	getJSON(t, agent, ts.URL+"/api/users?staff=1", &staff)
	for _, u := range staff {
		if u.Name == "Alice" || u.Name == "Bob" {
			t.Fatalf("non-staff in staff directory: %+v", staff)
		}
	}
	if len(staff) != 2 {
		t.Fatalf("staff directory: %+v", staff)
	}
}
