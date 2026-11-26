package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/lithium/lithium/internal/config"
	"github.com/lithium/lithium/internal/store"
)

func TestSLARulesAndAlerts(t *testing.T) {
	ts, srv := newTestServerWithMailer(t, config.Config{DataDir: t.TempDir()})
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")
	var hw store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware"}`, &hw)
	api := ts.URL + "/api/admin/sla-rules"

	// Admins only, and rules are checked.
	if code := postJSON(t, agent, api, `{"resolutionMinutes":60}`, nil); code != http.StatusForbidden {
		t.Fatalf("agent create: %d", code)
	}
	for body, want := range map[string]int{
		`{}`: http.StatusBadRequest, // no target
		`{"priority":"meh","resolutionMinutes":60}`:           http.StatusBadRequest,
		`{"resolutionMinutes":0}`:                             http.StatusBadRequest,
		`{"firstResponseMinutes":120,"resolutionMinutes":60}`: http.StatusBadRequest,
		`{"categoryId":9999,"resolutionMinutes":60}`:          http.StatusBadRequest,
	} {
		if code := postJSON(t, admin, api, body, nil); code != want {
			t.Errorf("%s: %d, want %d", body, code, want)
		}
	}
	var specific, byPriority store.SLARule
	if code := postJSON(t, admin, api, `{"categoryId":`+itoa(hw.ID)+`,"priority":"urgent","firstResponseMinutes":30,"resolutionMinutes":120}`, &specific); code != http.StatusCreated {
		t.Fatalf("create: %d", code)
	}
	postJSON(t, admin, api, `{"priority":"urgent","resolutionMinutes":240}`, &byPriority)
	postJSON(t, admin, api, `{"firstResponseMinutes":480}`, nil) // catch-all
	if code := postJSON(t, admin, api, `{"priority":"urgent","resolutionMinutes":10}`, nil); code != http.StatusConflict {
		t.Fatalf("duplicate: %d", code)
	}
	var rules []store.SLARule
	getJSON(t, admin, api, &rules)
	if len(rules) != 3 || rules[0].ID != specific.ID || rules[2].Category != nil || rules[2].Priority != "" {
		t.Fatalf("rules, most specific first: %+v", rules)
	}

	// An urgent hardware ticket gets the specific rule; staff see it, the requester doesn't.
	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Server down","description":"d","priority":"urgent","categoryId":`+itoa(hw.ID)+`}`, &tk)
	var seen store.Ticket
	seen = store.Ticket{}
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(tk.ID), &seen)
	if seen.SLA == nil || seen.SLA.RuleID != specific.ID || seen.SLA.FirstResponse.State != "running" || seen.SLA.FirstResponse.TargetMinutes != 30 {
		t.Fatalf("staff view: %+v", seen.SLA)
	}
	var asRequester store.Ticket
	getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID), &asRequester)
	if asRequester.SLA != nil {
		t.Fatal("requester can see the SLA")
	}
	var list struct{ Tickets []store.TicketSummary }
	getJSON(t, agent, ts.URL+"/api/tickets", &list)
	if len(list.Tickets) != 1 || list.Tickets[0].SLA == nil {
		t.Fatalf("list: %+v", list.Tickets)
	}

	// Pending pauses the clock; a staff reply records the first response
	// (the requester's own replies don't).
	do(t, agent, "PATCH", ts.URL+"/api/tickets/"+itoa(tk.ID), `{"status":"pending"}`)
	seen = store.Ticket{}
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(tk.ID), &seen)
	if seen.SLA.Resolution.State != "paused" {
		t.Fatalf("pending: %+v", seen.SLA.Resolution)
	}
	postJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/comments", `{"body":"any news?"}`, nil)
	postJSON(t, agent, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/comments", `{"body":"note","internal":true}`, nil)
	seen = store.Ticket{}
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(tk.ID), &seen)
	if seen.SLA.FirstResponse.State != "paused" {
		t.Fatalf("requester reply or internal note counted as a response: %+v", seen.SLA.FirstResponse)
	}
	do(t, agent, "PATCH", ts.URL+"/api/tickets/"+itoa(tk.ID), `{"status":"in_progress"}`)
	postJSON(t, agent, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/comments", `{"body":"on it"}`, nil)
	seen = store.Ticket{}
	getJSON(t, agent, ts.URL+"/api/tickets/"+itoa(tk.ID), &seen)
	if seen.SLA.FirstResponse.State != "met" {
		t.Fatalf("first response: %+v", seen.SLA.FirstResponse)
	}

	// Alerts: at 80% of the 2h resolution target a warning, past it a breach,
	// each once, to admins only.
	adminNotes := func() []store.Notification {
		var ns struct{ Items []store.Notification }
		getJSON(t, admin, ts.URL+"/api/notifications", &ns)
		var out []store.Notification
		for _, n := range ns.Items {
			if n.Kind == "sla" {
				out = append(out, n)
			}
		}
		return out
	}
	now := time.Now()
	srv.CheckSLAs(t.Context(), now.Add(96*time.Minute))
	srv.CheckSLAs(t.Context(), now.Add(97*time.Minute))
	if ns := adminNotes(); len(ns) != 1 || ns[0].Summary != "Resolution is due in 24m" || ns[0].Actor != nil {
		t.Fatalf("warning: %+v", ns)
	}
	srv.CheckSLAs(t.Context(), now.Add(121*time.Minute))
	srv.CheckSLAs(t.Context(), now.Add(125*time.Minute))
	if ns := adminNotes(); len(ns) != 2 || ns[0].Summary != "Resolution is overdue (target 2h)" {
		t.Fatalf("breach: %+v", ns)
	}
	var agentNotes struct{ Items []store.Notification }
	getJSON(t, agent, ts.URL+"/api/notifications", &agentNotes)
	for _, n := range agentNotes.Items {
		if n.Kind == "sla" {
			t.Fatal("agent got an SLA alert")
		}
	}

	// A ticket that was already long overdue when a rule appeared is marked
	// without an alert.
	var old store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"Old","description":"d","priority":"low"}`, &old)
	srv.CheckSLAs(t.Context(), now.Add(20*time.Hour)) // catch-all first response is 8h
	if ns := adminNotes(); len(ns) != 2 {
		t.Fatalf("stale breach alerted: %+v", ns)
	}

	// Deleting a rule; deleting a category removes its rules.
	if code := do(t, admin, "DELETE", api+"/"+itoa(byPriority.ID), ""); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	do(t, admin, "DELETE", ts.URL+"/api/admin/categories/"+itoa(hw.ID), "")
	getJSON(t, admin, api, &rules)
	if len(rules) > 2 {
		t.Fatalf("after deletes: %+v", rules)
	}
}
