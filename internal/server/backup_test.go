package server

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/lithium/lithium/internal/config"
)

func TestBackups(t *testing.T) {
	ts, _ := newTestServerWithConfig(t, config.Config{DataDir: t.TempDir()})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")
	api := ts.URL + "/api/admin/backups"

	// Admins only.
	if code := getJSON(t, agent, api, nil); code != http.StatusForbidden {
		t.Fatalf("agent list: %d", code)
	}
	if code := postJSON(t, agent, api, `{}`, nil); code != http.StatusForbidden {
		t.Fatalf("agent create: %d", code)
	}

	// Off by default, nothing yet.
	var st backupStatus
	getJSON(t, admin, api, &st)
	if st.Settings.Enabled || len(st.Backups) != 0 || st.NextRun != nil || st.TimeZone == "" {
		t.Fatalf("initial: %+v", st)
	}

	// Schedule settings are validated, then saved.
	for _, bad := range []string{
		`{"enabled":true,"frequency":"hourly","hour":2,"keep":7}`,
		`{"enabled":true,"frequency":"daily","hour":24,"keep":7}`,
		`{"enabled":true,"frequency":"daily","hour":2,"keep":0}`,
	} {
		if code := do(t, admin, "PUT", api+"/settings", bad); code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, code)
		}
	}
	if code := do(t, admin, "PUT", api+"/settings", `{"enabled":true,"frequency":"weekly","hour":3,"keep":4}`); code != 200 {
		t.Fatalf("save settings: %d", code)
	}
	getJSON(t, admin, api, &st)
	if !st.Settings.Enabled || st.Settings.Frequency != "weekly" || st.Settings.Hour != 3 || st.NextRun == nil {
		t.Fatalf("saved: %+v", st)
	}

	// Back up now runs in the background; wait for it.
	if code := postJSON(t, admin, api, `{}`, nil); code != http.StatusAccepted {
		t.Fatalf("create: %d", code)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		getJSON(t, admin, api, &st)
		if !st.Running && len(st.Backups) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("backup didn't finish: %+v", st)
		}
		time.Sleep(50 * time.Millisecond)
	}
	b := st.Backups[0]
	if b.Kind != "manual" || b.Size == 0 || st.LastError != "" {
		t.Fatalf("backup: %+v %q", b, st.LastError)
	}

	// Download it (admins only).
	resp, err := agent.Get(api + "/" + b.Name + "/download")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("agent download: %d", resp.StatusCode)
	}
	resp, err = admin.Get(api + "/" + b.Name + "/download")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || int64(len(body)) != b.Size || resp.Header.Get("Content-Disposition") == "" {
		t.Fatalf("download: %d %d bytes", resp.StatusCode, len(body))
	}

	// Names outside the backups folder are refused.
	for _, bad := range []string{"..%2Flithium.db", "lithium.db", "x"} {
		r, err := admin.Get(api + "/" + bad + "/download")
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusNotFound {
			t.Errorf("download %q: %d", bad, r.StatusCode)
		}
	}

	// Delete it.
	if code := do(t, admin, "DELETE", api+"/"+b.Name, ""); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	if code := do(t, admin, "DELETE", api+"/"+b.Name, ""); code != http.StatusNotFound {
		t.Fatalf("delete again: %d", code)
	}

	// It's all in the activity log.
	var log struct {
		Entries []struct{ Action, Detail string }
	}
	getJSON(t, admin, ts.URL+"/api/admin/activity?category=system", &log)
	seen := map[string]bool{}
	for _, e := range log.Entries {
		seen[e.Action] = true
	}
	for _, want := range []string{"backup.settings", "backup.create", "backup.download", "backup.delete"} {
		if !seen[want] {
			t.Errorf("activity log missing %s: %+v", want, log.Entries)
		}
	}
}
