package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lithium/lithium/internal/auth"
	"github.com/lithium/lithium/internal/store"
)

const (
	sessionCookie      = "lithium_session"
	sessionTTL         = 12 * time.Hour
	rememberSessionTTL = 30 * 24 * time.Hour
	// mfaPendingTTL is how long someone has to enter their two-factor code.
	mfaPendingTTL = 10 * time.Minute
	// maxMFAAttempts wrong codes end the pending session.
	maxMFAAttempts = 5
)

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
	rememberKey
)

func userFrom(ctx context.Context) store.User {
	u, _ := ctx.Value(userKey).(store.User)
	return u
}

// sessionFrom returns the hash of the current request's session token.
func sessionFrom(ctx context.Context) string {
	h, _ := ctx.Value(sessionKey).(string)
	return h
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if !s.limiter.allow(ip) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "too many login attempts, try again shortly")
		return
	}

	var req loginRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := s.store.UserByEmail(r.Context(), req.Email)
	hash := user.PasswordHash
	if errors.Is(err, store.ErrNotFound) {
		hash = s.dummyHash
	} else if err != nil {
		s.log.Error("lookup user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	ok, err := auth.VerifyPassword(req.Password, hash)
	if err != nil {
		s.log.Error("verify password", "err", err, "user_id", user.ID)
	}
	// In lockdown only the admin who started it gets in. Everyone else gets
	// the same answer whether or not their password was right.
	if l, err := s.store.Lockdown(r.Context()); err != nil {
		s.log.Error("read lockdown state", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	} else if l.Active && (!ok || user.ID == 0 || user.ID != l.ByID) {
		s.log.Warn("sign-in refused: lockdown", "email", req.Email, "ip", ip)
		writeError(w, http.StatusLocked, lockdownBlockedMsg)
		return
	}
	if !ok || user.ID == 0 {
		writeError(w, http.StatusUnauthorized, "incorrect email or password")
		return
	}

	auditActor(r.Context(), user)
	if user.TwoFactorEnabled {
		auditDetail(r.Context(), "password accepted, waiting for two-factor code")
	}

	// With two-factor on, the session only lets the user enter a code until
	// they do; it gets its real lifetime then.
	pending := user.TwoFactorEnabled
	token, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	expires := time.Now().Add(sessionLifetime(req.Remember))
	if pending {
		expires = time.Now().Add(mfaPendingTTL)
	}
	ua := r.UserAgent()
	if len(ua) > 512 {
		ua = ua[:512]
	}
	if err := s.store.CreateSession(r.Context(), store.NewSession{
		TokenHash: tokenHash, UserID: user.ID, ExpiresAt: expires, UserAgent: ua, IP: ip,
		MFAPending: pending, Remember: req.Remember,
	}); err != nil {
		s.log.Error("create session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.setSessionCookie(w, token, req.Remember && !pending, expires)

	if pending {
		user.TwoFactor = "verify"
		s.log.Info("login password accepted, awaiting two-factor code", "user_id", user.ID, "ip", ip)
	} else {
		if err := s.applyTwoFactorSetupState(r.Context(), &user); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		s.log.Info("login", "user_id", user.ID, "ip", ip)
	}
	writeJSON(w, http.StatusOK, user)
}

func sessionLifetime(remember bool) time.Duration {
	if remember {
		return rememberSessionTTL
	}
	return sessionTTL
}

// setSessionCookie sets the session cookie; persistent cookies outlive the
// browser session, others go when the browser closes.
func (s *Server) setSessionCookie(w http.ResponseWriter, token string, persistent bool, expires time.Time) {
	cookie := &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
	if persistent {
		cookie.Expires = expires
	}
	http.SetCookie(w, cookie)
}

// twoFactorSetupState is "setup" when the workspace requires two-factor and
// the user hasn't set it up yet.
func (s *Server) twoFactorSetupState(ctx context.Context, u store.User) (string, error) {
	if u.TwoFactorEnabled {
		return "", nil
	}
	st, err := s.store.Settings(ctx)
	if err != nil {
		s.log.Error("load settings", "err", err)
		return "", err
	}
	if st.RequireTwoFactor {
		return "setup", nil
	}
	return "", nil
}

// applyTwoFactorSetupState sets u.TwoFactor for a session past the code step,
// and whether this user may skip the setup (the first admin, once).
func (s *Server) applyTwoFactorSetupState(ctx context.Context, u *store.User) error {
	var err error
	if u.TwoFactor, err = s.twoFactorSetupState(ctx, *u); err != nil {
		return err
	}
	if u.TwoFactor == "setup" && u.Role == "admin" {
		if u.CanSkipTwoFactor, err = s.store.CanSkipTwoFactor(ctx, u.ID); err != nil {
			s.log.Error("check two-factor skip", "err", err)
			return err
		}
	}
	return nil
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if sess, err := s.store.SessionByToken(r.Context(), auth.HashToken(c.Value)); err == nil {
			auditActor(r.Context(), sess.User)
		}
		if err := s.store.DeleteSession(r.Context(), auth.HashToken(c.Value)); err != nil {
			s.log.Error("delete session", "err", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, userFrom(r.Context()))
}

// How far through signing in a session must be to reach an endpoint.
type authLevel int

const (
	// levelPending: any session, including one still waiting for its
	// two-factor code. Only for /me and entering the code.
	levelPending authLevel = iota
	// levelSetup: the code has been entered, but the user may still have to
	// change their password or set up two-factor. Only for those endpoints.
	levelSetup
	// levelFull: completely signed in.
	levelFull
)

// requireUser rejects requests without a fully signed-in session and puts
// the user in the request context.
func (s *Server) requireUser(next http.Handler) http.Handler {
	return s.authenticate(next, levelFull)
}

// requireSession also lets through users who must still change their
// password or set up two-factor. Only for the endpoints they need to do that.
func (s *Server) requireSession(next http.Handler) http.Handler {
	return s.authenticate(next, levelSetup)
}

// requirePendingSession also lets through sessions waiting for a two-factor code.
func (s *Server) requirePendingSession(next http.Handler) http.Handler {
	return s.authenticate(next, levelPending)
}

func (s *Server) authenticate(next http.Handler, level authLevel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}
		tokenHash := auth.HashToken(c.Value)
		sess, err := s.store.SessionByToken(r.Context(), tokenHash)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		} else if err != nil {
			s.log.Error("lookup session", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		user := sess.User
		auditActor(r.Context(), user)
		// Lockdown deletes other sessions when it starts; this also catches
		// any that slip through, and signs them out.
		if l, err := s.store.Lockdown(r.Context()); err != nil {
			s.log.Error("read lockdown state", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		} else if l.Active && l.ByID != user.ID {
			if err := s.store.DeleteSession(r.Context(), tokenHash); err != nil {
				s.log.Error("delete session", "err", err)
			}
			s.clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}
		if sess.MFAPending {
			user.TwoFactor = "verify"
			if level > levelPending {
				writeError(w, http.StatusForbidden, "two-factor code required")
				return
			}
		} else if err := s.applyTwoFactorSetupState(r.Context(), &user); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if level == levelFull {
			if user.MustChangePassword {
				writeError(w, http.StatusForbidden, "password change required")
				return
			}
			if user.TwoFactor == "setup" {
				writeError(w, http.StatusForbidden, "two-factor setup required")
				return
			}
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, sessionKey, tokenHash)
		ctx = context.WithValue(ctx, rememberKey, sess.Remember)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// handleChangePassword lets a user set their own password. The current
// password is required unless this is a forced change straight after signing
// in with a password someone else chose.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user := userFrom(r.Context())

	if !user.MustChangePassword {
		ok, err := auth.VerifyPassword(req.CurrentPassword, user.PasswordHash)
		if err != nil || !ok {
			writeError(w, http.StatusBadRequest, "current password is incorrect")
			return
		}
	}
	if msg := validatePassword(req.NewPassword); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if same, _ := auth.VerifyPassword(req.NewPassword, user.PasswordHash); same {
		writeError(w, http.StatusBadRequest, "choose a password different from your current one")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := s.store.SetPassword(r.Context(), user.ID, hash, false, user.ID, sessionFrom(r.Context())); err != nil {
		s.log.Error("change password", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("password changed", "user_id", user.ID, "forced", user.MustChangePassword)
	user.MustChangePassword = false
	writeJSON(w, http.StatusOK, user)
}
