package middleware

import (
	"fmt"
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
	// trusted lists the reverse proxies whose X-Forwarded-For is honoured;
	// nil/empty means the header is never consulted (see clientIP).
	trusted []*net.IPNet

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

// RateLimit returns middleware enforcing limit requests per window per client
// IP. trustedProxies (parsed from APP_TRUSTED_PROXIES, see ParseTrustedProxies)
// are the only peers whose X-Forwarded-For is believed.
func RateLimit(limit int, window time.Duration, trustedProxies []*net.IPNet) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, window, time.Now)
	rl.trusted = trustedProxies
	return rl.middleware
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, rl.trusted)
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

// ParseTrustedProxies parses APP_TRUSTED_PROXIES entries: each is a CIDR
// ("10.42.0.0/16") or a single IPv4/IPv6 address ("127.0.0.1", "::1"). An
// unparsable entry is a configuration error (fail at startup rather than
// silently trusting nothing or everything).
func ParseTrustedProxies(entries []string) ([]*net.IPNet, error) {
	var nets []*net.IPNet
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(e); err == nil {
			nets = append(nets, n)
			continue
		}
		ip := net.ParseIP(e)
		if ip == nil {
			return nil, fmt.Errorf("trusted proxy %q is neither a CIDR nor an IP address", e)
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return nets, nil
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP extracts the client IP used as the rate-limit bucket key.
//
// X-Forwarded-For is attacker-controlled unless the connecting peer is one of
// our own reverse proxies: anyone connecting directly could otherwise rotate
// the header and get a fresh budget per request. So:
//
//   - peer (RemoteAddr) not in trusted → RemoteAddr, header ignored;
//   - peer in trusted → walk X-Forwarded-For from the RIGHT (the rightmost hop
//     is the one our proxy itself appended, i.e. its peer), skipping hops that
//     are themselves trusted proxies (a chain: ingress → nginx → app). The
//     first non-trusted hop is the client. Left-of-that is forwarded verbatim
//     from the client and is never used.
//   - no usable hop (empty header, unparsable address) → RemoteAddr.
func clientIP(r *http.Request, trusted []*net.IPNet) string {
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	if len(trusted) == 0 {
		return remote
	}
	remoteIP := net.ParseIP(remote)
	if remoteIP == nil || !ipInNets(remoteIP, trusted) {
		return remote
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		ip := net.ParseIP(hop)
		if ip == nil {
			break // garbage from a trusted proxy: don't key on it
		}
		if ipInNets(ip, trusted) {
			continue // one of our own proxies in a chain
		}
		return ip.String()
	}
	return remote
}
