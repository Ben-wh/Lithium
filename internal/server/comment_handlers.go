package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/store"
)

const maxCommentLen = 10000

// visibleTicket loads a ticket the current user may see, writing a 404 (not
// 403, so ticket numbers can't be probed) if it doesn't exist or isn't theirs.
func (s *Server) visibleTicket(w http.ResponseWriter, r *http.Request) (store.Ticket, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return store.Ticket{}, false
	}
	t, err := s.store.TicketByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "ticket not found")
		return store.Ticket{}, false
	} else if err != nil {
		s.log.Error("get ticket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return store.Ticket{}, false
	}
	ok, err = s.userCanView(r.Context(), userFrom(r.Context()), t)
	if err != nil {
		s.log.Error("check access", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return store.Ticket{}, false
	}
	if !ok {
		writeError(w, http.StatusNotFound, "ticket not found")
		return store.Ticket{}, false
	}
	return t, true
}

func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	t, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	comments, err := s.store.ListComments(r.Context(), t.ID, isStaff(userFrom(r.Context())))
	if err != nil {
		s.log.Error("list comments", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, comments)
}

// handleAddComment posts a reply. Anyone who can see the ticket can reply;
// staff can also post internal notes, which the requester never sees.
func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request) {
	t, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	var req struct {
		Body     string `json:"body"`
		Internal bool   `json:"internal"`
	}
	if !decodeJSONLimit(w, r, &req, 64<<10) {
		return
	}
	author := userFrom(r.Context())
	if req.Internal && !isStaff(author) {
		writeError(w, http.StatusForbidden, "only staff can add internal notes")
		return
	}
	body := strings.TrimSpace(req.Body)
	switch {
	case body == "":
		writeError(w, http.StatusBadRequest, "write a reply first")
		return
	case utf8.RuneCountInString(body) > maxCommentLen:
		writeError(w, http.StatusBadRequest, "replies must be 10,000 characters or fewer")
		return
	}
	if !isStaff(author) {
		body = stripGroupMentions(body) // groups are for staff
		allowed, err := s.mentionsAllowed(r.Context())
		if err != nil {
			s.log.Error("get settings", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !allowed {
			body = stripMentions(body)
		}
	}
	c, err := s.store.AddComment(r.Context(), t.ID, author.ID, body, req.Internal)
	if err != nil {
		s.log.Error("add comment", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.notifyComment(r.Context(), t, author, c)
	writeJSON(w, http.StatusCreated, c)
}

// optionalID decodes a field that may be absent (set=false), null (0) or an id.
func optionalID(raw json.RawMessage) (set bool, id int64, err error) {
	if len(raw) == 0 {
		return false, 0, nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true, 0, nil
	}
	err = json.Unmarshal(raw, &id)
	return true, id, err
}

// handleUpdateTicket lets staff change status, priority, category and
// assignee. Each change is recorded in the conversation as a Lithium update.
func (s *Server) handleUpdateTicket(w http.ResponseWriter, r *http.Request) {
	current, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	id := current.ID
	var req struct {
		Status     *string         `json:"status"`
		Priority   *string         `json:"priority"`
		CategoryID json.RawMessage `json:"categoryId"`
		AssigneeID json.RawMessage `json:"assigneeId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	u := store.TicketUpdate{Status: req.Status, Priority: req.Priority}
	var err error
	if u.SetCategory, u.CategoryID, err = optionalID(req.CategoryID); err != nil {
		writeError(w, http.StatusBadRequest, "categoryId must be a number or null")
		return
	}
	if u.SetAssignee, u.AssigneeID, err = optionalID(req.AssigneeID); err != nil {
		writeError(w, http.StatusBadRequest, "assigneeId must be a number or null")
		return
	}

	if u.Status != nil && !validStatuses[*u.Status] {
		writeError(w, http.StatusBadRequest, "unknown status")
		return
	}
	if u.Priority != nil && !validPriorities[*u.Priority] {
		writeError(w, http.StatusBadRequest, "unknown priority")
		return
	}
	if u.SetCategory && u.CategoryID != 0 {
		c, err := s.store.CategoryByID(r.Context(), u.CategoryID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && c.Archived) {
			writeError(w, http.StatusBadRequest, "category not found")
			return
		} else if err != nil {
			s.log.Error("lookup category", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if u.SetAssignee && u.AssigneeID != 0 && (current.Assignee == nil || current.Assignee.ID != u.AssigneeID) {
		a, err := s.store.UserByID(r.Context(), u.AssigneeID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && !isStaff(a)) {
			writeError(w, http.StatusBadRequest, "tickets can only be assigned to admins or agents")
			return
		} else if err != nil {
			s.log.Error("lookup assignee", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		// With agent groups on, only agents whose groups cover the ticket.
		if ok, err := s.canBeAssigned(r.Context(), a, current); err != nil {
			s.log.Error("check access", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		} else if !ok {
			writeError(w, http.StatusBadRequest, a.Name+"'s groups don't cover this ticket's category")
			return
		}
	}

	actor := userFrom(r.Context())
	changes, err := s.store.UpdateTicket(r.Context(), id, actor.ID, u)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	} else if err != nil {
		s.log.Error("update ticket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	t, err := s.store.TicketByID(r.Context(), id)
	if err != nil {
		s.log.Error("get ticket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(changes) > 0 {
		s.log.Info("ticket updated", "ticket_id", id, "changes", len(changes), "by", actor.ID)
		s.notifyUpdate(r.Context(), t, actor, changes)
	}
	writeJSON(w, http.StatusOK, t)
}
