package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/lithium/lithium/internal/config"
	"github.com/lithium/lithium/internal/store"
)

// firstAdminServer is a new install as bootstrapAdmin leaves it: mandatory
// two-factor on, and the first admin (admin@example.com) allowed to skip it once.
func firstAdminServer(t *testing.T) (baseURL string, st *store.Store, adminID int64) {
	t.Helper()
	ts, st := newTestServerWithConfig(t, config.Config{DataDir: t.TempDir()})
	ctx := context.Background()
	admin, err := st.UserByEmail(ctx, "admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	on := true
	if _, err := st.UpdateSettings(ctx, store.SettingsUpdate{RequireTwoFactor: &on}); err != nil {
		t.Fatal(err)
	}
	if err := st.AllowFirstAdminTwoFactorSkip(ctx, admin.ID); err != nil {
		t.Fatal(err)
	}
	return ts.URL, st, admin.ID
}

func TestFirstAdminCanSkipTwoFactorOnce(t *testing.T) {
	baseURL, st, _ := firstAdminServer(t)
	admin := signIn(t, baseURL, "admin@example.com", "correct-horse")

	if u := me(t, admin, baseURL); u.TwoFactor != "setup" || !u.CanSkipTwoFactor {
		t.Fatalf("want setup state with skip offered, got %+v", u)
	}
	if code := get(t, admin, baseURL+"/api/tickets").StatusCode; code != 403 {
		t.Fatalf("tickets before skipping: got %d", code)
	}

	var u store.User
	if code := postJSON(t, admin, baseURL+"/api/auth/2fa/skip", `{}`, &u); code != 200 {
		t.Fatalf("skip: got %d", code)
	}
	if u.TwoFactor != "" || u.CanSkipTwoFactor {
		t.Fatalf("after skip: %+v", u)
	}
	if code := get(t, admin, baseURL+"/api/tickets").StatusCode; code != 200 {
		t.Fatalf("tickets after skipping: got %d", code)
	}
	settings, _ := st.Settings(context.Background())
	if settings.RequireTwoFactor {
		t.Fatal("skipping should turn mandatory two-factor off")
	}

	// One time only: with mandatory two-factor back on, there's no skip.
	setRequireTwoFactor(t, admin, baseURL, true)
	if u := me(t, admin, baseURL); u.TwoFactor != "setup" || u.CanSkipTwoFactor {
		t.Fatalf("skip offered a second time: %+v", u)
	}
	if code := postJSON(t, admin, baseURL+"/api/auth/2fa/skip", `{}`, nil); code != 403 {
		t.Fatalf("second skip: got %d", code)
	}
}

func TestOnlyFirstAdminCanSkipTwoFactor(t *testing.T) {
	baseURL, st, _ := firstAdminServer(t)
	ctx := context.Background()

	// Make a second admin and a user while two-factor is briefly optional.
	off, on := false, true
	if _, err := st.UpdateSettings(ctx, store.SettingsUpdate{RequireTwoFactor: &off}); err != nil {
		t.Fatal(err)
	}
	first := signIn(t, baseURL, "admin@example.com", "correct-horse")
	other, _ := newUser(t, baseURL, first, "Other", "other@example.com", "admin")
	alice, _ := newUser(t, baseURL, first, "Alice", "alice@example.com", "user")
	if _, err := st.UpdateSettings(ctx, store.SettingsUpdate{RequireTwoFactor: &on}); err != nil {
		t.Fatal(err)
	}

	for name, c := range map[string]*http.Client{"other admin": other, "user": alice} {
		if u := me(t, c, baseURL); u.TwoFactor != "setup" || u.CanSkipTwoFactor {
			t.Fatalf("%s: want setup state without skip, got %+v", name, u)
		}
		if code := postJSON(t, c, baseURL+"/api/auth/2fa/skip", `{}`, nil); code != 403 {
			t.Fatalf("%s skip: got %d", name, code)
		}
	}
	if s, _ := st.Settings(ctx); !s.RequireTwoFactor {
		t.Fatal("mandatory two-factor was turned off")
	}
}

func TestFirstAdminSkipEndsWhenTwoFactorIsSetUp(t *testing.T) {
	baseURL, _, _ := firstAdminServer(t)
	admin := signIn(t, baseURL, "admin@example.com", "correct-horse")
	enrol(t, admin, baseURL)
	if code := postJSON(t, admin, baseURL+"/api/auth/2fa/skip", `{}`, nil); code != 403 {
		t.Fatalf("skip after enrolling: got %d", code)
	}
}
