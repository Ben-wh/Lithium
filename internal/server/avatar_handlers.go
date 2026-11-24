package server

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/lithium/lithium/internal/store"
)

const maxAvatarBytes = 2 << 20 // 2 MB

func (s *Server) avatarDir() string { return filepath.Join(s.cfg.DataDir, "avatars") }

// replaceAvatar stores the user's new avatar and deletes the custom file it
// replaced, if any.
func (s *Server) replaceAvatar(w http.ResponseWriter, r *http.Request, avatar string) bool {
	user := userFrom(r.Context())
	prev, err := s.store.SetAvatar(r.Context(), user.ID, avatar)
	if err != nil {
		s.log.Error("set avatar", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return false
	}
	if old := store.CustomAvatarFile(prev); old != "" && old != store.CustomAvatarFile(avatar) {
		if err := os.Remove(filepath.Join(s.avatarDir(), old)); err != nil && !os.IsNotExist(err) {
			s.log.Warn("remove old avatar", "err", err)
		}
	}
	return true
}

// handleSetPresetAvatar picks one of the built-in avatars, or clears the
// avatar (back to initials) with preset 0.
func (s *Server) handleSetPresetAvatar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Preset int `json:"preset"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Preset < 0 || req.Preset > store.PresetAvatarCount {
		writeError(w, http.StatusBadRequest, "unknown avatar")
		return
	}
	avatar := ""
	if req.Preset > 0 {
		avatar = "preset:" + strconv.Itoa(req.Preset)
	}
	if s.replaceAvatar(w, r, avatar) {
		s.writeMe(w, r)
	}
}

// handleUploadAvatar sets a custom profile picture (multipart field "image"),
// unless an admin has disabled custom avatars.
func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if settings.DisableCustomAvatars {
		writeError(w, http.StatusForbidden, "custom profile pictures are turned off")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+1<<20)
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "pictures must be 2 MB or smaller")
		} else {
			writeError(w, http.StatusBadRequest, "invalid upload")
		}
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["image"]
	if len(files) != 1 {
		writeError(w, http.StatusBadRequest, "choose one image")
		return
	}

	saved, err := saveImageFile(s.avatarDir(), maxAvatarBytes, files[0])
	var userErr userError
	if err != nil {
		if saved.path != "" {
			os.Remove(saved.path)
		}
		if errors.As(err, &userErr) {
			writeError(w, http.StatusBadRequest, userErr.Error())
		} else {
			s.log.Error("save avatar", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	if !s.replaceAvatar(w, r, "custom:"+saved.storedName) {
		os.Remove(saved.path)
		return
	}
	s.writeMe(w, r)
}

// handleSetEmailNotifications changes the signed-in user's notification
// email choices: on or off, and which kinds of event. Fields left out stay
// as they are.
func (s *Server) handleSetEmailNotifications(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled *bool     `json:"enabled"`
		Events  *[]string `json:"events"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Enabled == nil && req.Events == nil {
		writeError(w, http.StatusBadRequest, "nothing to change")
		return
	}
	prefs := store.EmailPreferences{Enabled: req.Enabled}
	if req.Events != nil {
		// Keep them in the standard order, without repeats.
		events := store.EmailEvents{}
		for _, known := range store.AllEmailEvents {
			if slices.Contains(*req.Events, known) {
				events = append(events, known)
			}
		}
		for _, e := range *req.Events {
			if !slices.Contains(store.AllEmailEvents, e) {
				writeError(w, http.StatusBadRequest, "unknown email event "+strconv.Quote(e))
				return
			}
		}
		prefs.Events = &events
	}
	if err := s.store.SetEmailPreferences(r.Context(), userFrom(r.Context()).ID, prefs); err != nil {
		s.log.Error("set email notifications", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.writeMe(w, r)
}

// writeMe responds with the current user as it is now in the database.
func (s *Server) writeMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.UserByID(r.Context(), userFrom(r.Context()).ID)
	if err != nil {
		s.log.Error("reload user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// handleGetAvatar serves a user's custom profile picture to anyone signed in.
// It 404s when the user has none or custom pictures are disabled, and the
// app falls back to initials.
func (s *Server) handleGetAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	settings, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	u, err := s.store.UserByID(r.Context(), id)
	file := store.CustomAvatarFile(u.Avatar)
	if err != nil || file == "" || settings.DisableCustomAvatars {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	f, err := os.Open(filepath.Join(s.avatarDir(), file))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	h := w.Header()
	h.Set("Content-Type", mime.TypeByExtension(filepath.Ext(file)))
	// Short cache: the URL changes on every upload, and turning custom
	// pictures off should take effect quickly.
	h.Set("Cache-Control", "private, max-age=300")
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	http.ServeContent(w, r, "", st.ModTime(), f)
}
