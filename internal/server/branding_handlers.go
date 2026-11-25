package server

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

const maxLogoBytes = 2 << 20 // 2 MB

func (s *Server) brandingDir() string { return filepath.Join(s.cfg.DataDir, "branding") }

// handlePublicBranding returns the company name, logo and accent colour
// without needing to sign in, so the sign-in pages can show them too.
func (s *Server) handlePublicBranding(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"companyName": st.CompanyName, "logoUrl": st.LogoURL, "accent": st.Accent})
}

// replaceLogo stores the new logo file name ("" removes the logo) and deletes
// the file it replaced, if any.
func (s *Server) replaceLogo(w http.ResponseWriter, r *http.Request, file string) bool {
	prev, err := s.store.SetLogoFile(r.Context(), file)
	if err != nil {
		s.log.Error("set logo", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return false
	}
	if prev != "" && prev != file {
		if err := os.Remove(filepath.Join(s.brandingDir(), filepath.Base(prev))); err != nil && !os.IsNotExist(err) {
			s.log.Warn("remove old logo", "err", err)
		}
	}
	s.log.Info("company logo updated", "by", userFrom(r.Context()).ID, "removed", file == "")
	return true
}

// handleUploadLogo sets the company logo (multipart field "image").
func (s *Server) handleUploadLogo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoBytes+1<<20)
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "logos must be 2 MB or smaller")
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

	saved, err := saveImageFile(s.brandingDir(), maxLogoBytes, files[0])
	if err != nil {
		if saved.path != "" {
			os.Remove(saved.path)
		}
		var userErr userError
		if errors.As(err, &userErr) {
			writeError(w, http.StatusBadRequest, userErr.Error())
		} else {
			s.log.Error("save logo", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	if !s.replaceLogo(w, r, saved.storedName) {
		os.Remove(saved.path)
		return
	}
	s.writeSettings(w, r)
}

// handleDeleteLogo goes back to the Lithium logo.
func (s *Server) handleDeleteLogo(w http.ResponseWriter, r *http.Request) {
	if s.replaceLogo(w, r, "") {
		s.writeSettings(w, r)
	}
}

// writeSettings responds with the workspace settings as they are now.
func (s *Server) writeSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, s.withServerSettings(r.Context(), st))
}

// handleGetLogo serves the company logo to anyone, signed in or not, since
// it appears on the sign-in page.
func (s *Server) handleGetLogo(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Settings(r.Context())
	if err != nil {
		s.log.Error("get settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if st.LogoFile == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	file := filepath.Base(st.LogoFile)
	f, err := os.Open(filepath.Join(s.brandingDir(), file))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	h := w.Header()
	h.Set("Content-Type", mime.TypeByExtension(filepath.Ext(file)))
	// The URL changes with every upload, so it can be cached for a while.
	h.Set("Cache-Control", "public, max-age=86400")
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	http.ServeContent(w, r, "", info.ModTime(), f)
}
