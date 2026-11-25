// Package seed holds example content added to brand-new installs.
package seed

import (
	"context"
	_ "embed"

	"github.com/lithium/lithium/internal/store"
)

var (
	//go:embed how-to-use-lithium-users.md
	usersGuide string
	//go:embed how-to-use-lithium-agents.md
	agentsGuide string
	//go:embed how-to-use-lithium-admins.md
	adminsGuide string
)

// article is one seeded knowledge base article.
type article struct {
	Title      string
	Body       string
	Visibility string // "everyone", "staff" or "admins"
}

// articles are the "How to use Lithium" guides: one for each role, each
// readable by that role and anyone above it.
var articles = []article{
	{"How to use Lithium", usersGuide, "everyone"},
	{"How to use Lithium: for agents", agentsGuide, "staff"},
	{"How to use Lithium: for admins", adminsGuide, "admins"},
}

// KnowledgeBase adds a "Getting started" collection with the "How to use
// Lithium" guides, but only to an empty knowledge base. authorID may be 0.
func KnowledgeBase(ctx context.Context, st *store.Store, authorID int64) (bool, error) {
	cols, err := st.ListKBCollections(ctx, store.KBAdmin)
	if err != nil {
		return false, err
	}
	existing, err := st.ListKBArticles(ctx, store.KBFilter{Reader: store.KBAdmin})
	if err != nil {
		return false, err
	}
	if len(cols) > 0 || len(existing) > 0 {
		return false, nil
	}
	col, err := st.SaveKBCollection(ctx, 0, "Getting started", "How to use Lithium and get help quickly")
	if err != nil {
		return false, err
	}
	for _, art := range articles {
		a, err := st.CreateKBArticle(ctx, art.Title, col.ID, authorID)
		if err != nil {
			return false, err
		}
		if _, err := st.UpdateKBArticle(ctx, a.ID, authorID, store.KBArticleUpdate{
			Title:        art.Title,
			Body:         art.Body,
			CollectionID: col.ID,
			Visibility:   art.Visibility,
			Status:       "published",
		}); err != nil {
			return false, err
		}
	}
	return true, nil
}
