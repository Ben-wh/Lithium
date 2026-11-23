package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

func uploadAvatar(t *testing.T, c *http.Client, baseURL string, data []byte) (int, store.User) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "me.png")
	fw.Write(data)
	mw.Close()
	req, _ := http.NewRequest("POST", baseURL+"/api/profile/avatar", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", baseURL)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var u store.User
	if resp.StatusCode == 200 {
		json.NewDecoder(resp.Body).Decode(&u)
	}
	return resp.StatusCode, u
}

func TestAvatars(t *testing.T) {
	dataDir := t.TempDir()
	ts := newTestServerWithDataDir(t, dataDir)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, aliceUser := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, _ := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")
	avatars := filepath.Join(dataDir, "avatars")

	if me(t, alice, ts.URL).AvatarURL != "" {
		t.Fatal("new users should have no avatar")
	}

	// Presets.
	if code := do(t, alice, "PUT", ts.URL+"/api/profile/avatar", `{"preset":3}`); code != 200 {
		t.Fatalf("preset: %d", code)
	}
	if got := me(t, alice, ts.URL).AvatarURL; got != "/avatars/3.svg" {
		t.Fatalf("preset url: %q", got)
	}
	for _, body := range []string{`{"preset":11}`, `{"preset":-1}`} {
		if code := do(t, alice, "PUT", ts.URL+"/api/profile/avatar", body); code != http.StatusBadRequest {
			t.Fatalf("%s: %d", body, code)
		}
	}

	// Custom upload, visible to other signed-in users.
	code, u := uploadAvatar(t, alice, ts.URL, fakePNG)
	if code != 200 || !strings.HasPrefix(u.AvatarURL, "/api/avatars/"+itoa(aliceUser.ID)+"?v=") {
		t.Fatalf("upload: %d %q", code, u.AvatarURL)
	}
	resp, err := bob.Get(ts.URL + u.AvatarURL)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Equal(got, fakePNG) || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("fetch avatar: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if n := countFiles(t, avatars); n != 1 {
		t.Fatalf("expected 1 avatar file, got %d", n)
	}

	// Replacing it removes the old file; bad uploads are rejected and leave nothing.
	uploadAvatar(t, alice, ts.URL, fakePNG)
	if n := countFiles(t, avatars); n != 1 {
		t.Fatalf("old avatar not removed: %d files", n)
	}
	if code, _ := uploadAvatar(t, alice, ts.URL, []byte("not an image")); code != http.StatusBadRequest {
		t.Fatalf("non-image: %d", code)
	}
	if code, _ := uploadAvatar(t, alice, ts.URL, append(append([]byte{}, fakePNG...), make([]byte, maxAvatarBytes)...)); code == 200 {
		t.Fatal("oversized avatar accepted")
	}
	if n := countFiles(t, avatars); n != 1 {
		t.Fatalf("rejected uploads left files: %d", n)
	}

	// Avatars appear wherever the user does.
	var tk store.Ticket
	postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d"}`, &tk)
	postJSON(t, alice, ts.URL+"/api/tickets/"+itoa(tk.ID)+"/comments", `{"body":"hi"}`, nil)
	if tk.Requester.AvatarURL == "" {
		t.Fatal("requester avatar missing on ticket")
	}
	if c := comments(t, admin, ts.URL, tk.ID); c[0].Author.AvatarURL == "" {
		t.Fatal("author avatar missing on comment")
	}

	// Disabling custom pictures: uploads refused and existing ones hidden,
	// but presets still work.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"disableCustomAvatars":true}`)
	if code, _ := uploadAvatar(t, bob, ts.URL, fakePNG); code != http.StatusForbidden {
		t.Fatalf("upload while disabled: %d", code)
	}
	if r := get(t, bob, ts.URL+me(t, alice, ts.URL).AvatarURL); r.StatusCode != http.StatusNotFound {
		t.Fatalf("custom avatar served while disabled: %d", r.StatusCode)
	}
	if code := do(t, bob, "PUT", ts.URL+"/api/profile/avatar", `{"preset":7}`); code != 200 {
		t.Fatalf("preset while disabled: %d", code)
	}

	// Clearing goes back to initials and deletes the custom file.
	do(t, alice, "PUT", ts.URL+"/api/profile/avatar", `{"preset":0}`)
	if me(t, alice, ts.URL).AvatarURL != "" {
		t.Fatal("avatar not cleared")
	}
	if n := countFiles(t, avatars); n != 0 {
		t.Fatalf("custom file kept after clearing: %d", n)
	}
	if r := get(t, http.DefaultClient, ts.URL+"/api/avatars/"+itoa(aliceUser.ID)); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous avatar fetch: %d", r.StatusCode)
	}
}
