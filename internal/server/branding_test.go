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

func uploadLogo(t *testing.T, c *http.Client, baseURL string, data []byte) (int, store.Settings) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("image", "logo.png")
	fw.Write(data)
	mw.Close()
	req, _ := http.NewRequest("POST", baseURL+"/api/admin/branding/logo", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", baseURL)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var st store.Settings
	if resp.StatusCode == 200 {
		json.NewDecoder(resp.Body).Decode(&st)
	}
	return resp.StatusCode, st
}

func TestBranding(t *testing.T) {
	dataDir := t.TempDir()
	ts := newTestServerWithDataDir(t, dataDir)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	agent, _ := newUser(t, ts.URL, admin, "Agnes", "agnes@example.com", "agent")
	dir := filepath.Join(dataDir, "branding")
	var b map[string]any

	// Nothing set by default, and readable signed out.
	if code := getJSON(t, http.DefaultClient, ts.URL+"/api/public/branding", &b); code != 200 || b["companyName"] != "" || b["logoUrl"] != "" || b["accent"] != "coral" {
		t.Fatalf("default branding: %d %v", code, b)
	}

	// Company name: trimmed, length-limited, admin only.
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"companyName":"  Acme   Corp "}`); code != 200 {
		t.Fatalf("set name: %d", code)
	}
	getJSON(t, http.DefaultClient, ts.URL+"/api/public/branding", &b)
	if b["companyName"] != "Acme Corp" {
		t.Fatalf("name: %v", b)
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"companyName":"`+strings.Repeat("x", 61)+`"}`); code != http.StatusBadRequest {
		t.Fatalf("long name: %d", code)
	}
	if code := do(t, agent, "PATCH", ts.URL+"/api/admin/settings", `{"companyName":"Hijack"}`); code != http.StatusForbidden {
		t.Fatalf("agent set name: %d", code)
	}

	// Accent colour: only known pastels, visible signed out.
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"accent":"mint"}`); code != 200 {
		t.Fatalf("set accent: %d", code)
	}
	getJSON(t, http.DefaultClient, ts.URL+"/api/public/branding", &b)
	if b["accent"] != "mint" {
		t.Fatalf("accent: %v", b)
	}
	if code := do(t, admin, "PATCH", ts.URL+"/api/admin/settings", `{"accent":"neon-green"}`); code != http.StatusBadRequest {
		t.Fatalf("unknown accent: %d", code)
	}

	// Logo: only admins, only real images.
	if code, _ := uploadLogo(t, agent, ts.URL, fakePNG); code != http.StatusForbidden {
		t.Fatalf("agent upload: %d", code)
	}
	if code, _ := uploadLogo(t, admin, ts.URL, []byte("<svg onload=alert(1)>")); code != http.StatusBadRequest {
		t.Fatalf("non-image upload: %d", code)
	}
	code, st := uploadLogo(t, admin, ts.URL, fakePNG)
	if code != 200 || !strings.HasPrefix(st.LogoURL, "/api/public/branding/logo?v=") {
		t.Fatalf("upload: %d %q", code, st.LogoURL)
	}

	// Served to signed-out visitors (the sign-in page shows it).
	resp, err := http.Get(ts.URL + st.LogoURL)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Equal(got, fakePNG) || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("fetch logo: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	// Replacing removes the old file.
	_, st2 := uploadLogo(t, admin, ts.URL, fakePNG)
	if st2.LogoURL == st.LogoURL {
		t.Fatal("logo URL should change on re-upload")
	}
	if n := countFiles(t, dir); n != 1 {
		t.Fatalf("expected 1 logo file, got %d", n)
	}

	// Removing goes back to the Lithium logo.
	if code := do(t, agent, "DELETE", ts.URL+"/api/admin/branding/logo", ""); code != http.StatusForbidden {
		t.Fatalf("agent delete: %d", code)
	}
	if code := do(t, admin, "DELETE", ts.URL+"/api/admin/branding/logo", ""); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	getJSON(t, http.DefaultClient, ts.URL+"/api/public/branding", &b)
	if b["logoUrl"] != "" {
		t.Fatalf("logo still set: %v", b)
	}
	if n := countFiles(t, dir); n != 0 {
		t.Fatalf("expected no logo files, got %d", n)
	}
	if resp := get(t, http.DefaultClient, ts.URL+"/api/public/branding/logo"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("removed logo still served: %d", resp.StatusCode)
	}
}
