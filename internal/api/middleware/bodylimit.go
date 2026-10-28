package middleware

import (
	"errors"
	"net/http"
)

// MaxRequestBodyBytes is the global request body cap (32 MB) applied to every
// route by the router, including the unauthenticated inbound git webhook.
// Attachment uploads enforce their own, tighter, advertised limit (10 MB).
const MaxRequestBodyBytes = 32 << 20

// MaxBodyBytes wraps every request body in http.MaxBytesReader(limit): a
// handler reading past the limit gets a *http.MaxBytesError (and the server
// stops reading the connection), so no route can be made to buffer an
// unbounded body. It only caps reads; it does not pre-reject on Content-Length,
// so streaming/chunked uploads under the limit are unaffected.
func MaxBodyBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.Body != http.NoBody {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IsBodyTooLarge reports whether err (possibly wrapped) is the error returned
// by a MaxBytesReader-capped body, so handlers can answer 413 instead of a
// generic 400/500.
func IsBodyTooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}
