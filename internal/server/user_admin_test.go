package server

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func TestChangeRole(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	// The agent takes Alice's ticket.
	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d"}`, &tk)
	do(t, agent, "PATCH", ts.URL+"/api/tickets/"+itoa(tk.ID), `{"assigneeId":`+itoa(agentUser.ID)+`}`)

	url := ts.URL + "/api/admin/users/" + itoa(agentUser.ID)
	for body, want := range map[string]int{`{"role":"superuser"}`: 400, `{"role":""}`: 400} {
		if code := do(t, admin, "PATCH", url, body); code != want {
			t.Fatalf("%s: %d", body, code)
		}
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/users/1", `{"role":"user"}`); code != http.StatusBadRequest {
		t.Fatalf("changing own role: %d", code)
	}
	if code := do(t, agent, "PATCH", url, `{"role":"admin"}`); code != http.StatusForbidden {
		t.Fatalf("agent changing roles: %d", code)
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/users/9999", `{"role":"user"}`); code != http.StatusNotFound {
		t.Fatalf("missing user: %d", code)
	}

	// Promote: takes effect on the next request (role is read per request).
	do(t, admin, "PATCH", url, `{"role":"admin"}`)
	var users []store.AdminUser
	if code := getJSON(t, agent, ts.URL+"/api/admin/users", &users); code != 200 {
		t.Fatalf("promoted user can't reach admin: %d", code)
	}

	// Demote to user: unassigned from their ticket, which goes back to unassigned.
	if code := do(t, admin, "PATCH", url, `{"role":"user"}`); code != 200 {
		t.Fatalf("demote: %d", code)
	}
	var got store.Ticket
	getJSON(t, admin, ts.URL+"/api/tickets/"+itoa(tk.ID), &got)
	if got.Assignee != nil || got.Status != "unassigned" {
		t.Fatalf("after demotion: assignee %+v status %s", got.Assignee, got.Status)
	}
	ev := comments(t, admin, ts.URL, tk.ID)
	last := ev[len(ev)-1]
	if last.Event != "status" || last.To != "unassigned" || ev[len(ev)-2].Event != "assignee" || ev[len(ev)-2].From != "Agent" {
		t.Fatalf("demotion events: %+v", ev)
	}
	if code := getJSON(t, agent, ts.URL+"/api/admin/users", &users); code != http.StatusForbidden {
		t.Fatalf("demoted user still has admin: %d", code)
	}
}

func TestDeleteUser(t *testing.T) {
	dataDir := t.TempDir()
	ts := newTestServerWithDataDir(t, dataDir)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, aliceUser := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, bobUser := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "agent")
	carol, carolUser := newUser(t, ts.URL, admin, "Carol", "carol@example.com", "user")

	// Bob raises a ticket for himself (with an image), one on Carol's behalf,
	// takes Alice's ticket and replies on it.
	_, own, _ := raiseWithImages(t, bob, ts.URL, ts.URL, upload{"x.png", fakePNG})
	var forCarol, alices store.Ticket
	postJSON(t, bob, ts.URL+"/api/tickets", `{"subject":"For Carol","description":"d","requesterId":`+itoa(carolUser.ID)+`}`, &forCarol)
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Alice's","description":"d"}`, &alices)
	do(t, bob, "PATCH", ts.URL+"/api/tickets/"+itoa(alices.ID), `{"assigneeId":`+itoa(bobUser.ID)+`}`)
	postJSON(t, bob, ts.URL+"/api/tickets/"+itoa(alices.ID)+"/comments", `{"body":"On it"}`, nil)
	uploadAvatar(t, bob, ts.URL, fakePNG)

	// The admin list knows what will be lost.
	var users []store.AdminUser
	getJSON(t, admin, ts.URL+"/api/admin/users?q=bob", &users)
	if len(users) != 1 || users[0].TicketCount != 1 || users[0].AssignedCount != 1 {
		t.Fatalf("counts: %+v", users)
	}

	// Guard rails.
	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/users/1", ""); code != http.StatusBadRequest {
		t.Fatalf("self delete: %d", code)
	}
	if code := do(t, bob, "DELETE", ts.URL+"/api/admin/users/"+itoa(aliceUser.ID), ""); code != http.StatusForbidden {
		t.Fatalf("agent delete: %d", code)
	}

	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/users/"+itoa(bobUser.ID), ""); code != 200 {
		t.Fatalf("delete: %d", code)
	}

	// Bob can't sign in, and his session is gone.
	if r := post(t, http.DefaultClient, ts.URL+"/api/auth/login", `{"email":"bob@example.com","password":"chosen-password"}`); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("deleted user login: %d", r.StatusCode)
	}
	if r := get(t, bob, ts.URL+"/api/auth/me"); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("deleted user session: %d", r.StatusCode)
	}

	// His own ticket is gone, along with its image file and his avatar.
	var none store.Ticket
	if code := getJSON(t, admin, ts.URL+"/api/tickets/"+itoa(own.ID), &none); code != http.StatusNotFound {
		t.Fatalf("own ticket still there: %d", code)
	}
	if n := countFiles(t, filepath.Join(dataDir, "attachments")); n != 0 {
		t.Fatalf("attachment files left: %d", n)
	}
	if n := countFiles(t, filepath.Join(dataDir, "avatars")); n != 0 {
		t.Fatalf("avatar file left: %d", n)
	}

	// The ticket he raised for Carol stays, now credited to her.
	var c store.Ticket
	if code := getJSON(t, carol, ts.URL+"/api/tickets/"+itoa(forCarol.ID), &c); code != 200 || c.CreatedBy.ID != carolUser.ID {
		t.Fatalf("ticket raised for Carol: %d %+v", code, c.CreatedBy)
	}

	// Alice's ticket is unassigned, and his reply stays without an author.
	var a store.Ticket
	getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(alices.ID), &a)
	if a.Assignee != nil || a.Status != "unassigned" {
		t.Fatalf("alice's ticket: %+v %s", a.Assignee, a.Status)
	}
	var replyFound bool
	for _, cm := range comments(t, alice, ts.URL, alices.ID) {
		if cm.Kind == "comment" && cm.Body == "On it" {
			replyFound = cm.Author == nil
		}
	}
	if !replyFound {
		t.Fatal("reply should remain with no author")
	}

	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/users/"+itoa(bobUser.ID), ""); code != http.StatusNotFound {
		t.Fatalf("delete twice: %d", code)
	}
}
