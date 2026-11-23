package server

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/store"
)

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.store.ListGroups(r.Context())
	if err != nil {
		s.log.Error("list groups", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

type groupRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	MemberIDs   []int64 `json:"memberIds"`
	CategoryIDs []int64 `json:"categoryIds"`
}

// validate checks a group's details. Members must be agents (admins always
// see everything, so membership wouldn't mean anything for them).
func (s *Server) validateGroup(r *http.Request, req *groupRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	switch {
	case req.Name == "":
		return "name is required"
	case utf8.RuneCountInString(req.Name) > 60:
		return "name must be 60 characters or fewer"
	case utf8.RuneCountInString(req.Description) > 200:
		return "description must be 200 characters or fewer"
	}
	for _, id := range req.MemberIDs {
		u, err := s.store.UserByID(r.Context(), id)
		if err != nil || u.Role != "agent" {
			return "group members must be agents"
		}
	}
	for _, id := range req.CategoryIDs {
		if _, err := s.store.CategoryByID(r.Context(), id); err != nil {
			return "category not found"
		}
	}
	return ""
}

func (s *Server) saveGroup(w http.ResponseWriter, r *http.Request, id int64) {
	var req groupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := s.validateGroup(r, &req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	g, err := s.store.SaveGroup(r.Context(), id, req.Name, req.Description, req.MemberIDs, req.CategoryIDs)
	switch {
	case store.IsUniqueViolation(err):
		writeError(w, http.StatusConflict, "a group with that name already exists")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "group not found")
	case err != nil:
		s.log.Error("save group", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	case id == 0:
		writeJSON(w, http.StatusCreated, g)
	default:
		writeJSON(w, http.StatusOK, g)
	}
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) { s.saveGroup(w, r, 0) }

func (s *Server) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	if id, ok := pathID(w, r); ok {
		s.saveGroup(w, r, id)
	}
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := s.store.DeleteGroup(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "group not found")
		return
	} else if err != nil {
		s.log.Error("delete group", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
