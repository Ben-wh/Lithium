package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// ticketUpdateRequest is the change a single or bulk update asks for. Absent
// fields are left alone; null clears the category or assignee.
type ticketUpdateRequest struct {
	Status     *string         `json:"status"`
	Priority   *string         `json:"priority"`
	CategoryID json.RawMessage `json:"categoryId"`
	AssigneeID json.RawMessage `json:"assigneeId"`
}

// toUpdate checks the request's own values (not anything that depends on a
// particular ticket); a non-empty message is a 400 for the user.
func (req ticketUpdateRequest) toUpdate() (store.TicketUpdate, string) {
	u := store.TicketUpdate{Status: req.Status, Priority: req.Priority}
	var err error
	if u.SetCategory, u.CategoryID, err = optionalID(req.CategoryID); err != nil {
		return u, "categoryId must be a number or null"
	}
	if u.SetAssignee, u.AssigneeID, err = optionalID(req.AssigneeID); err != nil {
		return u, "assigneeId must be a number or null"
	}
	if u.Status != nil && !validStatuses[*u.Status] {
		return u, "unknown status"
	}
	if u.Priority != nil && !validPriorities[*u.Priority] {
		return u, "unknown priority"
	}
	return u, ""
}

// checkTicketUpdate checks u against one ticket: the category must exist and
// the assignee must be staff whose groups cover the ticket. A non-empty
// message is the reason it can't be applied.
func (s *Server) checkTicketUpdate(ctx context.Context, current store.Ticket, u store.TicketUpdate) (string, error) {
	if u.SetCategory && u.CategoryID != 0 {
		c, err := s.store.CategoryByID(ctx, u.CategoryID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && c.Archived) {
			return "category not found", nil
		} else if err != nil {
			return "", fmt.Errorf("lookup category: %w", err)
		}
	}
	if u.SetAssignee && u.AssigneeID != 0 && (current.Assignee == nil || current.Assignee.ID != u.AssigneeID) {
		a, err := s.store.UserByID(ctx, u.AssigneeID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && !isStaff(a)) {
			return "tickets can only be assigned to admins or agents", nil
		} else if err != nil {
			return "", fmt.Errorf("lookup assignee: %w", err)
		}
		// With agent groups on, only agents whose groups cover the ticket.
		if ok, err := s.canBeAssigned(ctx, a, current); err != nil {
			return "", fmt.Errorf("check access: %w", err)
		} else if !ok {
			return a.Name + "'s groups don't cover this ticket's category", nil
		}
	}
	return "", nil
}

// applyTicketUpdate saves u, records it in the conversation and notifies
// people, returning the updated ticket.
func (s *Server) applyTicketUpdate(ctx context.Context, id int64, actor store.User, u store.TicketUpdate) (store.Ticket, error) {
	changes, err := s.store.UpdateTicket(ctx, id, actor.ID, u)
	if err != nil {
		return store.Ticket{}, err
	}
	t, err := s.store.TicketByID(ctx, id)
	if err != nil {
		return store.Ticket{}, err
	}
	if len(changes) > 0 {
		s.log.Info("ticket updated", "ticket_id", id, "changes", len(changes), "by", actor.ID)
		s.notifyUpdate(ctx, t, actor, changes)
	}
	return t, nil
}

// handleUpdateTicket lets staff change status, priority, category and
// assignee. Each change is recorded in the conversation as a Lithium update.
func (s *Server) handleUpdateTicket(w http.ResponseWriter, r *http.Request) {
	current, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	var req ticketUpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	u, msg := req.toUpdate()
	if msg == "" {
		var err error
		if msg, err = s.checkTicketUpdate(r.Context(), current, u); err != nil {
			s.log.Error("check ticket update", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	t, err := s.applyTicketUpdate(r.Context(), current.ID, userFrom(r.Context()), u)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	} else if err != nil {
		s.log.Error("update ticket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

const maxBulkTickets = 100

type bulkSkip struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// handleBulkUpdateTickets applies one change (status, priority, category
// and/or assignee) to several tickets. Each ticket is checked as if updated on
// its own; any that can't take the change are skipped with a reason and the
// rest still update.
func (s *Server) handleBulkUpdateTickets(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
		ticketUpdateRequest
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	switch {
	case len(req.IDs) == 0:
		writeError(w, http.StatusBadRequest, "choose at least one ticket")
		return
	case len(req.IDs) > maxBulkTickets:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("you can change up to %d tickets at once", maxBulkTickets))
		return
	}
	u, msg := req.toUpdate()
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if u.Status == nil && u.Priority == nil && !u.SetCategory && !u.SetAssignee {
		writeError(w, http.StatusBadRequest, "choose a change to make")
		return
	}

	ctx := r.Context()
	actor := userFrom(ctx)
	res := struct {
		Updated int        `json:"updated"`
		Skipped []bulkSkip `json:"skipped"`
	}{Skipped: []bulkSkip{}}
	seen := map[int64]bool{}
	for _, id := range req.IDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		t, err := s.store.TicketByID(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			res.Skipped = append(res.Skipped, bulkSkip{id, "ticket not found"})
			continue
		} else if err != nil {
			s.log.Error("get ticket", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		// Tickets outside an agent's groups look the same as missing ones.
		if ok, err := s.userCanView(ctx, actor, t); err != nil {
			s.log.Error("check access", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		} else if !ok {
			res.Skipped = append(res.Skipped, bulkSkip{id, "ticket not found"})
			continue
		}
		msg, err := s.checkTicketUpdate(ctx, t, u)
		if err != nil {
			s.log.Error("check ticket update", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if msg != "" {
			res.Skipped = append(res.Skipped, bulkSkip{id, msg})
			continue
		}
		if _, err := s.applyTicketUpdate(ctx, id, actor, u); err != nil {
			s.log.Error("bulk update ticket", "ticket_id", id, "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		res.Updated++
	}
	writeJSON(w, http.StatusOK, res)
}

// handleReopenTicket moves a resolved, closed or raised-in-error ticket back
// into the queue.
// Staff can always do it; the requester can too, e.g. when a fix didn't hold,
// unless an admin has turned that off (AllowRequesterReopen). It
// goes back to its assignee (In progress) if it has one, otherwise to
// Unassigned, and is recorded and notified like any other status change.
func (s *Server) handleReopenTicket(w http.ResponseWriter, r *http.Request) {
	t, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	actor := userFrom(r.Context())
	if !isStaff(actor) {
		if t.Requester.ID != actor.ID {
			writeError(w, http.StatusForbidden, "only the requester or staff can reopen this ticket")
			return
		}
		settings, err := s.store.Settings(r.Context())
		if err != nil {
			s.log.Error("get settings", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !settings.AllowRequesterReopen {
			writeError(w, http.StatusForbidden, "reopening is turned off; reply to ask the team to reopen it")
			return
		}
	}
	if !store.IsDone(t.Status) {
		writeError(w, http.StatusConflict, "this ticket is already open")
		return
	}
	status := "unassigned"
	if t.Assignee != nil {
		status = "in_progress"
	}
	changes, err := s.store.UpdateTicket(r.Context(), t.ID, actor.ID, store.TicketUpdate{Status: &status})
	if err != nil {
		s.log.Error("reopen ticket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	t, err = s.store.TicketByID(r.Context(), t.ID)
	if err != nil {
		s.log.Error("get ticket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("ticket reopened", "ticket_id", t.ID, "by", actor.ID)
	s.notifyUpdate(r.Context(), t, actor, changes)
	writeJSON(w, http.StatusOK, t)
}
