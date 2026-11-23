package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lithium/lithium/internal/store"
)

const maxTreeNodes = 500

// handleTicketLinks returns a ticket's parent, children and incident root (staff only).
func (s *Server) handleTicketLinks(w http.ResponseWriter, r *http.Request) {
	t, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	links, err := s.store.Links(r.Context(), t.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	} else if err != nil {
		s.log.Error("ticket links", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	settings, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !settings.IncidentTree {
		links.IncidentRootID = 0
	}
	// Leave out linked tickets this agent's groups don't cover.
	a, err := s.accessFor(r.Context(), userFrom(r.Context()))
	if err != nil {
		s.log.Error("check access", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if links.Parent != nil && !a.canViewLink(*links.Parent) {
		links.Parent = nil
	}
	visible := links.Children[:0]
	for _, c := range links.Children {
		if a.canViewLink(c) {
			visible = append(visible, c)
		}
	}
	links.Children = visible
	writeJSON(w, http.StatusOK, links)
}

// handleSetParent links a ticket under another ({"parentId": n}) or unlinks
// it ({"parentId": null}). Staff only.
func (s *Server) handleSetParent(w http.ResponseWriter, r *http.Request) {
	t, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	id := t.ID
	var req struct {
		ParentID json.RawMessage `json:"parentId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	set, parentID, err := optionalID(req.ParentID)
	if err != nil || !set {
		writeError(w, http.StatusBadRequest, "parentId must be a ticket number or null")
		return
	}
	// Agents can only link to tickets they can see.
	if parentID != 0 {
		parent, err := s.store.TicketByID(r.Context(), parentID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "ticket not found")
			return
		} else if err != nil {
			s.log.Error("get ticket", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if ok, err := s.userCanView(r.Context(), userFrom(r.Context()), parent); err != nil || !ok {
			writeError(w, http.StatusNotFound, "ticket not found")
			return
		}
	}
	err = s.store.SetParent(r.Context(), id, parentID, userFrom(r.Context()).ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "ticket not found")
	case errors.Is(err, store.ErrLinkSelf), errors.Is(err, store.ErrLinkCycle):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		s.log.Error("set parent", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	default:
		s.handleTicketLinks(w, r)
	}
}

// handleIncidentTree returns every ticket linked into the same tree, for
// drawing the incident diagram. Only for trees topped by an urgent ticket,
// and only while the feature is turned on.
func (s *Server) handleIncidentTree(w http.ResponseWriter, r *http.Request) {
	t, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	id := t.ID
	settings, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !settings.IncidentTree {
		writeError(w, http.StatusNotFound, "incident trees are turned off")
		return
	}
	nodes, err := s.store.Tree(r.Context(), id, maxTreeNodes)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	} else if err != nil {
		s.log.Error("incident tree", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if nodes[0].Priority != "urgent" {
		writeError(w, http.StatusNotFound, "this ticket isn't part of an incident (the top ticket must be urgent)")
		return
	}
	// Tickets outside this agent's groups keep their place in the diagram
	// but show nothing about themselves.
	a, err := s.accessFor(r.Context(), userFrom(r.Context()))
	if err != nil {
		s.log.Error("check access", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	for i, n := range nodes {
		if !a.canViewLink(n) {
			nodes[i] = store.TicketLink{ID: n.ID, ParentID: n.ParentID, Priority: n.Priority, Restricted: true}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rootId":    nodes[0].ID,
		"nodes":     nodes,
		"truncated": len(nodes) >= maxTreeNodes,
	})
}

// handleAssignable lists the staff who can be assigned a ticket: everyone
// while agent groups are off, otherwise admins and agents whose groups cover
// the ticket's category.
func (s *Server) handleAssignable(w http.ResponseWriter, r *http.Request) {
	t, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	staff, err := s.store.SearchUsers(r.Context(), "", 500, true)
	if err != nil {
		s.log.Error("assignable", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]store.UserRef, 0, len(staff))
	for _, ref := range staff {
		u, err := s.store.UserByID(r.Context(), ref.ID)
		if err != nil {
			continue
		}
		if ok, err := s.canBeAssigned(r.Context(), u, t); err == nil && ok {
			out = append(out, ref)
		}
	}
	writeJSON(w, http.StatusOK, out)
}
