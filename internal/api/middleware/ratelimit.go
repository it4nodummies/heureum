package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	v3 "github.com/it4nodummies/heureum/internal/api/v3"
)

// rateLimiter is an in-memory, per-client sliding-window limiter. It keeps, per
// client IP, the timestamps of the requests inside the current window and
// rejects any request that would exceed limit within window. It is safe for
// concurrent use and lazily prunes stale entries to bound memory.
type rateLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	clients map[string][]time.Time
	// lastPrune throttles the full-map sweep so it runs at most once per window.
	lastPrune time.Time
}

// newRateLimiter builds a limiter with an injectable clock, used by tests to
// advance time without sleeping. RateLimit is the production wrapper.
func newRateLimiter(limit int, window time.Duration, now func() time.Time) *rateLimiter {
	if now == nil {
		now = time.Now
	}
	return &rateLimiter{
		limit:   limit,
		window:  window,
		now:     now,
		clients: make(map[string][]time.Time),
	}
}

// RateLimit returns middleware enforcing limit requests per window per client IP.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, window, time.Now)
	return rl.middleware
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		allowed, retryAfter := rl.allow(ip)
		if !allowed {
			secs := int(retryAfter.Seconds())
			if secs < 1 {
				secs = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			v3.WriteError(w, http.StatusTooManyRequests, []string{"rate limit exceeded"}, nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allow records a request from ip and reports whether it is within the limit.
// When rejected it also returns how long until the oldest in-window request
// expires (the Retry-After hint).
func (rl *rateLimiter) allow(ip string) (bool, time.Duration) {
	now := rl.now()
	cutoff := now.Add(-rl.window)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.pruneLocked(now, cutoff)

	hits := rl.clients[ip]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= rl.limit {
		// Oldest surviving hit determines when a slot frees up.
		retryAfter := kept[0].Add(rl.window).Sub(now)
		rl.clients[ip] = kept
		return false, retryAfter
	}

	kept = append(kept, now)
	rl.clients[ip] = kept
	return true, 0
}

// pruneLocked drops clients with no requests left in the window. It runs at most
// once per window to keep the hot path cheap. Caller must hold rl.mu.
func (rl *rateLimiter) pruneLocked(now, cutoff time.Time) {
	if !rl.lastPrune.IsZero() && now.Sub(rl.lastPrune) < rl.window {
		return
	}
	rl.lastPrune = now
	for ip, hits := range rl.clients {
		alive := false
		for _, t := range hits {
			if t.After(cutoff) {
				alive = true
				break
			}
		}
		if !alive {
			delete(rl.clients, ip)
		}
	}
}

// clientIP extracts the client IP: the first hop of X-Forwarded-For when
// present, otherwise the host part of RemoteAddr (tolerating a missing port).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
		if first != "" {
			return first
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
