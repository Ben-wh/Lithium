// Package seed holds example content added to brand-new installs.
package seed

import (
	"context"
	_ "embed"

	"github.com/lithium/lithium/internal/store"
)

//go:embed how-to-raise-a-good-ticket.md
var exampleArticle string

// KnowledgeBase adds a "Getting started" collection with an example article,
// but only to an empty knowledge base. authorID may be 0.
func KnowledgeBase(ctx context.Context, st *store.Store, authorID int64) (bool, error) {
	cols, err := st.ListKBCollections(ctx, true)
	if err != nil {
		return false, err
	}
	articles, err := st.ListKBArticles(ctx, store.KBFilter{Staff: true})
	if err != nil {
		return false, err
	}
	if len(cols) > 0 || len(articles) > 0 {
		return false, nil
	}
	col, err := st.SaveKBCollection(ctx, 0, "Getting started", "How to use Lithium and get help quickly")
	if err != nil {
		return false, err
	}
	a, err := st.CreateKBArticle(ctx, "How to raise a good ticket", col.ID, authorID)
	if err != nil {
		return false, err
	}
	_, err = st.UpdateKBArticle(ctx, a.ID, authorID, store.KBArticleUpdate{
		Title:        "How to raise a good ticket",
		Body:         exampleArticle,
		CollectionID: col.ID,
		Visibility:   "everyone",
		Status:       "published",
	})
	return err == nil, err
}
