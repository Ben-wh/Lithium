package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/store"
)

const (
	maxKBTitle     = 150
	maxKBBody      = 100_000 // characters of Markdown
	maxKBImageSize = 10 << 20
)

func (s *Server) kbImageDir() string { return filepath.Join(s.cfg.DataDir, "kb-images") }

// canReadKB reports whether a user may read an article: staff always,
// everyone else only published articles shared with everyone.
func canReadKB(u store.User, a store.KBArticle) bool {
	return isStaff(u) || (a.Status == "published" && a.Visibility == "everyone")
}

// --- Collections ---

func (s *Server) handleListKBCollections(w http.ResponseWriter, r *http.Request) {
	cols, err := s.store.ListKBCollections(r.Context(), isStaff(userFrom(r.Context())))
	if err != nil {
		s.log.Error("list kb collections", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, cols)
}

func (s *Server) saveKBCollection(w http.ResponseWriter, r *http.Request, id int64) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	name, desc := strings.TrimSpace(req.Name), strings.TrimSpace(req.Description)
	switch {
	case name == "" || utf8.RuneCountInString(name) > 60:
		writeError(w, http.StatusBadRequest, "name is required (max 60 characters)")
		return
	case utf8.RuneCountInString(desc) > 200:
		writeError(w, http.StatusBadRequest, "description must be 200 characters or fewer")
		return
	}
	c, err := s.store.SaveKBCollection(r.Context(), id, name, desc)
	switch {
	case store.IsUniqueViolation(err):
		writeError(w, http.StatusConflict, "a collection with that name already exists")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "collection not found")
	case err != nil:
		s.log.Error("save kb collection", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	case id == 0:
		writeJSON(w, http.StatusCreated, c)
	default:
		writeJSON(w, http.StatusOK, c)
	}
}

func (s *Server) handleCreateKBCollection(w http.ResponseWriter, r *http.Request) {
	s.saveKBCollection(w, r, 0)
}

func (s *Server) handleUpdateKBCollection(w http.ResponseWriter, r *http.Request) {
	if id, ok := pathID(w, r); ok {
		s.saveKBCollection(w, r, id)
	}
}

func (s *Server) handleDeleteKBCollection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteKBCollection(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "collection not found")
		return
	} else if err != nil {
		s.log.Error("delete kb collection", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Articles ---

func (s *Server) handleListKBArticles(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	f := store.KBFilter{Staff: isStaff(userFrom(r.Context())), Query: qs.Get("q")}
	switch v := qs.Get("collection"); v {
	case "":
	case "none":
		f.CollectionID = -1
	default:
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "collection must be an id or \"none\"")
			return
		}
		f.CollectionID = id
	}
	if v := qs.Get("status"); v == "draft" || v == "published" {
		f.Status = v
	}
	articles, err := s.store.ListKBArticles(r.Context(), f)
	if err != nil {
		s.log.Error("list kb articles", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, articles)
}

// readableArticle loads an article the user may read, or writes a 404.
func (s *Server) readableArticle(w http.ResponseWriter, r *http.Request) (store.KBArticle, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return store.KBArticle{}, false
	}
	a, err := s.store.KBArticleByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && !canReadKB(userFrom(r.Context()), a)) {
		writeError(w, http.StatusNotFound, "article not found")
		return store.KBArticle{}, false
	} else if err != nil {
		s.log.Error("get kb article", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return store.KBArticle{}, false
	}
	return a, true
}

func (s *Server) handleGetKBArticle(w http.ResponseWriter, r *http.Request) {
	if a, ok := s.readableArticle(w, r); ok {
		writeJSON(w, http.StatusOK, a)
	}
}

// handleCreateKBArticle starts a draft, so images can be attached straight away.
func (s *Server) handleCreateKBArticle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CollectionID int64 `json:"collectionId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.CollectionID != 0 {
		if _, err := s.store.KBCollectionByID(r.Context(), req.CollectionID); err != nil {
			writeError(w, http.StatusBadRequest, "collection not found")
			return
		}
	}
	a, err := s.store.CreateKBArticle(r.Context(), "Untitled article", req.CollectionID, userFrom(r.Context()).ID)
	if err != nil {
		s.log.Error("create kb article", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) handleUpdateKBArticle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Title        string `json:"title"`
		Body         string `json:"body"`
		CollectionID int64  `json:"collectionId"`
		Visibility   string `json:"visibility"`
		Status       string `json:"status"`
	}
	if !decodeJSONLimit(w, r, &req, 1<<20) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	switch {
	case req.Title == "" || utf8.RuneCountInString(req.Title) > maxKBTitle:
		writeError(w, http.StatusBadRequest, "title is required (max 150 characters)")
		return
	case utf8.RuneCountInString(req.Body) > maxKBBody:
		writeError(w, http.StatusBadRequest, "article is too long (max 100,000 characters)")
		return
	case req.Visibility != "staff" && req.Visibility != "everyone":
		writeError(w, http.StatusBadRequest, "visibility must be staff or everyone")
		return
	case req.Status != "draft" && req.Status != "published":
		writeError(w, http.StatusBadRequest, "status must be draft or published")
		return
	}
	if req.CollectionID != 0 {
		if _, err := s.store.KBCollectionByID(r.Context(), req.CollectionID); err != nil {
			writeError(w, http.StatusBadRequest, "collection not found")
			return
		}
	}
	a, err := s.store.UpdateKBArticle(r.Context(), id, userFrom(r.Context()).ID, store.KBArticleUpdate{
		Title: req.Title, Body: req.Body, CollectionID: req.CollectionID, Visibility: req.Visibility, Status: req.Status,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	} else if err != nil {
		s.log.Error("update kb article", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleDeleteKBArticle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	files, err := s.store.DeleteKBArticle(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	} else if err != nil {
		s.log.Error("delete kb article", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	for _, f := range files {
		if err := os.Remove(filepath.Join(s.kbImageDir(), f)); err != nil && !os.IsNotExist(err) {
			s.log.Warn("remove kb image", "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Images ---

// handleUploadKBImage attaches an image to an article (multipart field
// "image") and returns the URL to use in the article's Markdown.
func (s *Server) handleUploadKBImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.store.KBArticleByID(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "article not found")
		return
	} else if err != nil {
		s.log.Error("get kb article", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxKBImageSize+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "images must be 10 MB or smaller")
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
	saved, err := saveImageFile(s.kbImageDir(), maxKBImageSize, files[0])
	if err != nil {
		if saved.path != "" {
			os.Remove(saved.path)
		}
		var userErr userError
		if errors.As(err, &userErr) {
			writeError(w, http.StatusBadRequest, userErr.Error())
		} else {
			s.log.Error("save kb image", "err", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	if err := s.store.AddKBImage(r.Context(), id, saved.storedName, saved.contentType, saved.size, userFrom(r.Context()).ID); err != nil {
		os.Remove(saved.path)
		s.log.Error("record kb image", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"url": "/api/kb/images/" + saved.storedName, "name": saved.name})
}

var kbImageName = regexp.MustCompile(`^[0-9a-f]{32}\.(png|jpg|gif|webp)$`)

// handleGetKBImage serves an article image to anyone who can read the article.
func (s *Server) handleGetKBImage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !kbImageName.MatchString(name) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	img, err := s.store.KBImageByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	a, err := s.store.KBArticleByID(r.Context(), img.ArticleID)
	if err != nil || !canReadKB(userFrom(r.Context()), a) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	f, err := os.Open(filepath.Join(s.kbImageDir(), img.StoredName))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	defer f.Close()
	h := w.Header()
	h.Set("Content-Type", img.ContentType)
	h.Set("Cache-Control", "private, max-age=86400")
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	http.ServeContent(w, r, "", img.CreatedAt, f)
}
