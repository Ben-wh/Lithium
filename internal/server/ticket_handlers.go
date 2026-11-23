package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/store"
)

var validPriorities = map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}

// isStaff reports whether a user works tickets (as opposed to raising them).
func isStaff(u store.User) bool { return u.Role == "admin" || u.Role == "agent" }

// requireStaff allows only signed-in admins and agents.
func (s *Server) requireStaff(next http.Handler) http.Handler {
	return s.requireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isStaff(userFrom(r.Context())) {
			writeError(w, http.StatusForbidden, "staff access required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

type createTicketRequest struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	CategoryID  int64  `json:"categoryId"`
	RequesterID int64  `json:"requesterId"`
}

func (s *Server) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	// Tickets arrive as JSON, or as multipart/form-data when images are
	// attached: a "payload" field holding the same JSON, plus "images" files.
	var req createTicketRequest
	var files []*multipart.FileHeader
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt == "multipart/form-data" {
		r.Body = http.MaxBytesReader(w, r.Body, maxImagesPerTicket*maxImageBytes+1<<20)
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				writeError(w, http.StatusRequestEntityTooLarge, "images are too large")
			} else {
				writeError(w, http.StatusBadRequest, "invalid upload")
			}
			return
		}
		defer r.MultipartForm.RemoveAll()
		if err := json.Unmarshal([]byte(r.FormValue("payload")), &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		files = r.MultipartForm.File["images"]
	} else if !decodeJSONLimit(w, r, &req, 256<<10) { // 20k chars can be ~80KB of UTF-8, plus escaping
		return
	}
	user := userFrom(r.Context())
	req.Subject = strings.TrimSpace(req.Subject)
	req.Description = strings.TrimSpace(req.Description)
	if req.Priority == "" {
		req.Priority = "normal"
	}

	settings, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	switch {
	case req.Subject == "":
		writeError(w, http.StatusBadRequest, "subject is required")
		return
	case utf8.RuneCountInString(req.Subject) > settings.SubjectMaxLength:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("subject must be %d characters or fewer", settings.SubjectMaxLength))
		return
	case req.Description == "":
		writeError(w, http.StatusBadRequest, "description is required")
		return
	case utf8.RuneCountInString(req.Description) > settings.DescriptionMaxLength:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("description must be %s characters or fewer", thousands(settings.DescriptionMaxLength)))
		return
	case !validPriorities[req.Priority]:
		writeError(w, http.StatusBadRequest, "priority must be low, normal, high or urgent")
		return
	}

	// Staff can raise a ticket on someone's behalf; everyone else raises their own.
	requesterID := user.ID
	if isStaff(user) && req.RequesterID != 0 && req.RequesterID != user.ID {
		if _, err := s.store.UserByID(r.Context(), req.RequesterID); errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "requester not found")
			return
		} else if err != nil {
			s.log.Error("lookup requester", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		requesterID = req.RequesterID
	}

	if len(files) > 0 && !settings.AllowImageUploads {
		writeError(w, http.StatusBadRequest, "image uploads are turned off")
		return
	}
	if len(files) > maxImagesPerTicket {
		writeError(w, http.StatusBadRequest, "you can attach up to 5 images")
		return
	}

	// A category may be mandatory. Only enforced while there's an active
	// category to choose, so archiving them all can't block new tickets.
	if req.CategoryID == 0 {
		if settings.RequireCategory {
			if ok, err := s.store.HasActiveCategories(r.Context()); err != nil {
				s.log.Error("check categories", "err", err)
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			} else if ok {
				writeError(w, http.StatusBadRequest, "choose a category")
				return
			}
		}
	}

	// Only active categories can be chosen for new tickets.
	if req.CategoryID != 0 {
		c, err := s.store.CategoryByID(r.Context(), req.CategoryID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && c.Archived) {
			writeError(w, http.StatusBadRequest, "category not found")
			return
		} else if err != nil {
			s.log.Error("lookup category", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	attachments, cleanup, err := s.saveImages(files)
	var userErr userError
	if errors.As(err, &userErr) {
		writeError(w, http.StatusBadRequest, userErr.Error())
		return
	} else if err != nil {
		s.log.Error("save images", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	t, err := s.store.CreateTicket(r.Context(), store.NewTicket{
		Subject:     req.Subject,
		Description: req.Description,
		Priority:    req.Priority,
		CategoryID:  req.CategoryID,
		RequesterID: requesterID,
		CreatedBy:   user.ID,
		Attachments: attachments,
	})
	if err != nil {
		cleanup()
		s.log.Error("create ticket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("ticket created", "ticket_id", t.ID, "requester_id", requesterID, "by", user.ID)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	t, err := s.store.TicketByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	} else if err != nil {
		s.log.Error("get ticket", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Non-staff only see their own tickets. Answer 404, not 403, so ticket
	// numbers can't be probed.
	if ok, err := s.userCanView(r.Context(), userFrom(r.Context()), t); err != nil {
		s.log.Error("check access", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	} else if !ok {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// handleUserDirectory lets staff look people up, e.g. to pick a requester.
func (s *Server) handleUserDirectory(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	users, err := s.store.SearchUsers(r.Context(), qs.Get("q"), 50, qs.Get("staff") == "1")
	if err != nil {
		s.log.Error("search users", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

var validStatuses = map[string]bool{
	"unassigned": true, "in_progress": true, "pending": true, "resolved": true, "closed": true,
}

type ticketList struct {
	Tickets []store.TicketSummary `json:"tickets"`
	Total   int                   `json:"total"`
}

// handleListTickets lists tickets. Staff see everything; everyone else only
// sees tickets they requested.
//
// Query params: status (comma-separated), priority, assignee (me|unassigned),
// q (search), limit (1-100, default 50), offset.
func (s *Server) handleListTickets(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	qs := r.URL.Query()
	f := store.TicketFilter{Query: qs.Get("q"), Limit: 50}

	if v := qs.Get("status"); v != "" {
		for _, st := range strings.Split(v, ",") {
			if !validStatuses[st] {
				writeError(w, http.StatusBadRequest, "unknown status "+strconv.Quote(st))
				return
			}
			f.Statuses = append(f.Statuses, st)
		}
	}
	if v := qs.Get("priority"); v != "" {
		if !validPriorities[v] {
			writeError(w, http.StatusBadRequest, "unknown priority "+strconv.Quote(v))
			return
		}
		f.Priority = v
	}
	switch qs.Get("assignee") {
	case "":
	case "me":
		f.AssigneeID = user.ID
	case "unassigned":
		f.Unassigned = true
	default:
		writeError(w, http.StatusBadRequest, "assignee must be me or unassigned")
		return
	}
	if v := qs.Get("requester"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "requester must be a user id")
			return
		}
		f.RequesterID = id // non-staff are limited to their own tickets below regardless
	}
	if v := qs.Get("category"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "category must be a category id")
			return
		}
		f.CategoryID = id
	}
	if v := qs.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		f.Limit = n
	}
	if v := qs.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset must be 0 or more")
			return
		}
		f.Offset = n
	}
	a, err := s.accessFor(r.Context(), user)
	if err != nil {
		s.log.Error("check access", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	a.scope(&f)

	tickets, total, err := s.store.ListTickets(r.Context(), f)
	if err != nil {
		s.log.Error("list tickets", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, ticketList{Tickets: tickets, Total: total})
}

// handleTicketStats returns dashboard counts for the tickets the user can see.
func (s *Server) handleTicketStats(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	a, err := s.accessFor(r.Context(), user)
	if err != nil {
		s.log.Error("check access", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	var f store.TicketFilter
	a.scope(&f)
	st, err := s.store.TicketStats(r.Context(), user.ID, f)
	if err != nil {
		s.log.Error("ticket stats", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// thousands formats 20000 as "20,000".
func thousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
