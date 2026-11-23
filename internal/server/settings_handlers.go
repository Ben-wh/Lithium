package server

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/store"
)

// handleGetSettings returns workspace settings. Any signed-in user can read
// them, since some (like a mandatory category) shape the forms they fill in.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// Ticket ID prefixes are short and plain so references like "TID-42" are
// easy to read out, type and search for.
var ticketPrefixRe = regexp.MustCompile(`^[A-Z0-9]{1,6}$`)

// handleUpdateSettings applies a partial update. Only fields present in the
// request change.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequireCategory      *bool   `json:"requireCategory"`
		AllowImageUploads    *bool   `json:"allowImageUploads"`
		TicketPrefix         *string `json:"ticketPrefix"`
		DisableCustomAvatars *bool   `json:"disableCustomAvatars"`
		AllowUserMentions    *bool   `json:"allowUserMentions"`
		SubjectMaxLength     *int    `json:"subjectMaxLength"`
		DescriptionMaxLength *int    `json:"descriptionMaxLength"`
		IncidentTree         *bool   `json:"incidentTree"`
		AgentGroups          *bool   `json:"agentGroups"`
		Background           *string `json:"background"`
		SensitiveMode        *bool   `json:"sensitiveMode"`
		SensitiveMessage     *string `json:"sensitiveMessage"`
		RequireTwoFactor     *bool   `json:"requireTwoFactor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.TicketPrefix != nil {
		p := strings.ToUpper(strings.TrimSpace(*req.TicketPrefix))
		if !ticketPrefixRe.MatchString(p) {
			writeError(w, http.StatusBadRequest, "ticket ID prefix must be 1 to 6 letters or numbers")
			return
		}
		req.TicketPrefix = &p
	}
	if m := req.SensitiveMessage; m != nil {
		// Blank goes back to the default message.
		trimmed := strings.Join(strings.Fields(*m), " ")
		if utf8.RuneCountInString(trimmed) > 120 {
			writeError(w, http.StatusBadRequest, "sensitive mode message must be 120 characters or fewer")
			return
		}
		req.SensitiveMessage = &trimmed
	}
	if req.Background != nil && !store.Backgrounds[*req.Background] {
		writeError(w, http.StatusBadRequest, "unknown background")
		return
	}
	if n := req.SubjectMaxLength; n != nil && (*n < store.SubjectMaxLengthMin || *n > store.SubjectMaxLengthMax) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("subject limit must be between %d and %d characters",
			store.SubjectMaxLengthMin, store.SubjectMaxLengthMax))
		return
	}
	if n := req.DescriptionMaxLength; n != nil && (*n < store.DescriptionMaxLengthMin || *n > store.DescriptionMaxLengthMax) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("description limit must be between %d and %d characters",
			store.DescriptionMaxLengthMin, store.DescriptionMaxLengthMax))
		return
	}
	st, err := s.store.UpdateSettings(r.Context(), store.SettingsUpdate{
		RequireCategory:      req.RequireCategory,
		AllowImageUploads:    req.AllowImageUploads,
		TicketPrefix:         req.TicketPrefix,
		DisableCustomAvatars: req.DisableCustomAvatars,
		AllowUserMentions:    req.AllowUserMentions,
		SubjectMaxLength:     req.SubjectMaxLength,
		DescriptionMaxLength: req.DescriptionMaxLength,
		IncidentTree:         req.IncidentTree,
		AgentGroups:          req.AgentGroups,
		Background:           req.Background,
		SensitiveMode:        req.SensitiveMode,
		SensitiveMessage:     req.SensitiveMessage,
		RequireTwoFactor:     req.RequireTwoFactor,
	})
	if err != nil {
		s.log.Error("update settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.log.Info("settings updated", "by", userFrom(r.Context()).ID)
	writeJSON(w, http.StatusOK, st)
}

// handlePublicBanner returns only the sensitive mode banner, without needing
// to sign in, so it can be shown on the login page too.
func (s *Server) handlePublicBanner(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := map[string]any{"sensitiveMode": st.SensitiveMode, "sensitiveMessage": ""}
	if st.SensitiveMode {
		out["sensitiveMessage"] = st.SensitiveMessage
	}
	writeJSON(w, http.StatusOK, out)
}
