package api

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/api/handlers"
	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/api/ws"
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/sprint"
	"github.com/open-jira/open-jira/internal/domain/workflow"
)

func NewRouter(cfg *config.Config, db *gorm.DB) http.Handler {
	mux := http.NewServeMux()

	authSvc := auth.NewService(db, cfg.Secret)
	authH := handlers.NewAuthHandler(authSvc)
	userH := handlers.NewUserHandler(db)
	oauthH := handlers.NewOAuthHandler(db, cfg.Secret, cfg.BaseURL)
	projectSvc := project.NewService(db, nil)
	projectH := handlers.NewProjectHandler(projectSvc)
	issueSvc := issue.NewService(db)
	issueH := handlers.NewIssueHandler(issueSvc)
	commentSvc := issue.NewCommentService(db)
	commentH := handlers.NewCommentHandler(commentSvc, issueSvc)
	historyH := handlers.NewHistoryHandler(db, issueSvc)
	wfSvc := workflow.NewService(db)
	wfH := handlers.NewWorkflowHandler(wfSvc, issueSvc)
	sprintSvc := sprint.NewService(db)
	sprintH := handlers.NewSprintHandler(sprintSvc, projectSvc)
	boardH := handlers.NewBoardHandler(issueSvc, projectSvc, wfSvc)
	wsHub := ws.NewHub()
	go wsHub.Run()

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
	mux.Handle("GET /api/v1/issues/{issueKey}/history", authMw(http.HandlerFunc(historyH.GetHistory)))
	mux.Handle("GET /api/v1/issues/{issueKey}/comments", authMw(http.HandlerFunc(commentH.List)))
	mux.Handle("POST /api/v1/issues/{issueKey}/comments", authMw(http.HandlerFunc(commentH.Create)))
	mux.Handle("DELETE /api/v1/issues/{issueKey}/comments/{commentId}", authMw(http.HandlerFunc(commentH.Delete)))

	mux.Handle("GET /api/v1/projects/{key}/workflow", authMw(http.HandlerFunc(wfH.GetWorkflow)))
	mux.Handle("POST /api/v1/projects/{key}/workflow/statuses", authMw(http.HandlerFunc(wfH.AddStatus)))
	mux.Handle("PATCH /api/v1/projects/{key}/workflow/statuses/{id}", authMw(http.HandlerFunc(wfH.UpdateStatus)))
	mux.Handle("DELETE /api/v1/projects/{key}/workflow/statuses/{id}", authMw(http.HandlerFunc(wfH.DeleteStatus)))
	mux.Handle("POST /api/v1/projects/{key}/workflow/transitions", authMw(http.HandlerFunc(wfH.AddTransition)))
	mux.Handle("POST /api/v1/issues/{issueKey}/transition", authMw(http.HandlerFunc(wfH.TransitionIssue)))

	mux.Handle("GET /api/v1/projects/{key}/sprints", authMw(http.HandlerFunc(sprintH.List)))
	mux.Handle("POST /api/v1/projects/{key}/sprints", authMw(http.HandlerFunc(sprintH.Create)))
	mux.Handle("GET /api/v1/projects/{key}/sprints/{id}", authMw(http.HandlerFunc(sprintH.Get)))
	mux.Handle("PATCH /api/v1/projects/{key}/sprints/{id}", authMw(http.HandlerFunc(sprintH.Update)))
	mux.Handle("POST /api/v1/projects/{key}/sprints/{id}/start", authMw(http.HandlerFunc(sprintH.Start)))
	mux.Handle("POST /api/v1/projects/{key}/sprints/{id}/complete", authMw(http.HandlerFunc(sprintH.Complete)))

	mux.Handle("GET /api/v1/projects/{key}/board", authMw(http.HandlerFunc(boardH.GetBoard)))
	mux.Handle("POST /api/v1/issues/rank", authMw(http.HandlerFunc(boardH.RankIssue)))

	mux.HandleFunc("GET /ws/v1/projects/{key}/board", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(wsHub, w, r)
	})

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
