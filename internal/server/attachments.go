package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lithium/lithium/internal/store"
)

const (
	maxImagesPerTicket = 5
	maxImageBytes      = 10 << 20 // 10 MB
)

// Accepted image types, by sniffed content (not the client's claim), and the
// extension they're stored with. SVG is deliberately excluded: it can carry
// scripts.
var imageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// userError is a problem with the request that's safe to show the user.
type userError string

func (e userError) Error() string { return string(e) }

func (s *Server) attachmentDir() string { return filepath.Join(s.cfg.DataDir, "attachments") }

// saveImages checks each upload is a real image within the limits and writes
// it to disk. It returns the records to store and a cleanup func that removes
// the written files (call it if the ticket can't be saved). On error nothing
// is left on disk.
func (s *Server) saveImages(files []*multipart.FileHeader) ([]store.NewAttachment, func(), error) {
	var written []string
	cleanup := func() {
		for _, p := range written {
			os.Remove(p)
		}
	}
	if len(files) == 0 {
		return nil, cleanup, nil
	}
	var out []store.NewAttachment
	for _, fh := range files {
		a, path, err := s.saveImage(fh)
		if path != "" {
			written = append(written, path)
		}
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}
		out = append(out, a)
	}
	return out, cleanup, nil
}

func (s *Server) saveImage(fh *multipart.FileHeader) (store.NewAttachment, string, error) {
	f, err := saveImageFile(s.attachmentDir(), maxImageBytes, fh)
	if err != nil {
		return store.NewAttachment{}, f.path, err
	}
	return store.NewAttachment{Filename: f.name, StoredName: f.storedName, ContentType: f.contentType, Size: f.size}, f.path, nil
}

type savedImage struct {
	name, storedName, contentType, path string
	size                                int64
}

// saveImageFile checks an upload is a real PNG/JPEG/GIF/WebP no bigger than
// maxBytes and writes it into dir under a random name. If it fails after
// writing, path is set so the caller can remove the partial file.
func saveImageFile(dir string, maxBytes int64, fh *multipart.FileHeader) (savedImage, error) {
	name := cleanFilename(fh.Filename)
	tooBig := userError(fmt.Sprintf("%s is larger than %d MB", name, maxBytes>>20))
	if fh.Size > maxBytes {
		return savedImage{}, tooBig
	}
	src, err := fh.Open()
	if err != nil {
		return savedImage{}, err
	}
	defer src.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(src, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return savedImage{}, err
	}
	contentType := http.DetectContentType(head[:n])
	ext, ok := imageTypes[contentType]
	if !ok {
		return savedImage{}, userError(fmt.Sprintf("%s isn't a PNG, JPEG, GIF or WebP image", name))
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return savedImage{}, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return savedImage{}, err
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return savedImage{}, err
	}
	out := savedImage{name: name, storedName: hex.EncodeToString(random) + ext, contentType: contentType}
	out.path = filepath.Join(dir, out.storedName)
	dst, err := os.OpenFile(out.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return savedImage{}, err
	}
	out.size, err = io.Copy(dst, io.LimitReader(src, maxBytes+1))
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return out, err
	}
	if out.size > maxBytes {
		return out, tooBig
	}
	return out, nil
}

// cleanFilename keeps an uploaded file's name for display only: no path, no
// control characters, sensible length.
func cleanFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		name = "image"
	}
	if utf8.RuneCountInString(name) > 200 {
		name = string([]rune(name)[:200])
	}
	return name
}

// handleGetAttachment serves an attachment to anyone who can see its ticket.
func (s *Server) handleGetAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	a, err := s.store.AttachmentByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		s.log.Error("get attachment", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	t, err := s.store.TicketByID(r.Context(), a.TicketID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if ok, err := s.userCanView(r.Context(), userFrom(r.Context()), t); err != nil || !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	f, err := os.Open(filepath.Join(s.attachmentDir(), a.StoredName))
	if err != nil {
		s.log.Error("open attachment", "err", err, "attachment_id", a.ID)
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	defer f.Close()

	h := w.Header()
	h.Set("Content-Type", a.ContentType)
	h.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": a.Filename}))
	h.Set("Cache-Control", "private, max-age=86400")
	// Even if a file were opened directly, it can't run anything.
	h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	http.ServeContent(w, r, "", a.CreatedAt, f)
}
