package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/open-jira/open-jira/internal/domain/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
				return
			}
			claims, err := auth.ValidateToken(secret, parts[1])
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}
