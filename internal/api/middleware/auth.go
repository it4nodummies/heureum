package middleware

import (
	"context"
	"net/http"
	"strings"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// BasicVerifier validates an email + API token pair and returns the user ID.
type BasicVerifier func(email, token string) (string, error)

// Auth accepts either "Bearer <jwt>" (frontend session) or
// "Basic base64(email:api_token)" (API clients, like Jira Cloud).
func Auth(secret string, verifyBasic BasicVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				unauthorized(w, "Authentication required.")
				return
			}

			if email, token, ok := r.BasicAuth(); ok {
				userID, err := verifyBasic(email, token)
				if err != nil {
					unauthorized(w, "Basic authentication with an invalid email or API token.")
					return
				}
				next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), userID)))
				return
			} else if strings.HasPrefix(header, "Basic ") {
				// Basic scheme but undecodable credentials (bad base64 or no
				// "email:token" separator) — do not fall through to Bearer.
				unauthorized(w, "Basic authentication with an invalid email or API token.")
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				unauthorized(w, "Unsupported Authorization scheme.")
				return
			}
			claims, err := auth.ValidateToken(secret, parts[1])
			if err != nil {
				unauthorized(w, "The access token is invalid or expired.")
				return
			}
			next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), claims.UserID)))
		})
	}
}

func withUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, UserIDKey, id)
}

func unauthorized(w http.ResponseWriter, msg string) {
	v3.WriteError(w, http.StatusUnauthorized, []string{msg}, nil)
}

func UserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}
