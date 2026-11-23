package server

import (
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

type treeResp struct {
	RootID int64              `json:"rootId"`
	Nodes  []store.TicketLink `json:"nodes"`
}

func TestTicketLinksAndIncidentTree(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	raise := func(subject, priority string) int64 {
		t.Helper()
		var tk store.Ticket
		postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"`+subject+`","description":"d","priority":"`+priority+`"}`, &tk)
		return tk.ID
	}
	outage := raise("Website down", "urgent")
	checkout := raise("Can't check out", "high")
	login := raise("Login 500s", "normal")
	payments := raise("Payments failing", "high")
	link := func(c *http.Client, child int64, parent string) int {
		t.Helper()
		return do(t, c, "PUT", ts.URL+"/api/tickets/"+itoa(child)+"/parent", `{"parentId":`+parent+`}`)
	}

	// Only staff can link.
	if code := link(alice, checkout, itoa(outage)); code != http.StatusForbidden {
		t.Fatalf("requester linking: %d", code)
	}
	// Build: outage → checkout → payments; outage → login.
	for _, l := range [][2]int64{{checkout, outage}, {login, outage}, {payments, checkout}} {
		if code := link(agent, l[0], itoa(l[1])); code != 200 {
			t.Fatalf("link %d under %d: %d", l[0], l[1], code)
		}
	}

	// Loops and self-links are refused; unknown parents are 404.
	for _, c := range []struct {
		child  int64
		parent string
		want   int
	}{
		{outage, itoa(payments), http.StatusBadRequest}, // payments is under outage
		{outage, itoa(outage), http.StatusBadRequest},
		{login, "9999", http.StatusNotFound},
		{login, `"x"`, http.StatusBadRequest},
	} {
		if code := link(agent, c.child, c.parent); code != c.want {
			t.Fatalf("link %d → %s: got %d, want %d", c.child, c.parent, code, c.want)
		}
	}

	// Links as seen from the middle of the tree.
	var links store.TicketLinks
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(checkout)+"/links", &links)
	if links.Parent == nil || links.Parent.ID != outage || len(links.Children) != 1 || links.Children[0].ID != payments ||
		links.IncidentRootID != outage {
		t.Fatalf("links: %+v", links)
	}

	// The tree, from any ticket in it.
	var tree treeResp
	if code := getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(payments)+"/tree", &tree); code != 200 {
		t.Fatalf("tree: %d", code)
	}
	if tree.RootID != outage || len(tree.Nodes) != 4 || tree.Nodes[0].ID != outage {
		t.Fatalf("tree: %+v", tree)
	}
	parents := map[int64]int64{}
	for _, n := range tree.Nodes {
		parents[n.ID] = n.ParentID
	}
	if parents[payments] != checkout || parents[login] != outage || parents[outage] != 0 {
		t.Fatalf("tree parents: %+v", parents)
	}

	// Requesters can't see links or trees, nor the staff-only link updates.
	var none store.TicketLinks
	if code := getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(checkout)+"/links", &none); code != http.StatusForbidden {
		t.Fatalf("requester links: %d", code)
	}
	for _, c := range comments(t, alice, ts.URL, checkout) {
		if c.Event == "parent" || c.Event == "child" {
			t.Fatal("requester can see link updates")
		}
	}
	var sawLinkEvent bool
	for _, c := range comments(t, agent, ts.URL, checkout) {
		sawLinkEvent = sawLinkEvent || (c.Event == "parent" && c.To == itoa(outage))
	}
	if !sawLinkEvent {
		t.Fatal("staff can't see link update")
	}

	// Only urgent roots make an incident tree.
	quiet := raise("Quiet parent", "low")
	link(agent, login, itoa(quiet))
	if code := getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(login)+"/tree", &tree); code != http.StatusNotFound {
		t.Fatalf("non-urgent root: %d", code)
	}
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(login)+"/links", &links)
	if links.IncidentRootID != 0 {
		t.Fatalf("non-urgent incident root: %+v", links)
	}

	// Unlinking.
	if code := link(agent, login, "null"); code != 200 {
		t.Fatalf("unlink: %d", code)
	}
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(login)+"/links", &links)
	if links.Parent != nil {
		t.Fatal("still linked")
	}

	// Turning the feature off hides the tree (linking still works).
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"incidentTree":false}`)
	if code := getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(outage)+"/tree", &tree); code != http.StatusNotFound {
		t.Fatalf("tree while off: %d", code)
	}
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(checkout)+"/links", &links)
	if links.IncidentRootID != 0 || links.Parent == nil {
		t.Fatalf("links while off: %+v", links)
	}
}
