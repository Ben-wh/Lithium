package server

import (
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func TestAgentGroups(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	hwAgent, hwUser := newUser(t, ts.URL, admin, "Hardware Agent", "hw@example.com", "agent")
	swAgent, swUser := newUser(t, ts.URL, admin, "Software Agent", "sw@example.com", "agent")
	var hw, sw store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware"}`, &hw)
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Software"}`, &sw)
	raise := func(subject string, cat int64) int64 {
		t.Helper()
		body := `{"subject":"` + subject + `","description":"d"`
		if cat != 0 {
			body += `,"categoryId":` + itoa(cat)
		}
		var tk store.Ticket
		postJSON(t, alice, ts.URL+"/api/tickets", body+`}`, &tk)
		return tk.ID
	}
	laptop := raise("Laptop broken", hw.ID)
	excel := raise("Excel crashes", sw.ID)
	misc := raise("Not sure", 0)

	// Group management: admin only, validated.
	var g store.Group
	body := `{"name":"Hardware team","memberIds":[` + itoa(hwUser.ID) + `],"categoryIds":[` + itoa(hw.ID) + `]}`
	if code := postJSON(t, hwAgent, ts.URL+"/api/admin/groups", body, nil); code != http.StatusForbidden {
		t.Fatalf("agent creating group: %d", code)
	}
	if code := postJSON(t, admin, ts.URL+"/api/admin/groups", body, &g); code != http.StatusCreated ||
		len(g.Members) != 1 || len(g.Categories) != 1 {
		t.Fatalf("create group: %d %+v", code, g)
	}
	for b, want := range map[string]int{
		`{"name":"hardware TEAM"}`:                               http.StatusConflict,
		`{"name":""}`:                                            http.StatusBadRequest,
		`{"name":"x","memberIds":[1]}`:                           http.StatusBadRequest, // admins can't be members
		`{"name":"x","categoryIds":[999]}`:                       http.StatusBadRequest,
		`{"name":"y","memberIds":[` + itoa(swUser.ID+100) + `]}`: http.StatusBadRequest,
	} {
		if code := postJSON(t, admin, ts.URL+"/api/admin/groups", b, nil); code != want {
			t.Fatalf("%s: got %d, want %d", b, code, want)
		}
	}

	seen := func(c *http.Client) map[int64]bool {
		t.Helper()
		var r listResp
		getJSON(t, c, ts.URL+"/api/tickets", &r)
		out := map[int64]bool{}
		for _, tk := range r.Tickets {
			out[tk.ID] = true
		}
		return out
	}

	// Off by default: every agent sees everything.
	var st store.Settings
	getJSON(t, admin, ts.URL+"/api/settings", &st)
	if st.AgentGroups {
		t.Fatal("agent groups should be off by default")
	}
	if s := seen(swAgent); !s[laptop] || !s[excel] || !s[misc] {
		t.Fatalf("groups off, agent sees: %v", s)
	}

	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":true}`)

	// The hardware agent sees hardware and uncategorised tickets, not software.
	s := seen(hwAgent)
	if !s[laptop] || !s[misc] || s[excel] {
		t.Fatalf("hardware agent sees: %v", s)
	}
	// The software agent is in no group: only uncategorised.
	s = seen(swAgent)
	if s[laptop] || s[excel] || !s[misc] {
		t.Fatalf("ungrouped agent sees: %v", s)
	}
	// Admins and the requester are unaffected.
	if s := seen(admin); !s[laptop] || !s[excel] {
		t.Fatalf("admin sees: %v", s)
	}
	if s := seen(alice); !s[laptop] || !s[excel] || !s[misc] {
		t.Fatalf("requester sees: %v", s)
	}

	// Direct access is blocked too: page, replies, edits.
	var tk store.Ticket
	if code := getJSON(t, swAgent, ts.URL+"/api/tickets/"+itoa(laptop), &tk); code != http.StatusNotFound {
		t.Fatalf("out-of-group ticket: %d", code)
	}
	if code := postJSON(t, swAgent, ts.URL+"/api/tickets/"+itoa(laptop)+"/comments", `{"body":"hi"}`, nil); code != http.StatusNotFound {
		t.Fatalf("reply to out-of-group ticket: %d", code)
	}
	if code := do(t, swAgent, "PATCH", ts.URL+"/api/tickets/"+itoa(laptop), `{"status":"closed"}`); code != http.StatusNotFound {
		t.Fatalf("edit out-of-group ticket: %d", code)
	}

	// Dashboard counts follow the same rules.
	var stats store.TicketStats
	getJSON(t, hwAgent, ts.URL+"/api/tickets/stats", &stats)
	if stats.Unassigned != 2 {
		t.Fatalf("hardware agent stats: %+v", stats)
	}

	// An agent assigned a ticket outside their groups (e.g. before groups were
	// turned on) can still see it.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":false}`)
	if code := do(t, admin, "PATCH", ts.URL+"/api/tickets/"+itoa(excel), `{"assigneeId":`+itoa(hwUser.ID)+`}`); code != 200 {
		t.Fatalf("assign while groups off: %d", code)
	}
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":true}`)
	if !seen(hwAgent)[excel] {
		t.Fatal("assigned ticket hidden from assignee")
	}

	// Mention suggestions only offer people who can see the ticket.
	var people []store.UserRef
	getJSON(t, admin, ts.URL+"/api/tickets/"+itoa(laptop)+"/mentionable", &people)
	for _, p := range people {
		if p.ID == swUser.ID {
			t.Fatal("out-of-group agent offered as mention")
		}
	}

	// Adding the category to a group grants access; deleting the group revokes it.
	var g2 store.Group
	postJSON(t, admin, ts.URL+"/api/admin/groups", `{"name":"Software team","memberIds":[`+itoa(swUser.ID)+`],"categoryIds":[`+itoa(sw.ID)+`]}`, &g2)
	if !seen(swAgent)[excel] {
		t.Fatal("group didn't grant access")
	}
	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/groups/"+itoa(g2.ID), ""); code != http.StatusNoContent {
		t.Fatalf("delete group: %d", code)
	}
	if seen(swAgent)[excel] {
		t.Fatal("deleted group still grants access")
	}

	// Turning the feature off restores full access.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":false}`)
	if s := seen(swAgent); !s[laptop] || !s[excel] {
		t.Fatalf("groups off again, agent sees: %v", s)
	}

	var groups []store.Group
	getJSON(t, admin, ts.URL+"/api/admin/groups", &groups)
	if len(groups) != 1 || groups[0].Name != "Hardware team" {
		t.Fatalf("list groups: %+v", groups)
	}
}

func TestIncidentTreeRedactsOutOfGroupTickets(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var hw, sw store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware"}`, &hw)
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Software"}`, &sw)
	var root, child store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Outage","description":"d","priority":"urgent","categoryId":`+itoa(hw.ID)+`}`, &root)
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Secret software thing","description":"d","categoryId":`+itoa(sw.ID)+`}`, &child)
	do(t, admin, "PUT", ts.URL+"/api/tickets/"+itoa(child.ID)+"/parent", `{"parentId":`+itoa(root.ID)+`}`)
	postJSON(t, admin, ts.URL+"/api/admin/groups", `{"name":"HW","memberIds":[`+itoa(agentUser.ID)+`],"categoryIds":[`+itoa(hw.ID)+`]}`, nil)
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":true}`)

	var tree treeResp
	if code := getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(root.ID)+"/tree", &tree); code != 200 {
		t.Fatalf("tree: %d", code)
	}
	for _, n := range tree.Nodes {
		if n.ID == child.ID && (!n.Restricted || n.Subject != "") {
			t.Fatalf("out-of-group node not redacted: %+v", n)
		}
	}
	var links store.TicketLinks
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(root.ID)+"/links", &links)
	if len(links.Children) != 0 {
		t.Fatalf("out-of-group child listed: %+v", links.Children)
	}
	if code := do(t, agent, "PUT", ts.URL+"/api/tickets/"+itoa(root.ID)+"/parent", `{"parentId":`+itoa(child.ID)+`}`); code != http.StatusNotFound {
		t.Fatalf("linking to a hidden ticket: %d", code)
	}
}

func TestAssigneesFilteredByGroup(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	_, hwUser := newUser(t, ts.URL, admin, "Hardware Agent", "hw@example.com", "agent")
	_, swUser := newUser(t, ts.URL, admin, "Software Agent", "sw@example.com", "agent")

	var hw store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware"}`, &hw)
	var laptop, misc store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Laptop","description":"d","categoryId":`+itoa(hw.ID)+`}`, &laptop)
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Misc","description":"d"}`, &misc)
	postJSON(t, admin, ts.URL+"/api/admin/groups", `{"name":"HW","memberIds":[`+itoa(hwUser.ID)+`],"categoryIds":[`+itoa(hw.ID)+`]}`, nil)

	assignable := func(ticket int64) map[int64]bool {
		t.Helper()
		var people []store.UserRef
		if code := getJSON(t, admin, ts.URL+"/api/tickets/"+itoa(ticket)+"/assignable", &people); code != 200 {
			t.Fatalf("assignable: %d", code)
		}
		out := map[int64]bool{}
		for _, p := range people {
			out[p.ID] = true
		}
		return out
	}

	// Groups off: every staff member can be assigned.
	if a := assignable(laptop.ID); !a[hwUser.ID] || !a[swUser.ID] || !a[1] {
		t.Fatalf("groups off: %v", a)
	}

	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":true}`)

	// Groups on: only the hardware agent (and admins) for a hardware ticket…
	a := assignable(laptop.ID)
	if !a[hwUser.ID] || a[swUser.ID] || !a[1] {
		t.Fatalf("hardware ticket: %v", a)
	}
	// …but anyone for an uncategorised one.
	if a := assignable(misc.ID); !a[hwUser.ID] || !a[swUser.ID] {
		t.Fatalf("uncategorised ticket: %v", a)
	}

	// The server enforces it too.
	url := ts.URL + "/api/tickets/" + itoa(laptop.ID)
	if code := do(t, admin, "PATCH", url, `{"assigneeId":`+itoa(swUser.ID)+`}`); code != http.StatusBadRequest {
		t.Fatalf("assigning out-of-group agent: %d", code)
	}
	if code := do(t, admin, "PATCH", url, `{"assigneeId":`+itoa(hwUser.ID)+`}`); code != 200 {
		t.Fatalf("assigning in-group agent: %d", code)
	}
	// Re-saving the current assignee is fine even if groups changed since.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":false}`)
	do(t, admin, "PATCH", url, `{"assigneeId":`+itoa(swUser.ID)+`}`)
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":true}`)
	if code := do(t, admin, "PATCH", url, `{"assigneeId":`+itoa(swUser.ID)+`,"priority":"high"}`); code != 200 {
		t.Fatalf("keeping existing out-of-group assignee: %d", code)
	}
}
