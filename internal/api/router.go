package api

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/api/handlers"
	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/workflow"
)

func NewRouter(cfg *config.Config, db *gorm.DB) http.Handler {
	mux := http.NewServeMux()

	authSvc := auth.NewService(db, cfg.Secret)
	authH := handlers.NewAuthHandler(authSvc)
	userH := handlers.NewUserHandler(db)
	oauthH := handlers.NewOAuthHandler(db, cfg.Secret, cfg.BaseURL)
	projectH := handlers.NewProjectHandler(project.NewService(db, nil))
	issueSvc := issue.NewService(db)
	issueH := handlers.NewIssueHandler(issueSvc)
	wfH := handlers.NewWorkflowHandler(workflow.NewService(db), issueSvc)

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

	mux.Handle("GET /api/v1/projects/{key}/issues", authMw(http.HandlerFunc(issueH.List)))
	mux.Handle("POST /api/v1/projects/{key}/issues", authMw(http.HandlerFunc(issueH.Create)))
	mux.Handle("GET /api/v1/issues/{issueKey}", authMw(http.HandlerFunc(issueH.Get)))
	mux.Handle("PATCH /api/v1/issues/{issueKey}", authMw(http.HandlerFunc(issueH.Update)))
	mux.Handle("DELETE /api/v1/issues/{issueKey}", authMw(http.HandlerFunc(issueH.Delete)))
	mux.Handle("POST /api/v1/issues/{issueKey}/labels", authMw(http.HandlerFunc(issueH.AddLabel)))
	mux.Handle("GET /api/v1/issues/{issueKey}/links", authMw(http.HandlerFunc(issueH.ListLinks)))
	mux.Handle("POST /api/v1/issues/{issueKey}/watch", authMw(http.HandlerFunc(issueH.Watch)))
	mux.Handle("DELETE /api/v1/issues/{issueKey}/watch", authMw(http.HandlerFunc(issueH.Unwatch)))
	mux.Handle("GET /api/v1/issues/{issueKey}/history", authMw(http.HandlerFunc(issueH.GetHistory)))

	mux.Handle("GET /api/v1/projects/{key}/workflow", authMw(http.HandlerFunc(wfH.GetWorkflow)))
	mux.Handle("POST /api/v1/projects/{key}/workflow/statuses", authMw(http.HandlerFunc(wfH.AddStatus)))
	mux.Handle("PATCH /api/v1/projects/{key}/workflow/statuses/{id}", authMw(http.HandlerFunc(wfH.UpdateStatus)))
	mux.Handle("DELETE /api/v1/projects/{key}/workflow/statuses/{id}", authMw(http.HandlerFunc(wfH.DeleteStatus)))
	mux.Handle("POST /api/v1/projects/{key}/workflow/transitions", authMw(http.HandlerFunc(wfH.AddTransition)))
	mux.Handle("POST /api/v1/issues/{issueKey}/transition", authMw(http.HandlerFunc(wfH.TransitionIssue)))

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
