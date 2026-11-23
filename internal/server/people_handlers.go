package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/lithium/lithium/internal/store"
)

// handleListPeople returns the User book (staff only). Ticket counts only
// include tickets the viewer may see.
func (s *Server) handleListPeople(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	limit, offset := 50, 0
	if n, err := strconv.Atoi(qs.Get("limit")); err == nil && n >= 1 && n <= 100 {
		limit = n
	}
	if n, err := strconv.Atoi(qs.Get("offset")); err == nil && n >= 0 {
		offset = n
	}
	a, err := s.accessFor(r.Context(), userFrom(r.Context()))
	if err != nil {
		s.log.Error("check access", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	var scope store.TicketFilter
	a.scope(&scope)
	people, total, err := s.store.ListPeople(r.Context(), scope, qs.Get("q"), limit, offset)
	if err != nil {
		s.log.Error("list people", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"people": people, "total": total})
}

// handlePerson returns one person's User book entry (staff only).
func (s *Server) handlePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	a, err := s.accessFor(r.Context(), userFrom(r.Context()))
	if err != nil {
		s.log.Error("check access", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	var scope store.TicketFilter
	a.scope(&scope)
	p, err := s.store.PersonByID(r.Context(), scope, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "person not found")
		return
	} else if err != nil {
		s.log.Error("get person", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, p)
}
