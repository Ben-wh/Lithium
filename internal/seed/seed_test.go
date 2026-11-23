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
	// Readable by regular users: published and shared with everyone.
	public, err := st.ListKBArticles(ctx, store.KBFilter{Staff: false})
	if err != nil || len(public) != 1 {
		t.Fatalf("public articles: %v %+v", err, public)
	}
	a, err := st.KBArticleByID(ctx, public[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if a.Title != "How to raise a good ticket" || a.Collection == nil || a.Collection.Name != "Getting started" ||
		a.Author == nil || a.Author.ID != admin.ID || !strings.Contains(a.Body, "## Choosing a priority") {
		t.Fatalf("seeded article: %+v", a)
	}

	// Never seeds twice, or into a knowledge base that already has content.
	if added, err := KnowledgeBase(ctx, st, admin.ID); err != nil || added {
		t.Fatalf("second seed: added=%v err=%v", added, err)
	}
}
