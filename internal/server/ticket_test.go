package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func postJSON(t *testing.T, c *http.Client, url, body string, v any) int {
	t.Helper()
	resp, err := c.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

// newUser creates an account via the admin API and signs it in, getting past
// the forced password change.
func newUser(t *testing.T, baseURL string, admin *http.Client, name, email, role string) (*http.Client, store.User) {
	t.Helper()
	var u store.User
	body := `{"name":"` + name + `","email":"` + email + `","role":"` + role + `","password":"temporary-pass"}`
	if code := postJSON(t, admin, baseURL+"/api/admin/users", body, &u); code != http.StatusCreated {
		t.Fatalf("create %s: got %d", email, code)
	}
	c := signIn(t, baseURL, email, "temporary-pass")
	post(t, c, baseURL+"/api/auth/password", `{"newPassword":"chosen-password"}`)
	return c, u
}

func TestRaiseTicket(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, aliceUser := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, _ := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	// A user raises a ticket; trying to set someone else as requester is ignored.
	var tk store.Ticket
	code := postJSON(t, alice, ts.URL+"/api/tickets",
		`{"subject":"  Printer on fire  ","description":"It is very hot.","priority":"urgent","requesterId":1}`, &tk)
	if code != http.StatusCreated {
		t.Fatalf("create: got %d", code)
	}
	if tk.Subject != "Printer on fire" || tk.Status != "unassigned" || tk.Priority != "urgent" ||
		tk.Requester.ID != aliceUser.ID || tk.CreatedBy.ID != aliceUser.ID || tk.Assignee != nil {
		t.Fatalf("unexpected ticket: %+v", tk)
	}

	// Alice and staff can see it; Bob can't, and gets a 404 rather than a 403.
	var got store.Ticket
	if code := getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID), &got); code != 200 || got.ID != tk.ID {
		t.Fatalf("requester view: %d", code)
	}
	if code := getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(tk.ID), &got); code != 200 {
		t.Fatalf("agent view: %d", code)
	}
	if code := getJSON(t, bob, ts.URL+"/api/tickets/"+itoa(tk.ID), &got); code != http.StatusNotFound {
		t.Fatalf("other user view: %d", code)
	}
	if code := getJSON(t, alice, ts.URL+"/api/tickets/9999", &got); code != http.StatusNotFound {
		t.Fatalf("missing ticket: %d", code)
	}

	// An agent raises one on Alice's behalf; priority defaults to normal.
	code = postJSON(t, agent, ts.URL+"/api/tickets",
		`{"subject":"Phoned in","description":"Alice called about VPN.","requesterId":`+itoa(aliceUser.ID)+`}`, &tk)
	if code != http.StatusCreated || tk.Requester.ID != aliceUser.ID || tk.CreatedBy.ID != agentUser.ID || tk.Priority != "normal" {
		t.Fatalf("on behalf: %d %+v", code, tk)
	}
	if code := postJSON(t, agent, ts.URL+"/api/tickets",
		`{"subject":"x","description":"y","requesterId":9999}`, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown requester: %d", code)
	}

	// Validation.
	long := strings.Repeat("a", 201)
	for _, body := range []string{
		`{"subject":"","description":"y"}`,
		`{"subject":"   ","description":"y"}`,
		`{"subject":"x","description":""}`,
		`{"subject":"` + long + `","description":"y"}`,
		`{"subject":"x","description":"y","priority":"critical"}`,
	} {
		if code := postJSON(t, alice, ts.URL+"/api/tickets", body, nil); code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %.60s, got %d", body, code)
		}
	}
}

func TestTicketLengthLimits(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")
	raise := func(subjectLen, descLen int) int {
		t.Helper()
		// Multi-byte characters: limits count characters, not bytes.
		body := `{"subject":"` + strings.Repeat("é", subjectLen) + `","description":"` + strings.Repeat("é", descLen) + `"}`
		return postJSON(t, alice, ts.URL+"/api/tickets", body, nil)
	}

	// Defaults: 150 / 5,000.
	var st store.Settings
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if st.SubjectMaxLength != 150 || st.DescriptionMaxLength != 5000 {
		t.Fatalf("defaults: %d / %d", st.SubjectMaxLength, st.DescriptionMaxLength)
	}
	for _, c := range []struct{ subject, desc, want int }{
		{150, 5000, http.StatusCreated},
		{151, 10, http.StatusBadRequest},
		{10, 5001, http.StatusBadRequest},
	} {
		if code := raise(c.subject, c.desc); code != c.want {
			t.Fatalf("%d/%d chars: got %d, want %d", c.subject, c.desc, code, c.want)
		}
	}

	// Only admins can change them, and only within range.
	if code := do(t, agent, "PATCH", ts.URL+"/api/admin/settings", `{"subjectMaxLength":100}`); code != http.StatusForbidden {
		t.Fatalf("agent change: %d", code)
	}
	for _, body := range []string{
		`{"subjectMaxLength":19}`, `{"subjectMaxLength":201}`,
		`{"descriptionMaxLength":99}`, `{"descriptionMaxLength":20001}`,
	} {
		if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", body); code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, code)
		}
	}

	// Tighten: enforced straight away.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"subjectMaxLength":40,"descriptionMaxLength":200}`)
	if code := raise(41, 10); code != http.StatusBadRequest {
		t.Fatalf("tightened subject: %d", code)
	}
	if code := raise(40, 200); code != http.StatusCreated {
		t.Fatalf("at tightened limits: %d", code)
	}

	// At the maximums, a full-length description still fits the request size limit.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"subjectMaxLength":200,"descriptionMaxLength":20000}`)
	if code := raise(200, 20000); code != http.StatusCreated {
		t.Fatalf("max lengths: %d", code)
	}
	if code := raise(10, 20001); code != http.StatusBadRequest {
		t.Fatalf("over max: %d", code)
	}
}

func TestUserDirectoryIsStaffOnly(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var users []store.UserRef
	if code := getJSON(t, agent, ts.URL+"/api/users?q=ali", &users); code != 200 || len(users) != 1 || users[0].Name != "Alice" {
		t.Fatalf("agent directory: %d %+v", code, users)
	}
	if code := getJSON(t, alice, ts.URL+"/api/users", &users); code != http.StatusForbidden {
		t.Fatalf("user directory: %d", code)
	}
}
