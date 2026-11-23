package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/auth"
	"github.com/lithium/lithium/internal/store"
)

var validRoles = map[string]bool{"admin": true, "agent": true, "user": true}

// requireAdmin allows only signed-in admins.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return s.requireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userFrom(r.Context()).Role != "admin" {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeJSONLimit(w, r, v, 16<<10)
}

func decodeJSONLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "not found")
		return 0, false
	}
	return id, true
}

func validatePassword(pw string) string {
	switch n := utf8.RuneCountInString(pw); {
	case n < 8:
		return "password must be at least 8 characters"
	case len(pw) > 256:
		return "password is too long"
	}
	return ""
}

func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email && len(email) <= 254
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		s.log.Error("list users", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

type createUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	switch {
	case req.Name == "" || utf8.RuneCountInString(req.Name) > 100:
		writeError(w, http.StatusBadRequest, "name is required (max 100 characters)")
		return
	case !validEmail(req.Email):
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	case !validRoles[req.Role]:
		writeError(w, http.StatusBadRequest, "role must be admin, agent or user")
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	user, err := s.store.CreateUser(r.Context(), req.Email, req.Name, hash, req.Role, true)
	if store.IsUniqueViolation(err) {
		writeError(w, http.StatusConflict, "a user with that email already exists")
		return
	} else if err != nil {
		s.log.Error("create user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("user created", "user_id", user.ID, "role", user.Role, "by", userFrom(r.Context()).ID)
	writeJSON(w, http.StatusCreated, user)
}

type setPasswordRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req setPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	admin := userFrom(r.Context())
	// Someone else now knows this password, so the user must replace it,
	// unless an admin is setting their own.
	mustChange := id != admin.ID
	err = s.store.SetPassword(r.Context(), id, hash, mustChange, admin.ID, sessionFrom(r.Context()))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		s.log.Error("reset password", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("password reset by admin", "user_id", id, "by", admin.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListResetRequests(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	reqs, err := s.store.ListResetRequests(r.Context(), status)
	if err != nil {
		s.log.Error("list reset requests", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, reqs)
}

func (s *Server) handleDismissResetRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	err := s.store.DismissResetRequest(r.Context(), id, userFrom(r.Context()).ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no pending request with that id")
		return
	} else if err != nil {
		s.log.Error("dismiss reset request", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRequestPasswordReset is public. It always responds the same way so it
// can't be used to discover which emails have accounts.
func (s *Server) handleRequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if !s.resetLimiter.allow(ip) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "too many requests, try again shortly")
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email := strings.TrimSpace(req.Email)
	if !validEmail(email) {
		writeError(w, http.StatusBadRequest, "enter a valid email address")
		return
	}
	user, err := s.store.UserByEmail(r.Context(), email)
	if err == nil {
		if err := s.store.CreateResetRequest(r.Context(), user.ID, ip); err != nil {
			s.log.Error("create reset request", "err", err)
		} else {
			s.log.Info("password reset requested", "user_id", user.ID, "ip", ip)
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		s.log.Error("lookup user", "err", err)
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleUpdateUser changes a user's role. Admins can't change their own, so
// there's always at least one admin.
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validRoles[req.Role] {
		writeError(w, http.StatusBadRequest, "role must be admin, agent or user")
		return
	}
	admin := userFrom(r.Context())
	if id == admin.ID {
		writeError(w, http.StatusBadRequest, "you can't change your own role")
		return
	}
	u, err := s.store.SetUserRole(r.Context(), id, req.Role, admin.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		s.log.Error("set role", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("role changed", "user_id", id, "role", req.Role, "by", admin.ID)
	writeJSON(w, http.StatusOK, u)
}

// handleDeleteUser deletes an account and the tickets they raised. Admins
// can't delete themselves.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	admin := userFrom(r.Context())
	if id == admin.ID {
		writeError(w, http.StatusBadRequest, "you can't delete your own account")
		return
	}
	res, err := s.store.DeleteUser(r.Context(), id, admin.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		s.log.Error("delete user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// The database is already consistent; leftover files are only wasted space.
	for _, f := range res.AttachmentFiles {
		if err := os.Remove(filepath.Join(s.attachmentDir(), f)); err != nil && !os.IsNotExist(err) {
			s.log.Warn("remove attachment", "err", err)
		}
	}
	if res.AvatarFile != "" {
		if err := os.Remove(filepath.Join(s.avatarDir(), res.AvatarFile)); err != nil && !os.IsNotExist(err) {
			s.log.Warn("remove avatar", "err", err)
		}
	}
	s.log.Info("user deleted", "user_id", id, "tickets_deleted", res.TicketsDeleted, "by", admin.ID)
	writeJSON(w, http.StatusOK, map[string]int{"ticketsDeleted": res.TicketsDeleted})
}
