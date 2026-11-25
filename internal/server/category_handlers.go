package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/store"
)

const (
	maxCategoryName        = 60
	maxCategoryDescription = 200
)

var validColors = map[string]bool{
	"gray": true, "red": true, "orange": true, "amber": true, "green": true,
	"teal": true, "blue": true, "indigo": true, "purple": true, "pink": true,
}

func validateCategory(name, description, color string) string {
	switch {
	case !validColors[color]:
		return "unknown colour"
	case name == "":
		return "name is required"
	case utf8.RuneCountInString(name) > maxCategoryName:
		return "name must be 60 characters or fewer"
	case utf8.RuneCountInString(description) > maxCategoryDescription:
		return "description must be 200 characters or fewer"
	}
	return ""
}

// handleListCategories returns the active categories, for anyone raising a ticket.
func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.ListCategories(r.Context(), false)
	if err != nil {
		s.log.Error("list categories", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	templates, err := s.store.ListTemplates(r.Context())
	if err != nil {
		s.log.Error("list templates", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	byID := map[int64]*store.TicketTemplate{}
	for i := range templates {
		templates[i].CategoryCount = 0 // an admin concern
		byID[templates[i].ID] = &templates[i]
	}
	// Ticket counts are an admin concern. The template shapes the new-ticket form.
	type publicCategory struct {
		ID          int64                 `json:"id"`
		Name        string                `json:"name"`
		Description string                `json:"description"`
		Color       string                `json:"color"`
		Template    *store.TicketTemplate `json:"template"`
	}
	out := make([]publicCategory, len(cats))
	for i, c := range cats {
		out[i] = publicCategory{ID: c.ID, Name: c.Name, Description: c.Description, Color: c.Color}
		if c.TemplateID != nil {
			out[i].Template = byID[*c.TemplateID]
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAdminListCategories returns every category, including archived ones,
// with how many tickets use each.
func (s *Server) handleAdminListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.ListCategories(r.Context(), true)
	if err != nil {
		s.log.Error("list categories", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
		TemplateID  int64  `json:"templateId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.checkTemplateID(w, r, req.TemplateID) {
		return
	}
	name, desc := strings.TrimSpace(req.Name), strings.TrimSpace(req.Description)
	if req.Color == "" {
		req.Color = "gray"
	}
	if msg := validateCategory(name, desc, req.Color); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	c, err := s.store.CreateCategory(r.Context(), name, desc, req.Color)
	if store.IsUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a category with that name already exists")
		return
	} else if err != nil {
		s.log.Error("create category", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if req.TemplateID != 0 {
		if err := s.store.SetCategoryTemplate(r.Context(), c.ID, req.TemplateID); err != nil {
			s.log.Error("link template", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		c.TemplateID = &req.TemplateID
	}
	writeJSON(w, http.StatusCreated, c)
}

// handleUpdateCategory applies a partial update: any of name, description, color, archived.
func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Color       *string `json:"color"`
		Archived    *bool   `json:"archived"`
		// TemplateID: absent leaves it; null or 0 unlinks; an id links.
		TemplateID json.RawMessage `json:"templateId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	c, err := s.store.CategoryByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "category not found")
		return
	} else if err != nil {
		s.log.Error("get category", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if req.Name != nil {
		c.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		c.Description = strings.TrimSpace(*req.Description)
	}
	if req.Color != nil {
		c.Color = *req.Color
	}
	if req.Archived != nil {
		c.Archived = *req.Archived
	}
	if msg := validateCategory(c.Name, c.Description, c.Color); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	var templateID *int64
	if len(req.TemplateID) > 0 {
		var tid int64
		if string(req.TemplateID) != "null" {
			if err := json.Unmarshal(req.TemplateID, &tid); err != nil {
				writeError(w, http.StatusBadRequest, "invalid template")
				return
			}
		}
		if !s.checkTemplateID(w, r, tid) {
			return
		}
		templateID = &tid
	}
	c, err = s.store.UpdateCategory(r.Context(), id, c.Name, c.Description, c.Color, c.Archived)
	if store.IsUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a category with that name already exists")
		return
	} else if err != nil {
		s.log.Error("update category", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if templateID != nil {
		if err := s.store.SetCategoryTemplate(r.Context(), id, *templateID); err != nil {
			s.log.Error("link template", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if c, err = s.store.CategoryByID(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	writeJSON(w, http.StatusOK, c)
}

// checkTemplateID makes sure a template being linked exists (0 is "none").
func (s *Server) checkTemplateID(w http.ResponseWriter, r *http.Request, id int64) bool {
	if id == 0 {
		return true
	}
	if _, err := s.store.TemplateByID(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "template not found")
		return false
	} else if err != nil {
		s.log.Error("get template", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return false
	}
	return true
}

func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := s.store.DeleteCategory(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "category not found")
	case errors.Is(err, store.ErrCategoryInUse):
		writeError(w, http.StatusConflict, "this category is used by tickets; archive it instead")
	case err != nil:
		s.log.Error("delete category", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
