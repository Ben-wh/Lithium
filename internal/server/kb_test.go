package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func uploadKBImage(t *testing.T, c *http.Client, baseURL string, articleID int64, data []byte) (int, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "diagram.png")
	fw.Write(data)
	mw.Close()
	req, _ := http.NewRequest("POST", baseURL+"/api/kb/articles/"+itoa(articleID)+"/images", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", baseURL)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct{ URL string }
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out.URL
}

func TestKnowledgeBase(t *testing.T) {
	dataDir := t.TempDir()
	ts := newTestServerWithDataDir(t, dataDir)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	// Collections: staff manage them.
	var col store.KBCollection
	if code := postJSON(t, alice, ts.URL+"/api/kb/collections", `{"name":"Hardware"}`, nil); code != http.StatusForbidden {
		t.Fatalf("user creating collection: %d", code)
	}
	if code := postJSON(t, agent, ts.URL+"/api/kb/collections", `{"name":"Hardware","description":"Kit guides"}`, &col); code != http.StatusCreated {
		t.Fatalf("create collection: %d", code)
	}
	if code := postJSON(t, agent, ts.URL+"/api/kb/collections", `{"name":"hardware"}`, nil); code != http.StatusConflict {
		t.Fatalf("duplicate collection: %d", code)
	}

	// Articles start as staff-only drafts.
	var art store.KBArticle
	if code := postJSON(t, alice, ts.URL+"/api/kb/articles", `{}`, nil); code != http.StatusForbidden {
		t.Fatalf("user creating article: %d", code)
	}
	postJSON(t, agent, ts.URL+"/api/kb/articles", `{"collectionId":`+itoa(col.ID)+`}`, &art)
	if art.Status != "draft" || art.Visibility != "staff" || art.Collection == nil || art.Author == nil {
		t.Fatalf("new article: %+v", art)
	}
	url := ts.URL + "/api/kb/articles/" + itoa(art.ID)
	save := func(c *http.Client, body string) int {
		t.Helper()
		return do(t, c, "PUT", url, body)
	}
	for body, want := range map[string]int{
		`{"title":"","body":"x","visibility":"staff","status":"draft"}`:                     http.StatusBadRequest,
		`{"title":"T","body":"x","visibility":"public","status":"draft"}`:                   http.StatusBadRequest,
		`{"title":"T","body":"x","visibility":"staff","status":"live"}`:                     http.StatusBadRequest,
		`{"title":"T","body":"x","visibility":"staff","status":"draft","collectionId":999}`: http.StatusBadRequest,
	} {
		if code := save(agent, body); code != want {
			t.Fatalf("%s: got %d, want %d", body, code, want)
		}
	}
	if code := save(alice, `{"title":"T","body":"x","visibility":"staff","status":"draft"}`); code != http.StatusForbidden {
		t.Fatalf("user editing: %d", code)
	}

	// An image goes with the article.
	code, imgURL := uploadKBImage(t, agent, ts.URL, art.ID, fakePNG)
	if code != http.StatusCreated || imgURL == "" {
		t.Fatalf("upload: %d %q", code, imgURL)
	}
	if code, _ := uploadKBImage(t, agent, ts.URL, art.ID, []byte("not an image")); code != http.StatusBadRequest {
		t.Fatalf("non-image: %d", code)
	}
	body := `# Docking stations\n\nUse the **USB-C** port.\n\n![diagram](` + imgURL + `)`
	if code := save(agent, `{"title":"Setting up a dock","body":"`+body+`","collectionId":`+itoa(col.ID)+`,"visibility":"staff","status":"draft"}`); code != 200 {
		t.Fatalf("save: %d", code)
	}

	readable := func(c *http.Client) bool {
		t.Helper()
		var a store.KBArticle
		return getJSON(t, c, url, &a) == 200
	}
	listed := func(c *http.Client, query string) int {
		t.Helper()
		var list []store.KBArticle
		getJSON(t, c, ts.URL+"/api/kb/articles"+query, &list)
		return len(list)
	}

	// Drafts and staff-only articles are hidden from users, including images.
	if readable(alice) || listed(alice, "") != 0 {
		t.Fatal("user can see a staff-only draft")
	}
	if r := get(t, alice, ts.URL+imgURL); r.StatusCode != http.StatusNotFound {
		t.Fatalf("user fetching draft's image: %d", r.StatusCode)
	}
	if !readable(admin) || listed(agent, "?status=draft") != 1 {
		t.Fatal("staff can't see draft")
	}

	// Published + staff-only: still hidden from users.
	save(agent, `{"title":"Setting up a dock","body":"`+body+`","collectionId":`+itoa(col.ID)+`,"visibility":"staff","status":"published"}`)
	if readable(alice) {
		t.Fatal("user can see published staff-only article")
	}
	// Published for everyone: users can read it and its image.
	save(agent, `{"title":"Setting up a dock","body":"`+body+`","collectionId":`+itoa(col.ID)+`,"visibility":"everyone","status":"published"}`)
	if !readable(alice) {
		t.Fatal("user can't read public article")
	}
	resp, err := alice.Get(ts.URL + imgURL)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Equal(got, fakePNG) {
		t.Fatalf("user fetching public image: %d", resp.StatusCode)
	}
	var a store.KBArticle
	getJSON(t, alice, url, &a)
	if a.PublishedAt == nil || a.Body == "" {
		t.Fatalf("published article: %+v", a)
	}

	// Search, collection filter and excerpts.
	var list []store.KBArticle
	getJSON(t, alice, ts.URL+"/api/kb/articles?q=usb-c", &list)
	if len(list) != 1 || list[0].Body != "" || list[0].Excerpt == "" {
		t.Fatalf("search: %+v", list)
	}
	if listed(alice, "?collection="+itoa(col.ID)) != 1 || listed(alice, "?collection=none") != 0 {
		t.Fatal("collection filter")
	}
	var cols []store.KBCollection
	getJSON(t, alice, ts.URL+"/api/kb/collections", &cols)
	if len(cols) != 1 || cols[0].ArticleCount != 1 {
		t.Fatalf("collections for user: %+v", cols)
	}

	// Deleting a collection keeps its articles (now uncategorised).
	do(t, agent, "DELETE", ts.URL+"/api/kb/collections/"+itoa(col.ID), "")
	getJSON(t, agent, url, &a)
	if a.Collection != nil {
		t.Fatal("article still in deleted collection")
	}

	// Deleting the article removes its image files.
	if n := countFiles(t, filepath.Join(dataDir, "kb-images")); n != 1 {
		t.Fatalf("expected 1 image file, got %d", n)
	}
	if code := do(t, alice, "DELETE", url, ""); code != http.StatusForbidden {
		t.Fatalf("user delete: %d", code)
	}
	if code := do(t, agent, "DELETE", url, ""); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
	if n := countFiles(t, filepath.Join(dataDir, "kb-images")); n != 0 {
		t.Fatalf("image files left after delete: %d", n)
	}
	if r := get(t, agent, ts.URL+imgURL); r.StatusCode != http.StatusNotFound {
		t.Fatalf("image after delete: %d", r.StatusCode)
	}
}

func TestKnowledgeBaseAdminOnly(t *testing.T) {
	ts := newTestServerWithDataDir(t, t.TempDir())
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	agent, _ := newUser(t, ts.URL, admin, "Agent", "agent@example.com", "agent")

	var col store.KBCollection
	postJSON(t, admin, ts.URL+"/api/kb/collections", `{"name":"Runbooks"}`, &col)

	// Agents can't make an article admin-only.
	var mine store.KBArticle
	postJSON(t, agent, ts.URL+"/api/kb/articles", `{}`, &mine)
	if code := do(t, agent, "PUT", ts.URL+"/api/kb/articles/"+itoa(mine.ID),
		`{"title":"T","body":"x","visibility":"admins","status":"published"}`); code != http.StatusForbidden {
		t.Fatalf("agent making article admin-only: %d", code)
	}

	// An admin can.
	var art store.KBArticle
	postJSON(t, admin, ts.URL+"/api/kb/articles", `{"collectionId":`+itoa(col.ID)+`}`, &art)
	url := ts.URL + "/api/kb/articles/" + itoa(art.ID)
	code, imgURL := uploadKBImage(t, admin, ts.URL, art.ID, fakePNG)
	if code != http.StatusCreated {
		t.Fatalf("upload: %d", code)
	}
	if code := do(t, admin, "PUT", url, `{"title":"Restoring backups","body":"![x](`+imgURL+`)","collectionId":`+
		itoa(col.ID)+`,"visibility":"admins","status":"published"}`); code != 200 {
		t.Fatalf("admin saving admin-only article: %d", code)
	}

	listed := func(c *http.Client) int {
		t.Helper()
		var list []store.KBArticle
		getJSON(t, c, ts.URL+"/api/kb/articles?collection="+itoa(col.ID), &list)
		return len(list)
	}
	counted := func(c *http.Client) int {
		t.Helper()
		var cols []store.KBCollection
		getJSON(t, c, ts.URL+"/api/kb/collections", &cols)
		return cols[0].ArticleCount
	}

	// Admins see it; agents and users don't, anywhere.
	var a store.KBArticle
	if getJSON(t, admin, url, &a) != 200 || a.Visibility != "admins" || listed(admin) != 1 || counted(admin) != 1 {
		t.Fatalf("admin can't see admin-only article: %+v", a)
	}
	for name, c := range map[string]*http.Client{"agent": agent, "user": alice} {
		if code := getJSON(t, c, url, &store.KBArticle{}); code != http.StatusNotFound {
			t.Fatalf("%s reading: %d", name, code)
		}
		if listed(c) != 0 || counted(c) != 0 {
			t.Fatalf("%s sees admin-only article in lists", name)
		}
		if r := get(t, c, ts.URL+imgURL); r.StatusCode != http.StatusNotFound {
			t.Fatalf("%s fetching image: %d", name, r.StatusCode)
		}
	}

	// Agents can't change, add images to or delete it.
	if code := do(t, agent, "PUT", url, `{"title":"T","body":"x","visibility":"staff","status":"published"}`); code != http.StatusNotFound {
		t.Fatalf("agent editing admin-only article: %d", code)
	}
	if code, _ := uploadKBImage(t, agent, ts.URL, art.ID, fakePNG); code != http.StatusNotFound {
		t.Fatalf("agent uploading to admin-only article: %d", code)
	}
	if code := do(t, agent, "DELETE", url, ""); code != http.StatusNotFound {
		t.Fatalf("agent deleting admin-only article: %d", code)
	}
	if code := do(t, admin, "DELETE", url, ""); code != http.StatusNoContent {
		t.Fatalf("admin deleting: %d", code)
	}
}
