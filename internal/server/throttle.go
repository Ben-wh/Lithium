package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lithium/lithium/internal/auth"
)

// Per-account limits on wrong passwords and two-factor codes. The IP limits
// stop one machine guessing quickly; these stop many machines guessing one
// account's password or code. They're kept in memory, so a restart clears them.
const (
	// freeFailures wrong attempts in a row are allowed before any wait.
	freeFailures = 5
	// After that, each failure doubles the wait, from failureBaseDelay up
	// to failureMaxDelay.
	failureBaseDelay = 30 * time.Second
	failureMaxDelay  = 15 * time.Minute
	// failureMemory: a quiet account's failures are forgotten after this.
	failureMemory = 24 * time.Hour
)

type accountThrottle struct {
	mu      sync.Mutex
	entries map[string]*failureEntry
	now     func() time.Time // tests only
}

type failureEntry struct {
	failures int
	until    time.Time // no attempts before this
	last     time.Time
}

func newAccountThrottle() *accountThrottle {
	t := &accountThrottle{entries: map[string]*failureEntry{}, now: time.Now}
	go func() {
		for range time.Tick(time.Hour) {
			t.mu.Lock()
			for k, e := range t.entries {
				if t.now().Sub(e.last) > failureMemory {
					delete(t.entries, k)
				}
			}
			t.mu.Unlock()
		}
	}()
	return t
}

// wait returns how long until key may try again (0 if it may now).
func (t *accountThrottle) wait(key string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entries[key]
	if !ok {
		return 0
	}
	if d := e.until.Sub(t.now()); d > 0 {
		return d
	}
	return 0
}

// fail records a wrong attempt and returns the wait it now has (0 if none).
func (t *accountThrottle) fail(key string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	e, ok := t.entries[key]
	if !ok || now.Sub(e.last) > failureMemory {
		e = &failureEntry{}
		t.entries[key] = e
	}
	e.failures++
	e.last = now
	if e.failures < freeFailures {
		return 0
	}
	delay := failureBaseDelay << min(e.failures-freeFailures, 10)
	if delay > failureMaxDelay {
		delay = failureMaxDelay
	}
	e.until = now.Add(delay)
	return delay
}

// succeed clears key's failures.
func (t *accountThrottle) succeed(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

// Keys. Password failures for an account are shared between signing in and
// confirming a password while signed in, so a stolen session can't be used
// to guess the password either. Sign-in attempts for an email with no
// account are keyed by the email, so they're limited the same way and the
// answer doesn't reveal whether the account exists.
func passwordKey(userID int64) string { return "pw:" + strconv.FormatInt(userID, 10) }
func unknownEmailKey(email string) string {
	return "pw-email:" + strings.ToLower(strings.TrimSpace(email))
}
func secondFactorKey(userID int64) string { return "2fa:" + strconv.FormatInt(userID, 10) }

// writeThrottled answers a request refused because the account is waiting.
func writeThrottled(w http.ResponseWriter, wait time.Duration) {
	secs := int(wait.Round(time.Second) / time.Second)
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	writeError(w, http.StatusTooManyRequests, "too many incorrect attempts for this account, try again in "+humanWait(wait))
}

func humanWait(d time.Duration) string {
	if d <= time.Minute {
		return "a minute"
	}
	m := int((d + time.Minute - 1) / time.Minute)
	return fmt.Sprintf("%d minutes", m)
}

// checkPassword verifies a signed-in user's password for a sensitive change,
// counting wrong answers against the account. It writes the response and
// returns false when the change mustn't go ahead.
func (s *Server) checkPassword(w http.ResponseWriter, r *http.Request, userID int64, hash, password string, wrongMsg string) bool {
	key := passwordKey(userID)
	if wait := s.throttle.wait(key); wait > 0 {
		writeThrottled(w, wait)
		return false
	}
	if ok, err := auth.VerifyPassword(password, hash); err != nil || !ok {
		if wait := s.throttle.fail(key); wait > 0 {
			auditDetail(r.Context(), "account paused for %s after repeated wrong passwords", wait)
			s.log.Warn("account throttled: wrong passwords", "user_id", userID, "ip", s.clientIP(r))
		}
		writeError(w, http.StatusBadRequest, wrongMsg)
		return false
	}
	s.throttle.succeed(key)
	return true
}
