package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func doReq(t *testing.T, h http.Handler, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/rest/api/3/auth/login", nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimit_AllowsUpToLimitThenBlocks(t *testing.T) {
	fake := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return fake }
	rl := newRateLimiter(3, time.Minute, now)
	h := rl.middleware(okHandler())

	for i := 1; i <= 3; i++ {
		rec := doReq(t, h, "10.0.0.1:5000")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rec.Code)
		}
	}

	rec := doReq(t, h, "10.0.0.1:5000")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request: status = %d, want 429", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" {
		t.Fatalf("4th request: missing Retry-After header")
	}
	var body struct {
		ErrorMessages []string `json:"errorMessages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode 429 body: %v", err)
	}
	if len(body.ErrorMessages) == 0 {
		t.Fatalf("429 body missing errorMessages")
	}
}

func TestRateLimit_IndependentPerClient(t *testing.T) {
	fake := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	rl := newRateLimiter(3, time.Minute, func() time.Time { return fake })
	h := rl.middleware(okHandler())

	for i := 0; i < 4; i++ {
		doReq(t, h, "10.0.0.1:5000")
	}
	// A different IP has its own budget.
	rec := doReq(t, h, "10.0.0.2:5000")
	if rec.Code != http.StatusOK {
		t.Fatalf("different IP: status = %d, want 200", rec.Code)
	}
}

func TestRateLimit_WindowResets(t *testing.T) {
	fake := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return fake }
	rl := newRateLimiter(3, time.Minute, now)
	h := rl.middleware(okHandler())

	for i := 0; i < 3; i++ {
		doReq(t, h, "10.0.0.1:5000")
	}
	if rec := doReq(t, h, "10.0.0.1:5000"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked request: status = %d, want 429", rec.Code)
	}

	// Advance the clock past the window; the budget frees up.
	fake = fake.Add(time.Minute + time.Second)
	if rec := doReq(t, h, "10.0.0.1:5000"); rec.Code != http.StatusOK {
		t.Fatalf("after window: status = %d, want 200", rec.Code)
	}
}

// ── Round 27 / Task 8: X-Forwarded-For is only honoured from a trusted proxy ──

func xffReq(remoteAddr, xff string) *http.Request {
	req := httptest.NewRequest("POST", "/rest/api/3/auth/login", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	return req
}

func TestClientIP_IgnoresXFFFromUntrustedPeer(t *testing.T) {
	trusted := mustParseTrustedProxies(t, "10.0.0.0/8")
	// The peer 198.51.100.7 is not a trusted proxy: whatever it claims in XFF
	// is attacker-controlled and must not become the bucket key.
	if got := clientIP(xffReq("198.51.100.7:4242", "203.0.113.9, 10.0.0.1"), trusted); got != "198.51.100.7" {
		t.Errorf("clientIP = %q, want RemoteAddr 198.51.100.7", got)
	}
	// With no trusted proxies configured at all, XFF is never consulted.
	if got := clientIP(xffReq("10.0.0.1:4242", "203.0.113.9"), nil); got != "10.0.0.1" {
		t.Errorf("clientIP (no trusted proxies) = %q, want 10.0.0.1", got)
	}
}

func TestClientIP_UsesRightmostHopFromTrustedProxy(t *testing.T) {
	trusted := mustParseTrustedProxies(t, "10.0.0.0/8, 127.0.0.1")
	// The rightmost hop was appended by the trusted proxy itself (its peer);
	// anything to its left is forwarded verbatim from the client.
	if got := clientIP(xffReq("10.0.0.1:4242", "1.2.3.4, 203.0.113.9"), trusted); got != "203.0.113.9" {
		t.Errorf("clientIP = %q, want rightmost hop 203.0.113.9", got)
	}
	// A single trusted proxy with a one-hop XFF.
	if got := clientIP(xffReq("127.0.0.1:4242", "203.0.113.9"), trusted); got != "203.0.113.9" {
		t.Errorf("clientIP = %q, want 203.0.113.9", got)
	}
	// Chained trusted proxies (ingress -> nginx): hops that are themselves
	// trusted proxies are skipped from the right.
	if got := clientIP(xffReq("10.0.0.1:4242", "203.0.113.9, 10.0.0.2"), trusted); got != "203.0.113.9" {
		t.Errorf("clientIP (chained) = %q, want 203.0.113.9", got)
	}
	// A garbage rightmost hop falls back to RemoteAddr instead of keying the
	// bucket on attacker-shaped text.
	if got := clientIP(xffReq("10.0.0.1:4242", "not-an-ip"), trusted); got != "10.0.0.1" {
		t.Errorf("clientIP (garbage hop) = %q, want 10.0.0.1", got)
	}
	// Trusted proxy but no XFF header: RemoteAddr.
	if got := clientIP(xffReq("10.0.0.1:4242", ""), trusted); got != "10.0.0.1" {
		t.Errorf("clientIP (no header) = %q, want 10.0.0.1", got)
	}
}

// TestRateLimit_ForgedXFFFromUntrustedPeerDoesNotChangeBucket: an attacker
// connecting directly (not through a trusted proxy) cannot get a fresh budget
// per request by rotating X-Forwarded-For.
func TestRateLimit_ForgedXFFFromUntrustedPeerDoesNotChangeBucket(t *testing.T) {
	fake := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	rl := newRateLimiter(2, time.Minute, func() time.Time { return fake })
	rl.trusted = mustParseTrustedProxies(t, "10.0.0.0/8")
	h := rl.middleware(okHandler())

	send := func(xff string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, xffReq("198.51.100.7:4242", xff))
		return rec.Code
	}
	send("203.0.113.1")
	send("203.0.113.2")
	if code := send("203.0.113.3"); code != http.StatusTooManyRequests {
		t.Fatalf("3rd request from the same untrusted peer with rotating XFF: status = %d, want 429", code)
	}
}

// TestRateLimit_TrustedProxyClientsGetSeparateBuckets: behind a trusted proxy
// the real clients (rightmost hop) are limited independently, and the proxy's
// own address does not become one shared bucket.
func TestRateLimit_TrustedProxyClientsGetSeparateBuckets(t *testing.T) {
	fake := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	rl := newRateLimiter(2, time.Minute, func() time.Time { return fake })
	rl.trusted = mustParseTrustedProxies(t, "10.0.0.0/8")
	h := rl.middleware(okHandler())

	send := func(xff string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, xffReq("10.0.0.1:4242", xff))
		return rec.Code
	}
	send("203.0.113.9")
	send("203.0.113.9")
	if code := send("203.0.113.9"); code != http.StatusTooManyRequests {
		t.Fatalf("3rd request from the same client via trusted proxy: status = %d, want 429", code)
	}
	if code := send("203.0.113.10"); code != http.StatusOK {
		t.Fatalf("different client via the same trusted proxy: status = %d, want 200", code)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	nets, err := ParseTrustedProxies([]string{"10.0.0.0/8", "127.0.0.1", "::1", "fd00::/8"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	if len(nets) != 4 {
		t.Fatalf("len = %d, want 4", len(nets))
	}
	if _, err := ParseTrustedProxies([]string{"not-an-ip"}); err == nil {
		t.Errorf("ParseTrustedProxies(garbage) = nil error, want error")
	}
	if nets, err := ParseTrustedProxies(nil); err != nil || nets != nil {
		t.Errorf("ParseTrustedProxies(nil) = %v, %v; want nil, nil", nets, err)
	}
}

func mustParseTrustedProxies(t *testing.T, csv string) []*net.IPNet {
	t.Helper()
	var entries []string
	for _, e := range strings.Split(csv, ",") {
		entries = append(entries, strings.TrimSpace(e))
	}
	nets, err := ParseTrustedProxies(entries)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", csv, err)
	}
	return nets
}
