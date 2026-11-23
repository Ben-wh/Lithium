package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// Upgrading a database from before the role rename converts customers to
// users and keeps their sessions and reset requests.
func TestCustomerRoleMigratesToUser(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")

	// Build a database at version 2, as an existing install would have.
	old, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	p, err := newProvider(old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 2); err != nil {
		t.Fatal(err)
	}
	mustExec(t, old, `INSERT INTO users (id, email, name, password_hash, role, created_at, updated_at)
		VALUES (1, 'c@example.com', 'Cust', 'x', 'customer', 0, 0), (2, 'a@example.com', 'Agent', 'x', 'agent', 0, 0)`)
	mustExec(t, old, `INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES ('tok', 1, 0, 9999999999)`)
	mustExec(t, old, `INSERT INTO password_reset_requests (user_id, created_at) VALUES (1, 0)`)
	old.Close()

	conn, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var role string
	conn.QueryRow(`SELECT role FROM users WHERE id = 1`).Scan(&role)
	if role != "user" {
		t.Fatalf("role = %q, want user", role)
	}
	conn.QueryRow(`SELECT role FROM users WHERE id = 2`).Scan(&role)
	if role != "agent" {
		t.Fatalf("agent role changed to %q", role)
	}
	var n int
	conn.QueryRow(`SELECT (SELECT COUNT(*) FROM sessions) + (SELECT COUNT(*) FROM password_reset_requests)`).Scan(&n)
	if n != 2 {
		t.Fatalf("sessions/reset requests lost during rebuild: %d rows left, want 2", n)
	}
	if _, err := conn.Exec(`UPDATE users SET role = 'customer' WHERE id = 1`); err == nil {
		t.Fatal("'customer' role should now be rejected")
	}
	// Cascades still work against the rebuilt table.
	mustExec(t, conn, `DELETE FROM users WHERE id = 1`)
	conn.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n)
	if n != 0 {
		t.Fatalf("session not cascade-deleted: %d left", n)
	}
}

func mustExec(t *testing.T, db *sql.DB, q string) {
	t.Helper()
	if _, err := db.Exec(q); err != nil {
		t.Fatal(err)
	}
}

// Upgrading converts 'open' tickets to unassigned or in progress depending
// on whether someone is assigned.
func TestOpenTicketsMigrateToNewStatuses(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")

	old, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	p, err := newProvider(old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 5); err != nil {
		t.Fatal(err)
	}
	mustExec(t, old, `INSERT INTO users (id, email, name, password_hash, role, created_at, updated_at)
		VALUES (1, 'a@example.com', 'A', 'x', 'agent', 0, 0)`)
	mustExec(t, old, `INSERT INTO tickets (id, subject, description, status, requester_id, created_by, assignee_id, created_at, updated_at) VALUES
		(1, 's', 'd', 'open', 1, 1, NULL, 0, 0),
		(2, 's', 'd', 'open', 1, 1, 1, 0, 0),
		(3, 's', 'd', 'resolved', 1, 1, 1, 0, 0)`)
	old.Close()

	conn, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for id, want := range map[int]string{1: "unassigned", 2: "in_progress", 3: "resolved"} {
		var got string
		conn.QueryRow(`SELECT status FROM tickets WHERE id = ?`, id).Scan(&got)
		if got != want {
			t.Errorf("ticket %d: status %q, want %q", id, got, want)
		}
	}
	if _, err := conn.Exec(`UPDATE tickets SET status = 'open' WHERE id = 1`); err == nil {
		t.Error("'open' should now be rejected")
	}
	var def string
	mustExec(t, conn, `INSERT INTO tickets (id, subject, description, requester_id, created_by, created_at, updated_at) VALUES (4, 's', 'd', 1, 1, 0, 0)`)
	conn.QueryRow(`SELECT status FROM tickets WHERE id = 4`).Scan(&def)
	if def != "unassigned" {
		t.Errorf("default status %q, want unassigned", def)
	}
}

// The links migration rebuilds ticket_comments; existing replies, notes and
// updates must survive it.
func TestLinksMigrationKeepsComments(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	old, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	p, err := newProvider(old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 13); err != nil {
		t.Fatal(err)
	}
	mustExec(t, old, `INSERT INTO users (id, email, name, password_hash, role, created_at, updated_at) VALUES (1, 'a@x.com', 'A', 'x', 'agent', 0, 0)`)
	mustExec(t, old, `INSERT INTO tickets (id, subject, description, requester_id, created_by, created_at, updated_at) VALUES (1, 's', 'd', 1, 1, 0, 0)`)
	mustExec(t, old, `INSERT INTO ticket_comments (ticket_id, kind, author_id, body, internal, created_at) VALUES (1, 'comment', 1, 'secret', 1, 5)`)
	mustExec(t, old, `INSERT INTO ticket_comments (ticket_id, kind, author_id, event, old_value, new_value, created_at) VALUES (1, 'event', 1, 'status', 'unassigned', 'pending', 6)`)
	old.Close()

	conn, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var n, internal int
	conn.QueryRow(`SELECT COUNT(*), SUM(internal) FROM ticket_comments`).Scan(&n, &internal)
	if n != 2 || internal != 1 {
		t.Fatalf("comments after migration: %d rows, %d internal", n, internal)
	}
	mustExec(t, conn, `INSERT INTO ticket_comments (ticket_id, kind, event, new_value, created_at) VALUES (1, 'event', 'parent', '2', 7)`)
	mustExec(t, conn, `UPDATE tickets SET parent_id = NULL WHERE id = 1`)
}
