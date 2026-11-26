package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lithium/lithium/internal/db"
)

// Sorting by time, with distinct timestamps (tickets created in a test all
// share the same second otherwise).
func TestListTicketsSortByTime(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	s := New(conn)
	if _, err := conn.Exec(`INSERT INTO users (id, email, name, password_hash, role, created_at, updated_at) VALUES (1, 'a@example.com', 'A', 'x', 'user', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	// subject, created, updated: a is newest, b oldest; c was updated last.
	if _, err := conn.Exec(`INSERT INTO tickets (subject, description, requester_id, created_by, created_at, updated_at) VALUES
		('a', 'd', 1, 1, 300, 300), ('b', 'd', 1, 1, 100, 200), ('c', 'd', 1, 1, 200, 400)`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		sort string
		desc bool
		want string
	}{
		{"", false, "cab"}, // default: most recently updated
		{"updated", true, "cab"},
		{"updated", false, "bac"},
		{"created", true, "acb"},
		{"created", false, "bca"},
	} {
		list, _, err := s.ListTickets(ctx, TicketFilter{Sort: tc.sort, Descending: tc.desc, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		for _, tk := range list {
			got += tk.Subject
		}
		if got != tc.want {
			t.Errorf("sort %q desc=%v: got %s, want %s", tc.sort, tc.desc, got, tc.want)
		}
	}
}
