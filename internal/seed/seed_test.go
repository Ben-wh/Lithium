package seed

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/db"
	"github.com/lithium/lithium/internal/store"
)

func TestKnowledgeBaseSeedsOnceIntoEmptyKB(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	admin, err := st.CreateUser(ctx, "admin@example.com", "Admin", "x", "admin", false)
	if err != nil {
		t.Fatal(err)
	}

	added, err := KnowledgeBase(ctx, st, admin.ID)
	if err != nil || !added {
		t.Fatalf("first seed: added=%v err=%v", added, err)
	}

	// Admins see all three guides.
	all, err := st.ListKBArticles(ctx, store.KBFilter{Reader: store.KBAdmin})
	if err != nil || len(all) != 3 {
		t.Fatalf("admin articles: %v %+v", err, all)
	}
	want := map[string]string{
		"How to use Lithium":             "## Raising a ticket",
		"How to use Lithium: for agents": "## Replies and internal notes",
		"How to use Lithium: for admins": "## Lockdown mode",
	}
	for _, s := range all {
		a, err := st.KBArticleByID(ctx, s.ID)
		if err != nil {
			t.Fatal(err)
		}
		heading, ok := want[a.Title]
		if !ok || a.Collection == nil || a.Collection.Name != "Getting started" || a.Status != "published" ||
			a.Author == nil || a.Author.ID != admin.ID || !strings.Contains(a.Body, heading) {
			t.Fatalf("seeded article: %+v", a)
		}
		delete(want, a.Title)
	}

	// Agents see the user and agent guides; regular users only the user guide.
	staff, err := st.ListKBArticles(ctx, store.KBFilter{Reader: store.KBStaff})
	if err != nil || len(staff) != 2 {
		t.Fatalf("staff articles: %v %+v", err, staff)
	}
	for _, a := range staff {
		if a.Title == "How to use Lithium: for admins" {
			t.Fatal("agents can see the admin guide")
		}
	}
	public, err := st.ListKBArticles(ctx, store.KBFilter{Reader: store.KBUser})
	if err != nil || len(public) != 1 || public[0].Title != "How to use Lithium" {
		t.Fatalf("public articles: %v %+v", err, public)
	}

	// Never seeds twice, or into a knowledge base that already has content.
	if added, err := KnowledgeBase(ctx, st, admin.ID); err != nil || added {
		t.Fatalf("second seed: added=%v err=%v", added, err)
	}
}
