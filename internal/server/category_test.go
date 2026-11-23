package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func do(t *testing.T, c *http.Client, method, url, body string) int {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestCategories(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	// Create.
	var hw, sw store.Category
	if code := postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":" Hardware ","description":"Laptops, phones"}`, &hw); code != http.StatusCreated || hw.Name != "Hardware" {
		t.Fatalf("create: %d %+v", code, hw)
	}
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Software","color":"blue"}`, &sw)
	if hw.Color != "gray" || sw.Color != "blue" {
		t.Fatalf("colours: hardware %q (want default gray), software %q", hw.Color, sw.Color)
	}
	for body, want := range map[string]int{
		`{"name":"hardware"}`:                        http.StatusConflict, // names are case-insensitive
		`{"name":"  "}`:                              http.StatusBadRequest,
		`{"name":"Pretty","color":"#ff0000"}`:        http.StatusBadRequest,
		`{"name":"` + strings.Repeat("x", 61) + `"}`: http.StatusBadRequest,
	} {
		if code := postJSON(t, admin, ts.URL+"/api/admin/categories", body, nil); code != want {
			t.Fatalf("%s: got %d, want %d", body, code, want)
		}
	}

	// Only admins manage categories; everyone can list the active ones.
	if code := postJSON(t, agent, ts.URL+"/api/admin/categories", `{"name":"X"}`, nil); code != http.StatusForbidden {
		t.Fatalf("agent create: %d", code)
	}
	var public []store.Category
	if code := getJSON(t, alice, ts.URL+"/api/categories", &public); code != 200 || len(public) != 2 {
		t.Fatalf("public list: %d %+v", code, public)
	}

	// Raise tickets with a category.
	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d","categoryId":`+itoa(hw.ID)+`}`, &tk)
	if tk.Category == nil || tk.Category.Name != "Hardware" {
		t.Fatalf("ticket category: %+v", tk.Category)
	}
	if code := postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d","categoryId":999}`, nil); code != http.StatusBadRequest {
		t.Fatalf("unknown category: %d", code)
	}
	var list listResp
	getJSON(t, agent, ts.URL+"/api/tickets?category="+itoa(hw.ID), &list)
	if list.Total != 1 || list.Tickets[0].Category == nil {
		t.Fatalf("category filter: %+v", list)
	}

	// Rename shows through on existing tickets.
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/categories/"+itoa(hw.ID), `{"color":"rainbow"}`); code != http.StatusBadRequest {
		t.Fatalf("bad colour: %d", code)
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/categories/"+itoa(hw.ID), `{"name":"Devices","color":"teal"}`); code != 200 {
		t.Fatalf("rename: %d", code)
	}
	var got store.Ticket
	getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID), &got)
	if got.Category == nil || got.Category.Name != "Devices" || got.Category.Color != "teal" {
		t.Fatalf("renamed category on ticket: %+v", got.Category)
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/categories/"+itoa(hw.ID), `{"name":"software"}`); code != http.StatusConflict {
		t.Fatalf("rename to duplicate: %d", code)
	}

	// Archiving hides it from new tickets but keeps it on old ones.
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/categories/"+itoa(hw.ID), `{"archived":true}`); code != 200 {
		t.Fatalf("archive: %d", code)
	}
	getJSON(t, alice, ts.URL+"/api/categories", &public)
	if len(public) != 1 || public[0].Name != "Software" {
		t.Fatalf("archived category still offered: %+v", public)
	}
	if code := postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d","categoryId":`+itoa(hw.ID)+`}`, nil); code != http.StatusBadRequest {
		t.Fatalf("archived category accepted: %d", code)
	}
	getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID), &got)
	if got.Category == nil {
		t.Fatal("archived category dropped from existing ticket")
	}
	var all []store.Category
	getJSON(t, admin, ts.URL+"/api/admin/categories", &all)
	if len(all) != 2 || all[1].Name != "Devices" || !all[1].Archived || all[1].TicketCount != 1 {
		t.Fatalf("admin list: %+v", all)
	}

	// Delete: refused while in use, allowed when unused.
	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/categories/"+itoa(hw.ID), ""); code != http.StatusConflict {
		t.Fatalf("delete in use: %d", code)
	}
	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/categories/"+itoa(sw.ID), ""); code != http.StatusNoContent {
		t.Fatalf("delete unused: %d", code)
	}
	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/categories/"+itoa(sw.ID), ""); code != http.StatusNotFound {
		t.Fatalf("delete twice: %d", code)
	}
}
