// Package backup makes, lists and schedules backups: one .tar.gz holding a
// consistent snapshot of the database and every uploaded file.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lithium/lithium/internal/store"
	"github.com/lithium/lithium/internal/version"
)

// Folders under the data directory that hold uploads. Everything else there
// is either the database (snapshotted separately) or backups themselves.
var uploadDirs = []string{"attachments", "kb-images", "avatars", "branding"}

const (
	KindManual    = "manual"
	KindScheduled = "scheduled"
	timeLayout    = "2006-01-02T15-04-05Z"
)

// Names look like lithium-backup-2026-11-26T02-00-00Z-scheduled.tar.gz.
// Only names matching this are ever opened or deleted, so a request can't
// reach outside the backups folder.
var nameRE = regexp.MustCompile(`^lithium-backup-(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z)-(manual|scheduled)\.tar\.gz$`)

var (
	ErrNotFound = errors.New("backup not found")
	ErrRunning  = errors.New("a backup is already running")
)

// Info describes one backup file.
type Info struct {
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

// Manager makes backups of DataDir into DataDir/backups, one at a time.
type Manager struct {
	DataDir string
	Store   *store.Store
	Log     *slog.Logger
	// Now is the clock; tests replace it.
	Now func() time.Time

	mu        sync.Mutex
	running   bool
	lastError string
}

func New(dataDir string, st *store.Store, log *slog.Logger) *Manager {
	return &Manager{DataDir: dataDir, Store: st, Log: log, Now: time.Now}
}

// Dir is where backups are kept.
func (m *Manager) Dir() string { return filepath.Join(m.DataDir, "backups") }

// Status reports whether a backup is being made, and why the last one
// failed ("" if it didn't).
func (m *Manager) Status() (running bool, lastError string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running, m.lastError
}

// Start begins a backup in the background, returning ErrRunning if one is
// already being made.
func (m *Manager) Start(kind string) error {
	if !m.begin() {
		return ErrRunning
	}
	go func() {
		// Not tied to the request: the backup carries on after it returns.
		_, _ = m.run(context.Background(), kind)
	}()
	return nil
}

// Create makes a backup and waits for it, returning ErrRunning if one is
// already being made.
func (m *Manager) Create(ctx context.Context, kind string) (Info, error) {
	if !m.begin() {
		return Info{}, ErrRunning
	}
	return m.run(ctx, kind)
}

func (m *Manager) begin() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return false
	}
	m.running = true
	return true
}

func (m *Manager) run(ctx context.Context, kind string) (Info, error) {
	start := m.Now()
	info, err := m.write(ctx, kind)
	m.mu.Lock()
	m.running = false
	m.lastError = ""
	if err != nil {
		m.lastError = err.Error()
	}
	m.mu.Unlock()
	if err != nil {
		m.Log.Error("backup failed", "kind", kind, "err", err)
		return Info{}, err
	}
	m.Log.Info("backup made", "name", info.Name, "size", info.Size, "took", time.Since(start).Round(time.Millisecond))
	return info, nil
}

func (m *Manager) write(ctx context.Context, kind string) (Info, error) {
	if err := os.MkdirAll(m.Dir(), 0o750); err != nil {
		return Info{}, fmt.Errorf("create backups folder: %w", err)
	}
	now := m.Now().UTC()
	name := fmt.Sprintf("lithium-backup-%s-%s.tar.gz", now.Format(timeLayout), kind)
	final := filepath.Join(m.Dir(), name)
	if _, err := os.Stat(final); err == nil {
		return Info{}, errors.New("a backup was made this second; try again")
	}

	// Snapshot the database first, beside the backups (same disk, so no
	// surprises about space elsewhere).
	snap := filepath.Join(m.Dir(), ".snapshot-"+now.Format(timeLayout)+".db")
	defer os.Remove(snap)
	if err := m.Store.SnapshotDatabase(ctx, snap); err != nil {
		return Info{}, fmt.Errorf("snapshot database: %w", err)
	}

	// Write to a temporary name, renamed when complete, so a crash never
	// leaves a half-written file that looks like a backup.
	partial := final + ".partial"
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Info{}, err
	}
	defer os.Remove(partial) // no-op once renamed
	if err := m.archive(f, snap, now); err != nil {
		f.Close()
		return Info{}, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return Info{}, err
	}
	if err := f.Close(); err != nil {
		return Info{}, err
	}
	if err := os.Rename(partial, final); err != nil {
		return Info{}, err
	}
	st, err := os.Stat(final)
	if err != nil {
		return Info{}, err
	}
	return Info{Name: name, Kind: kind, Size: st.Size(), CreatedAt: now}, nil
}

// archive writes the tar.gz: backup.json, lithium.db, then the upload folders.
func (m *Manager) archive(w io.Writer, dbSnapshot string, now time.Time) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	manifest, _ := json.MarshalIndent(map[string]string{
		"createdAt": now.Format(time.RFC3339),
		"commit":    version.Commit(),
		"restore":   "Stop Lithium, put lithium.db and the folders back in the data directory, delete any lithium.db-wal and lithium.db-shm there, then start Lithium.",
	}, "", "  ")
	if err := tw.WriteHeader(&tar.Header{Name: "backup.json", Mode: 0o600, Size: int64(len(manifest)), ModTime: now}); err != nil {
		return err
	}
	if _, err := tw.Write(manifest); err != nil {
		return err
	}
	if err := addFile(tw, dbSnapshot, "lithium.db"); err != nil {
		return fmt.Errorf("add database: %w", err)
	}
	for _, dir := range uploadDirs {
		root := filepath.Join(m.DataDir, dir)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) && path == root {
				return fs.SkipDir // nothing uploaded of this kind yet
			}
			if err != nil {
				return err
			}
			if !d.Type().IsRegular() {
				return nil // folders are implied by paths; skip links and the like
			}
			rel, err := filepath.Rel(m.DataDir, path)
			if err != nil {
				return err
			}
			return addFile(tw, path, filepath.ToSlash(rel))
		})
		if err != nil {
			return fmt.Errorf("add %s: %w", dir, err)
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func addFile(tw *tar.Writer, path, name string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // deleted while we were walking; fine
		}
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: st.Size(), ModTime: st.ModTime()}); err != nil {
		return err
	}
	_, err = io.CopyN(tw, f, st.Size())
	return err
}

// List returns the backups, newest first.
func (m *Manager) List() ([]Info, error) {
	entries, err := os.ReadDir(m.Dir())
	if errors.Is(err, fs.ErrNotExist) {
		return []Info{}, nil
	} else if err != nil {
		return nil, err
	}
	out := []Info{}
	for _, e := range entries {
		match := nameRE.FindStringSubmatch(e.Name())
		if match == nil || !e.Type().IsRegular() {
			continue
		}
		created, err := time.Parse(timeLayout, match[1])
		if err != nil {
			continue
		}
		st, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Info{Name: e.Name(), Kind: match[2], Size: st.Size(), CreatedAt: created})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// Path returns the file for a backup name, or ErrNotFound.
func (m *Manager) Path(name string) (string, error) {
	if !nameRE.MatchString(name) {
		return "", ErrNotFound
	}
	p := filepath.Join(m.Dir(), name)
	if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
		return "", ErrNotFound
	}
	return p, nil
}

func (m *Manager) Delete(name string) error {
	p, err := m.Path(name)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// Prune deletes the oldest scheduled backups so at most keep remain. Manual
// backups are never removed automatically.
func (m *Manager) Prune(keep int) (int, error) {
	all, err := m.List()
	if err != nil {
		return 0, err
	}
	removed, seen := 0, 0
	for _, b := range all { // newest first
		if b.Kind != KindScheduled {
			continue
		}
		seen++
		if seen <= keep {
			continue
		}
		if err := os.Remove(filepath.Join(m.Dir(), b.Name)); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// NextRun is when the next scheduled backup is due, given the last one
// (zero if there's never been one, in which case it's the first slot after
// scheduling was turned on). Zero if scheduling is off. A slot that
// was missed (e.g. the server was down) is due straight away.
func (m *Manager) NextRun(s store.BackupSettings, last time.Time) time.Time {
	if !s.Enabled {
		return time.Time{}
	}
	now := m.Now()
	slotOn := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), s.Hour, 0, 0, 0, now.Location())
	}
	if last.IsZero() {
		// The first time the chosen hour came round after scheduling was
		// turned on (from now, if that isn't known).
		since := s.EnabledAt
		if since.IsZero() {
			since = now
		}
		since = since.In(now.Location())
		next := slotOn(since)
		if !next.After(since) {
			next = slotOn(since.AddDate(0, 0, 1))
		}
		if next.Before(now) {
			return now
		}
		return next
	}
	gap := 24 * time.Hour
	if s.Frequency == "weekly" {
		gap = 7 * 24 * time.Hour
	}
	// An hour's slack, so a backup that finished a few minutes past the hour
	// doesn't push the next one back a whole day.
	earliest := last.In(now.Location()).Add(gap - time.Hour)
	next := slotOn(earliest)
	if next.Before(earliest) {
		next = slotOn(earliest.AddDate(0, 0, 1))
	}
	if next.Before(now) {
		return now
	}
	return next
}

// LastScheduled is when the newest scheduled backup was made (zero if none).
func (m *Manager) LastScheduled() time.Time {
	all, err := m.List()
	if err != nil {
		return time.Time{}
	}
	for _, b := range all {
		if b.Kind == KindScheduled {
			return b.CreatedAt
		}
	}
	return time.Time{}
}

// Tick makes a scheduled backup if one is due, then prunes old ones.
func (m *Manager) Tick(ctx context.Context) {
	s, err := m.Store.BackupSettings(ctx)
	if err != nil {
		if ctx.Err() == nil {
			m.Log.Error("read backup settings", "err", err)
		}
		return
	}
	next := m.NextRun(s, m.LastScheduled())
	if next.IsZero() || next.After(m.Now()) {
		return
	}
	if _, err := m.Create(ctx, KindScheduled); err != nil {
		if errors.Is(err, ErrRunning) {
			return // a manual one is running; try again next tick
		}
		return // logged by run; retried next tick
	}
	if n, err := m.Prune(s.Keep); err != nil {
		m.Log.Error("prune backups", "err", err)
	} else if n > 0 {
		m.Log.Info("removed old scheduled backups", "count", n)
	}
}

// Schedule checks once a minute whether a scheduled backup is due, until ctx ends.
func (m *Manager) Schedule(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		m.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// CleanUp removes leftovers from a backup interrupted by a restart.
func (m *Manager) CleanUp() {
	entries, err := os.ReadDir(m.Dir())
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".partial") || strings.HasPrefix(n, ".snapshot-") {
			os.Remove(filepath.Join(m.Dir(), n))
		}
	}
}
