package server

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

// signIn returns a client with a session for the given account.
func signIn(t *testing.T, baseURL, email, password string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	body := `{"email":"` + email + `","password":"` + password + `"}`
	if r := post(t, c, baseURL+"/api/auth/login", body); r.StatusCode != 200 {
		t.Fatalf("sign in %s: got %d", email, r.StatusCode)
	}
	return c
}

func getJSON(t *testing.T, c *http.Client, url string, v any) int {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

func TestAdminUserManagement(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")

	// Create an agent.
	r := post(t, admin, ts.URL+"/api/admin/users",
		`{"name":"Jamie Agent","email":"jamie@example.com","role":"agent","password":"agent-pass-1"}`)
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("create user: got %d", r.StatusCode)
	}
	// Same email (different case) is rejected.
	if r := post(t, admin, ts.URL+"/api/admin/users",
		`{"name":"Dup","email":"JAMIE@example.com","role":"agent","password":"agent-pass-1"}`); r.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate: got %d", r.StatusCode)
	}
	// Validation.
	for _, body := range []string{
		`{"name":"","email":"a@example.com","role":"agent","password":"longenough"}`,
		`{"name":"A","email":"not-an-email","role":"agent","password":"longenough"}`,
		`{"name":"A","email":"a@example.com","role":"superuser","password":"longenough"}`,
		`{"name":"A","email":"a@example.com","role":"agent","password":"short"}`,
	} {
		if r := post(t, admin, ts.URL+"/api/admin/users", body); r.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d", body, r.StatusCode)
		}
	}

	// Search.
	var users []store.AdminUser
	getJSON(t, admin, ts.URL+"/api/admin/users?q=jam", &users)
	if len(users) != 1 || users[0].Email != "jamie@example.com" {
		t.Fatalf("search: got %+v", users)
	}
	getJSON(t, admin, ts.URL+"/api/admin/users?q=%25", &users) // literal %, not a wildcard
	if len(users) != 0 {
		t.Fatalf("wildcard search should match nothing, got %d", len(users))
	}
	getJSON(t, admin, ts.URL+"/api/admin/users", &users)
	if len(users) != 2 {
		t.Fatalf("list all: got %d", len(users))
	}

	// The agent can sign in but can't reach admin endpoints.
	agent := signIn(t, ts.URL, "jamie@example.com", "agent-pass-1")
	if code := getJSON(t, agent, ts.URL+"/api/admin/users", &users); code != http.StatusForbidden {
		t.Fatalf("agent list users: got %d", code)
	}
	if r := post(t, agent, ts.URL+"/api/admin/users",
		`{"name":"X","email":"x@example.com","role":"admin","password":"longenough"}`); r.StatusCode != http.StatusForbidden {
		t.Fatalf("agent create user: got %d", r.StatusCode)
	}
	if code := getJSON(t, http.DefaultClient, ts.URL+"/api/admin/users", &users); code != http.StatusUnauthorized {
		t.Fatalf("anonymous list users: got %d", code)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	post(t, admin, ts.URL+"/api/admin/users",
		`{"name":"Sam","email":"sam@example.com","role":"agent","password":"old-password"}`)
	sam := signIn(t, ts.URL, "sam@example.com", "old-password")

	// Requests look identical whether or not the account exists; repeats don't duplicate.
	for _, email := range []string{"sam@example.com", "sam@example.com", "ghost@example.com"} {
		if r := post(t, http.DefaultClient, ts.URL+"/api/auth/password-reset-request", `{"email":"`+email+`"}`); r.StatusCode != http.StatusAccepted {
			t.Fatalf("reset request %s: got %d", email, r.StatusCode)
		}
	}
	var reqs []store.ResetRequest
	getJSON(t, admin, ts.URL+"/api/admin/password-resets", &reqs)
	if len(reqs) != 1 || reqs[0].User.Email != "sam@example.com" {
		t.Fatalf("pending requests: got %+v", reqs)
	}
	var users []store.AdminUser
	getJSON(t, admin, ts.URL+"/api/admin/users?q=sam", &users)
	if len(users) != 1 || !users[0].PendingReset {
		t.Fatalf("expected pendingReset flag, got %+v", users)
	}

	// Admin sets a new password.
	if r := post(t, admin, ts.URL+"/api/admin/users/"+itoa(users[0].ID)+"/password", `{"password":"short"}`); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("short password: got %d", r.StatusCode)
	}
	if r := post(t, admin, ts.URL+"/api/admin/users/9999/password", `{"password":"new-password"}`); r.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown user: got %d", r.StatusCode)
	}
	if r := post(t, admin, ts.URL+"/api/admin/users/"+itoa(users[0].ID)+"/password", `{"password":"new-password"}`); r.StatusCode != http.StatusNoContent {
		t.Fatalf("set password: got %d", r.StatusCode)
	}

	// Sam's old session is gone, the old password fails, the new one works.
	if r := get(t, sam, ts.URL+"/api/auth/me"); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old session should be revoked, got %d", r.StatusCode)
	}
	if r := post(t, http.DefaultClient, ts.URL+"/api/auth/login", `{"email":"sam@example.com","password":"old-password"}`); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old password: got %d", r.StatusCode)
	}
	signIn(t, ts.URL, "sam@example.com", "new-password")

	// The request was completed.
	getJSON(t, admin, ts.URL+"/api/admin/password-resets", &reqs)
	if len(reqs) != 0 {
		t.Fatalf("expected no pending requests, got %d", len(reqs))
	}
	getJSON(t, admin, ts.URL+"/api/admin/password-resets?status=completed", &reqs)
	if len(reqs) != 1 {
		t.Fatalf("expected 1 completed request, got %d", len(reqs))
	}

	// A new request can be dismissed, but only once.
	post(t, http.DefaultClient, ts.URL+"/api/auth/password-reset-request", `{"email":"sam@example.com"}`)
	getJSON(t, admin, ts.URL+"/api/admin/password-resets", &reqs)
	if len(reqs) != 1 {
		t.Fatalf("expected new pending request, got %d", len(reqs))
	}
	dismiss := ts.URL + "/api/admin/password-resets/" + itoa(reqs[0].ID) + "/dismiss"
	if r := post(t, admin, dismiss, `{}`); r.StatusCode != http.StatusNoContent {
		t.Fatalf("dismiss: got %d", r.StatusCode)
	}
	if r := post(t, admin, dismiss, `{}`); r.StatusCode != http.StatusNotFound {
		t.Fatalf("dismiss twice: got %d", r.StatusCode)
	}
}

func TestAdminResettingOwnPasswordStaysSignedIn(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	other := signIn(t, ts.URL, "admin@example.com", "correct-horse") // e.g. another device

	if r := post(t, admin, ts.URL+"/api/admin/users/1/password", `{"password":"brand-new-pass"}`); r.StatusCode != http.StatusNoContent {
		t.Fatalf("set own password: got %d", r.StatusCode)
	}
	if r := get(t, admin, ts.URL+"/api/auth/me"); r.StatusCode != 200 {
		t.Fatalf("current session should survive, got %d", r.StatusCode)
	}
	if r := get(t, other, ts.URL+"/api/auth/me"); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("other sessions should be revoked, got %d", r.StatusCode)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
