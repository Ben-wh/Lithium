package server

import (
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

type peopleResp struct {
	People []store.Person `json:"people"`
	Total  int            `json:"total"`
}

func TestUserBook(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, aliceUser := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, bobUser := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")
	newUser(t, ts.URL, admin, "Quiet Agent", "quiet@example.com", "agent")

	var hw store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware"}`, &hw)
	raise := func(c *http.Client, body string) int64 {
		t.Helper()
		var tk store.Ticket
		postJSON(t, c, ts.URL+"/api/tickets", body, &tk)
		return tk.ID
	}
	raise(alice, `{"subject":"a1","description":"d"}`)
	a2 := raise(alice, `{"subject":"a2","description":"d","categoryId":`+itoa(hw.ID)+`}`)
	do(t, admin, "PATCH", ts.URL+"/api/tickets/"+itoa(a2), `{"status":"resolved"}`)
	raise(agent, `{"subject":"agent's own","description":"d"}`)

	// Staff only.
	var resp peopleResp
	if code := getJSON(t, alice, ts.URL+"/api/people", &resp); code != http.StatusForbidden {
		t.Fatalf("user listing people: %d", code)
	}

	// Users always appear; staff only if they've raised tickets. Most recent first.
	getJSON(t, agent, ts.URL+"/api/people", &resp)
	byID := map[int64]store.Person{}
	for _, p := range resp.People {
		byID[p.ID] = p
	}
	if _, ok := byID[bobUser.ID]; !ok {
		t.Fatal("user with no tickets missing")
	}
	if _, ok := byID[agentUser.ID]; !ok {
		t.Fatal("staff member who raised a ticket missing")
	}
	for _, p := range resp.People {
		if p.Name == "Quiet Agent" || p.Name == "Admin" {
			t.Fatalf("staff without tickets listed: %s", p.Name)
		}
	}
	if a := byID[aliceUser.ID]; a.TotalTickets != 2 || a.OpenTickets != 1 || a.LastTicketAt == nil {
		t.Fatalf("alice counts: %+v", a)
	}
	if byID[bobUser.ID].LastTicketAt != nil {
		t.Fatal("bob has no tickets but has a last ticket time")
	}
	if resp.People[0].ID != agentUser.ID {
		t.Fatalf("not most-recent first: %s", resp.People[0].Name)
	}

	// Search and profile.
	getJSON(t, agent, ts.URL+"/api/people?q=ali", &resp)
	if resp.Total != 1 || resp.People[0].ID != aliceUser.ID {
		t.Fatalf("search: %+v", resp)
	}
	var p store.Person
	if code := getJSON(t, agent, ts.URL+"/api/people/"+itoa(aliceUser.ID), &p); code != 200 || p.TotalTickets != 2 {
		t.Fatalf("profile: %d %+v", code, p)
	}
	if code := getJSON(t, agent, ts.URL+"/api/people/9999", &p); code != http.StatusNotFound {
		t.Fatalf("missing person: %d", code)
	}

	// Their ticket history via the requester filter; users can't use it to peek.
	var list listResp
	getJSON(t, agent, ts.URL+"/api/tickets?status=&requester="+itoa(aliceUser.ID), &list)
	if list.Total != 2 {
		t.Fatalf("history: %d", list.Total)
	}
	getJSON(t, bob, ts.URL+"/api/tickets?status=&requester="+itoa(aliceUser.ID), &list)
	if list.Total != 0 {
		t.Fatalf("user saw someone else's tickets via requester filter: %d", list.Total)
	}

	// With agent groups on, counts only include tickets the agent can see.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":true}`)
	getJSON(t, agent, ts.URL+"/api/people/"+itoa(aliceUser.ID), &p)
	if p.TotalTickets != 1 {
		t.Fatalf("scoped count: %+v", p)
	}
	getJSON(t, admin, ts.URL+"/api/people/"+itoa(aliceUser.ID), &p)
	if p.TotalTickets != 2 {
		t.Fatalf("admin count: %+v", p)
	}
}
