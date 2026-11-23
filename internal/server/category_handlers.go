package server

import (
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
	// Ticket counts are an admin concern.
	type publicCategory struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	out := make([]publicCategory, len(cats))
	for i, c := range cats {
		out[i] = publicCategory{c.ID, c.Name, c.Description, c.Color}
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
	}
	if !decodeJSON(w, r, &req) {
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
	c, err = s.store.UpdateCategory(r.Context(), id, c.Name, c.Description, c.Color, c.Archived)
	if store.IsUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a category with that name already exists")
		return
	} else if err != nil {
		s.log.Error("update category", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, c)
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
