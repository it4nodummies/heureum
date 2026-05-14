package api

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/api/handlers"
	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/project"
)

func NewRouter(cfg *config.Config, db *gorm.DB) http.Handler {
	mux := http.NewServeMux()

	authSvc := auth.NewService(db, cfg.Secret)
	authH := handlers.NewAuthHandler(authSvc)
	userH := handlers.NewUserHandler(db)
	oauthH := handlers.NewOAuthHandler(db, cfg.Secret, cfg.BaseURL)
	projectH := handlers.NewProjectHandler(project.NewService(db, nil))

	mux.HandleFunc("POST /api/v1/auth/register", authH.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authH.Login)
	mux.HandleFunc("GET /api/v1/auth/oauth/{provider}/redirect", oauthH.Redirect)
	mux.HandleFunc("GET /api/v1/auth/oauth/{provider}/callback", oauthH.Callback)

	authMw := middleware.Auth(cfg.Secret)
	mux.Handle("GET /api/v1/users/me", authMw(http.HandlerFunc(userH.GetMe)))

	mux.Handle("GET /api/v1/projects", authMw(http.HandlerFunc(projectH.List)))
	mux.Handle("POST /api/v1/projects", authMw(http.HandlerFunc(projectH.Create)))
	mux.Handle("GET /api/v1/projects/{key}", authMw(http.HandlerFunc(projectH.Get)))
	mux.Handle("PATCH /api/v1/projects/{key}", authMw(http.HandlerFunc(projectH.Update)))
	mux.Handle("DELETE /api/v1/projects/{key}", authMw(http.HandlerFunc(projectH.Delete)))
	mux.Handle("GET /api/v1/projects/{key}/members", authMw(http.HandlerFunc(projectH.ListMembers)))
	mux.Handle("POST /api/v1/projects/{key}/members", authMw(http.HandlerFunc(projectH.AddMember)))
	mux.Handle("DELETE /api/v1/projects/{key}/members/{userId}", authMw(http.HandlerFunc(projectH.RemoveMember)))
	mux.Handle("POST /api/v1/projects/{key}/invites", authMw(http.HandlerFunc(projectH.Invite)))

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
