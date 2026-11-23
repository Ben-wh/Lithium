package server

import (
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

type notifList struct {
	Items  []store.Notification `json:"items"`
	Unread int                  `json:"unread"`
}

func notifications(t *testing.T, c *http.Client, baseURL string) notifList {
	t.Helper()
	var n notifList
	if code := getJSON(t, c, baseURL+"/api/notifications", &n); code != 200 {
		t.Fatalf("notifications: %d", code)
	}
	return n
}

func TestNotificationsAndInternalNotes(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent Smith", "agent@example.com", "agent")
	other, _ := newUser(t, ts.URL, admin, "Other Agent", "other@example.com", "agent")

	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Printer","description":"d"}`, &tk)
	ticketURL := ts.URL + "/api/tickets/" + itoa(tk.ID)
	commentsURL := ticketURL + "/comments"

	// Assigning: the agent hears "assigned this to you"; Alice hears about
	// the change; the admin who did it hears nothing.
	do(t, admin, "PATCH", ticketURL, `{"assigneeId":`+itoa(agentUser.ID)+`}`)
	if n := notifications(t, agent, ts.URL); n.Unread != 1 || n.Items[0].Kind != "assigned" || n.Items[0].Summary != "assigned this to you" {
		t.Fatalf("agent after assign: %+v", n)
	}
	a := notifications(t, alice, ts.URL)
	if a.Unread != 1 || a.Items[0].Kind != "update" || a.Items[0].Summary != "updated the assignee and status" ||
		a.Items[0].Ticket.Subject != "Printer" || a.Items[0].Actor == nil {
		t.Fatalf("alice after assign: %+v", a)
	}
	if n := notifications(t, admin, ts.URL); n.Unread != 0 {
		t.Fatalf("actor notified about own change: %+v", n)
	}

	// A public reply from the agent reaches Alice but not the agent.
	postJSON(t, agent, commentsURL, `{"body":"Looking now"}`, nil)
	if a := notifications(t, alice, ts.URL); a.Unread != 2 || a.Items[0].Kind != "reply" {
		t.Fatalf("alice after reply: %+v", a)
	}

	// Internal notes: only staff can write them; Alice never sees or hears of them.
	if code := postJSON(t, alice, commentsURL, `{"body":"sneaky","internal":true}`, nil); code != http.StatusForbidden {
		t.Fatalf("requester internal note: %d", code)
	}
	var note store.Comment
	postJSON(t, admin, commentsURL, `{"body":"Toner is out of stock","internal":true}`, &note)
	if !note.Internal {
		t.Fatal("note not marked internal")
	}
	for _, c := range comments(t, alice, ts.URL, tk.ID) {
		if c.Internal || c.Body == "Toner is out of stock" {
			t.Fatal("requester can see internal note")
		}
	}
	var staffSees bool
	for _, c := range comments(t, other, ts.URL, tk.ID) {
		staffSees = staffSees || (c.Internal && c.Body == "Toner is out of stock")
	}
	if !staffSees {
		t.Fatal("staff can't see internal note")
	}
	if a := notifications(t, alice, ts.URL); a.Unread != 2 {
		t.Fatalf("requester notified about internal note: %+v", a.Items[0])
	}
	if n := notifications(t, agent, ts.URL); n.Items[0].Kind != "note" {
		t.Fatalf("assignee not notified about note: %+v", n.Items[0])
	}

	// Opening the ticket marks its notifications read.
	var counts map[string]int
	postJSON(t, alice, ts.URL+"/api/notifications/read", `{"ticketId":`+itoa(tk.ID)+`}`, &counts)
	if counts["unread"] != 0 || notifications(t, alice, ts.URL).Unread != 0 {
		t.Fatalf("mark ticket read: %+v", counts)
	}
	postJSON(t, agent, ts.URL+"/api/notifications/read", `{}`, &counts)
	if counts["unread"] != 0 {
		t.Fatalf("mark all read: %+v", counts)
	}
}

func TestMentions(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, aliceUser := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, bobUser := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent Smith", "agent@example.com", "agent")

	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Printer","description":"d"}`, &tk)
	commentsURL := ts.URL + "/api/tickets/" + itoa(tk.ID) + "/comments"
	mention := func(name string, id int64) string { return `@[` + name + `](` + itoa(id) + `)` }

	// Alice mentions the agent: they get "mentioned you".
	postJSON(t, alice, commentsURL, `{"body":"Hi `+mention("Agent Smith", agentUser.ID)+`, any news?"}`, nil)
	if n := notifications(t, agent, ts.URL); n.Unread != 1 || n.Items[0].Kind != "mention" || n.Items[0].Summary != "mentioned you" {
		t.Fatalf("mention: %+v", n)
	}

	// Mentioning someone who can't see the ticket notifies nobody (Bob isn't involved).
	postJSON(t, agent, commentsURL, `{"body":"cc `+mention("Bob", bobUser.ID)+`"}`, nil)
	if n := notifications(t, bob, ts.URL); n.Unread != 0 {
		t.Fatalf("outsider notified via mention: %+v", n)
	}

	// The requester mentioned in an internal note isn't told (they can't see it).
	postJSON(t, agent, commentsURL, `{"body":"`+mention("Alice", aliceUser.ID)+` seems frustrated","internal":true}`, nil)
	for _, n := range notifications(t, alice, ts.URL).Items {
		if n.Kind == "mention" {
			t.Fatal("requester notified of mention in internal note")
		}
	}

	// A mentioned requester gets one notification ("mentioned you"), not two.
	before := notifications(t, alice, ts.URL).Unread
	postJSON(t, agent, commentsURL, `{"body":"`+mention("Alice", aliceUser.ID)+` can you try again?"}`, nil)
	a := notifications(t, alice, ts.URL)
	if a.Unread != before+1 || a.Items[0].Kind != "mention" {
		t.Fatalf("mentioned requester: %+v", a.Items[:2])
	}

	// Who can be mentioned: staff and the requester, not yourself; internal → staff only.
	var people []store.UserRef
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/mentionable", &people)
	names := map[string]bool{}
	for _, p := range people {
		names[p.Name] = true
	}
	if !names["Alice"] || !names["Admin"] || names["Agent Smith"] || names["Bob"] {
		t.Fatalf("mentionable: %+v", people)
	}
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/mentionable?internal=1", &people)
	for _, p := range people {
		if p.Name == "Alice" {
			t.Fatal("requester offered in internal note")
		}
	}
	// Non-staff don't get emails.
	getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/mentionable", &people)
	for _, p := range people {
		if p.Email != "" {
			t.Fatalf("email leaked to requester: %+v", p)
		}
	}
}

func TestUserMentionsSetting(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, aliceUser := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent Smith", "agent@example.com", "agent")

	var st store.Settings
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if !st.AllowUserMentions {
		t.Fatal("user mentions should be allowed by default")
	}

	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d"}`, &tk)
	base := ts.URL + "/api/tickets/" + itoa(tk.ID)
	mentionAgent := `@[Agent Smith](` + itoa(agentUser.ID) + `)`

	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"allowUserMentions":false}`)

	// Users get no one to mention, and hand-typed tokens become plain text.
	var people []store.UserRef
	getJSON(t, alice, base+"/mentionable", &people)
	if len(people) != 0 {
		t.Fatalf("mentionable while disabled: %+v", people)
	}
	var c store.Comment
	postJSON(t, alice, base+"/comments", `{"body":"hey `+mentionAgent+`"}`, &c)
	if c.Body != "hey @Agent Smith" {
		t.Fatalf("token not stripped: %q", c.Body)
	}
	for _, n := range notifications(t, agent, ts.URL).Items {
		if n.Kind == "mention" {
			t.Fatal("mention notification sent while disabled")
		}
	}

	// Staff can still mention.
	getJSON(t, agent, base+"/mentionable", &people)
	if len(people) == 0 {
		t.Fatal("staff lost mentions")
	}
	postJSON(t, agent, base+"/comments", `{"body":"@[Alice](`+itoa(aliceUser.ID)+`) thanks"}`, nil)
	if n := notifications(t, alice, ts.URL); n.Items[0].Kind != "mention" {
		t.Fatalf("staff mention: %+v", n.Items[0])
	}

	// Turning it back on restores it for users.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"allowUserMentions":true}`)
	getJSON(t, alice, base+"/mentionable", &people)
	if len(people) == 0 {
		t.Fatal("mentions not restored")
	}
}

func TestGroupMentions(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	hw1, hw1User := newUser(t, ts.URL, admin, "HW One", "hw1@example.com", "agent")
	hw2, hw2User := newUser(t, ts.URL, admin, "HW Two", "hw2@example.com", "agent")
	sw, _ := newUser(t, ts.URL, admin, "SW", "sw@example.com", "agent")

	var cat store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Software"}`, &cat)
	var g store.Group
	postJSON(t, admin, ts.URL+"/api/admin/groups", `{"name":"Hardware team","memberIds":[`+itoa(hw1User.ID)+`,`+itoa(hw2User.ID)+`],"categoryIds":[]}`, &g)
	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d"}`, &tk)
	url := ts.URL + "/api/tickets/" + itoa(tk.ID)
	token := `@[Hardware team](g:` + itoa(g.ID) + `)`

	// Staff can see the groups to mention; users can't.
	var groups []struct {
		ID          int64
		Name        string
		MemberCount int
	}
	if code := getJSON(t, sw, url+"/mentionable-groups", &groups); code != 200 || len(groups) != 1 || groups[0].MemberCount != 2 {
		t.Fatalf("mentionable groups: %d %+v", code, groups)
	}
	if code := getJSON(t, alice, url+"/mentionable-groups", &groups); code != http.StatusForbidden {
		t.Fatalf("user listing groups: %d", code)
	}

	// Mentioning the group notifies each member once; a member also mentioned
	// directly gets "mentioned you" instead of a second notification.
	postJSON(t, sw, url+"/comments", `{"body":"cc `+token+` and @[HW Two](`+itoa(hw2User.ID)+`)"}`, nil)
	n1 := notifications(t, hw1, ts.URL)
	if n1.Unread != 1 || n1.Items[0].Kind != "mention" || n1.Items[0].Summary != "mentioned Hardware team" {
		t.Fatalf("group member: %+v", n1.Items)
	}
	n2 := notifications(t, hw2, ts.URL)
	if n2.Unread != 1 || n2.Items[0].Summary != "mentioned you" {
		t.Fatalf("member mentioned directly too: %+v", n2.Items)
	}
	if notifications(t, sw, ts.URL).Unread != 0 {
		t.Fatal("author notified")
	}

	// Members who can't see the ticket (agent groups on, other category) aren't notified.
	do(t, admin, "PATCH", url, `{"categoryId":`+itoa(cat.ID)+`}`)
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":true}`)
	postJSON(t, admin, url+"/comments", `{"body":"again `+token+`"}`, nil)
	if n := notifications(t, hw1, ts.URL); n.Unread != 1 {
		t.Fatalf("out-of-scope member notified: %+v", n.Items)
	}
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":false}`)

	// Users can't mention groups: the token is stored as plain text.
	var c store.Comment
	postJSON(t, alice, url+"/comments", `{"body":"hey `+token+`"}`, &c)
	if c.Body != "hey @Hardware team" {
		t.Fatalf("user group mention not stripped: %q", c.Body)
	}
}
