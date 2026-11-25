package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/store"
)

// Limits for templates and the answers people give.
const (
	templateNameMax    = 60
	templateIntroMax   = 1000
	templateFieldsMax  = 25
	fieldLabelMax      = 80
	fieldHelpMax       = 200
	fieldOptionsMax    = 50
	fieldOptionMax     = 80
	answerTextMax      = 500
	answerTextareaMax  = 5000
	answerNumberDigits = 30
)

var fieldTypes = map[string]bool{
	store.FieldText: true, store.FieldTextarea: true, store.FieldNumber: true,
	store.FieldSelect: true, store.FieldCheckbox: true, store.FieldDate: true,
}

var descriptionModes = map[string]bool{
	store.DescriptionRequired: true, store.DescriptionOptional: true, store.DescriptionHidden: true,
}

var fieldIDRe = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

func newFieldID() string {
	b := make([]byte, 5)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// cleanTemplate trims and checks a template from an admin, giving new fields
// an ID. It returns a message for the admin when something's wrong.
func cleanTemplate(t *store.TicketTemplate) string {
	t.Name = strings.TrimSpace(t.Name)
	t.Intro = strings.TrimSpace(t.Intro)
	switch {
	case t.Name == "":
		return "give the template a name"
	case utf8.RuneCountInString(t.Name) > templateNameMax:
		return fmt.Sprintf("the name must be %d characters or fewer", templateNameMax)
	case utf8.RuneCountInString(t.Intro) > templateIntroMax:
		return fmt.Sprintf("the instructions must be %d characters or fewer", templateIntroMax)
	}
	if t.DescriptionMode == "" {
		t.DescriptionMode = store.DescriptionRequired
	}
	if !descriptionModes[t.DescriptionMode] {
		return "the description must be required, optional or hidden"
	}
	if t.DefaultPriority != "" && !validPriorities[t.DefaultPriority] {
		return "the default priority must be low, normal, high or urgent"
	}
	if len(t.Fields) > templateFieldsMax {
		return fmt.Sprintf("a template can have up to %d fields", templateFieldsMax)
	}
	if t.DescriptionMode == store.DescriptionHidden && len(t.Fields) == 0 {
		return "add at least one field, or show the description box"
	}

	ids := map[string]bool{}
	labels := map[string]bool{}
	for i := range t.Fields {
		f := &t.Fields[i]
		f.Label = strings.TrimSpace(f.Label)
		f.Help = strings.TrimSpace(f.Help)
		n := i + 1
		switch {
		case f.Label == "":
			return fmt.Sprintf("field %d needs a label", n)
		case utf8.RuneCountInString(f.Label) > fieldLabelMax:
			return fmt.Sprintf("field %d's label must be %d characters or fewer", n, fieldLabelMax)
		case labels[strings.ToLower(f.Label)]:
			return fmt.Sprintf("two fields are called %q; give each a different label", f.Label)
		case !fieldTypes[f.Type]:
			return fmt.Sprintf("field %q has an unknown type", f.Label)
		case utf8.RuneCountInString(f.Help) > fieldHelpMax:
			return fmt.Sprintf("the help text for %q must be %d characters or fewer", f.Label, fieldHelpMax)
		}
		labels[strings.ToLower(f.Label)] = true

		// Keep IDs across edits so answers stay matched; make new ones as needed.
		if !fieldIDRe.MatchString(f.ID) || ids[f.ID] {
			f.ID = newFieldID()
		}
		ids[f.ID] = true

		if f.Type != store.FieldSelect {
			f.Options = nil
			continue
		}
		seen := map[string]bool{}
		opts := make([]string, 0, len(f.Options))
		for _, o := range f.Options {
			o = strings.TrimSpace(o)
			if o == "" || seen[strings.ToLower(o)] {
				continue
			}
			if utf8.RuneCountInString(o) > fieldOptionMax {
				return fmt.Sprintf("choices for %q must be %d characters or fewer", f.Label, fieldOptionMax)
			}
			seen[strings.ToLower(o)] = true
			opts = append(opts, o)
		}
		if len(opts) < 2 {
			return fmt.Sprintf("give %q at least two choices", f.Label)
		}
		if len(opts) > fieldOptionsMax {
			return fmt.Sprintf("%q can have up to %d choices", f.Label, fieldOptionsMax)
		}
		f.Options = opts
	}
	return ""
}

// answerTemplate checks the answers given for a template's fields and turns
// them into the ticket's copy. answers maps field IDs to JSON values.
func answerTemplate(t store.TicketTemplate, answers map[string]json.RawMessage) ([]store.TicketField, string) {
	out := []store.TicketField{}
	for _, f := range t.Fields {
		raw, given := answers[f.ID]
		var v any
		if given && json.Unmarshal(raw, &v) != nil {
			return nil, fmt.Sprintf("%q has an invalid value", f.Label)
		}
		value := ""
		switch f.Type {
		case store.FieldCheckbox:
			b, _ := v.(bool)
			if f.Required && !b {
				return nil, fmt.Sprintf("tick %q to continue", f.Label)
			}
			value = "No"
			if b {
				value = "Yes"
			}
		case store.FieldNumber:
			switch n := v.(type) {
			case float64:
				value = strconv.FormatFloat(n, 'f', -1, 64)
			case string:
				n = strings.TrimSpace(n)
				if n != "" {
					if _, err := strconv.ParseFloat(n, 64); err != nil || len(n) > answerNumberDigits {
						return nil, fmt.Sprintf("%q must be a number", f.Label)
					}
				}
				value = n
			}
		default:
			s, _ := v.(string)
			value = strings.TrimSpace(s)
			max := answerTextMax
			if f.Type == store.FieldTextarea {
				max = answerTextareaMax
			}
			if utf8.RuneCountInString(value) > max {
				return nil, fmt.Sprintf("%q must be %s characters or fewer", f.Label, thousands(max))
			}
			if value != "" && f.Type == store.FieldSelect && !containsString(f.Options, value) {
				return nil, fmt.Sprintf("choose one of the options for %q", f.Label)
			}
			if value != "" && f.Type == store.FieldDate {
				if _, err := time.Parse("2006-01-02", value); err != nil {
					return nil, fmt.Sprintf("%q must be a date", f.Label)
				}
			}
		}
		if f.Required && value == "" {
			return nil, fmt.Sprintf("%q is required", f.Label)
		}
		// Unanswered optional fields are left off the ticket.
		if value == "" {
			continue
		}
		out = append(out, store.TicketField{Label: f.Label, Type: f.Type, Value: value})
	}
	return out, ""
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	ts, err := s.store.ListTemplates(r.Context())
	if err != nil {
		s.log.Error("list templates", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, ts)
}

// handleSaveTemplate creates (POST) or replaces (PUT /{id}) a template.
func (s *Server) handleSaveTemplate(w http.ResponseWriter, r *http.Request) {
	var t store.TicketTemplate
	if !decodeJSONLimit(w, r, &t, 256<<10) {
		return
	}
	t.ID = 0
	if r.PathValue("id") != "" {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		t.ID = id
	}
	if msg := cleanTemplate(&t); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	saved, err := s.store.SaveTemplate(r.Context(), t)
	if store.IsUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a template with that name already exists")
		return
	} else if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "template not found")
		return
	} else if err != nil {
		s.log.Error("save template", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	status := http.StatusOK
	if t.ID == 0 {
		status = http.StatusCreated
	}
	writeJSON(w, status, saved)
}

func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteTemplate(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "template not found")
		return
	} else if err != nil {
		s.log.Error("delete template", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
