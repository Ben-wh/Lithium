package server

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		// SvelteKit's static fallback page bootstraps with an inline script.
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

// requireJSONForWrites blocks cross-site form posts (CSRF): browsers can't
// send a cross-origin application/json request without a CORS preflight,
// which we never approve. File uploads have to be multipart, which any site
// could submit, so those must also prove they come from our own pages.
func requireJSONForWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if strings.HasPrefix(r.URL.Path, "/api/") {
				mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
				if mt == "multipart/form-data" && !sameOrigin(r) {
					writeError(w, http.StatusForbidden, "cross-site upload rejected")
					return
				}
				if mt != "application/json" && mt != "multipart/form-data" {
					writeError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// sameOrigin reports whether a request came from one of our own pages.
// Modern browsers send Sec-Fetch-Site; otherwise fall back to Origin.
func sameOrigin(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin"
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

// Unwrap lets http.ResponseController reach the real writer (e.g. to lift
// the write deadline for a large download).
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.log.Debug("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "dur", time.Since(start))
		}
	})
}

// clientIP is the address a request came from. Behind a trusted proxy it's
// the rightmost X-Forwarded-For entry: the address the proxy itself saw.
// Entries further left are whatever the client sent, so they can be faked
// (a proxy such as nginx with $proxy_add_x_forwarded_for appends to them
// rather than replacing them) and must never be used for rate limiting or
// the activity log.
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if ip := lastForwardedFor(r.Header.Values("X-Forwarded-For")); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// lastForwardedFor returns the rightmost valid IP address across all
// X-Forwarded-For headers, or "" if the last entry isn't an IP address.
func lastForwardedFor(values []string) string {
	if len(values) == 0 {
		return ""
	}
	parts := strings.Split(values[len(values)-1], ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	if ip := net.ParseIP(last); ip != nil {
		return ip.String()
	}
	return ""
}

// maxLimiterEntries caps how many IPs an ipLimiter tracks, so a flood of
// addresses can't grow it without limit.
const maxLimiterEntries = 50_000

// ipLimiter allows a burst of 5 login attempts per IP, refilling one every 12s.
type ipLimiter struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
}

type limiterEntry struct {
	lim  *rate.Limiter
	seen time.Time
}

func newIPLimiter() *ipLimiter {
	l := &ipLimiter{entries: map[string]*limiterEntry{}}
	go func() {
		for range time.Tick(5 * time.Minute) {
			l.mu.Lock()
			for ip, e := range l.entries {
				if time.Since(e.seen) > 10*time.Minute {
					delete(l.entries, ip)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	if !ok {
		if len(l.entries) >= maxLimiterEntries {
			l.pruneIdle()
		}
		e = &limiterEntry{lim: rate.NewLimiter(rate.Every(12*time.Second), 5)}
		l.entries[ip] = e
	}
	e.seen = time.Now()
	return e.lim.Allow()
}

// pruneIdle drops entries whose limiter has fully refilled: forgetting them
// changes nothing, since a new entry starts full too. Callers hold l.mu.
func (l *ipLimiter) pruneIdle() {
	now := time.Now()
	for ip, e := range l.entries {
		if e.lim.TokensAt(now) >= float64(e.lim.Burst()) {
			delete(l.entries, ip)
		}
	}
}
