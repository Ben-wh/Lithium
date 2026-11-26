package server

import (
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

type bulkResult struct {
	Updated int
	Skipped []bulkSkip
}

func TestBulkUpdateTickets(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	hwAgent, hwUser := newUser(t, ts.URL, admin, "Hardware Agent", "hw@example.com", "agent")
	var hw, sw store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware"}`, &hw)
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Software"}`, &sw)
	raise := func(cat int64) int64 {
		t.Helper()
		var tk store.Ticket
		postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d","categoryId":`+itoa(cat)+`}`, &tk)
		return tk.ID
	}
	a, b, c := raise(hw.ID), raise(hw.ID), raise(sw.ID)
	ids := `[` + itoa(a) + `,` + itoa(b) + `,` + itoa(c) + `,999999]`
	bulk := func(cl *http.Client, body string) (int, bulkResult) {
		t.Helper()
		var res bulkResult
		code := postJSON(t, cl, ts.URL+"/api/tickets/bulk", body, &res)
		return code, res
	}
	status := func(id int64) store.Ticket {
		var tk store.Ticket
		getJSON(t, admin, ts.URL+"/api/tickets/"+itoa(id), &tk)
		return tk
	}

	// Staff only, and it must actually change something.
	if code, _ := bulk(alice, `{"ids":`+ids+`,"status":"closed"}`); code != http.StatusForbidden {
		t.Fatalf("user bulk: %d", code)
	}
	for body, want := range map[string]int{
		`{"ids":[],"status":"closed"}`:      http.StatusBadRequest,
		`{"ids":` + ids + `}`:               http.StatusBadRequest,
		`{"ids":` + ids + `,"status":"no"}`: http.StatusBadRequest,
	} {
		if code, _ := bulk(admin, body); code != want {
			t.Errorf("%s: %d, want %d", body, code, want)
		}
	}

	// Status and priority for all; the unknown id is skipped.
	code, res := bulk(admin, `{"ids":`+ids+`,"status":"pending","priority":"high"}`)
	if code != 200 || res.Updated != 3 || len(res.Skipped) != 1 || res.Skipped[0].ID != 999999 {
		t.Fatalf("bulk status: %d %+v", code, res)
	}
	if tk := status(c); tk.Status != "pending" || tk.Priority != "high" {
		t.Fatalf("not applied: %+v", tk)
	}
	var comments []store.Comment
	getJSON(t, admin, ts.URL+"/api/tickets/"+itoa(a)+"/comments", &comments)
	if len(comments) != 2 {
		t.Fatalf("want status and priority updates in the conversation, got %+v", comments)
	}

	// With agent groups on, assigning to an agent skips tickets outside their groups.
	var g store.Group
	postJSON(t, admin, ts.URL+"/api/admin/groups", `{"name":"HW","memberIds":[`+itoa(hwUser.ID)+`],"categoryIds":[`+itoa(hw.ID)+`]}`, &g)
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":true}`)
	code, res = bulk(admin, `{"ids":`+ids+`,"assigneeId":`+itoa(hwUser.ID)+`}`)
	if code != 200 || res.Updated != 2 || len(res.Skipped) != 2 {
		t.Fatalf("bulk assign: %d %+v", code, res)
	}
	if status(a).Assignee == nil || status(c).Assignee != nil {
		t.Fatal("assignment applied to the wrong tickets")
	}

	// An agent can't touch tickets they can't see; they look missing.
	code, res = bulk(hwAgent, `{"ids":`+ids+`,"status":"resolved"}`)
	if code != 200 || res.Updated != 2 || len(res.Skipped) != 2 || status(c).Status != "pending" {
		t.Fatalf("agent bulk: %d %+v", code, res)
	}
	for _, s := range res.Skipped {
		if s.Reason != "ticket not found" {
			t.Fatalf("hidden ticket reason leaks: %+v", s)
		}
	}
}
