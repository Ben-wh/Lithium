package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lithium/lithium/internal/store"
)

// A PNG signature followed by padding is enough for content sniffing.
var fakePNG = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)

type upload struct {
	name string
	data []byte
}

// raiseWithImages posts a multipart ticket. origin "" sends no Origin header.
func raiseWithImages(t *testing.T, c *http.Client, baseURL, origin string, files ...upload) (int, store.Ticket, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("payload", `{"subject":"With images","description":"See attached"}`)
	for _, f := range files {
		fw, _ := mw.CreateFormFile("images", f.name)
		fw.Write(f.data)
	}
	mw.Close()

	req, _ := http.NewRequest("POST", baseURL+"/api/tickets", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var tk store.Ticket
	if resp.StatusCode == http.StatusCreated {
		json.Unmarshal(raw, &tk)
	}
	return resp.StatusCode, tk, string(raw)
}

func countFiles(t *testing.T, dir string) int {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0
	} else if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestImageUploads(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	bob, _ := newUser(t, ts.URL, admin, "Bob", "bob@example.com", "user")

	// On by default.
	var st store.Settings
	getJSON(t, alice, ts.URL+"/api/settings", &st)
	if !st.AllowImageUploads {
		t.Fatal("image uploads should be allowed by default")
	}

	// Two images attach to the ticket.
	code, tk, body := raiseWithImages(t, alice, ts.URL, ts.URL,
		upload{"screen shot.png", fakePNG}, upload{`C:\Users\a\photo.png`, fakePNG})
	if code != http.StatusCreated {
		t.Fatalf("upload: %d %s", code, body)
	}
	if len(tk.Attachments) != 2 || tk.Attachments[0].Filename != "screen shot.png" ||
		tk.Attachments[1].Filename != "photo.png" || tk.Attachments[0].ContentType != "image/png" {
		t.Fatalf("attachments: %+v", tk.Attachments)
	}

	// The requester and staff can fetch them; other users can't.
	url := ts.URL + "/api/attachments/" + itoa(tk.Attachments[0].ID)
	for _, c := range []*http.Client{alice, admin} {
		resp, err := c.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !bytes.Equal(got, fakePNG) || resp.Header.Get("Content-Type") != "image/png" ||
			!strings.Contains(resp.Header.Get("Content-Security-Policy"), "sandbox") {
			t.Fatalf("fetch: %d %q", resp.StatusCode, resp.Header)
		}
	}
	if r := get(t, bob, url); r.StatusCode != http.StatusNotFound {
		t.Fatalf("other user fetch: %d", r.StatusCode)
	}
	if r := get(t, http.DefaultClient, url); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous fetch: %d", r.StatusCode)
	}

	// Rejections leave nothing behind on disk.
	six := make([]upload, 6)
	for i := range six {
		six[i] = upload{"x.png", fakePNG}
	}
	cases := []struct {
		name   string
		origin string
		files  []upload
		want   int
	}{
		{"too many", ts.URL, six, http.StatusBadRequest},
		{"not an image", ts.URL, []upload{{"notes.png", []byte("just some text pretending to be a png")}}, http.StatusBadRequest},
		{"svg", ts.URL, []upload{{"logo.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)}}, http.StatusBadRequest},
		{"good then bad", ts.URL, []upload{{"ok.png", fakePNG}, {"bad.png", []byte("nope")}}, http.StatusBadRequest},
		{"too big", ts.URL, []upload{{"huge.png", append(append([]byte{}, fakePNG...), make([]byte, maxImageBytes)...)}}, http.StatusBadRequest},
		{"cross-site", "https://evil.example", []upload{{"x.png", fakePNG}}, http.StatusForbidden},
		{"no origin", "", []upload{{"x.png", fakePNG}}, http.StatusForbidden},
	}
	for _, c := range cases {
		if code, _, body := raiseWithImages(t, alice, ts.URL, c.origin, c.files...); code != c.want {
			t.Fatalf("%s: got %d (%s), want %d", c.name, code, body, c.want)
		}
	}

	// None of the rejected uploads created a ticket.
	var list listResp
	getJSON(t, alice, ts.URL+"/api/tickets", &list)
	if list.Total != 1 {
		t.Fatalf("rejected uploads created tickets: %d", list.Total)
	}

	// Turning uploads off blocks them, but plain tickets still work.
	do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"allowImageUploads":false}`)
	if code, _, _ := raiseWithImages(t, alice, ts.URL, ts.URL, upload{"x.png", fakePNG}); code != http.StatusBadRequest {
		t.Fatalf("uploads off: %d", code)
	}
	if code := postJSON(t, alice, ts.URL+"/api/tickets", `{"subject":"s","description":"d"}`, nil); code != http.StatusCreated {
		t.Fatalf("plain ticket with uploads off: %d", code)
	}
}

func TestFailedUploadsLeaveNoFiles(t *testing.T) {
	dataDir := t.TempDir()
	ts := newTestServerWithDataDir(t, dataDir)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	attachments := filepath.Join(dataDir, "attachments")

	raiseWithImages(t, admin, ts.URL, ts.URL, upload{"ok.png", fakePNG}, upload{"bad.png", []byte("nope")})
	if n := countFiles(t, attachments); n != 0 {
		t.Fatalf("%d files left after a rejected upload", n)
	}
	raiseWithImages(t, admin, ts.URL, ts.URL, upload{"ok.png", fakePNG})
	if n := countFiles(t, attachments); n != 1 {
		t.Fatalf("expected 1 stored file, got %d", n)
	}
}
