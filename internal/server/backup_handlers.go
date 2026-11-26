package server

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/lithium/lithium/internal/backup"
	"github.com/lithium/lithium/internal/store"
)

// Backups returns the backup manager, so main can run the schedule.
func (s *Server) Backups() *backup.Manager { return s.backups }

type backupStatus struct {
	Settings store.BackupSettings `json:"settings"`
	Backups  []backup.Info        `json:"backups"`
	Running  bool                 `json:"running"`
	// LastError is why the most recent backup failed ("" if it didn't).
	LastError string `json:"lastError"`
	// NextRun is when the next scheduled backup is due (absent when off).
	NextRun *time.Time `json:"nextRun,omitempty"`
	// TimeZone is the server's, which the schedule's hour is in (e.g. "UTC").
	TimeZone string `json:"timeZone"`
	// Folder is where backups are kept on the server.
	Folder string `json:"folder"`
}

func (s *Server) handleGetBackups(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.BackupSettings(r.Context())
	if err != nil {
		s.log.Error("read backup settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	list, err := s.backups.List()
	if err != nil {
		s.log.Error("list backups", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	running, lastErr := s.backups.Status()
	st := backupStatus{
		Settings:  settings,
		Backups:   list,
		Running:   running,
		LastError: lastErr,
		TimeZone:  time.Now().Location().String(),
		Folder:    s.backups.Dir(),
	}
	if st.TimeZone == "Local" {
		st.TimeZone, _ = time.Now().Zone()
	}
	if next := s.backups.NextRun(settings, s.backups.LastScheduled()); !next.IsZero() {
		st.NextRun = &next
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleSaveBackupSettings(w http.ResponseWriter, r *http.Request) {
	var req store.BackupSettings
	if !decodeJSON(w, r, &req) {
		return
	}
	switch {
	case req.Frequency != "daily" && req.Frequency != "weekly":
		writeError(w, http.StatusBadRequest, "frequency must be daily or weekly")
		return
	case req.Hour < 0 || req.Hour > 23:
		writeError(w, http.StatusBadRequest, "hour must be between 0 and 23")
		return
	case req.Keep < 1 || req.Keep > 90:
		writeError(w, http.StatusBadRequest, "keep between 1 and 90 backups")
		return
	}
	if err := s.store.SaveBackupSettings(r.Context(), req); err != nil {
		s.log.Error("save backup settings", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.handleGetBackups(w, r)
}

// handleCreateBackup starts a manual backup in the background; the page
// polls GET /api/admin/backups until it's done.
func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	if err := s.backups.Start(backup.KindManual); errors.Is(err, backup.ErrRunning) {
		writeError(w, http.StatusConflict, "a backup is already running")
		return
	} else if err != nil {
		s.log.Error("start backup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	path, err := s.backups.Path(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	defer f.Close()
	auditDetail(r.Context(), "%s", name)
	st, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Backups can be large; don't let the server's usual write timeout cut
	// the download off.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, name, st.ModTime(), f)
}

func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	auditDetail(r.Context(), "%s", r.PathValue("name"))
	if err := s.backups.Delete(r.PathValue("name")); errors.Is(err, backup.ErrNotFound) {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	} else if err != nil {
		s.log.Error("delete backup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
