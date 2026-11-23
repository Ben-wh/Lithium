package server

import (
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/lithium/lithium/internal/auth"
	"github.com/lithium/lithium/internal/store"
)

func setRequireTwoFactor(t *testing.T, admin *http.Client, baseURL string, on bool) {
	t.Helper()
	body := `{"requireTwoFactor":false}`
	if on {
		body = `{"requireTwoFactor":true}`
	}
	if code := do(t, admin, "PATCH", baseURL+"/api/admin/settings", body); code != 200 {
		t.Fatalf("set requireTwoFactor: got %d", code)
	}
}

// enrol sets up two-factor for a signed-in client and returns the secret and recovery codes.
func enrol(t *testing.T, c *http.Client, baseURL string) (string, []string) {
	t.Helper()
	var setup struct{ Secret, URI, QR string }
	if code := postJSON(t, c, baseURL+"/api/auth/2fa/setup", `{}`, &setup); code != 200 {
		t.Fatalf("start setup: got %d", code)
	}
	if setup.Secret == "" || !strings.HasPrefix(setup.URI, "otpauth://totp/Lithium:") || !strings.HasPrefix(setup.QR, "data:image/svg+xml;base64,") {
		t.Fatalf("bad setup response %+v", setup)
	}
	if code := postJSON(t, c, baseURL+"/api/auth/2fa/enable", `{"code":"000000"}`, nil); code != 400 {
		t.Fatalf("wrong code enabled two-factor: %d", code)
	}
	var res struct{ RecoveryCodes []string }
	code, _ := auth.TOTPCode(setup.Secret, auth.TOTPStep(time.Now()))
	if c := postJSON(t, c, baseURL+"/api/auth/2fa/enable", `{"code":"`+code+`"}`, &res); c != 200 {
		t.Fatalf("enable: got %d", c)
	}
	if len(res.RecoveryCodes) != 10 {
		t.Fatalf("want 10 recovery codes, got %d", len(res.RecoveryCodes))
	}
	return setup.Secret, res.RecoveryCodes
}

func passwordStep(t *testing.T, baseURL, email, password string) (*http.Client, store.User) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	var u store.User
	if code := postJSON(t, c, baseURL+"/api/auth/login", `{"email":"`+email+`","password":"`+password+`"}`, &u); code != 200 {
		t.Fatalf("login: got %d", code)
	}
	return c, u
}

func TestMandatoryTwoFactorSetup(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	setRequireTwoFactor(t, admin, ts.URL, true)

	// Already signed in without 2FA: blocked until it's set up.
	if u := me(t, alice, ts.URL); u.TwoFactor != "setup" {
		t.Fatalf("want setup state, got %q", u.TwoFactor)
	}
	if code := get(t, alice, ts.URL+"/api/tickets").StatusCode; code != 403 {
		t.Fatalf("tickets before setup: got %d", code)
	}
	// Can't turn it off while it's mandatory.
	enrol(t, alice, ts.URL)
	if u := me(t, alice, ts.URL); u.TwoFactor != "" || !u.TwoFactorEnabled {
		t.Fatalf("after setup: %+v", u)
	}
	if code := get(t, alice, ts.URL+"/api/tickets").StatusCode; code != 200 {
		t.Fatalf("tickets after setup: got %d", code)
	}
	if code := postJSON(t, alice, ts.URL+"/api/auth/2fa/disable", `{"password":"chosen-password"}`, nil); code != 403 {
		t.Fatalf("disable while mandatory: got %d", code)
	}
	if code := postJSON(t, alice, ts.URL+"/api/auth/2fa/setup", `{}`, nil); code != 409 {
		t.Fatalf("setup when enabled: got %d", code)
	}
}

func TestTwoFactorLogin(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, _ := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	secret, _ := enrol(t, alice, ts.URL)

	// Password alone leaves a pending session that can only enter a code.
	c, u := passwordStep(t, ts.URL, "alice@example.com", "chosen-password")
	if u.TwoFactor != "verify" {
		t.Fatalf("want verify state, got %q", u.TwoFactor)
	}
	if code := get(t, c, ts.URL+"/api/tickets").StatusCode; code != 403 {
		t.Fatalf("pending session reached tickets: %d", code)
	}
	if code := post(t, c, ts.URL+"/api/auth/2fa/setup", `{}`).StatusCode; code != 403 {
		t.Fatalf("pending session reached setup: %d", code)
	}
	if code := postJSON(t, c, ts.URL+"/api/auth/2fa/verify", `{"code":"123456"}`, nil); code != 400 {
		t.Fatalf("wrong code: got %d", code)
	}
	// The enrolment code's step was used, so wait-free: use the next step's code (allowed drift).
	next, _ := auth.TOTPCode(secret, auth.TOTPStep(time.Now())+1)
	if code := postJSON(t, c, ts.URL+"/api/auth/2fa/verify", `{"code":"`+next+`"}`, &u); code != 200 || u.TwoFactor != "" {
		t.Fatalf("verify: got %d %+v", code, u)
	}
	if code := get(t, c, ts.URL+"/api/tickets").StatusCode; code != 200 {
		t.Fatalf("after verify: got %d", code)
	}

}

func TestTwoFactorCodesAreSingleUse(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	secret, recovery := enrol(t, admin, ts.URL)
	next, _ := auth.TOTPCode(secret, auth.TOTPStep(time.Now())+1)
	c, _ := passwordStep(t, ts.URL, "admin@example.com", "correct-horse")
	if code := postJSON(t, c, ts.URL+"/api/auth/2fa/verify", `{"code":"`+next+`"}`, nil); code != 200 {
		t.Fatalf("verify: got %d", code)
	}

	// The same code can't be used twice.
	c2, _ := passwordStep(t, ts.URL, "admin@example.com", "correct-horse")
	if code := postJSON(t, c2, ts.URL+"/api/auth/2fa/verify", `{"code":"`+next+`"}`, nil); code != 400 {
		t.Fatalf("replayed code: got %d", code)
	}
	// A recovery code works once, typed loosely.
	loose := strings.ToUpper(strings.ReplaceAll(recovery[0], "-", " "))
	if code := postJSON(t, c2, ts.URL+"/api/auth/2fa/verify", `{"code":"`+loose+`"}`, nil); code != 200 {
		t.Fatalf("recovery code: got %d", code)
	}
	var status twoFactorStatus
	getJSON(t, c2, ts.URL+"/api/auth/2fa", &status)
	if !status.Enabled || status.RecoveryCodesLeft != 9 {
		t.Fatalf("status %+v", status)
	}
	c3, _ := passwordStep(t, ts.URL, "admin@example.com", "correct-horse")
	if code := postJSON(t, c3, ts.URL+"/api/auth/2fa/verify", `{"code":"`+recovery[0]+`"}`, nil); code != 400 {
		t.Fatalf("reused recovery code: got %d", code)
	}

}

func TestTwoFactorLockoutAndAdminReset(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	alice, aliceUser := newUser(t, ts.URL, admin, "Alice", "alice@example.com", "user")
	enrol(t, alice, ts.URL)

	// Too many wrong codes end the pending session.
	c4, _ := passwordStep(t, ts.URL, "alice@example.com", "chosen-password")
	var last int
	for i := 0; i < maxMFAAttempts; i++ {
		last = postJSON(t, c4, ts.URL+"/api/auth/2fa/verify", `{"code":"aaaaa-aaaaa"}`, nil)
	}
	if last != 401 {
		t.Fatalf("after %d wrong codes: got %d", maxMFAAttempts, last)
	}
	if code := get(t, c4, ts.URL+"/api/auth/me").StatusCode; code != 401 {
		t.Fatalf("session survived: %d", code)
	}

	// Admin reset: signed out everywhere, and two-factor is off.
	if code := postJSON(t, admin, ts.URL+"/api/admin/users/"+itoa(aliceUser.ID)+"/2fa/reset", `{}`, nil); code != 204 {
		t.Fatalf("admin reset: got %d", code)
	}
	if code := get(t, alice, ts.URL+"/api/auth/me").StatusCode; code != 401 {
		t.Fatalf("session kept after reset: %d", code)
	}
	if _, u := passwordStep(t, ts.URL, "alice@example.com", "chosen-password"); u.TwoFactor != "" || u.TwoFactorEnabled {
		t.Fatalf("after reset: %+v", u)
	}
	// Only admins can reset.
	alice = signIn(t, ts.URL, "alice@example.com", "chosen-password")
	if code := postJSON(t, alice, ts.URL+"/api/admin/users/1/2fa/reset", `{}`, nil); code != 401 && code != 403 {
		t.Fatalf("non-admin reset: got %d", code)
	}
}

func TestDisableTwoFactorAndNewCodes(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")
	_, recovery := enrol(t, admin, ts.URL)

	var res struct{ RecoveryCodes []string }
	if code := postJSON(t, admin, ts.URL+"/api/auth/2fa/recovery-codes", `{"password":"wrong"}`, nil); code != 400 {
		t.Fatalf("new codes with wrong password: %d", code)
	}
	if code := postJSON(t, admin, ts.URL+"/api/auth/2fa/recovery-codes", `{"password":"correct-horse"}`, &res); code != 200 || len(res.RecoveryCodes) != 10 || res.RecoveryCodes[0] == recovery[0] {
		t.Fatalf("new codes: %d", code)
	}
	if code := postJSON(t, admin, ts.URL+"/api/auth/2fa/disable", `{"password":"correct-horse"}`, nil); code != 204 {
		t.Fatalf("disable: %d", code)
	}
	// Still signed in, and the next login needs no code.
	if u := me(t, admin, ts.URL); u.TwoFactorEnabled {
		t.Fatal("still enabled")
	}
	if _, u := passwordStep(t, ts.URL, "admin@example.com", "correct-horse"); u.TwoFactor != "" {
		t.Fatalf("login after disable: %+v", u)
	}
}
