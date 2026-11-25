package server

import (
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lithium/lithium/internal/config"
)

func TestAccountThrottleBackoff(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	th := &accountThrottle{entries: map[string]*failureEntry{}, now: func() time.Time { return now }}

	for i := 1; i < freeFailures; i++ {
		if wait := th.fail("k"); wait != 0 {
			t.Fatalf("failure %d: wait %s, want none", i, wait)
		}
	}
	if th.wait("k") != 0 {
		t.Fatal("waiting before the free failures are used up")
	}
	// The next failure starts the wait, and each one after doubles it.
	for i, want := range []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute} {
		if got := th.fail("k"); got != want {
			t.Fatalf("paused failure %d: wait %s, want %s", i, got, want)
		}
	}
	if got := th.wait("k"); got != 4*time.Minute {
		t.Fatalf("wait: %s", got)
	}
	for range 20 {
		th.fail("k")
	}
	if got := th.wait("k"); got != failureMaxDelay {
		t.Fatalf("wait isn't capped: %s", got)
	}
	// Other accounts aren't affected, and the wait runs out.
	if th.wait("other") != 0 {
		t.Fatal("another account is waiting")
	}
	now = now.Add(failureMaxDelay)
	if th.wait("k") != 0 {
		t.Fatal("still waiting after the wait")
	}
	// Success clears it; so does a long quiet spell.
	th.succeed("k")
	if th.fail("k") != 0 {
		t.Fatal("failures not cleared by success")
	}
	for range freeFailures {
		th.fail("q")
	}
	now = now.Add(failureMemory + time.Minute)
	if th.fail("q") != 0 {
		t.Fatal("old failures not forgotten")
	}
}

func TestClientIPUsesRightmostForwardedFor(t *testing.T) {
	s := &Server{cfg: config.Config{TrustProxy: true}}
	for _, tc := range []struct {
		name    string
		headers []string
		want    string
	}{
		{"proxy appended to a faked header", []string{"6.6.6.6, 203.0.113.9"}, "203.0.113.9"},
		{"single entry", []string{"203.0.113.9"}, "203.0.113.9"},
		{"several headers", []string{"6.6.6.6", "198.51.100.1, 203.0.113.9"}, "203.0.113.9"},
		{"ipv6", []string{"2001:db8::1"}, "2001:db8::1"},
		{"not an address", []string{"203.0.113.9, nonsense"}, "192.0.2.1"},
		{"none", nil, "192.0.2.1"},
	} {
		r, _ := http.NewRequest("GET", "/", nil)
		r.RemoteAddr = "192.0.2.1:5555"
		for _, h := range tc.headers {
			r.Header.Add("X-Forwarded-For", h)
		}
		if got := s.clientIP(r); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}

	// Without a trusted proxy the header is ignored.
	s.cfg.TrustProxy = false
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.1:5555"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := s.clientIP(r); got != "192.0.2.1" {
		t.Fatalf("untrusted proxy: got %q", got)
	}
}

func TestIPLimiterPrunesIdleEntries(t *testing.T) {
	l := &ipLimiter{entries: map[string]*limiterEntry{}}
	for i := range maxLimiterEntries {
		l.allow("10.0.0." + strconv.Itoa(i)) // one attempt each: still refilling
	}
	for _, e := range l.entries {
		e.lim.SetBurstAt(time.Now(), 1) // pretend they've all refilled
	}
	l.allow("192.0.2.1")
	if len(l.entries) != 1 {
		t.Fatalf("idle entries not pruned: %d left", len(l.entries))
	}
}

// loginFrom signs in with a given X-Forwarded-For, as if from another machine.
func loginFrom(t *testing.T, baseURL, ip, email, password string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", baseURL+"/api/auth/login",
		strings.NewReader(`{"email":"`+email+`","password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", ip)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

// Wrong passwords from many IPs still pause the account they're aimed at.
func TestLoginThrottledPerAccountAcrossIPs(t *testing.T) {
	ts, _ := newTestServerWithConfig(t, config.Config{DataDir: t.TempDir(), TrustProxy: true})

	for i := range freeFailures {
		ip := "203.0.113." + strconv.Itoa(i+1)
		if r := loginFrom(t, ts.URL, ip, "admin@example.com", "wrong"); r.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i+1, r.StatusCode)
		}
	}
	// Paused now, even with the right password, from a fresh IP.
	r := loginFrom(t, ts.URL, "198.51.100.7", "admin@example.com", "correct-horse")
	if r.StatusCode != http.StatusTooManyRequests || r.Header.Get("Retry-After") == "" {
		t.Fatalf("paused account: %d", r.StatusCode)
	}
	// An email with no account is paused the same way, so the answer doesn't
	// reveal which emails exist.
	for i := range freeFailures {
		loginFrom(t, ts.URL, "203.0.114."+strconv.Itoa(i+1), "nobody@example.com", "wrong")
	}
	if r := loginFrom(t, ts.URL, "198.51.100.8", "nobody@example.com", "wrong"); r.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("unknown email: %d", r.StatusCode)
	}
	// Other accounts are unaffected.
	if r := loginFrom(t, ts.URL, "198.51.100.9", "someone@example.com", "wrong"); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("other account: %d", r.StatusCode)
	}
}

// Faking the leftmost X-Forwarded-For entry no longer gets around the IP limit.
func TestLoginIPLimitIgnoresFakedForwardedFor(t *testing.T) {
	ts, _ := newTestServerWithConfig(t, config.Config{DataDir: t.TempDir(), TrustProxy: true})
	var last int
	for i := range 10 {
		// Different fake address each time, but the proxy's entry is the same.
		r := loginFrom(t, ts.URL, "10.9.9."+strconv.Itoa(i)+", 203.0.113.50", "user"+strconv.Itoa(i)+"@example.com", "wrong")
		last = r.StatusCode
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("faked addresses got past the IP limit: last status %d", last)
	}
}

// Wrong current passwords while signed in count against the account, so a
// stolen session can't be used to guess the password.
func TestPasswordConfirmationThrottled(t *testing.T) {
	ts := newTestServer(t)
	admin := signIn(t, ts.URL, "admin@example.com", "correct-horse")

	for i := range freeFailures {
		if code := do(t, admin, "POST", ts.URL+"/api/auth/password",
			`{"currentPassword":"guess`+strconv.Itoa(i)+`","newPassword":"another-password"}`); code != http.StatusBadRequest {
			t.Fatalf("guess %d: %d", i+1, code)
		}
	}
	if code := do(t, admin, "POST", ts.URL+"/api/auth/password",
		`{"currentPassword":"correct-horse","newPassword":"another-password"}`); code != http.StatusTooManyRequests {
		t.Fatalf("right password while paused: %d", code)
	}
	// The same count covers signing in.
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	if r := post(t, c, ts.URL+"/api/auth/login", `{"email":"admin@example.com","password":"correct-horse"}`); r.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("sign in while paused: %d", r.StatusCode)
	}
}
