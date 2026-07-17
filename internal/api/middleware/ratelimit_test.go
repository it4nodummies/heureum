package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestRateLimit_XForwardedForFirstHop(t *testing.T) {
	fake := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	rl := newRateLimiter(2, time.Minute, func() time.Time { return fake })
	h := rl.middleware(okHandler())

	send := func(xff string) int {
		req := httptest.NewRequest("POST", "/rest/api/3/auth/login", nil)
		req.RemoteAddr = "127.0.0.1:9999"
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// Same first hop (client IP) should share a bucket even behind a proxy.
	send("203.0.113.9, 10.0.0.1")
	send("203.0.113.9, 10.0.0.2")
	if code := send("203.0.113.9, 10.0.0.3"); code != http.StatusTooManyRequests {
		t.Fatalf("3rd request from same client via proxy: status = %d, want 429", code)
	}
}
