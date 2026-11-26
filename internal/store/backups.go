package store

import (
	"context"
	"strconv"
	"time"
)

// BackupSettings is the schedule for automatic backups. Manual backups work
// whether or not it's enabled.
type BackupSettings struct {
	Enabled   bool   `json:"enabled"`
	Frequency string `json:"frequency"` // "daily" or "weekly"
	Hour      int    `json:"hour"`      // 0-23, server time
	Keep      int    `json:"keep"`      // scheduled backups to keep; older ones are deleted
	// EnabledAt is when scheduling was last turned on; the first scheduled
	// backup is the first slot after it.
	EnabledAt time.Time `json:"-"`
}

// DefaultBackupSettings: off, but ready to turn on as a nightly backup.
func DefaultBackupSettings() BackupSettings {
	return BackupSettings{Enabled: false, Frequency: "daily", Hour: 2, Keep: 7}
}

const (
	keyBackupEnabled   = "backups.enabled"
	keyBackupFrequency = "backups.frequency"
	keyBackupHour      = "backups.hour"
	keyBackupKeep      = "backups.keep"
	keyBackupEnabledAt = "backups.enabled_at"
)

func (s *Store) BackupSettings(ctx context.Context) (BackupSettings, error) {
	b := DefaultBackupSettings()
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM app_settings WHERE key LIKE 'backups.%'`)
	if err != nil {
		return b, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return b, err
		}
		switch key {
		case keyBackupEnabled:
			b.Enabled, _ = strconv.ParseBool(value)
		case keyBackupFrequency:
			b.Frequency = value
		case keyBackupHour:
			if n, err := strconv.Atoi(value); err == nil {
				b.Hour = n
			}
		case keyBackupKeep:
			if n, err := strconv.Atoi(value); err == nil {
				b.Keep = n
			}
		case keyBackupEnabledAt:
			if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
				b.EnabledAt = time.Unix(n, 0).UTC()
			}
		}
	}
	return b, rows.Err()
}

// SaveBackupSettings stores b; the caller validates it. Turning scheduling
// on records when, unless b.EnabledAt is already set.
func (s *Store) SaveBackupSettings(ctx context.Context, b BackupSettings) error {
	if b.Enabled && b.EnabledAt.IsZero() {
		prev, err := s.BackupSettings(ctx)
		if err != nil {
			return err
		}
		if prev.Enabled {
			b.EnabledAt = prev.EnabledAt
		} else {
			b.EnabledAt = time.Now().UTC()
		}
	}
	return s.saveBackupSettings(ctx, b)
}

func (s *Store) saveBackupSettings(ctx context.Context, b BackupSettings) error {
	values := map[string]string{
		keyBackupEnabled:   strconv.FormatBool(b.Enabled),
		keyBackupFrequency: b.Frequency,
		keyBackupHour:      strconv.Itoa(b.Hour),
		keyBackupKeep:      strconv.Itoa(b.Keep),
		keyBackupEnabledAt: "",
	}
	if !b.EnabledAt.IsZero() {
		values[keyBackupEnabledAt] = strconv.FormatInt(b.EnabledAt.Unix(), 10)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	for k, v := range values {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			k, v, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SnapshotDatabase writes a consistent copy of the whole database to path,
// which must not exist yet. Lithium keeps running while it's taken.
func (s *Store) SnapshotDatabase(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path)
	return err
}
