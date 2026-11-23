package server

import (
	"net/http"

	"github.com/lithium/lithium/internal/store"
)

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	items, err := s.store.ListNotifications(r.Context(), user.ID, 30)
	if err != nil {
		s.log.Error("list notifications", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	unread, err := s.store.UnreadNotificationCount(r.Context(), user.ID)
	if err != nil {
		s.log.Error("count notifications", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "unread": unread})
}

// handleUnreadCount is cheap enough to poll.
func (s *Server) handleUnreadCount(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.UnreadNotificationCount(r.Context(), userFrom(r.Context()).ID)
	if err != nil {
		s.log.Error("count notifications", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread": n})
}

// handleMarkRead marks the user's notifications read: specific ids, all
// about one ticket, or (with neither) all of them.
func (s *Server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs      []int64 `json:"ids"`
		TicketID int64   `json:"ticketId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user := userFrom(r.Context())
	if err := s.store.MarkNotificationsRead(r.Context(), user.ID, req.TicketID, req.IDs); err != nil {
		s.log.Error("mark read", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.handleUnreadCount(w, r)
}

// handleMentionable lists who can be @mentioned on a ticket: people who can
// see it. Internal notes can only mention staff. Emails are only shown to staff.
func (s *Server) handleMentionable(w http.ResponseWriter, r *http.Request) {
	t, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	viewer := userFrom(r.Context())
	internal := r.URL.Query().Get("internal") == "1" && isStaff(viewer)
	if !isStaff(viewer) {
		allowed, err := s.mentionsAllowed(r.Context())
		if err != nil {
			s.log.Error("get settings", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !allowed {
			writeJSON(w, http.StatusOK, []store.UserRef{})
			return
		}
	}

	staff, err := s.store.SearchUsers(r.Context(), "", 200, true)
	if err != nil {
		s.log.Error("mentionable", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	people := staff
	if !internal && !containsUser(people, t.Requester.ID) {
		people = append([]store.UserRef{t.Requester}, people...)
	}
	out := make([]store.UserRef, 0, len(people))
	for _, p := range people {
		if p.ID == viewer.ID {
			continue
		}
		// Only offer people who can actually see this ticket.
		if u, err := s.store.UserByID(r.Context(), p.ID); err != nil || !s.canSee(r.Context(), u, t, internal) {
			continue
		}
		if !isStaff(viewer) {
			p.Email = ""
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}

func containsUser(list []store.UserRef, id int64) bool {
	for _, u := range list {
		if u.ID == id {
			return true
		}
	}
	return false
}

type mentionableGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Members who can see this ticket (they're who a mention notifies).
	MemberCount int `json:"memberCount"`
}

// handleMentionableGroups lists the agent groups staff can @mention on a
// ticket, with how many members would be notified.
func (s *Server) handleMentionableGroups(w http.ResponseWriter, r *http.Request) {
	t, ok := s.visibleTicket(w, r)
	if !ok {
		return
	}
	internal := r.URL.Query().Get("internal") == "1"
	viewer := userFrom(r.Context())
	groups, err := s.store.ListGroups(r.Context())
	if err != nil {
		s.log.Error("mentionable groups", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]mentionableGroup, 0, len(groups))
	for _, g := range groups {
		n := 0
		for _, m := range g.Members {
			if m.ID == viewer.ID {
				continue
			}
			if u, err := s.store.UserByID(r.Context(), m.ID); err == nil && s.canSee(r.Context(), u, t, internal) {
				n++
			}
		}
		out = append(out, mentionableGroup{ID: g.ID, Name: g.Name, MemberCount: n})
	}
	writeJSON(w, http.StatusOK, out)
}
