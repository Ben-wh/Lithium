package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func me(t *testing.T, c *http.Client, baseURL string) store.User {
	t.Helper()
	var u store.User
	if code := getJSON(t, c, baseURL+"/api/auth/me", &u); code != 200 {
		t.Fatalf("me: got %d", code)
	}
	return u
}

func TestNewUserMustChangePassword(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	post(t, admin, ts.URL+"/api/admin/users",
		`{"name":"Robin","email":"robin@example.com","role":"admin","password":"given-by-admin"}`)

	robin := signIn(t, ts.URL, "robin@example.com", "given-by-admin")
	if !me(t, robin, ts.URL).MustChangePassword {
		t.Fatal("new user should have to change password")
	}
	// Everything else is blocked until they do, even for an admin.
	var users []store.AdminUser
	if code := getJSON(t, robin, ts.URL+"/api/admin/users", &users); code != http.StatusForbidden {
		t.Fatalf("blocked endpoint: got %d", code)
	}

	// Validation still applies; reusing the given password isn't allowed.
	for body, want := range map[string]int{
		`{"newPassword":"short"}`:          http.StatusBadRequest,
		`{"newPassword":"given-by-admin"}`: http.StatusBadRequest,
	} {
		if r := post(t, robin, ts.URL+"/api/auth/password", body); r.StatusCode != want {
			t.Fatalf("%s: got %d, want %d", body, r.StatusCode, want)
		}
	}
	// A forced change doesn't need the current password again.
	if r := post(t, robin, ts.URL+"/api/auth/password", `{"newPassword":"robins-own-pass"}`); r.StatusCode != 200 {
		t.Fatalf("forced change: got %d", r.StatusCode)
	}
	if me(t, robin, ts.URL).MustChangePassword {
		t.Fatal("flag should be cleared")
	}
	if code := getJSON(t, robin, ts.URL+"/api/admin/users", &users); code != 200 {
		t.Fatalf("after change: got %d", code)
	}
	signIn(t, ts.URL, "robin@example.com", "robins-own-pass")
}

func TestVoluntaryChangeNeedsCurrentPassword(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	other := signIn(t, ts.URL, "admin@example.com", "correct-horse")

	for _, body := range []string{
		`{"newPassword":"another-pass"}`,
		`{"currentPassword":"wrong","newPassword":"another-pass"}`,
	} {
		if r := post(t, admin, ts.URL+"/api/auth/password", body); r.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: got %d", body, r.StatusCode)
		}
	}
	if r := post(t, admin, ts.URL+"/api/auth/password",
		`{"currentPassword":"correct-horse","newPassword":"another-pass"}`); r.StatusCode != 200 {
		t.Fatalf("change: got %d", r.StatusCode)
	}
	me(t, admin, ts.URL) // this session survives
	if r := get(t, other, ts.URL+"/api/auth/me"); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("other session should be signed out, got %d", r.StatusCode)
	}
}

func TestAdminResetForcesChangeExceptForSelf(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	resp, err := admin.Post(ts.URL+"/api/admin/users", "application/json",
		jsonBody(`{"name":"Kit","email":"kit@example.com","role":"agent","password":"initial-pass"}`))
	if err != nil {
		t.Fatal(err)
	}
	var kit store.User
	json.NewDecoder(resp.Body).Decode(&kit)
	resp.Body.Close()

	// Kit picks their own password, then an admin resets it.
	k := signIn(t, ts.URL, "kit@example.com", "initial-pass")
	post(t, k, ts.URL+"/api/auth/password", `{"newPassword":"kits-own-pass"}`)
	post(t, admin, ts.URL+"/api/admin/users/"+itoa(kit.ID)+"/password", `{"password":"admin-chosen"}`)
	k = signIn(t, ts.URL, "kit@example.com", "admin-chosen")
	if !me(t, k, ts.URL).MustChangePassword {
		t.Fatal("admin reset should force a change")
	}

	// An admin setting their own password from the admin page isn't forced.
	post(t, admin, ts.URL+"/api/admin/users/1/password", `{"password":"admins-new-pass"}`)
	if me(t, admin, ts.URL).MustChangePassword {
		t.Fatal("admin's own reset shouldn't force a change")
	}
}

func jsonBody(s string) *strings.Reader { return strings.NewReader(s) }
