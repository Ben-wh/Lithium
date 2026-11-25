package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

const laptopTemplate = `{
	"name": "Hardware request",
	"intro": "Tell us what you need.",
	"descriptionMode": "optional",
	"defaultPriority": "low",
	"fields": [
		{"label": "Device", "type": "select", "required": true, "options": ["Laptop", "Monitor", " Laptop ", ""]},
		{"label": "Quantity", "type": "number", "required": true},
		{"label": "Needed by", "type": "date"},
		{"label": "Manager approved", "type": "checkbox", "required": true},
		{"label": "Notes", "type": "textarea"}
	]
}`

func TestTicketTemplates(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")

	// Only admins manage templates.
	if code := postJSON(t, alice, ts.URL+"/api/admin/ticket-templates", laptopTemplate, nil); code != http.StatusForbidden {
		t.Fatalf("user create: %d", code)
	}

	// Bad templates are rejected with a reason.
	for name, body := range map[string]string{
		"no name":       `{"name":" ","fields":[]}`,
		"bad type":      `{"name":"X","fields":[{"label":"A","type":"colour"}]}`,
		"no label":      `{"name":"X","fields":[{"label":"","type":"text"}]}`,
		"same label":    `{"name":"X","fields":[{"label":"A","type":"text"},{"label":"a","type":"text"}]}`,
		"one choice":    `{"name":"X","fields":[{"label":"A","type":"select","options":["only"]}]}`,
		"bad mode":      `{"name":"X","descriptionMode":"sometimes"}`,
		"bad priority":  `{"name":"X","defaultPriority":"asap"}`,
		"hidden, empty": `{"name":"X","descriptionMode":"hidden","fields":[]}`,
	} {
		if code := postJSON(t, admin, ts.URL+"/api/admin/ticket-templates", body, nil); code != http.StatusBadRequest {
			t.Fatalf("%s: got %d", name, code)
		}
	}

	var tpl store.TicketTemplate
	if code := postJSON(t, admin, ts.URL+"/api/admin/ticket-templates", laptopTemplate, &tpl); code != http.StatusCreated {
		t.Fatalf("create: %d", code)
	}
	if len(tpl.Fields) != 5 || tpl.Fields[0].ID == "" || len(tpl.Fields[0].Options) != 2 {
		t.Fatalf("saved template: %+v", tpl)
	}
	if code := postJSON(t, admin, ts.URL+"/api/admin/ticket-templates", laptopTemplate, nil); code != http.StatusConflict {
		t.Fatalf("duplicate name: %d", code)
	}

	// Link it to a new category; linking a missing template fails.
	var cat store.Category
	if code := postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware","templateId":99}`, nil); code != http.StatusBadRequest {
		t.Fatalf("missing template: %d", code)
	}
	if code := postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Hardware","templateId":`+itoa(tpl.ID)+`}`, &cat); code != http.StatusCreated || cat.TemplateID == nil {
		t.Fatalf("category: %d %+v", code, cat)
	}

	// Everyone sees the template with the category, to build the form.
	var cats []struct {
		ID       int64
		Template *store.TicketTemplate
	}
	getJSON(t, alice, ts.URL+"/api/categories", &cats)
	if len(cats) != 1 || cats[0].Template == nil || cats[0].Template.Name != "Hardware request" || cats[0].Template.CategoryCount != 0 {
		t.Fatalf("public categories: %+v", cats)
	}

	f := tpl.Fields
	answers := func(device, qty, date string, approved bool) string {
		a := `"` + f[0].ID + `":` + device + `,"` + f[1].ID + `":` + qty + `,"` + f[3].ID + `":` + map[bool]string{true: "true", false: "false"}[approved]
		if date != "" {
			a += `,"` + f[2].ID + `":` + date
		}
		return `{"subject":"New laptop","categoryId":` + itoa(cat.ID) + `,"fields":{` + a + `}}`
	}
	for name, body := range map[string]string{
		"missing required": `{"subject":"New laptop","categoryId":` + itoa(cat.ID) + `}`,
		"bad choice":       answers(`"Phone"`, `2`, "", true),
		"not a number":     answers(`"Laptop"`, `"lots"`, "", true),
		"bad date":         answers(`"Laptop"`, `2`, `"next week"`, true),
		"not approved":     answers(`"Laptop"`, `2`, "", false),
	} {
		resp := post(t, alice, ts.URL+"/api/tickets", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: got %d", name, resp.StatusCode)
		}
	}

	// Description is optional for this template, and answers are kept in order.
	var tk store.Ticket
	if code := postJSON(t, alice, ts.URL+"/api/tickets", answers(`"Laptop"`, `"2"`, `"2026-12-01"`, true), &tk); code != http.StatusCreated {
		t.Fatalf("create ticket: %d", code)
	}
	if tk.TemplateName != "Hardware request" || len(tk.Fields) != 4 ||
		tk.Fields[0] != (store.TicketField{Label: "Device", Type: "select", Value: "Laptop"}) ||
		tk.Fields[1].Value != "2" || tk.Fields[2].Value != "2026-12-01" || tk.Fields[3].Value != "Yes" {
		t.Fatalf("ticket fields: %+v", tk)
	}

	// Editing and then deleting the template doesn't change the ticket.
	edited := strings.Replace(laptopTemplate, `"label": "Device"`, `"id": "`+f[0].ID+`", "label": "Equipment"`, 1)
	if code := do(t, admin, "PUT", ts.URL+"/api/admin/ticket-templates/"+itoa(tpl.ID), edited); code != 200 {
		t.Fatalf("edit: %d", code)
	}
	getJSONList := func() []store.TicketTemplate {
		var list []store.TicketTemplate
		getJSON(t, admin, ts.URL+"/api/admin/ticket-templates", &list)
		return list
	}
	again := getJSONList()[0]
	if again.Fields[0].ID != f[0].ID || again.Fields[0].Label != "Equipment" || again.CategoryCount != 1 {
		t.Fatalf("field id should survive edits: %+v", again)
	}
	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/ticket-templates/"+itoa(tpl.ID), ``); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	var got store.Ticket
	getJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID), &got)
	if got.Fields[0].Label != "Device" || got.TemplateName != "Hardware request" {
		t.Fatalf("ticket changed after template edit/delete: %+v", got.Fields)
	}

	// The category is unlinked, so the usual form (description required) is back.
	if code := post(t, alice, ts.URL+"/api/tickets", `{"subject":"x","categoryId":`+itoa(cat.ID)+`}`).StatusCode; code != http.StatusBadRequest {
		t.Fatalf("description should be required again: %d", code)
	}
}

func TestHiddenDescriptionTemplate(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	var tpl store.TicketTemplate
	postJSON(t, admin, ts.URL+"/api/admin/ticket-templates",
		`{"name":"Access","descriptionMode":"hidden","fields":[{"label":"System","type":"text","required":true}]}`, &tpl)
	var cat store.Category
	postJSON(t, admin, ts.URL+"/api/admin/categories", `{"name":"Access"}`, &cat)
	// Link it with an edit this time.
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/categories/"+itoa(cat.ID), `{"templateId":`+itoa(tpl.ID)+`}`); code != 200 {
		t.Fatalf("link: %d", code)
	}
	var tk store.Ticket
	body := `{"subject":"VPN access","description":"ignored","categoryId":` + itoa(cat.ID) + `,"fields":{"` + tpl.Fields[0].ID + `":"VPN"}}`
	if code := postJSON(t, admin, ts.URL+"/api/tickets", body, &tk); code != http.StatusCreated {
		t.Fatalf("create: %d", code)
	}
	if tk.Description != "" || tk.Fields[0].Value != "VPN" {
		t.Fatalf("hidden description: %+v", tk)
	}
	// Unlink with null.
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/categories/"+itoa(cat.ID), `{"templateId":null}`); code != 200 {
		t.Fatalf("unlink: %d", code)
	}
	if cs, _ := getCategories(t, admin, ts.URL); cs[0].TemplateID != nil {
		t.Fatalf("still linked: %+v", cs[0])
	}
}

func getCategories(t *testing.T, c *http.Client, baseURL string) ([]store.Category, int) {
	t.Helper()
	var cs []store.Category
	code := getJSON(t, c, baseURL+"/api/admin/categories", &cs)
	return cs, code
}
