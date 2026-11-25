package server

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/auth"
	"github.com/lithium/lithium/internal/store"
)

// lockdownPhrase must be typed exactly to start a lockdown, so it can't
// happen by a stray click.
const lockdownPhrase = "LOCKDOWN"

const lockdownReasonMax = 200

// lockdownBlockedMsg is the same whether or not the password was right, so
// lockdown can't be used to test passwords.
const lockdownBlockedMsg = "Lithium is in lockdown mode, so signing in is turned off. Only the administrator who started the lockdown can sign in."

func (s *Server) handleGetLockdown(w http.ResponseWriter, r *http.Request) {
	l, err := s.store.Lockdown(r.Context())
	if err != nil {
		s.log.Error("read lockdown state", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// handleStartLockdown signs everyone else out and stops anyone but this
// admin signing in. It needs the typed phrase and the admin's password, on
// top of the confirmations in the app.
func (s *Server) handleStartLockdown(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	var req struct {
		Confirm  string `json:"confirm"`
		Password string `json:"password"`
		Reason   string `json:"reason"`
		// Code is an authenticator or recovery code, needed when the admin
		// has two-factor on.
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Confirm != lockdownPhrase {
		writeError(w, http.StatusBadRequest, "type "+lockdownPhrase+" to confirm")
		return
	}
	if ok, err := auth.VerifyPassword(req.Password, user.PasswordHash); err != nil || !ok {
		writeError(w, http.StatusBadRequest, "password is incorrect")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if utf8.RuneCountInString(reason) > lockdownReasonMax {
		writeError(w, http.StatusBadRequest, "keep the reason under 200 characters")
		return
	}
	if reason != "" {
		auditDetail(r.Context(), "reason: %s", reason)
	}
	if cur, err := s.store.Lockdown(r.Context()); err != nil {
		s.log.Error("read lockdown state", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	} else if cur.Active {
		writeError(w, http.StatusConflict, "lockdown is already on")
		return
	}
	// Two-factor last, once everything else is fine, so a good code isn't
	// used up by an unrelated mistake.
	if user.TwoFactorEnabled {
		if strings.TrimSpace(req.Code) == "" {
			writeError(w, http.StatusBadRequest, "enter the code from your authenticator app")
			return
		}
		if !s.mfaLimiter.allow(s.clientIP(r)) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "too many attempts, try again shortly")
			return
		}
		ok, err := s.checkSecondFactor(r.Context(), user, req.Code)
		if err != nil {
			s.log.Error("check two-factor code", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !ok {
			auditDetail(r.Context(), "wrong two-factor code")
			writeError(w, http.StatusBadRequest, "that two-factor code isn't right")
			return
		}
		auditDetail(r.Context(), "two-factor confirmed")
	}

	ended, err := s.store.StartLockdown(r.Context(), user, reason, sessionFrom(r.Context()))
	if errors.Is(err, store.ErrLockdownActive) {
		writeError(w, http.StatusConflict, "lockdown is already on")
		return
	} else if err != nil {
		s.log.Error("start lockdown", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	auditDetail(r.Context(), "%d other sessions signed out", ended)
	s.log.Warn("LOCKDOWN STARTED: everyone else signed out", "by", user.ID, "sessions_ended", ended, "reason", reason)
	s.handleGetLockdown(w, r)
}

func (s *Server) handleEndLockdown(w http.ResponseWriter, r *http.Request) {
	was, err := s.store.EndLockdown(r.Context())
	if err != nil {
		s.log.Error("end lockdown", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !was {
		writeError(w, http.StatusConflict, "lockdown isn't on")
		return
	}
	s.log.Warn("lockdown ended", "by", userFrom(r.Context()).ID)
	s.handleGetLockdown(w, r)
}
