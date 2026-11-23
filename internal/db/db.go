// Package db opens the SQLite database and applies migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens (creating if needed) the SQLite database at path and runs all
// pending migrations.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	q := url.Values{}
	for _, p := range []string{
		"journal_mode(WAL)",
		"foreign_keys(ON)",
		"busy_timeout(5000)",
		"synchronous(NORMAL)",
	} {
		q.Add("_pragma", p)
	}
	conn, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := migrate(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func newProvider(conn *sql.DB) (*goose.Provider, error) {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectSQLite3, conn, sub)
}

func migrate(ctx context.Context, conn *sql.DB) error {
	// Run migrations on a single connection: table rebuilds toggle
	// PRAGMA foreign_keys, which is per-connection.
	conn.SetMaxOpenConns(1)
	defer conn.SetMaxOpenConns(0)

	p, err := newProvider(conn)
	if err != nil {
		return fmt.Errorf("migration provider: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	// Belt and braces: make sure no rebuild left dangling references.
	rows, err := conn.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("foreign key check failed after migrations")
	}
	return rows.Err()
}
