package server

import (
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

type listResp struct {
	Tickets []store.TicketSummary `json:"tickets"`
	Total   int                   `json:"total"`
}

func TestListTickets(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, aliceUser := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, _ := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	raise := func(c *http.Client, subject, priority string) store.Ticket {
		t.Helper()
		var tk store.Ticket
		if code := postJSON(t, c, ts.URL+"/api/tickets",
			`{"subject":"`+subject+`","description":"d","priority":"`+priority+`"}`, &tk); code != http.StatusCreated {
			t.Fatalf("raise %s: %d", subject, code)
		}
		return tk
	}
	raise(alice, "VPN broken", "high")
	raise(alice, "New laptop please", "low")
	bobs := raise(bob, "Printer jam 50%", "urgent")
	// Agent raises one for Alice.
	postJSON(t, agent, ts.URL+"/api/tickets",
		`{"subject":"Phoned in","description":"d","requesterId":`+itoa(aliceUser.ID)+`}`, nil)

	list := func(c *http.Client, query string) listResp {
		t.Helper()
		var r listResp
		if code := getJSON(t, c, ts.URL+"/api/tickets"+query, &r); code != 200 {
			t.Fatalf("list %q: %d", query, code)
		}
		return r
	}

	// Staff see everything, newest first.
	if r := list(agent, ""); r.Total != 4 || len(r.Tickets) != 4 || r.Tickets[0].Subject != "Phoned in" {
		t.Fatalf("staff list: %+v", r)
	}
	// Users only see their own (including ones raised on their behalf), whatever they ask for.
	if r := list(alice, ""); r.Total != 3 {
		t.Fatalf("alice sees %d tickets, want 3", r.Total)
	}
	if r := list(bob, "?q=VPN"); r.Total != 0 {
		t.Fatalf("bob can find alice's ticket by search")
	}

	// Filters.
	if r := list(agent, "?priority=urgent"); r.Total != 1 || r.Tickets[0].ID != bobs.ID {
		t.Fatalf("priority filter: %+v", r)
	}
	if r := list(agent, "?status=unassigned,in_progress,pending"); r.Total != 4 {
		t.Fatalf("status filter: %d", r.Total)
	}
	if r := list(agent, "?status=in_progress"); r.Total != 0 {
		t.Fatalf("in progress filter: %d", r.Total)
	}
	if r := list(agent, "?status=resolved"); r.Total != 0 {
		t.Fatalf("resolved filter: %d", r.Total)
	}
	if r := list(agent, "?assignee=unassigned"); r.Total != 4 {
		t.Fatalf("unassigned filter: %d", r.Total)
	}
	if r := list(agent, "?assignee=me"); r.Total != 0 {
		t.Fatalf("assigned-to-me filter: %d", r.Total)
	}

	// Search: subject, requester name, ticket number, and literal wildcards.
	if r := list(agent, "?q=laptop"); r.Total != 1 {
		t.Fatalf("subject search: %d", r.Total)
	}
	if r := list(agent, "?q=bob"); r.Total != 1 {
		t.Fatalf("requester search: %d", r.Total)
	}
	if r := list(agent, "?q=%23"+itoa(bobs.ID)); r.Total != 1 || r.Tickets[0].ID != bobs.ID {
		t.Fatalf("#id search: %+v", r)
	}
	if r := list(agent, "?q=50%25"); r.Total != 1 {
		t.Fatalf("literal %% search: %d", r.Total)
	}

	// Paging.
	p1, p2 := list(agent, "?limit=3"), list(agent, "?limit=3&offset=3")
	if len(p1.Tickets) != 3 || len(p2.Tickets) != 1 || p1.Total != 4 || p2.Tickets[0].ID == p1.Tickets[2].ID {
		t.Fatalf("paging: %d + %d of %d", len(p1.Tickets), len(p2.Tickets), p1.Total)
	}

	// Bad params.
	for _, q := range []string{"?status=open", "?status=nope", "?priority=nope", "?assignee=bob", "?limit=0", "?limit=101", "?offset=-1"} {
		var r listResp
		if code := getJSON(t, agent, ts.URL+"/api/tickets"+q, &r); code != http.StatusBadRequest {
			t.Fatalf("%s: got %d", q, code)
		}
	}
}

func TestTicketStats(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, _ := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")
	for _, c := range []*http.Client{alice, alice, bob} {
		postJSON(t, c, ts.URL+"/api/tickets", `{"subject":"s","description":"d"}`, nil)
	}

	var st store.TicketStats
	getJSON(t, admin, ts.URL+"/api/tickets/stats", &st)
	if st.Unassigned != 3 || st.InProgress != 0 || st.Pending != 0 || st.AssignedToMe != 0 {
		t.Fatalf("staff stats: %+v", st)
	}
	getJSON(t, alice, ts.URL+"/api/tickets/stats", &st)
	if st.Unassigned != 2 {
		t.Fatalf("alice stats: %+v", st)
	}
}
