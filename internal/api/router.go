package api

import (
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/api/handlers"
	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/api/ws"
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/automation"
	"github.com/open-jira/open-jira/internal/domain/board"
	"github.com/open-jira/open-jira/internal/domain/calendar"
	"github.com/open-jira/open-jira/internal/domain/customfield"
	"github.com/open-jira/open-jira/internal/domain/dashboard"
	"github.com/open-jira/open-jira/internal/domain/git"
	"github.com/open-jira/open-jira/internal/domain/group"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/notification"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/report"
	"github.com/open-jira/open-jira/internal/domain/search"
	"github.com/open-jira/open-jira/internal/domain/sprint"
	"github.com/open-jira/open-jira/internal/domain/timeline"
	"github.com/open-jira/open-jira/internal/domain/webhook"
	"github.com/open-jira/open-jira/internal/domain/workflow"
	"github.com/open-jira/open-jira/internal/integration"
)

func NewRouter(cfg *config.Config, db *gorm.DB) http.Handler {
	mux := http.NewServeMux()

	authSvc := auth.NewService(db, cfg.Secret)
	authH := handlers.NewAuthHandler(authSvc)
	userH := handlers.NewUserHandler(db, cfg.BaseURL)
	oauthH := handlers.NewOAuthHandler(db, cfg.Secret, cfg.BaseURL)
	projectSvc := project.NewService(db, nil)
	permH := handlers.NewPermissionHandler(db, projectSvc)
	wfSvc := workflow.NewService(db)
	projectH := handlers.NewProjectHandler(projectSvc, wfSvc, cfg.BaseURL)
	pcH := handlers.NewProjectCategoryHandler(db, cfg.BaseURL)
	issueSvc := issue.NewService(db)
	issueH := handlers.NewIssueHandler(issueSvc, projectSvc, wfSvc, cfg.BaseURL)
	commentSvc := issue.NewCommentService(db)
	commentH := handlers.NewCommentHandler(commentSvc, issueSvc, cfg.BaseURL)
	worklogSvc := issue.NewWorklogService(db)
	worklogH := handlers.NewWorklogHandler(worklogSvc, issueSvc, cfg.BaseURL)
	voteSvc := issue.NewVoteService(db)
	votesH := handlers.NewVotesHandler(voteSvc, issueSvc, cfg.BaseURL)
	remoteLinkSvc := issue.NewRemoteLinkService(db)
	remoteLinkH := handlers.NewRemoteLinkHandler(remoteLinkSvc, issueSvc, cfg.BaseURL)
	historyH := handlers.NewHistoryHandler(db, issueSvc, cfg.BaseURL)
	attachmentSvc := issue.NewAttachmentService(db)
	attachmentH := handlers.NewAttachmentHandler(attachmentSvc, issueSvc)
	issueLinkH := handlers.NewIssueLinkHandler(issueSvc, cfg.BaseURL)
	wfH := handlers.NewWorkflowHandler(wfSvc, issueSvc, projectSvc, cfg.BaseURL)
	sprintSvc := sprint.NewService(db)
	sprintH := handlers.NewSprintHandler(sprintSvc, projectSvc)
	boardH := handlers.NewBoardHandler(issueSvc, projectSvc, wfSvc)
	searchSvc := search.NewService(db)
	searchH := handlers.NewSearchHandler(searchSvc, issueH)
	filterSvc := search.NewFilterService(db)
	filterH := handlers.NewFilterHandler(filterSvc, db, cfg.BaseURL)
	reportSvc := report.NewService(db)
	reportH := handlers.NewReportHandler(reportSvc, projectSvc)
	dashboardSvc := dashboard.NewService(db)
	dashboardH := handlers.NewDashboardHandler(dashboardSvc)
	wsHub := ws.NewHub()
	go wsHub.Run()
	notifSvc := notification.NewService(db)
	notifSvc.SetBroadcaster(func(msg []byte) { wsHub.Broadcast(msg) })
	gitConfigSvc := git.NewConfigService(db)
	gitH := handlers.NewGitHandler(gitConfigSvc, issueSvc, projectSvc, commentSvc)
	notifH := handlers.NewNotificationHandler(notifSvc)
	issueSvc.SetNotifier(notifSvc)
	commentSvc.SetNotifier(notifSvc)
	sprintSvc.SetNotifier(notifSvc)

	cfSvc := customfield.NewService(db)
	cfH := handlers.NewCustomFieldHandler(cfSvc)
	autoSvc := automation.NewService(db)
	autoH := handlers.NewAutomationHandler(autoSvc)
	webhookSvc := webhook.NewService(db)
	webhookH := handlers.NewWebhookHandler(webhookSvc, projectSvc)
	dispatcher := integration.NewDispatcher(webhookSvc, autoSvc, &http.Client{Timeout: 10 * time.Second})
	issueSvc.SetEventSink(dispatcher)
	timelineSvc := timeline.NewService(db)
	timelineH := handlers.NewTimelineHandler(timelineSvc)
	calendarSvc := calendar.NewService(db)
	calendarH := handlers.NewCalendarHandler(calendarSvc)
	refH := handlers.NewReferenceHandler(db, cfg.BaseURL)

	groupSvc := group.NewService(db)
	groupH := handlers.NewGroupHandler(groupSvc, db, cfg.BaseURL)

	boardSvc := board.NewService(db)
	agileBoardH := handlers.NewAgileBoardHandler(boardSvc, projectSvc, issueSvc, sprintSvc, wfSvc, issueH, cfg.BaseURL)
	agileSprintH := handlers.NewAgileSprintHandler(sprintSvc, boardSvc, issueSvc, issueH, cfg.BaseURL)
	agileMiscH := handlers.NewAgileMiscHandler(issueSvc, sprintSvc, issueH, cfg.BaseURL)

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
	mux.Handle("PUT /rest/api/3/myself", authMw(http.HandlerFunc(userH.UpdateMyself)))
	mux.Handle("GET /rest/api/3/users/search", authMw(http.HandlerFunc(userH.SearchUsers)))
	mux.Handle("GET /rest/api/3/user/search", authMw(http.HandlerFunc(userH.SearchV3)))
	mux.Handle("GET /rest/api/3/user/assignable/search", authMw(http.HandlerFunc(userH.AssignableSearch)))
	mux.Handle("GET /rest/api/3/user", authMw(http.HandlerFunc(userH.GetUser)))
	mux.Handle("GET /rest/api/3/permissions", authMw(http.HandlerFunc(permH.Permissions)))
	mux.Handle("GET /rest/api/3/mypermissions", authMw(http.HandlerFunc(permH.MyPermissions)))

	mux.Handle("GET /rest/api/3/project", authMw(http.HandlerFunc(projectH.List)))
	mux.Handle("POST /rest/api/3/project", authMw(http.HandlerFunc(projectH.Create)))
	mux.Handle("GET /rest/api/3/project/search", authMw(http.HandlerFunc(projectH.Search)))
	mux.Handle("GET /rest/api/3/project/type", authMw(http.HandlerFunc(projectH.ProjectTypes)))
	// Registered as literal routes (not "/project/type/{projectTypeKey}"):
	// net/http's ServeMux treats a wildcard here as ambiguous against the many
	// existing "/project/{key}/<literal>" routes below (members, workflow,
	// sprints, ...) — same segment depth, opposite literal/wildcard positions —
	// and panics at startup. The set of valid project type keys is fixed, so
	// literal routes plus SetPathValue keep ProjectTypeByKey's handler code
	// (which reads r.PathValue("projectTypeKey")) unchanged.
	mux.Handle("GET /rest/api/3/project/type/software", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("projectTypeKey", "software")
		projectH.ProjectTypeByKey(w, r)
	})))
	mux.Handle("GET /rest/api/3/project/type/business", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("projectTypeKey", "business")
		projectH.ProjectTypeByKey(w, r)
	})))
	mux.Handle("GET /rest/api/3/projectCategory", authMw(http.HandlerFunc(pcH.List)))
	mux.Handle("POST /rest/api/3/projectCategory", authMw(http.HandlerFunc(pcH.Create)))
	mux.Handle("GET /rest/api/3/project/{key}", authMw(http.HandlerFunc(projectH.Get)))
	mux.Handle("PUT /rest/api/3/project/{key}", authMw(http.HandlerFunc(projectH.Update)))
	mux.Handle("DELETE /rest/api/3/project/{key}", authMw(http.HandlerFunc(projectH.Delete)))
	mux.Handle("POST /rest/api/3/project/{key}/archive", authMw(http.HandlerFunc(projectH.Archive)))
	mux.Handle("POST /rest/api/3/project/{key}/restore", authMw(http.HandlerFunc(projectH.Restore)))
	mux.Handle("GET /rest/api/3/project/{key}/members", authMw(http.HandlerFunc(projectH.ListMembers)))
	mux.Handle("POST /rest/api/3/project/{key}/members", authMw(http.HandlerFunc(projectH.AddMember)))
	mux.Handle("DELETE /rest/api/3/project/{key}/members/{userId}", authMw(http.HandlerFunc(projectH.RemoveMember)))
	mux.Handle("POST /rest/api/3/project/{key}/invites", authMw(http.HandlerFunc(projectH.Invite)))
	mux.Handle("PUT /rest/api/3/project/{key}/star", authMw(http.HandlerFunc(projectH.StarProject)))
	mux.Handle("DELETE /rest/api/3/project/{key}/star", authMw(http.HandlerFunc(projectH.UnstarProject)))
	mux.Handle("GET /rest/api/3/project/{key}/git/providers", authMw(http.HandlerFunc(gitH.GetProvider)))
	mux.Handle("POST /rest/api/3/project/{key}/git/providers", authMw(http.HandlerFunc(gitH.ConfigureProvider)))
	mux.Handle("DELETE /rest/api/3/project/{key}/git/providers", authMw(http.HandlerFunc(gitH.DeleteProvider)))
	mux.Handle("GET /rest/api/3/project/{key}/webhooks", authMw(http.HandlerFunc(webhookH.List)))
	mux.Handle("POST /rest/api/3/project/{key}/webhooks", authMw(http.HandlerFunc(webhookH.Create)))
	mux.Handle("DELETE /rest/api/3/project/{key}/webhooks/{id}", authMw(http.HandlerFunc(webhookH.Delete)))

	mux.Handle("GET /rest/api/3/project/{key}/issues", authMw(http.HandlerFunc(issueH.List)))
	mux.Handle("POST /rest/api/3/issue", authMw(http.HandlerFunc(issueH.Create)))
	// Literal segment "createmeta" registered ahead of the "{issueKey}"
	// wildcard below: Go 1.22+ ServeMux resolves the more specific literal
	// route first, so this does not collide with GET /issue/{issueKey}.
	mux.Handle("GET /rest/api/3/issue/createmeta", authMw(http.HandlerFunc(refH.CreateMeta)))
	mux.Handle("GET /rest/api/3/issue/{issueKey}", authMw(http.HandlerFunc(issueH.Get)))
	mux.Handle("GET /rest/api/3/issue/{issueKey}/editmeta", authMw(http.HandlerFunc(refH.EditMeta)))
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

	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/worklog", authMw(http.HandlerFunc(worklogH.List)))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/worklog", authMw(http.HandlerFunc(worklogH.Create)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/worklog/{id}", authMw(http.HandlerFunc(worklogH.Delete)))

	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/votes", authMw(http.HandlerFunc(votesH.List)))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/votes", authMw(http.HandlerFunc(votesH.Add)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/votes", authMw(http.HandlerFunc(votesH.Remove)))

	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/remotelink", authMw(http.HandlerFunc(remoteLinkH.List)))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/remotelink", authMw(http.HandlerFunc(remoteLinkH.Create)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/remotelink/{id}", authMw(http.HandlerFunc(remoteLinkH.Delete)))

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
	mux.Handle("GET /rest/api/3/project/{key}/workflow/transitions", authMw(http.HandlerFunc(wfH.ListTransitions)))
	mux.Handle("PATCH /rest/api/3/project/{key}/workflow/transitions/{id}", authMw(http.HandlerFunc(wfH.UpdateTransition)))
	mux.Handle("DELETE /rest/api/3/project/{key}/workflow/transitions/{id}", authMw(http.HandlerFunc(wfH.DeleteTransition)))
	mux.Handle("PUT /rest/api/3/project/{key}/workflow/statuses/order", authMw(http.HandlerFunc(wfH.ReorderStatuses)))
	mux.Handle("GET /rest/api/3/issue/{issueKey}/transitions", authMw(http.HandlerFunc(wfH.AvailableTransitions)))
	mux.Handle("POST /rest/api/3/issue/{issueKey}/transitions", authMw(http.HandlerFunc(wfH.DoTransition)))
	// /status (collezione) è servito da ReferenceHandler per rispettare lo
	// schema v3 StatusDetails (statusCategory come oggetto {self,id,key,colorName,name});
	// il vecchio wfH.ListStatuses restituiva la forma di dominio grezza (workflow_id,category,...).
	mux.Handle("GET /rest/api/3/status", authMw(http.HandlerFunc(refH.Statuses)))
	mux.Handle("GET /rest/api/3/status/{idOrName}", authMw(http.HandlerFunc(wfH.GetStatus)))
	mux.Handle("GET /rest/api/3/statuscategory", authMw(http.HandlerFunc(refH.StatusCategories)))
	mux.Handle("GET /rest/api/3/statuscategory/{idOrKey}", authMw(http.HandlerFunc(refH.StatusCategoryByID)))
	mux.Handle("GET /rest/api/3/priority", authMw(http.HandlerFunc(refH.Priorities)))
	mux.Handle("GET /rest/api/3/issuetype", authMw(http.HandlerFunc(refH.IssueTypes)))
	mux.Handle("GET /rest/api/3/resolution", authMw(http.HandlerFunc(refH.Resolutions)))
	mux.Handle("GET /rest/api/3/field", authMw(http.HandlerFunc(refH.Fields)))
	mux.Handle("GET /rest/api/3/label", authMw(http.HandlerFunc(refH.Labels)))
	mux.Handle("GET /rest/api/3/workflow/search", authMw(http.HandlerFunc(wfH.SearchWorkflows)))

	mux.Handle("GET /rest/api/3/project/{key}/sprints", authMw(http.HandlerFunc(sprintH.List)))
	mux.Handle("POST /rest/api/3/project/{key}/sprints", authMw(http.HandlerFunc(sprintH.Create)))
	mux.Handle("GET /rest/api/3/project/{key}/sprints/{id}", authMw(http.HandlerFunc(sprintH.Get)))
	mux.Handle("PATCH /rest/api/3/project/{key}/sprints/{id}", authMw(http.HandlerFunc(sprintH.Update)))
	mux.Handle("POST /rest/api/3/project/{key}/sprints/{id}/start", authMw(http.HandlerFunc(sprintH.Start)))
	mux.Handle("POST /rest/api/3/project/{key}/sprints/{id}/complete", authMw(http.HandlerFunc(sprintH.Complete)))

	mux.Handle("GET /rest/api/3/project/{key}/board", authMw(http.HandlerFunc(boardH.GetBoard)))
	mux.Handle("POST /rest/api/3/issues/rank", authMw(http.HandlerFunc(boardH.RankIssue)))

	// --- Agile API 1.0 (Round 5) ---
	mux.Handle("GET /rest/agile/1.0/board", authMw(http.HandlerFunc(agileBoardH.List)))
	mux.Handle("POST /rest/agile/1.0/board", authMw(http.HandlerFunc(agileBoardH.Create)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}", authMw(http.HandlerFunc(agileBoardH.Get)))
	mux.Handle("DELETE /rest/agile/1.0/board/{boardId}", authMw(http.HandlerFunc(agileBoardH.Delete)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/configuration", authMw(http.HandlerFunc(agileBoardH.Configuration)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/backlog", authMw(http.HandlerFunc(agileBoardH.Backlog)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/issue", authMw(http.HandlerFunc(agileBoardH.BoardIssues)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/sprint", authMw(http.HandlerFunc(agileBoardH.BoardSprints)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/epic", authMw(http.HandlerFunc(agileBoardH.BoardEpics)))

	mux.Handle("POST /rest/agile/1.0/sprint", authMw(http.HandlerFunc(agileSprintH.Create)))
	mux.Handle("GET /rest/agile/1.0/sprint/{sprintId}", authMw(http.HandlerFunc(agileSprintH.Get)))
	mux.Handle("POST /rest/agile/1.0/sprint/{sprintId}", authMw(http.HandlerFunc(agileSprintH.Update)))
	mux.Handle("PUT /rest/agile/1.0/sprint/{sprintId}", authMw(http.HandlerFunc(agileSprintH.Update)))
	mux.Handle("DELETE /rest/agile/1.0/sprint/{sprintId}", authMw(http.HandlerFunc(agileSprintH.Delete)))
	mux.Handle("GET /rest/agile/1.0/sprint/{sprintId}/issue", authMw(http.HandlerFunc(agileSprintH.SprintIssues)))
	mux.Handle("POST /rest/agile/1.0/sprint/{sprintId}/issue", authMw(http.HandlerFunc(agileSprintH.MoveToSprint)))

	mux.Handle("PUT /rest/agile/1.0/issue/rank", authMw(http.HandlerFunc(agileMiscH.Rank)))
	mux.Handle("GET /rest/agile/1.0/issue/{issueIdOrKey}", authMw(http.HandlerFunc(agileMiscH.GetIssue)))
	mux.Handle("POST /rest/agile/1.0/backlog/issue", authMw(http.HandlerFunc(agileMiscH.MoveToBacklog)))
	mux.Handle("GET /rest/agile/1.0/epic/{epicIdOrKey}", authMw(http.HandlerFunc(agileMiscH.GetEpic)))

	// --- Ricerca (Round 4) ---
	mux.Handle("GET /rest/api/3/search/jql", authMw(http.HandlerFunc(searchH.SearchJQL)))
	mux.Handle("POST /rest/api/3/search/jql", authMw(http.HandlerFunc(searchH.SearchJQL)))
	mux.Handle("GET /rest/api/3/search", authMw(http.HandlerFunc(searchH.SearchLegacy)))
	mux.Handle("POST /rest/api/3/search", authMw(http.HandlerFunc(searchH.SearchLegacy)))
	mux.Handle("POST /rest/api/3/search/approximate-count", authMw(http.HandlerFunc(searchH.ApproximateCount)))
	mux.Handle("GET /rest/api/3/jql/autocompletedata", authMw(http.HandlerFunc(searchH.Autocomplete)))

	// --- Filtri salvati (Round 4) ---
	mux.Handle("POST /rest/api/3/filter", authMw(http.HandlerFunc(filterH.Create)))
	mux.Handle("GET /rest/api/3/filter/search", authMw(http.HandlerFunc(filterH.Search)))
	mux.Handle("GET /rest/api/3/filter/my", authMw(http.HandlerFunc(filterH.My)))
	mux.Handle("GET /rest/api/3/filter/favourite", authMw(http.HandlerFunc(filterH.Favourite)))
	mux.Handle("GET /rest/api/3/filter/{id}", authMw(http.HandlerFunc(filterH.Get)))
	mux.Handle("PUT /rest/api/3/filter/{id}", authMw(http.HandlerFunc(filterH.Update)))
	mux.Handle("DELETE /rest/api/3/filter/{id}", authMw(http.HandlerFunc(filterH.Delete)))
	mux.Handle("PUT /rest/api/3/filter/{id}/favourite", authMw(http.HandlerFunc(filterH.AddFavourite)))
	mux.Handle("DELETE /rest/api/3/filter/{id}/favourite", authMw(http.HandlerFunc(filterH.RemoveFavourite)))

	mux.Handle("GET /rest/api/3/project/{key}/reports/burndown", authMw(http.HandlerFunc(reportH.Burndown)))
	mux.Handle("GET /rest/api/3/project/{key}/reports/velocity", authMw(http.HandlerFunc(reportH.Velocity)))
	mux.Handle("GET /rest/api/3/project/{key}/reports/burnup", authMw(http.HandlerFunc(reportH.Burnup)))
	mux.Handle("GET /rest/api/3/project/{key}/reports/cfd", authMw(http.HandlerFunc(reportH.CFD)))
	mux.Handle("GET /rest/api/3/project/{key}/reports/pie", authMw(http.HandlerFunc(reportH.Pie)))
	mux.Handle("GET /rest/api/3/project/{key}/reports/created-vs-resolved", authMw(http.HandlerFunc(reportH.CreatedVsResolved)))
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

	// --- Gruppi (Round 8) ---
	mux.Handle("GET /rest/api/3/group", authMw(http.HandlerFunc(groupH.Get)))
	mux.Handle("POST /rest/api/3/group", authMw(http.HandlerFunc(groupH.Create)))
	mux.Handle("DELETE /rest/api/3/group", authMw(http.HandlerFunc(groupH.Delete)))
	mux.Handle("GET /rest/api/3/group/member", authMw(http.HandlerFunc(groupH.Members)))
	mux.Handle("POST /rest/api/3/group/user", authMw(http.HandlerFunc(groupH.AddUser)))
	mux.Handle("DELETE /rest/api/3/group/user", authMw(http.HandlerFunc(groupH.RemoveUser)))
	mux.Handle("GET /rest/api/3/groups/picker", authMw(http.HandlerFunc(groupH.Picker)))

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
