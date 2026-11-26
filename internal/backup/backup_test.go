package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lithium/lithium/internal/db"
	"github.com/lithium/lithium/internal/store"
)

func newManager(t *testing.T) (*Manager, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	conn, err := db.Open(context.Background(), filepath.Join(dir, "lithium.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return New(dir, store.New(conn), slog.New(slog.NewTextHandler(io.Discard, nil))), conn
}

func TestCreateBackup(t *testing.T) {
	m, conn := newManager(t)
	ctx := context.Background()
	if _, err := conn.Exec(`INSERT INTO users (email, name, password_hash, role, created_at, updated_at) VALUES ('a@example.com', 'Ada', 'x', 'admin', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(m.DataDir, "attachments", "sub"), 0o750)
	os.WriteFile(filepath.Join(m.DataDir, "attachments", "sub", "a.png"), []byte("png!"), 0o600)
	os.MkdirAll(filepath.Join(m.DataDir, "branding"), 0o750)
	os.WriteFile(filepath.Join(m.DataDir, "branding", "logo.svg"), []byte("<svg/>"), 0o600)

	info, err := m.Create(ctx, KindManual)
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != KindManual || info.Size == 0 || !nameRE.MatchString(info.Name) {
		t.Fatalf("info: %+v", info)
	}

	// The archive has the manifest, the database and the uploads, but not
	// the live database files or other backups.
	f, err := os.Open(filepath.Join(m.Dir(), info.Name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	files := map[string][]byte{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(tr)
		files[h.Name] = b
	}
	for _, want := range []string{"backup.json", "lithium.db", "attachments/sub/a.png", "branding/logo.svg"} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s; have %v", want, keys(files))
		}
	}
	if len(files) != 4 {
		t.Errorf("unexpected files: %v", keys(files))
	}

	// The database in the backup opens and has the data.
	restored := filepath.Join(t.TempDir(), "restored.db")
	os.WriteFile(restored, files["lithium.db"], 0o600)
	rdb, err := sql.Open("sqlite", "file:"+restored)
	if err != nil {
		t.Fatal(err)
	}
	defer rdb.Close()
	var name string
	if err := rdb.QueryRow(`SELECT name FROM users`).Scan(&name); err != nil || name != "Ada" {
		t.Fatalf("restored database: %q %v", name, err)
	}

	// No leftovers beside the backup.
	entries, _ := os.ReadDir(m.Dir())
	if len(entries) != 1 {
		t.Fatalf("leftover files in backups folder: %v", entries)
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestPruneAndNames(t *testing.T) {
	m, _ := newManager(t)
	os.MkdirAll(m.Dir(), 0o750)
	base := time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		ts := base.AddDate(0, 0, i).Format(timeLayout)
		os.WriteFile(filepath.Join(m.Dir(), "lithium-backup-"+ts+"-scheduled.tar.gz"), []byte("x"), 0o600)
	}
	os.WriteFile(filepath.Join(m.Dir(), "lithium-backup-"+base.Format(timeLayout)+"-manual.tar.gz"), []byte("x"), 0o600)
	os.WriteFile(filepath.Join(m.Dir(), "notes.txt"), []byte("x"), 0o600)

	if n, err := m.Prune(2); err != nil || n != 3 {
		t.Fatalf("prune: %d %v", n, err)
	}
	list, _ := m.List()
	if len(list) != 3 || list[0].CreatedAt != base.AddDate(0, 0, 4) || list[2].Kind != KindManual {
		t.Fatalf("after prune: %+v", list)
	}
	if got := m.LastScheduled(); !got.Equal(base.AddDate(0, 0, 4)) {
		t.Fatalf("last scheduled: %v", got)
	}

	// Only real backup names can be opened or deleted.
	for _, bad := range []string{"notes.txt", "../lithium.db", "lithium-backup-x-manual.tar.gz", ""} {
		if _, err := m.Path(bad); err != ErrNotFound {
			t.Errorf("Path(%q) = %v", bad, err)
		}
	}
}

func TestNextRun(t *testing.T) {
	m, _ := newManager(t)
	at := func(day, hour, min int) time.Time { return time.Date(2026, 11, day, hour, min, 0, 0, time.UTC) }
	daily := store.BackupSettings{Enabled: true, Frequency: "daily", Hour: 2, Keep: 7}
	weekly := store.BackupSettings{Enabled: true, Frequency: "weekly", Hour: 2, Keep: 7}

	for _, tc := range []struct {
		name     string
		settings store.BackupSettings
		now      time.Time
		last     time.Time
		want     time.Time
	}{
		{"off", store.DefaultBackupSettings(), at(10, 12, 0), time.Time{}, time.Time{}},
		{"never, before the hour", daily, at(10, 1, 0), time.Time{}, at(10, 2, 0)},
		{"never, after the hour waits for tomorrow", daily, at(10, 3, 0), time.Time{}, at(11, 2, 0)},
		{"turned on before the hour, now past it", withEnabled(daily, at(10, 1, 0)), at(10, 2, 0), time.Time{}, at(10, 2, 0)},
		{"turned on after the hour", withEnabled(daily, at(10, 3, 0)), at(10, 9, 0), time.Time{}, at(11, 2, 0)},
		{"daily, ran last night", daily, at(10, 12, 0), at(10, 2, 1), at(11, 2, 0)},
		{"daily, ran a bit late", daily, at(10, 12, 0), at(10, 2, 40), at(11, 2, 0)},
		{"daily, missed one runs now", daily, at(12, 9, 0), at(10, 2, 0), at(12, 9, 0)},
		{"weekly", weekly, at(10, 12, 0), at(10, 2, 0), at(17, 2, 0)},
	} {
		m.Now = func() time.Time { return tc.now }
		if got := m.NextRun(tc.settings, tc.last); !got.Equal(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func withEnabled(s store.BackupSettings, at time.Time) store.BackupSettings {
	s.EnabledAt = at
	return s
}

func TestTickMakesScheduledBackupsAndPrunes(t *testing.T) {
	m, _ := newManager(t)
	ctx := context.Background()
	day := time.Date(2026, 11, 10, 2, 0, 30, 0, time.UTC)
	// Turned on the evening before.
	on := store.BackupSettings{Enabled: true, Frequency: "daily", Hour: 2, Keep: 2, EnabledAt: day.Add(-6 * time.Hour)}
	if err := m.Store.SaveBackupSettings(ctx, on); err != nil {
		t.Fatal(err)
	}
	// Checks before the first slot, then three nights in a row, plus a
	// mid-morning check that does nothing.
	ticks := []time.Time{day.Add(-time.Hour), day, day.Add(time.Minute), day.Add(8 * time.Hour), day.AddDate(0, 0, 1), day.AddDate(0, 0, 2)}
	made := 0
	for _, now := range ticks {
		m.Now = func() time.Time { return now }
		before := m.LastScheduled()
		m.Tick(ctx)
		if !m.LastScheduled().Equal(before) {
			made++
		}
	}
	if made != 3 {
		t.Fatalf("made %d scheduled backups, want 3", made)
	}
	list, _ := m.List()
	if len(list) != 2 || list[0].Kind != KindScheduled || !list[0].CreatedAt.Equal(day.AddDate(0, 0, 2).Truncate(time.Second)) {
		t.Fatalf("scheduled backups: %+v", list)
	}
}
