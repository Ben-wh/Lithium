package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lithium/lithium/internal/auth"
	"github.com/lithium/lithium/internal/config"
	"github.com/lithium/lithium/internal/db"
	"github.com/lithium/lithium/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newTestServerWithDataDir(t, t.TempDir())
}

func newTestServerWithDataDir(t *testing.T, dataDir string) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	st := store.New(conn)
	hash, _ := auth.HashPassword("correct-horse")
	if _, err := st.CreateUser(ctx, "admin@example.com", "Admin", hash, "admin", false); err != nil {
		t.Fatal(err)
	}
	// Mandatory two-factor is on by default; most tests don't care about it.
	// The two-factor tests turn it back on.
	off := false
	if _, err := st.UpdateSettings(ctx, store.SettingsUpdate{RequireTwoFactor: &off}); err != nil {
		t.Fatal(err)
	}
	srv, err := New(config.Config{DataDir: dataDir}, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler(fstest.MapFS{"index.html": {Data: []byte("app")}}))
	t.Cleanup(ts.Close)
	return ts
}

func post(t *testing.T, c *http.Client, url, body string) *http.Response {
	t.Helper()
	resp, err := c.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func get(t *testing.T, c *http.Client, url string) *http.Response {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestLoginFlow(t *testing.T) {
	ts := newTestServer(t)
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}

	if r := get(t, c, ts.URL+"/api/auth/me"); r.StatusCode != 401 {
		t.Fatalf("me before login: got %d", r.StatusCode)
	}
	if r := post(t, c, ts.URL+"/api/auth/login", `{"email":"admin@example.com","password":"wrong"}`); r.StatusCode != 401 {
		t.Fatalf("bad password: got %d", r.StatusCode)
	}
	if r := post(t, c, ts.URL+"/api/auth/login", `{"email":"nobody@example.com","password":"x"}`); r.StatusCode != 401 {
		t.Fatalf("unknown user: got %d", r.StatusCode)
	}
	if r := post(t, c, ts.URL+"/api/auth/login", `{"email":"ADMIN@example.com","password":"correct-horse"}`); r.StatusCode != 200 {
		t.Fatalf("login: got %d", r.StatusCode)
	}
	if r := get(t, c, ts.URL+"/api/auth/me"); r.StatusCode != 200 {
		t.Fatalf("me after login: got %d", r.StatusCode)
	}
	if r := post(t, c, ts.URL+"/api/auth/logout", `{}`); r.StatusCode != 204 {
		t.Fatalf("logout: got %d", r.StatusCode)
	}
	if r := get(t, c, ts.URL+"/api/auth/me"); r.StatusCode != 401 {
		t.Fatalf("me after logout: got %d", r.StatusCode)
	}
}

func TestLoginRejectsFormPosts(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/auth/login", "application/x-www-form-urlencoded",
		strings.NewReader("email=admin@example.com&password=correct-horse"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestLoginRateLimited(t *testing.T) {
	ts := newTestServer(t)
	var last int
	for range 7 {
		last = post(t, http.DefaultClient, ts.URL+"/api/auth/login", `{"email":"a@b.c","password":"x"}`).StatusCode
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", last)
	}
}

func TestSPAFallback(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "app" {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
}
