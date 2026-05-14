package api

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/api/handlers"
	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/api/ws"
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/dashboard"
	"github.com/open-jira/open-jira/internal/domain/git"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/notification"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/report"
	"github.com/open-jira/open-jira/internal/domain/search"
	"github.com/open-jira/open-jira/internal/domain/automation"
	"github.com/open-jira/open-jira/internal/domain/calendar"
	"github.com/open-jira/open-jira/internal/domain/customfield"
	"github.com/open-jira/open-jira/internal/domain/sprint"
	"github.com/open-jira/open-jira/internal/domain/timeline"
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
	attachmentSvc := issue.NewAttachmentService(db)
	attachmentH := handlers.NewAttachmentHandler(attachmentSvc, issueSvc)
	wfSvc := workflow.NewService(db)
	wfH := handlers.NewWorkflowHandler(wfSvc, issueSvc)
	sprintSvc := sprint.NewService(db)
	sprintH := handlers.NewSprintHandler(sprintSvc, projectSvc)
	boardH := handlers.NewBoardHandler(issueSvc, projectSvc, wfSvc)
	searchSvc := search.NewService(db)
	filterSvc := search.NewFilterService(db)
	searchH := handlers.NewSearchHandler(searchSvc, filterSvc)
	reportSvc := report.NewService(db)
	reportH := handlers.NewReportHandler(reportSvc, projectSvc)
	dashboardSvc := dashboard.NewService(db)
	dashboardH := handlers.NewDashboardHandler(dashboardSvc)
	wsHub := ws.NewHub()
	go wsHub.Run()
	notifSvc := notification.NewService(db)
	notifSvc.SetBroadcaster(func(msg []byte) { wsHub.Broadcast(msg) })
	gitConfigSvc := git.NewConfigService(db)
	gitH := handlers.NewGitHandler(gitConfigSvc, issueSvc, projectSvc)
	notifH := handlers.NewNotificationHandler(notifSvc)
	issueSvc.SetNotifier(notifSvc)
	commentSvc.SetNotifier(notifSvc)
	sprintSvc.SetNotifier(notifSvc)

	cfSvc := customfield.NewService(db)
	cfH := handlers.NewCustomFieldHandler(cfSvc)
	autoSvc := automation.NewService(db)
	autoH := handlers.NewAutomationHandler(autoSvc)
	timelineSvc := timeline.NewService(db)
	timelineH := handlers.NewTimelineHandler(timelineSvc)
	calendarSvc := calendar.NewService(db)
	calendarH := handlers.NewCalendarHandler(calendarSvc)

	mux.HandleFunc("POST /api/v1/auth/register", authH.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authH.Login)
	mux.HandleFunc("GET /api/v1/auth/oauth/{provider}/redirect", oauthH.Redirect)
	mux.HandleFunc("GET /api/v1/auth/oauth/{provider}/callback", oauthH.Callback)

	mux.HandleFunc("POST /api/v1/webhooks/git/{token}", gitH.Webhook)

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
	mux.Handle("GET /api/v1/projects/{key}/git/providers", authMw(http.HandlerFunc(gitH.GetProvider)))
	mux.Handle("POST /api/v1/projects/{key}/git/providers", authMw(http.HandlerFunc(gitH.ConfigureProvider)))
	mux.Handle("DELETE /api/v1/projects/{key}/git/providers", authMw(http.HandlerFunc(gitH.DeleteProvider)))

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
	mux.Handle("GET /api/v1/issues/{issueKey}/attachments", authMw(http.HandlerFunc(attachmentH.List)))
	mux.Handle("POST /api/v1/issues/{issueKey}/attachments", authMw(http.HandlerFunc(attachmentH.Upload)))
	mux.Handle("GET /api/v1/attachments/{attachmentId}", authMw(http.HandlerFunc(attachmentH.ServeFile)))
	mux.Handle("DELETE /api/v1/issues/{issueKey}/attachments/{attachmentId}", authMw(http.HandlerFunc(attachmentH.Delete)))
	mux.Handle("GET /api/v1/issues/{issueKey}/git", authMw(http.HandlerFunc(gitH.GetIssueGitInfo)))

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

	mux.Handle("GET /api/v1/search", authMw(http.HandlerFunc(searchH.Search)))

	mux.Handle("GET /api/v1/filters", authMw(http.HandlerFunc(searchH.ListFilters)))
	mux.Handle("POST /api/v1/filters", authMw(http.HandlerFunc(searchH.CreateFilter)))
	mux.Handle("GET /api/v1/filters/{id}", authMw(http.HandlerFunc(searchH.GetFilter)))
	mux.Handle("DELETE /api/v1/filters/{id}", authMw(http.HandlerFunc(searchH.DeleteFilter)))

	mux.Handle("GET /api/v1/projects/{key}/reports/burndown", authMw(http.HandlerFunc(reportH.Burndown)))
	mux.Handle("GET /api/v1/projects/{key}/reports/velocity", authMw(http.HandlerFunc(reportH.Velocity)))
	mux.Handle("GET /api/v1/projects/{key}/reports/burnup", authMw(http.HandlerFunc(reportH.Burnup)))
	mux.Handle("GET /api/v1/projects/{key}/reports/cfd", authMw(http.HandlerFunc(reportH.CFD)))
	mux.Handle("GET /api/v1/projects/{key}/summary", authMw(http.HandlerFunc(reportH.Summary)))

	mux.Handle("GET /api/v1/dashboards", authMw(http.HandlerFunc(dashboardH.List)))
	mux.Handle("POST /api/v1/dashboards", authMw(http.HandlerFunc(dashboardH.Create)))
	mux.Handle("GET /api/v1/dashboards/{id}", authMw(http.HandlerFunc(dashboardH.Get)))
	mux.Handle("PATCH /api/v1/dashboards/{id}", authMw(http.HandlerFunc(dashboardH.Update)))
	mux.Handle("DELETE /api/v1/dashboards/{id}", authMw(http.HandlerFunc(dashboardH.Delete)))
	mux.Handle("POST /api/v1/dashboards/{id}/widgets", authMw(http.HandlerFunc(dashboardH.AddWidget)))
	mux.Handle("DELETE /api/v1/dashboards/{id}/widgets/{widgetId}", authMw(http.HandlerFunc(dashboardH.RemoveWidget)))

	mux.HandleFunc("GET /ws/v1/projects/{key}/board", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(wsHub, w, r)
	})

	mux.Handle("GET /api/v1/notifications", authMw(http.HandlerFunc(notifH.List)))
	mux.Handle("GET /api/v1/notifications/unread-count", authMw(http.HandlerFunc(notifH.UnreadCount)))
	mux.Handle("PATCH /api/v1/notifications/read-all", authMw(http.HandlerFunc(notifH.MarkAllRead)))
	mux.Handle("PATCH /api/v1/notifications/{id}/read", authMw(http.HandlerFunc(notifH.MarkRead)))
	mux.Handle("GET /api/v1/notifications/settings", authMw(http.HandlerFunc(notifH.GetSettings)))
	mux.Handle("PATCH /api/v1/notifications/settings", authMw(http.HandlerFunc(notifH.UpdateSettings)))

	mux.Handle("GET /api/v1/projects/{key}/issues/export", authMw(http.HandlerFunc(issueH.ExportCSV)))

	mux.Handle("GET /api/v1/projects/{projectID}/custom-fields", authMw(http.HandlerFunc(cfH.ListFields)))
	mux.Handle("POST /api/v1/projects/{projectID}/custom-fields", authMw(http.HandlerFunc(cfH.CreateField)))
	mux.Handle("DELETE /api/v1/custom-fields/{fieldID}", authMw(http.HandlerFunc(cfH.DeleteField)))
	mux.Handle("GET /api/v1/custom-fields/{fieldID}/options", authMw(http.HandlerFunc(cfH.ListOptions)))
	mux.Handle("POST /api/v1/custom-fields/{fieldID}/options", authMw(http.HandlerFunc(cfH.AddOption)))
	mux.Handle("DELETE /api/v1/custom-fields/options/{optionID}", authMw(http.HandlerFunc(cfH.RemoveOption)))
	mux.Handle("GET /api/v1/issues/{issueID}/custom-values", authMw(http.HandlerFunc(cfH.GetValues)))
	mux.Handle("PUT /api/v1/issues/{issueID}/custom-values/{fieldID}", authMw(http.HandlerFunc(cfH.SetValue)))

	mux.Handle("GET /api/v1/projects/{projectID}/automation", authMw(http.HandlerFunc(autoH.ListRules)))
	mux.Handle("POST /api/v1/projects/{projectID}/automation", authMw(http.HandlerFunc(autoH.CreateRule)))
	mux.Handle("GET /api/v1/automation/{ruleID}", authMw(http.HandlerFunc(autoH.GetRule)))
	mux.Handle("PATCH /api/v1/automation/{ruleID}", authMw(http.HandlerFunc(autoH.UpdateRule)))
	mux.Handle("DELETE /api/v1/automation/{ruleID}", authMw(http.HandlerFunc(autoH.DeleteRule)))
	mux.Handle("POST /api/v1/automation/{ruleID}/execute", authMw(http.HandlerFunc(autoH.ExecuteRule)))
	mux.Handle("GET /api/v1/automation/{ruleID}/runs", authMw(http.HandlerFunc(autoH.ListRuns)))

	mux.Handle("GET /api/v1/projects/{projectID}/timeline", authMw(http.HandlerFunc(timelineH.GetTimeline)))
	mux.Handle("GET /api/v1/projects/{projectID}/calendar", authMw(http.HandlerFunc(calendarH.GetCalendar)))

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
