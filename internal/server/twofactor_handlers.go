package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lithium/lithium/internal/auth"
	"github.com/lithium/lithium/internal/store"
)

const recoveryCodeCount = 10

// checkSecondFactor accepts a 6-digit authenticator code or an unused
// recovery code, and spends it so it can't be used again.
func (s *Server) checkSecondFactor(ctx context.Context, u store.User, code string) (bool, error) {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 32 || !u.TwoFactorEnabled {
		return false, nil
	}
	digits := strings.ReplaceAll(code, " ", "")
	if len(digits) == 6 && strings.Trim(digits, "0123456789") == "" {
		step, ok := auth.VerifyTOTP(u.TOTPSecret, digits, time.Now(), u.TOTPLastStep)
		if !ok {
			return false, nil
		}
		// Atomic, so two requests racing with the same code can't both win.
		return s.store.UseTOTPStep(ctx, u.ID, step)
	}
	return s.store.UseRecoveryCode(ctx, u.ID, auth.HashRecoveryCode(code))
}

// handleVerifyTwoFactor finishes signing in with a code after the password.
func (s *Server) handleVerifyTwoFactor(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	if user.TwoFactor != "verify" {
		writeError(w, http.StatusBadRequest, "no two-factor check is pending")
		return
	}
	ip := s.clientIP(r)
	if !s.mfaLimiter.allow(ip) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "too many attempts, try again shortly")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	tokenHash := sessionFrom(r.Context())
	ok, err := s.checkSecondFactor(r.Context(), user, req.Code)
	if err != nil {
		s.log.Error("check two-factor code", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !ok {
		ended, err := s.store.FailMFA(r.Context(), tokenHash, maxMFAAttempts)
		if err != nil {
			s.log.Error("count failed two-factor code", "err", err)
		}
		s.log.Warn("incorrect two-factor code", "user_id", user.ID, "ip", ip)
		if ended {
			s.clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "too many incorrect codes, please sign in again")
			return
		}
		writeError(w, http.StatusBadRequest, "that code isn't right")
		return
	}

	remember, _ := r.Context().Value(rememberKey).(bool)
	expires := time.Now().Add(sessionLifetime(remember))
	if err := s.store.CompleteMFA(r.Context(), tokenHash, expires); err != nil {
		s.log.Error("complete two-factor", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if remember {
		if c, err := r.Cookie(sessionCookie); err == nil {
			s.setSessionCookie(w, c.Value, true, expires)
		}
	}
	s.log.Info("login", "user_id", user.ID, "ip", ip, "two_factor", true)
	user.TwoFactor = ""
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}

type twoFactorStatus struct {
	Enabled           bool `json:"enabled"`
	Required          bool `json:"required"`
	RecoveryCodesLeft int  `json:"recoveryCodesLeft"`
}

func (s *Server) handleTwoFactorStatus(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	st, err := s.store.Settings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := twoFactorStatus{Enabled: user.TwoFactorEnabled, Required: st.RequireTwoFactor}
	if user.TwoFactorEnabled {
		if out.RecoveryCodesLeft, err = s.store.RecoveryCodesLeft(r.Context(), user.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleStartTwoFactor creates a new secret for the user to add to their
// authenticator app. It isn't active until confirmed with a code.
func (s *Server) handleStartTwoFactor(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	if user.TwoFactorEnabled {
		writeError(w, http.StatusConflict, "two-factor authentication is already on")
		return
	}
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := s.store.SetTOTPSecret(r.Context(), user.ID, secret); err != nil {
		s.log.Error("set totp secret", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	uri := auth.TOTPURI("Lithium", user.Email, secret)
	img, err := qrDataURI(uri)
	if err != nil {
		s.log.Error("render qr", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": secret, "uri": uri, "qr": img})
}

// handleEnableTwoFactor turns two-factor on once the user shows their app
// produces the right codes, and returns their recovery codes (shown once).
func (s *Server) handleEnableTwoFactor(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if user.TwoFactorEnabled {
		writeError(w, http.StatusConflict, "two-factor authentication is already on")
		return
	}
	if user.TOTPSecret == "" {
		writeError(w, http.StatusBadRequest, "start two-factor setup first")
		return
	}
	step, ok := auth.VerifyTOTP(user.TOTPSecret, req.Code, time.Now(), 0)
	if !ok {
		writeError(w, http.StatusBadRequest, "that code isn't right, check your device's clock is set automatically")
		return
	}
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := s.store.EnableTOTP(r.Context(), user.ID, step, hashes); err != nil {
		s.log.Error("enable totp", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("two-factor enabled", "user_id", user.ID)
	// Set up properly, so the first admin's one-time skip is no longer offered.
	if err := s.store.EndFirstAdminTwoFactorSkip(r.Context(), user.ID); err != nil {
		s.log.Error("end first admin two-factor skip", "err", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"recoveryCodes": codes})
}

// handleSkipTwoFactor lets the admin created at first startup skip mandatory
// two-factor setup, once, which turns mandatory two-factor off for everyone.
// Meant for trying Lithium out; admins can turn it back on in settings.
func (s *Server) handleSkipTwoFactor(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	if user.TwoFactor != "setup" || !user.CanSkipTwoFactor {
		writeError(w, http.StatusForbidden, "two-factor setup can't be skipped")
		return
	}
	ok, err := s.store.SkipFirstAdminTwoFactor(r.Context(), user.ID)
	if err != nil {
		s.log.Error("skip two-factor", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "two-factor setup can't be skipped")
		return
	}
	s.log.Warn("first admin skipped two-factor setup; mandatory two-factor is now off", "user_id", user.ID, "ip", s.clientIP(r))
	user.TwoFactor = ""
	user.CanSkipTwoFactor = false
	writeJSON(w, http.StatusOK, user)
}

func newRecoveryCodes() (codes, hashes []string, err error) {
	codes, err = auth.NewRecoveryCodes(recoveryCodeCount)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range codes {
		hashes = append(hashes, auth.HashRecoveryCode(c))
	}
	return codes, hashes, nil
}

// confirmPassword checks the password sent with a sensitive change.
func confirmPassword(w http.ResponseWriter, r *http.Request, u store.User) bool {
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return false
	}
	if ok, err := auth.VerifyPassword(req.Password, u.PasswordHash); err != nil || !ok {
		writeError(w, http.StatusBadRequest, "password is incorrect")
		return false
	}
	return true
}

// handleNewRecoveryCodes replaces the user's recovery codes.
func (s *Server) handleNewRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	if !user.TwoFactorEnabled {
		writeError(w, http.StatusBadRequest, "two-factor authentication is off")
		return
	}
	if !confirmPassword(w, r, user) {
		return
	}
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := s.store.ReplaceRecoveryCodes(r.Context(), user.ID, hashes); err != nil {
		s.log.Error("replace recovery codes", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("recovery codes regenerated", "user_id", user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"recoveryCodes": codes})
}

// handleDisableTwoFactor turns the user's own two-factor off, unless the
// workspace requires it. Other sessions are signed out.
func (s *Server) handleDisableTwoFactor(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	st, err := s.store.Settings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if st.RequireTwoFactor {
		writeError(w, http.StatusForbidden, "two-factor authentication is required in this workspace")
		return
	}
	if !confirmPassword(w, r, user) {
		return
	}
	if err := s.store.ResetTwoFactor(r.Context(), user.ID, sessionFrom(r.Context())); err != nil {
		s.log.Error("disable two-factor", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("two-factor disabled", "user_id", user.ID)
	w.WriteHeader(http.StatusNoContent)
}

// handleResetTwoFactor lets an admin turn off someone's two-factor, e.g.
// after they lose their phone. They're signed out everywhere and (if it's
// mandatory) set it up again at their next sign-in.
func (s *Server) handleResetTwoFactor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	admin := userFrom(r.Context())
	if _, err := s.store.UserByID(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	keep := ""
	if id == admin.ID {
		keep = sessionFrom(r.Context()) // don't sign yourself out
	}
	if err := s.store.ResetTwoFactor(r.Context(), id, keep); err != nil {
		s.log.Error("reset two-factor", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("two-factor reset by admin", "user_id", id, "by", admin.ID)
	w.WriteHeader(http.StatusNoContent)
}
