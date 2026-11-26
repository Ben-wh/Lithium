package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lithium/lithium/internal/config"
)

func fakeGitHub(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	commit := func(sha, msg string) string {
		return fmt.Sprintf(`{"sha":%q,"html_url":"https://github.com/x/%s","commit":{"message":%q,"committer":{"date":"2026-11-01T10:00:00Z"}}}`, sha, sha, msg)
	}
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch {
		case strings.HasSuffix(r.URL.Path, "/commits/main"):
			fmt.Fprint(w, commit("ccc", "Third\n\nbody"))
		case strings.HasSuffix(r.URL.Path, "/compare/aaa...main"):
			fmt.Fprintf(w, `{"status":"ahead","ahead_by":2,"behind_by":0,"commits":[%s,%s]}`, commit("bbb", "Second"), commit("ccc", "Third\n\nbody"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(gh.Close)
	return gh
}

func TestUpdateCheck(t *testing.T) {
	var calls atomic.Int32
	gh := fakeGitHub(t, &calls)
	ts, srv := newTestServerWithMailer(t, config.Config{DataDir: t.TempDir()})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	check := func(commit string) updateStatus {
		t.Helper()
		srv.updates = newUpdateChecker()
		srv.updates.api = gh.URL
		srv.updates.commit = commit
		var st updateStatus
		if code := getJSON(t, admin, ts.URL+"/api/admin/updates", &st); code != 200 {
			t.Fatalf("check: %d", code)
		}
		return st
	}

	if code := do(t, agent, "GET", ts.URL+"/api/admin/updates", ""); code != http.StatusForbidden {
		t.Fatalf("agent: %d", code)
	}

	st := check("aaa")
	if st.Status != "behind" || st.Behind != 2 || len(st.Commits) != 2 || st.Commits[0].SHA != "ccc" || st.Commits[0].Message != "Third" {
		t.Fatalf("behind: %+v", st)
	}

	// The answer is cached; a refresh straight away is still throttled.
	before := calls.Load()
	getJSON(t, admin, ts.URL+"/api/admin/updates?refresh=1", &st)
	if calls.Load() != before {
		t.Fatal("refresh within the gap went to GitHub")
	}

	if st := check("ccc"); st.Status != "current" || st.Latest == nil || st.Latest.SHA != "ccc" {
		t.Fatalf("current: %+v", st)
	}
	if st := check(""); st.Status != "unknown" || st.Latest == nil {
		t.Fatalf("unknown: %+v", st)
	}
	if st := check("zzz"); st.Status != "unrecognised" {
		t.Fatalf("unrecognised: %+v", st)
	}

	gh.Close()
	if st := check("aaa"); st.Status != "error" || st.Error == "" {
		t.Fatalf("unreachable: %+v", st)
	}
}
