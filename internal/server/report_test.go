package server

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lithium/lithium/internal/store"
)

func TestReport(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, agentUser := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var hw store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware"}`, &hw)
	var a, b, c store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"a","description":"d","priority":"urgent","categoryId":`+itoa(hw.ID)+`}`, &a)
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"b","description":"d"}`, &b)
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"c","description":"d"}`, &c)
	do(t, admin, "PATCH", ts.URL+"/api/tickets/"+itoa(a.ID), `{"assigneeId":`+itoa(agentUser.ID)+`}`)
	postJSON(t, agent, ts.URL+"/api/tickets/"+itoa(a.ID)+"/comments", `{"body":"on it"}`, nil)
	do(t, agent, "PATCH", ts.URL+"/api/tickets/"+itoa(c.ID), `{"status":"resolved"}`)

	var rep store.Report
	if code := getJSON(t, alice, ts.URL+"/api/reports", &rep); code != http.StatusForbidden {
		t.Fatalf("user viewing reports: %d", code)
	}
	if code := getJSON(t, agent, ts.URL+"/api/reports?days=7&tz=0", &rep); code != 200 {
		t.Fatalf("report: %d", code)
	}
	if rep.Days != 7 || len(rep.Series) != 7 {
		t.Fatalf("series: %d days, %d points", rep.Days, len(rep.Series))
	}
	today := rep.Series[len(rep.Series)-1]
	if today.Date != time.Now().UTC().Format("2006-01-02") || today.Raised != 3 || today.Resolved != 1 {
		t.Fatalf("today: %+v", today)
	}
	if rep.Totals.Raised != 3 || rep.Totals.Resolved != 1 || rep.Totals.Open != 2 ||
		rep.Totals.MedianFirstResponseHours == nil || rep.Totals.MedianResolutionHours == nil {
		t.Fatalf("totals: %+v", rep.Totals)
	}
	counts := func(list []store.NamedCount) map[string]int {
		m := map[string]int{}
		for _, n := range list {
			m[n.Key] = n.Count
		}
		return m
	}
	if s := counts(rep.ByStatus); s["in_progress"] != 1 || s["unassigned"] != 1 || s["resolved"] != 1 {
		t.Fatalf("by status: %+v", rep.ByStatus)
	}
	if p := counts(rep.ByPriority); p["urgent"] != 1 || p["normal"] != 1 {
		t.Fatalf("by priority: %+v", rep.ByPriority)
	}
	// Active only, "none" last.
	if len(rep.ByCategory) != 2 || rep.ByCategory[0].Name != "Hardware" || rep.ByCategory[1].Key != "" {
		t.Fatalf("by category: %+v", rep.ByCategory)
	}
	if as := counts(rep.ByAssignee); as[itoa(agentUser.ID)] != 1 || as[""] != 1 {
		t.Fatalf("by assignee: %+v", rep.ByAssignee)
	}

	// Agent groups limit the figures too.
	postJSON(t, admin, ts.URL+"/api/admin/groups", `{"name":"Nobody's","memberIds":[],"categoryIds":[]}`, nil)
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"agentGroups":true}`)
	other, _ := newUser(t, ts.URL, admin, "Other", "other@example.com", "agent")
	getJSON(t, other, ts.URL+"/api/reports?days=7", &rep)
	if rep.Totals.Raised != 2 { // the Hardware ticket is hidden from an agent with no groups
		t.Fatalf("scoped report raised: %d", rep.Totals.Raised)
	}
}

func TestExport(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"=HYPERLINK(\"http://evil\")","description":"Line one\nLine, two"}`, nil)
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Second","description":"d"}`, nil)
	today := time.Now().UTC().Format("2006-01-02")
	base := ts.URL + "/api/reports/export?tz=0&from=" + today + "&to=" + today

	fetch := func(c *http.Client, url string) (*http.Response, string) {
		t.Helper()
		resp, err := c.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, string(body)
	}

	if resp, _ := fetch(alice, base); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("user export: %d", resp.StatusCode)
	}
	if resp, _ := fetch(admin, ts.URL+"/api/reports/export?from=2026-02-01&to=2026-01-01"); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad range: %d", resp.StatusCode)
	}

	// CSV: attachment, BOM, header, formulas neutralised, descriptions on request.
	resp, body := fetch(admin, base+"&description=1")
	if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment") ||
		!strings.HasPrefix(body, "\ufeff") {
		t.Fatalf("csv response: %d %q", resp.StatusCode, resp.Header)
	}
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || records[0][0] != "Reference" || records[0][len(records[0])-1] != "Description" {
		t.Fatalf("csv shape: %v", records)
	}
	if records[1][0] != "TID-1" || !strings.HasPrefix(records[1][1], "'=") || records[1][2] != "Unassigned" ||
		records[1][len(records[1])-1] != "Line one\nLine, two" {
		t.Fatalf("csv row: %v", records[1])
	}

	// JSON, filtered by status.
	resp, body = fetch(admin, base+"&format=json&status=unassigned")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(body), &rows); err != nil || len(rows) != 2 || rows[0]["reference"] != "TID-1" {
		t.Fatalf("json: %v %s", err, body)
	}
	_, body = fetch(admin, base+"&format=json&status=closed")
	if strings.TrimSpace(body) != "[]" {
		t.Fatalf("status filter: %s", body)
	}
}
