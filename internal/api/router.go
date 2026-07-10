package api

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/api/handlers"
	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/api/ws"
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/automation"
	"github.com/open-jira/open-jira/internal/domain/calendar"
	"github.com/open-jira/open-jira/internal/domain/customfield"
	"github.com/open-jira/open-jira/internal/domain/dashboard"
	"github.com/open-jira/open-jira/internal/domain/git"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/notification"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/report"
	"github.com/open-jira/open-jira/internal/domain/search"
	"github.com/open-jira/open-jira/internal/domain/sprint"
	"github.com/open-jira/open-jira/internal/domain/timeline"
	"github.com/open-jira/open-jira/internal/domain/workflow"
)

func NewRouter(cfg *config.Config, db *gorm.DB) http.Handler {
	mux := http.NewServeMux()

	authSvc := auth.NewService(db, cfg.Secret)
	authH := handlers.NewAuthHandler(authSvc)
	userH := handlers.NewUserHandler(db, cfg.BaseURL)
	oauthH := handlers.NewOAuthHandler(db, cfg.Secret, cfg.BaseURL)
	projectSvc := project.NewService(db, nil)
	wfSvc := workflow.NewService(db)
	projectH := handlers.NewProjectHandler(projectSvc, wfSvc, cfg.BaseURL)
	issueSvc := issue.NewService(db)
	issueH := handlers.NewIssueHandler(issueSvc, projectSvc, wfSvc)
	commentSvc := issue.NewCommentService(db)
	commentH := handlers.NewCommentHandler(commentSvc, issueSvc)
	historyH := handlers.NewHistoryHandler(db, issueSvc)
	attachmentSvc := issue.NewAttachmentService(db)
	attachmentH := handlers.NewAttachmentHandler(attachmentSvc, issueSvc)
	issueLinkH := handlers.NewIssueLinkHandler(issueSvc)
	wfH := handlers.NewWorkflowHandler(wfSvc, issueSvc, projectSvc)
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

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Avatar di default servito da JiraUser quando l'utente non ne ha uno.
	// Non autenticato: gli avatar sono risorse pubbliche.
	mux.HandleFunc("GET /static/default-avatar.svg", serveDefaultAvatar)

	// Avatar di default per i progetti.
	mux.HandleFunc("GET /static/default-project-avatar.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 48 48"><rect width="48" height="48" rx="6" fill="#0052CC"/><path d="M14 14h20v20H14z" fill="#fff" opacity="0.85"/></svg>`))
	})

	mux.HandleFunc("POST /rest/api/3/auth/register", authH.Register)
	mux.HandleFunc("POST /rest/api/3/auth/login", authH.Login)
	mux.HandleFunc("GET /rest/api/3/auth/oauth/{provider}/redirect", oauthH.Redirect)
	mux.HandleFunc("GET /rest/api/3/auth/oauth/{provider}/callback", oauthH.Callback)

	mux.HandleFunc("POST /rest/api/3/webhooks/git/{token}", gitH.Webhook)

	authMw := middleware.Auth(cfg.Secret, authSvc.VerifyAPIToken)
	mux.Handle("POST /rest/api/3/auth/api-tokens", authMw(http.HandlerFunc(authH.CreateAPIToken)))
	mux.Handle("GET /rest/api/3/users/me", authMw(http.HandlerFunc(userH.GetMe)))
	mux.Handle("GET /rest/api/3/myself", authMw(http.HandlerFunc(userH.GetMyself)))
	mux.Handle("GET /rest/api/3/users/search", authMw(http.HandlerFunc(userH.SearchUsers)))
	mux.Handle("GET /rest/api/3/user", authMw(http.HandlerFunc(userH.GetUser)))

	mux.Handle("GET /rest/api/3/project", authMw(http.HandlerFunc(projectH.List)))
	mux.Handle("POST /rest/api/3/project", authMw(http.HandlerFunc(projectH.Create)))
	mux.Handle("GET /rest/api/3/project/search", authMw(http.HandlerFunc(projectH.Search)))
	mux.Handle("GET /rest/api/3/project/{key}", authMw(http.HandlerFunc(projectH.Get)))
	mux.Handle("PUT /rest/api/3/project/{key}", authMw(http.HandlerFunc(projectH.Update)))
	mux.Handle("DELETE /rest/api/3/project/{key}", authMw(http.HandlerFunc(projectH.Delete)))
	mux.Handle("GET /rest/api/3/project/{key}/members", authMw(http.HandlerFunc(projectH.ListMembers)))
	mux.Handle("POST /rest/api/3/project/{key}/members", authMw(http.HandlerFunc(projectH.AddMember)))
	mux.Handle("DELETE /rest/api/3/project/{key}/members/{userId}", authMw(http.HandlerFunc(projectH.RemoveMember)))
	mux.Handle("POST /rest/api/3/project/{key}/invites", authMw(http.HandlerFunc(projectH.Invite)))
	mux.Handle("PUT /rest/api/3/project/{key}/star", authMw(http.HandlerFunc(projectH.StarProject)))
	mux.Handle("DELETE /rest/api/3/project/{key}/star", authMw(http.HandlerFunc(projectH.UnstarProject)))
	mux.Handle("GET /rest/api/3/project/{key}/git/providers", authMw(http.HandlerFunc(gitH.GetProvider)))
	mux.Handle("POST /rest/api/3/project/{key}/git/providers", authMw(http.HandlerFunc(gitH.ConfigureProvider)))
	mux.Handle("DELETE /rest/api/3/project/{key}/git/providers", authMw(http.HandlerFunc(gitH.DeleteProvider)))

	mux.Handle("GET /rest/api/3/project/{key}/issues", authMw(http.HandlerFunc(issueH.List)))
	mux.Handle("POST /rest/api/3/issue", authMw(http.HandlerFunc(issueH.Create)))
	mux.Handle("GET /rest/api/3/issue/{issueKey}", authMw(http.HandlerFunc(issueH.Get)))
	mux.Handle("PUT /rest/api/3/issue/{issueKey}", authMw(http.HandlerFunc(issueH.Update)))
	mux.Handle("DELETE /rest/api/3/issue/{issueKey}", authMw(http.HandlerFunc(issueH.Delete)))
	mux.Handle("POST /rest/api/3/issue/{issueKey}/labels", authMw(http.HandlerFunc(issueH.AddLabel)))
	mux.Handle("GET /rest/api/3/issue/{issueKey}/changelog", authMw(http.HandlerFunc(historyH.GetHistory)))
	mux.Handle("GET /rest/api/3/issue/{issueKey}/git", authMw(http.HandlerFunc(gitH.GetIssueGitInfo)))
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/watchers", authMw(http.HandlerFunc(issueH.GetWatchers)))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/watchers", authMw(http.HandlerFunc(issueH.AddWatcher)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/watchers", authMw(http.HandlerFunc(issueH.RemoveWatcher)))

	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/comment", authMw(http.HandlerFunc(commentH.List)))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/comment", authMw(http.HandlerFunc(commentH.Create)))
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/comment/{id}", authMw(http.HandlerFunc(commentH.Get)))
	mux.Handle("PUT /rest/api/3/issue/{issueIdOrKey}/comment/{id}", authMw(http.HandlerFunc(commentH.Update)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/comment/{id}", authMw(http.HandlerFunc(commentH.Delete)))
	mux.Handle("POST /rest/api/3/comment/list", authMw(http.HandlerFunc(commentH.ListByIDs)))

	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/attachments", authMw(http.HandlerFunc(attachmentH.Upload)))
	mux.Handle("GET /rest/api/3/attachment/{id}", authMw(http.HandlerFunc(attachmentH.Get)))
	mux.Handle("DELETE /rest/api/3/attachment/{id}", authMw(http.HandlerFunc(attachmentH.Delete)))
	mux.Handle("GET /rest/api/3/attachment/content/{id}", authMw(http.HandlerFunc(attachmentH.ServeFile)))
	mux.Handle("GET /rest/api/3/attachment/meta", authMw(http.HandlerFunc(attachmentH.Meta)))

	mux.Handle("GET /rest/api/3/issueLink/{linkId}", authMw(http.HandlerFunc(issueLinkH.Get)))
	mux.Handle("POST /rest/api/3/issueLink", authMw(http.HandlerFunc(issueLinkH.Create)))
	mux.Handle("DELETE /rest/api/3/issueLink/{linkId}", authMw(http.HandlerFunc(issueLinkH.Delete)))

	mux.Handle("GET /rest/api/3/project/{key}/workflow", authMw(http.HandlerFunc(wfH.GetWorkflow)))
	mux.Handle("POST /rest/api/3/project/{key}/workflow/statuses", authMw(http.HandlerFunc(wfH.AddStatus)))
	mux.Handle("PATCH /rest/api/3/project/{key}/workflow/statuses/{id}", authMw(http.HandlerFunc(wfH.UpdateStatus)))
	mux.Handle("DELETE /rest/api/3/project/{key}/workflow/statuses/{id}", authMw(http.HandlerFunc(wfH.DeleteStatus)))
	mux.Handle("POST /rest/api/3/project/{key}/workflow/transitions", authMw(http.HandlerFunc(wfH.AddTransition)))
	mux.Handle("POST /rest/api/3/issue/{issueKey}/transitions", authMw(http.HandlerFunc(wfH.TransitionIssue)))
	mux.Handle("GET /rest/api/3/status", authMw(http.HandlerFunc(wfH.ListStatuses)))
	mux.Handle("GET /rest/api/3/status/{idOrName}", authMw(http.HandlerFunc(wfH.GetStatus)))
	mux.Handle("GET /rest/api/3/workflow/search", authMw(http.HandlerFunc(wfH.SearchWorkflows)))

	mux.Handle("GET /rest/api/3/project/{key}/sprints", authMw(http.HandlerFunc(sprintH.List)))
	mux.Handle("POST /rest/api/3/project/{key}/sprints", authMw(http.HandlerFunc(sprintH.Create)))
	mux.Handle("GET /rest/api/3/project/{key}/sprints/{id}", authMw(http.HandlerFunc(sprintH.Get)))
	mux.Handle("PATCH /rest/api/3/project/{key}/sprints/{id}", authMw(http.HandlerFunc(sprintH.Update)))
	mux.Handle("POST /rest/api/3/project/{key}/sprints/{id}/start", authMw(http.HandlerFunc(sprintH.Start)))
	mux.Handle("POST /rest/api/3/project/{key}/sprints/{id}/complete", authMw(http.HandlerFunc(sprintH.Complete)))

	mux.Handle("GET /rest/api/3/project/{key}/board", authMw(http.HandlerFunc(boardH.GetBoard)))
	mux.Handle("POST /rest/api/3/issues/rank", authMw(http.HandlerFunc(boardH.RankIssue)))

	mux.Handle("GET /rest/api/3/search", authMw(http.HandlerFunc(searchH.Search)))
	mux.Handle("POST /rest/api/3/search", authMw(http.HandlerFunc(searchH.SearchPost)))
	mux.Handle("GET /rest/api/3/search/jql", authMw(http.HandlerFunc(searchH.Search)))

	mux.Handle("GET /rest/api/3/filters", authMw(http.HandlerFunc(searchH.ListMyFilters)))
	mux.Handle("POST /rest/api/3/filters", authMw(http.HandlerFunc(searchH.CreateFilter)))
	mux.Handle("GET /rest/api/3/filters/{id}", authMw(http.HandlerFunc(searchH.GetFilter)))
	mux.Handle("DELETE /rest/api/3/filters/{id}", authMw(http.HandlerFunc(searchH.DeleteFilter)))
	mux.Handle("GET /rest/api/3/filter/my", authMw(http.HandlerFunc(searchH.ListMyFilters)))
	mux.Handle("GET /rest/api/3/filter/favourite", authMw(http.HandlerFunc(searchH.ListFavouriteFilters)))
	mux.Handle("GET /rest/api/3/filter/search", authMw(http.HandlerFunc(searchH.ListMyFilters)))
	mux.Handle("POST /rest/api/3/filter", authMw(http.HandlerFunc(searchH.CreateFilter)))
	mux.Handle("GET /rest/api/3/filter/{id}", authMw(http.HandlerFunc(searchH.GetFilter)))
	mux.Handle("PUT /rest/api/3/filter/{id}", authMw(http.HandlerFunc(searchH.UpdateFilter)))
	mux.Handle("DELETE /rest/api/3/filter/{id}", authMw(http.HandlerFunc(searchH.DeleteFilter)))
	mux.Handle("PUT /rest/api/3/filter/{id}/favourite", authMw(http.HandlerFunc(searchH.AddFavourite)))
	mux.Handle("DELETE /rest/api/3/filter/{id}/favourite", authMw(http.HandlerFunc(searchH.RemoveFavourite)))
	mux.Handle("PUT /rest/api/3/filter/{id}/owner", authMw(http.HandlerFunc(searchH.ChangeFilterOwner)))

	mux.Handle("GET /rest/api/3/project/{key}/reports/burndown", authMw(http.HandlerFunc(reportH.Burndown)))
	mux.Handle("GET /rest/api/3/project/{key}/reports/velocity", authMw(http.HandlerFunc(reportH.Velocity)))
	mux.Handle("GET /rest/api/3/project/{key}/reports/burnup", authMw(http.HandlerFunc(reportH.Burnup)))
	mux.Handle("GET /rest/api/3/project/{key}/reports/cfd", authMw(http.HandlerFunc(reportH.CFD)))
	mux.Handle("GET /rest/api/3/project/{key}/summary", authMw(http.HandlerFunc(reportH.Summary)))

	mux.Handle("GET /rest/api/3/dashboards", authMw(http.HandlerFunc(dashboardH.List)))
	mux.Handle("POST /rest/api/3/dashboards", authMw(http.HandlerFunc(dashboardH.Create)))
	mux.Handle("GET /rest/api/3/dashboards/{id}", authMw(http.HandlerFunc(dashboardH.Get)))
	mux.Handle("PATCH /rest/api/3/dashboards/{id}", authMw(http.HandlerFunc(dashboardH.Update)))
	mux.Handle("DELETE /rest/api/3/dashboards/{id}", authMw(http.HandlerFunc(dashboardH.Delete)))
	mux.Handle("POST /rest/api/3/dashboards/{id}/widgets", authMw(http.HandlerFunc(dashboardH.AddWidget)))
	mux.Handle("DELETE /rest/api/3/dashboards/{id}/widgets/{widgetId}", authMw(http.HandlerFunc(dashboardH.RemoveWidget)))
	mux.Handle("GET /rest/api/3/dashboard", authMw(http.HandlerFunc(dashboardH.List)))
	mux.Handle("POST /rest/api/3/dashboard", authMw(http.HandlerFunc(dashboardH.Create)))
	mux.Handle("GET /rest/api/3/dashboard/search", authMw(http.HandlerFunc(dashboardH.SearchDashboards)))
	mux.Handle("GET /rest/api/3/dashboard/{id}", authMw(http.HandlerFunc(dashboardH.Get)))
	mux.Handle("PUT /rest/api/3/dashboard/{id}", authMw(http.HandlerFunc(dashboardH.Update)))
	mux.Handle("DELETE /rest/api/3/dashboard/{id}", authMw(http.HandlerFunc(dashboardH.Delete)))
	mux.Handle("POST /rest/api/3/dashboard/{id}/copy", authMw(http.HandlerFunc(dashboardH.CopyDashboard)))
	mux.Handle("POST /rest/api/3/dashboard/{dashboardId}/gadget", authMw(http.HandlerFunc(dashboardH.AddWidget)))
	mux.Handle("DELETE /rest/api/3/dashboard/{dashboardId}/gadget/{gadgetId}", authMw(http.HandlerFunc(dashboardH.RemoveGadget)))

	mux.HandleFunc("GET /ws/v1/projects/{key}/board", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(wsHub, w, r)
	})

	mux.Handle("GET /rest/api/3/notifications", authMw(http.HandlerFunc(notifH.List)))
	mux.Handle("GET /rest/api/3/notifications/unread-count", authMw(http.HandlerFunc(notifH.UnreadCount)))
	mux.Handle("PATCH /rest/api/3/notifications/read-all", authMw(http.HandlerFunc(notifH.MarkAllRead)))
	mux.Handle("PATCH /rest/api/3/notifications/{id}/read", authMw(http.HandlerFunc(notifH.MarkRead)))
	mux.Handle("GET /rest/api/3/notifications/settings", authMw(http.HandlerFunc(notifH.GetSettings)))
	mux.Handle("PATCH /rest/api/3/notifications/settings", authMw(http.HandlerFunc(notifH.UpdateSettings)))

	mux.Handle("GET /rest/api/3/project/{key}/issues/export", authMw(http.HandlerFunc(issueH.ExportCSV)))

	mux.Handle("GET /rest/api/3/project/{projectID}/custom-fields", authMw(http.HandlerFunc(cfH.ListFields)))
	mux.Handle("POST /rest/api/3/project/{projectID}/custom-fields", authMw(http.HandlerFunc(cfH.CreateField)))
	mux.Handle("DELETE /rest/api/3/custom-fields/{fieldID}", authMw(http.HandlerFunc(cfH.DeleteField)))
	mux.Handle("GET /rest/api/3/custom-fields/{fieldID}/options", authMw(http.HandlerFunc(cfH.ListOptions)))
	mux.Handle("POST /rest/api/3/custom-fields/{fieldID}/options", authMw(http.HandlerFunc(cfH.AddOption)))
	mux.Handle("DELETE /rest/api/3/custom-fields/options/{optionID}", authMw(http.HandlerFunc(cfH.RemoveOption)))
	mux.Handle("GET /rest/api/3/issue/{issueID}/custom-values", authMw(http.HandlerFunc(cfH.GetValues)))
	mux.Handle("PUT /rest/api/3/issue/{issueID}/custom-values/{fieldID}", authMw(http.HandlerFunc(cfH.SetValue)))

	mux.Handle("GET /rest/api/3/project/{projectID}/automation", authMw(http.HandlerFunc(autoH.ListRules)))
	mux.Handle("POST /rest/api/3/project/{projectID}/automation", authMw(http.HandlerFunc(autoH.CreateRule)))
	mux.Handle("GET /rest/api/3/automation/{ruleID}", authMw(http.HandlerFunc(autoH.GetRule)))
	mux.Handle("PATCH /rest/api/3/automation/{ruleID}", authMw(http.HandlerFunc(autoH.UpdateRule)))
	mux.Handle("DELETE /rest/api/3/automation/{ruleID}", authMw(http.HandlerFunc(autoH.DeleteRule)))
	mux.Handle("POST /rest/api/3/automation/{ruleID}/execute", authMw(http.HandlerFunc(autoH.ExecuteRule)))
	mux.Handle("GET /rest/api/3/automation/{ruleID}/runs", authMw(http.HandlerFunc(autoH.ListRuns)))

	mux.Handle("GET /rest/api/3/project/{projectID}/timeline", authMw(http.HandlerFunc(timelineH.GetTimeline)))
	mux.Handle("GET /rest/api/3/project/{projectID}/calendar", authMw(http.HandlerFunc(calendarH.GetCalendar)))

	return corsMiddleware(mux)
}

// defaultAvatarSVG è un avatar neutro (cerchio grigio con silhouette) servito
// inline, senza file su disco, come fallback per gli utenti senza AvatarURL.
const defaultAvatarSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 48 48"><circle cx="24" cy="24" r="24" fill="#C1C7D0"/><circle cx="24" cy="19" r="8" fill="#FFFFFF"/><path d="M8 44c0-8.837 7.163-14 16-14s16 5.163 16 14z" fill="#FFFFFF"/></svg>`

func serveDefaultAvatar(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(defaultAvatarSVG))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
