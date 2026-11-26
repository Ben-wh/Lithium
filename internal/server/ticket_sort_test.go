package server

import (
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func TestTicketListSort(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	_, zed := newUser(t, ts.URL, admin, "Zed", "zed@example.com", "agent")

	ids := map[string]int64{}
	for _, tk := range []struct{ subject, priority string }{
		{"banana", "low"}, {"apple", "urgent"}, {"cherry", "normal"},
	} {
		var got store.Ticket
		postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"`+tk.subject+`","description":"d","priority":"`+tk.priority+`"}`, &got)
		ids[tk.subject] = got.ID
	}
	// banana: pending and assigned to Zed; apple: assigned to Admin; cherry: unassigned.
	do(t, admin, "PATCH", ts.URL+"/api/tickets/"+itoa(ids["banana"]), `{"status":"pending","assigneeId":`+itoa(zed.ID)+`}`)
	do(t, admin, "PATCH", ts.URL+"/api/tickets/"+itoa(ids["apple"]), `{"assigneeId":1}`)

	order := func(c *http.Client, query string) string {
		t.Helper()
		var list struct{ Tickets []store.TicketSummary }
		if code := getJSON(t, c, ts.URL+"/api/tickets?"+query, &list); code != 200 {
			t.Fatalf("%s: %d", query, code)
		}
		s := ""
		for _, tk := range list.Tickets {
			s += tk.Subject[:1]
		}
		return s
	}
	// Sorting by time is tested in the store, where timestamps can be set.
	for query, want := range map[string]string{
		"sort=priority&order=desc": "acb",
		"sort=priority&order=asc":  "bca",
		"sort=subject&order=asc":   "abc",
		"sort=subject&order=desc":  "cba",
		"sort=status&order=asc":    "cab", // unassigned, in progress, pending
		"sort=assignee&order=asc":  "abc", // Admin, Zed, then unassigned
		"sort=assignee&order=desc": "bac", // Zed, Admin, unassigned still last
	} {
		if got := order(admin, query); got != want {
			t.Errorf("%q: got %s, want %s", query, got, want)
		}
	}
	// Requesters can sort their own list too.
	if got := order(alice, "sort=priority&order=desc"); got != "acb" {
		t.Errorf("requester priority sort: %s", got)
	}
	for _, bad := range []string{"sort=password", "order=sideways"} {
		if code := getJSON(t, admin, ts.URL+"/api/tickets?"+bad, nil); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", bad, code)
		}
	}
}
