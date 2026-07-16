package api

import (
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/handlers"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/api/ws"
	"github.com/it4nodummies/heureum/internal/config"
	"github.com/it4nodummies/heureum/internal/domain/auth"
	"github.com/it4nodummies/heureum/internal/domain/automation"
	"github.com/it4nodummies/heureum/internal/domain/board"
	"github.com/it4nodummies/heureum/internal/domain/calendar"
	"github.com/it4nodummies/heureum/internal/domain/customfield"
	"github.com/it4nodummies/heureum/internal/domain/dashboard"
	"github.com/it4nodummies/heureum/internal/domain/git"
	"github.com/it4nodummies/heureum/internal/domain/group"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/notification"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/report"
	"github.com/it4nodummies/heureum/internal/domain/search"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
	"github.com/it4nodummies/heureum/internal/domain/timeline"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/webhook"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
	"github.com/it4nodummies/heureum/internal/integration"
)

func NewRouter(cfg *config.Config, db *gorm.DB) http.Handler {
	mux := http.NewServeMux()

	authSvc := auth.NewService(db, cfg.Secret)
	authH := handlers.NewAuthHandler(authSvc, cfg.SignupOpen)
	oauthH := handlers.NewOAuthHandler(db, cfg.Secret, cfg.BaseURL)
	projectSvc := project.NewService(db, nil)
	permH := handlers.NewPermissionHandler(db, projectSvc)
	wfSvc := workflow.NewService(db)
	pcH := handlers.NewProjectCategoryHandler(db, cfg.BaseURL)
	issueSvc := issue.NewService(db)

	// Authz checker: built early (before the handler constructors below) because
	// several handlers (Task 5, Round 11 plan) need the *authz.Checker injected
	// to enforce permissions in-handler for body-scoped/two-hop mutating routes
	// (issue create, issueLink, attachment delete, agile board/sprint create,
	// rank/backlog, custom-field option delete). Its dependent services below
	// only need `db`, so they can be constructed ahead of their other uses.
	boardSvc := board.NewService(db)
	sprintSvc := sprint.NewService(db)
	autoSvc := automation.NewService(db)
	cfSvc := customfield.NewService(db)
	userSvc := user.NewService(db)
	chk := authz.New(userSvc, projectSvc, issueSvc, boardSvc, sprintSvc, autoSvc, cfSvc)
	userH := handlers.NewUserHandler(db, cfg.BaseURL, chk)
	projectH := handlers.NewProjectHandler(projectSvc, wfSvc, chk, cfg.BaseURL)

	issueH := handlers.NewIssueHandler(issueSvc, projectSvc, wfSvc, chk, cfg.BaseURL)
	commentSvc := issue.NewCommentService(db)
	commentH := handlers.NewCommentHandler(commentSvc, issueSvc, cfg.BaseURL)
	worklogSvc := issue.NewWorklogService(db)
	worklogH := handlers.NewWorklogHandler(worklogSvc, issueSvc, cfg.BaseURL)
	voteSvc := issue.NewVoteService(db)
	votesH := handlers.NewVotesHandler(voteSvc, issueSvc, cfg.BaseURL)
	remoteLinkSvc := issue.NewRemoteLinkService(db)
	remoteLinkH := handlers.NewRemoteLinkHandler(remoteLinkSvc, issueSvc, cfg.BaseURL)
	historyH := handlers.NewHistoryHandler(db, issueSvc, cfg.BaseURL)
	attachmentSvc := issue.NewAttachmentService(db, cfg.UploadsDir)
	attachmentH := handlers.NewAttachmentHandler(attachmentSvc, issueSvc, chk)
	issueLinkH := handlers.NewIssueLinkHandler(issueSvc, chk, cfg.BaseURL)
	wfH := handlers.NewWorkflowHandler(wfSvc, issueSvc, projectSvc, cfg.BaseURL)
	sprintH := handlers.NewSprintHandler(sprintSvc, projectSvc)
	boardH := handlers.NewBoardHandler(issueSvc, projectSvc, wfSvc, chk)
	searchSvc := search.NewService(db)
	searchH := handlers.NewSearchHandler(searchSvc, issueH, chk, projectSvc)
	filterSvc := search.NewFilterService(db)
	filterH := handlers.NewFilterHandler(filterSvc, db, cfg.BaseURL, chk)
	reportSvc := report.NewService(db)
	reportH := handlers.NewReportHandler(reportSvc, projectSvc)
	dashboardSvc := dashboard.NewService(db)
	dashboardH := handlers.NewDashboardHandler(dashboardSvc, chk)
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

	cfH := handlers.NewCustomFieldHandler(cfSvc, chk)
	autoH := handlers.NewAutomationHandler(autoSvc)
	webhookSvc := webhook.NewService(db)
	webhookH := handlers.NewWebhookHandler(webhookSvc, projectSvc)
	dispatcher := integration.NewDispatcher(webhookSvc, autoSvc, &http.Client{Timeout: 10 * time.Second})
	issueSvc.SetEventSink(dispatcher)
	timelineSvc := timeline.NewService(db)
	timelineH := handlers.NewTimelineHandler(timelineSvc)
	calendarSvc := calendar.NewService(db)
	calendarH := handlers.NewCalendarHandler(calendarSvc)
	refH := handlers.NewReferenceHandler(db, cfg.BaseURL, chk, projectSvc)

	groupSvc := group.NewService(db)
	groupH := handlers.NewGroupHandler(groupSvc, db, cfg.BaseURL)

	agileBoardH := handlers.NewAgileBoardHandler(boardSvc, projectSvc, issueSvc, sprintSvc, wfSvc, issueH, chk, cfg.BaseURL)
	agileSprintH := handlers.NewAgileSprintHandler(sprintSvc, boardSvc, issueSvc, issueH, chk, cfg.BaseURL)
	agileMiscH := handlers.NewAgileMiscHandler(issueSvc, sprintSvc, issueH, chk, cfg.BaseURL)

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
	mux.Handle("POST /rest/api/3/projectCategory", authMw(chk.EnforceGlobalAdmin(http.HandlerFunc(pcH.Create))))
	mux.Handle("GET /rest/api/3/project/{key}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(projectH.Get))))
	mux.Handle("PUT /rest/api/3/project/{key}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(projectH.Update))))
	mux.Handle("DELETE /rest/api/3/project/{key}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(projectH.Delete))))
	mux.Handle("POST /rest/api/3/project/{key}/archive", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(projectH.Archive))))
	mux.Handle("POST /rest/api/3/project/{key}/restore", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(projectH.Restore))))
	mux.Handle("GET /rest/api/3/project/{key}/members", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(projectH.ListMembers))))
	mux.Handle("POST /rest/api/3/project/{key}/members", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(projectH.AddMember))))
	mux.Handle("DELETE /rest/api/3/project/{key}/members/{userId}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(projectH.RemoveMember))))
	mux.Handle("POST /rest/api/3/project/{key}/invites", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(projectH.Invite))))
	mux.Handle("PUT /rest/api/3/project/{key}/star", authMw(chk.Enforce(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(projectH.StarProject))))
	mux.Handle("DELETE /rest/api/3/project/{key}/star", authMw(chk.Enforce(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(projectH.UnstarProject))))
	mux.Handle("GET /rest/api/3/project/{key}/git/providers", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(gitH.GetProvider))))
	mux.Handle("POST /rest/api/3/project/{key}/git/providers", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(gitH.ConfigureProvider))))
	mux.Handle("DELETE /rest/api/3/project/{key}/git/providers", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(gitH.DeleteProvider))))
	mux.Handle("GET /rest/api/3/project/{key}/webhooks", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(webhookH.List))))
	mux.Handle("POST /rest/api/3/project/{key}/webhooks", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(webhookH.Create))))
	mux.Handle("DELETE /rest/api/3/project/{key}/webhooks/{id}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(webhookH.Delete))))

	mux.Handle("GET /rest/api/3/project/{key}/issues", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(issueH.List))))
	mux.Handle("POST /rest/api/3/issue", authMw(http.HandlerFunc(issueH.Create)))
	// Literal segment "createmeta" registered ahead of the "{issueKey}"
	// wildcard below: Go 1.22+ ServeMux resolves the more specific literal
	// route first, so this does not collide with GET /issue/{issueKey}.
	mux.Handle("GET /rest/api/3/issue/createmeta", authMw(http.HandlerFunc(refH.CreateMeta)))
	mux.Handle("GET /rest/api/3/issue/{issueKey}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueKey"), http.HandlerFunc(issueH.Get))))
	mux.Handle("GET /rest/api/3/issue/{issueKey}/editmeta", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueKey"), http.HandlerFunc(refH.EditMeta))))
	mux.Handle("PUT /rest/api/3/issue/{issueKey}", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueKey"), http.HandlerFunc(issueH.Update))))
	mux.Handle("DELETE /rest/api/3/issue/{issueKey}", authMw(chk.Enforce(permission.DeleteIssues, chk.ByIssueParam("issueKey"), http.HandlerFunc(issueH.Delete))))
	mux.Handle("POST /rest/api/3/issue/{issueKey}/labels", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueKey"), http.HandlerFunc(issueH.AddLabel))))
	mux.Handle("GET /rest/api/3/issue/{issueKey}/changelog", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueKey"), http.HandlerFunc(historyH.GetHistory))))
	mux.Handle("GET /rest/api/3/issue/{issueKey}/git", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueKey"), http.HandlerFunc(gitH.GetIssueGitInfo))))
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/subtasks", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(issueH.Subtasks))))
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/watchers", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(issueH.GetWatchers))))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/watchers", authMw(chk.Enforce(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(issueH.AddWatcher))))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/watchers", authMw(chk.Enforce(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(issueH.RemoveWatcher))))

	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/comment", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(commentH.List))))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/comment", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(commentH.Create))))
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/comment/{id}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(commentH.Get))))
	mux.Handle("PUT /rest/api/3/issue/{issueIdOrKey}/comment/{id}", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(commentH.Update))))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/comment/{id}", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(commentH.Delete))))
	mux.Handle("POST /rest/api/3/comment/list", authMw(http.HandlerFunc(commentH.ListByIDs)))

	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/worklog", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(worklogH.List))))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/worklog", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(worklogH.Create))))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/worklog/{id}", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(worklogH.Delete))))

	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/votes", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(votesH.List))))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/votes", authMw(chk.Enforce(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(votesH.Add))))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/votes", authMw(chk.Enforce(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(votesH.Remove))))

	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/remotelink", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(remoteLinkH.List))))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/remotelink", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(remoteLinkH.Create))))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/remotelink/{id}", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(remoteLinkH.Delete))))

	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/attachments", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(attachmentH.Upload))))
	mux.Handle("GET /rest/api/3/attachment/{id}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByAttachment("id"), http.HandlerFunc(attachmentH.Get))))
	// DELETE /attachment/{id}: two-hop resolver (attachment -> issue -> project);
	// enforced in-handler (AttachmentHandler.Delete) per the Round 11 plan.
	mux.Handle("DELETE /rest/api/3/attachment/{id}", authMw(http.HandlerFunc(attachmentH.Delete)))
	mux.Handle("GET /rest/api/3/attachment/content/{id}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByAttachment("id"), http.HandlerFunc(attachmentH.ServeFile))))
	// GET /attachment/meta: global metadata (max upload size / allowed types),
	// no project-scoped data — left open per the Task 7 plan.
	mux.Handle("GET /rest/api/3/attachment/meta", authMw(http.HandlerFunc(attachmentH.Meta)))

	mux.Handle("GET /rest/api/3/issueLink/{linkId}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueLink("linkId"), http.HandlerFunc(issueLinkH.Get))))
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/issuelinks", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(issueLinkH.ListForIssue))))
	// POST /issueLink: project resolved from body (source/outward issue) ->
	// enforced in-handler (IssueLinkHandler.Create) per the Round 11 plan.
	mux.Handle("POST /rest/api/3/issueLink", authMw(http.HandlerFunc(issueLinkH.Create)))
	// DELETE /issueLink/{linkId}: two-hop resolver (link -> issue -> project);
	// enforced in-handler (IssueLinkHandler.Delete) per the Round 11 plan.
	mux.Handle("DELETE /rest/api/3/issueLink/{linkId}", authMw(http.HandlerFunc(issueLinkH.Delete)))

	mux.Handle("GET /rest/api/3/project/{key}/workflow", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(wfH.GetWorkflow))))
	mux.Handle("POST /rest/api/3/project/{key}/workflow/statuses", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.AddStatus))))
	mux.Handle("PATCH /rest/api/3/project/{key}/workflow/statuses/{id}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.UpdateStatus))))
	mux.Handle("DELETE /rest/api/3/project/{key}/workflow/statuses/{id}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.DeleteStatus))))
	mux.Handle("POST /rest/api/3/project/{key}/workflow/transitions", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.AddTransition))))
	mux.Handle("GET /rest/api/3/project/{key}/workflow/transitions", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(wfH.ListTransitions))))
	mux.Handle("PATCH /rest/api/3/project/{key}/workflow/transitions/{id}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.UpdateTransition))))
	mux.Handle("DELETE /rest/api/3/project/{key}/workflow/transitions/{id}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.DeleteTransition))))
	mux.Handle("PUT /rest/api/3/project/{key}/workflow/statuses/order", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(wfH.ReorderStatuses))))
	mux.Handle("GET /rest/api/3/issue/{issueKey}/transitions", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueKey"), http.HandlerFunc(wfH.AvailableTransitions))))
	mux.Handle("POST /rest/api/3/issue/{issueKey}/transitions", authMw(chk.Enforce(permission.TransitionIssues, chk.ByIssueParam("issueKey"), http.HandlerFunc(wfH.DoTransition))))
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

	mux.Handle("GET /rest/api/3/project/{key}/sprints", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(sprintH.List))))
	mux.Handle("POST /rest/api/3/project/{key}/sprints", authMw(chk.Enforce(permission.ManageSprints, chk.ByKey, http.HandlerFunc(sprintH.Create))))
	mux.Handle("GET /rest/api/3/project/{key}/sprints/{id}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(sprintH.Get))))
	mux.Handle("PATCH /rest/api/3/project/{key}/sprints/{id}", authMw(chk.Enforce(permission.ManageSprints, chk.ByKey, http.HandlerFunc(sprintH.Update))))
	mux.Handle("POST /rest/api/3/project/{key}/sprints/{id}/start", authMw(chk.Enforce(permission.ManageSprints, chk.ByKey, http.HandlerFunc(sprintH.Start))))
	mux.Handle("POST /rest/api/3/project/{key}/sprints/{id}/complete", authMw(chk.Enforce(permission.ManageSprints, chk.ByKey, http.HandlerFunc(sprintH.Complete))))

	mux.Handle("GET /rest/api/3/project/{key}/board", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(boardH.GetBoard))))
	// POST /issues/rank: project resolved from body (target issue_id) ->
	// enforced in-handler (BoardHandler.RankIssue) per the Round 11 plan.
	mux.Handle("POST /rest/api/3/issues/rank", authMw(http.HandlerFunc(boardH.RankIssue)))

	// --- Agile API 1.0 (Round 5) ---
	mux.Handle("GET /rest/agile/1.0/board", authMw(http.HandlerFunc(agileBoardH.List)))
	// POST /board: project resolved from body (projectKeyOrId) ->
	// enforced in-handler (AgileBoardHandler.Create) per the Round 11 plan.
	mux.Handle("POST /rest/agile/1.0/board", authMw(http.HandlerFunc(agileBoardH.Create)))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByBoardSeq("boardId"), http.HandlerFunc(agileBoardH.Get))))
	mux.Handle("DELETE /rest/agile/1.0/board/{boardId}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByBoardSeq("boardId"), http.HandlerFunc(agileBoardH.Delete))))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/configuration", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByBoardSeq("boardId"), http.HandlerFunc(agileBoardH.Configuration))))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/backlog", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByBoardSeq("boardId"), http.HandlerFunc(agileBoardH.Backlog))))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/issue", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByBoardSeq("boardId"), http.HandlerFunc(agileBoardH.BoardIssues))))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/sprint", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByBoardSeq("boardId"), http.HandlerFunc(agileBoardH.BoardSprints))))
	mux.Handle("GET /rest/agile/1.0/board/{boardId}/epic", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByBoardSeq("boardId"), http.HandlerFunc(agileBoardH.BoardEpics))))

	// POST /sprint: project resolved from body (originBoardId -> board.ProjectID) ->
	// enforced in-handler (AgileSprintHandler.Create) per the Round 11 plan.
	mux.Handle("POST /rest/agile/1.0/sprint", authMw(http.HandlerFunc(agileSprintH.Create)))
	mux.Handle("GET /rest/agile/1.0/sprint/{sprintId}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.BySprintSeq("sprintId"), http.HandlerFunc(agileSprintH.Get))))
	mux.Handle("POST /rest/agile/1.0/sprint/{sprintId}", authMw(chk.Enforce(permission.ManageSprints, chk.BySprintSeq("sprintId"), http.HandlerFunc(agileSprintH.Update))))
	mux.Handle("PUT /rest/agile/1.0/sprint/{sprintId}", authMw(chk.Enforce(permission.ManageSprints, chk.BySprintSeq("sprintId"), http.HandlerFunc(agileSprintH.Update))))
	mux.Handle("DELETE /rest/agile/1.0/sprint/{sprintId}", authMw(chk.Enforce(permission.ManageSprints, chk.BySprintSeq("sprintId"), http.HandlerFunc(agileSprintH.Delete))))
	mux.Handle("GET /rest/agile/1.0/sprint/{sprintId}/issue", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.BySprintSeq("sprintId"), http.HandlerFunc(agileSprintH.SprintIssues))))
	mux.Handle("POST /rest/agile/1.0/sprint/{sprintId}/issue", authMw(chk.Enforce(permission.ManageSprints, chk.BySprintSeq("sprintId"), http.HandlerFunc(agileSprintH.MoveToSprint))))

	// PUT /issue/rank, POST /backlog/issue: project resolved from body (first
	// issue in the list) -> enforced in-handler (AgileMiscHandler) per the
	// Round 11 plan; see the 1.0 cross-project simplification comment there.
	mux.Handle("PUT /rest/agile/1.0/issue/rank", authMw(http.HandlerFunc(agileMiscH.Rank)))
	mux.Handle("GET /rest/agile/1.0/issue/{issueIdOrKey}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(agileMiscH.GetIssue))))
	mux.Handle("POST /rest/agile/1.0/backlog/issue", authMw(http.HandlerFunc(agileMiscH.MoveToBacklog)))
	mux.Handle("GET /rest/agile/1.0/epic/{epicIdOrKey}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("epicIdOrKey"), http.HandlerFunc(agileMiscH.GetEpic))))

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

	mux.Handle("GET /rest/api/3/project/{key}/reports/burndown", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(reportH.Burndown))))
	mux.Handle("GET /rest/api/3/project/{key}/reports/velocity", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(reportH.Velocity))))
	mux.Handle("GET /rest/api/3/project/{key}/reports/burnup", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(reportH.Burnup))))
	mux.Handle("GET /rest/api/3/project/{key}/reports/cfd", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(reportH.CFD))))
	mux.Handle("GET /rest/api/3/project/{key}/reports/pie", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(reportH.Pie))))
	mux.Handle("GET /rest/api/3/project/{key}/reports/created-vs-resolved", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(reportH.CreatedVsResolved))))
	mux.Handle("GET /rest/api/3/project/{key}/summary", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(reportH.Summary))))

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

	// Autenticata come le altre rotte REST (authMw legge il bearer token
	// dall'header Authorization). FOLLOW-UP: nessun client browser consuma
	// ancora questa rotta (i WebSocket browser non possono impostare header
	// custom sull'handshake), quindi servirà un meccanismo dedicato — token
	// short-lived in query string o subprotocol — quando un client verrà
	// implementato.
	mux.Handle("GET /ws/v1/projects/{key}/board", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(wsHub, w, r)
	})))

	mux.Handle("GET /rest/api/3/notifications", authMw(http.HandlerFunc(notifH.List)))
	mux.Handle("GET /rest/api/3/notifications/unread-count", authMw(http.HandlerFunc(notifH.UnreadCount)))
	mux.Handle("PATCH /rest/api/3/notifications/read-all", authMw(http.HandlerFunc(notifH.MarkAllRead)))
	mux.Handle("PATCH /rest/api/3/notifications/{id}/read", authMw(http.HandlerFunc(notifH.MarkRead)))
	mux.Handle("GET /rest/api/3/notifications/settings", authMw(http.HandlerFunc(notifH.GetSettings)))
	mux.Handle("PATCH /rest/api/3/notifications/settings", authMw(http.HandlerFunc(notifH.UpdateSettings)))

	mux.Handle("GET /rest/api/3/project/{key}/issues/export", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(issueH.ExportCSV))))

	mux.Handle("GET /rest/api/3/project/{projectID}/custom-fields", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByProjectID, http.HandlerFunc(cfH.ListFields))))
	mux.Handle("POST /rest/api/3/project/{projectID}/custom-fields", authMw(chk.Enforce(permission.AdministerProjects, chk.ByProjectID, http.HandlerFunc(cfH.CreateField))))
	mux.Handle("DELETE /rest/api/3/custom-fields/{fieldID}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByCustomField("fieldID"), http.HandlerFunc(cfH.DeleteField))))
	mux.Handle("GET /rest/api/3/custom-fields/{fieldID}/options", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByCustomField("fieldID"), http.HandlerFunc(cfH.ListOptions))))
	mux.Handle("POST /rest/api/3/custom-fields/{fieldID}/options", authMw(chk.Enforce(permission.AdministerProjects, chk.ByCustomField("fieldID"), http.HandlerFunc(cfH.AddOption))))
	// DELETE /custom-fields/options/{optionID}: two-hop resolver (option -> field -> project);
	// enforced in-handler (CustomFieldHandler.RemoveOption) per the Round 11 plan
	// (customfield.Service.GetOption added for the option->field lookup).
	mux.Handle("DELETE /rest/api/3/custom-fields/options/{optionID}", authMw(http.HandlerFunc(cfH.RemoveOption)))
	mux.Handle("GET /rest/api/3/issue/{issueID}/custom-values", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueUUID, http.HandlerFunc(cfH.GetValues))))
	mux.Handle("PUT /rest/api/3/issue/{issueID}/custom-values/{fieldID}", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueUUID, http.HandlerFunc(cfH.SetValue))))

	mux.Handle("GET /rest/api/3/project/{projectID}/automation", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByProjectID, http.HandlerFunc(autoH.ListRules))))
	mux.Handle("POST /rest/api/3/project/{projectID}/automation", authMw(chk.Enforce(permission.AdministerProjects, chk.ByProjectID, http.HandlerFunc(autoH.CreateRule))))
	mux.Handle("GET /rest/api/3/automation/{ruleID}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByAutomationRule("ruleID"), http.HandlerFunc(autoH.GetRule))))
	mux.Handle("PATCH /rest/api/3/automation/{ruleID}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByAutomationRule("ruleID"), http.HandlerFunc(autoH.UpdateRule))))
	mux.Handle("DELETE /rest/api/3/automation/{ruleID}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByAutomationRule("ruleID"), http.HandlerFunc(autoH.DeleteRule))))
	mux.Handle("POST /rest/api/3/automation/{ruleID}/execute", authMw(chk.Enforce(permission.AdministerProjects, chk.ByAutomationRule("ruleID"), http.HandlerFunc(autoH.ExecuteRule))))
	mux.Handle("GET /rest/api/3/automation/{ruleID}/runs", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByAutomationRule("ruleID"), http.HandlerFunc(autoH.ListRuns))))

	mux.Handle("GET /rest/api/3/project/{projectID}/timeline", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByProjectID, http.HandlerFunc(timelineH.GetTimeline))))
	mux.Handle("GET /rest/api/3/project/{projectID}/calendar", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByProjectID, http.HandlerFunc(calendarH.GetCalendar))))

	// --- Gruppi (Round 8) ---
	mux.Handle("GET /rest/api/3/group", authMw(http.HandlerFunc(groupH.Get)))
	mux.Handle("POST /rest/api/3/group", authMw(chk.EnforceGlobalAdmin(http.HandlerFunc(groupH.Create))))
	mux.Handle("DELETE /rest/api/3/group", authMw(chk.EnforceGlobalAdmin(http.HandlerFunc(groupH.Delete))))
	mux.Handle("GET /rest/api/3/group/member", authMw(http.HandlerFunc(groupH.Members)))
	mux.Handle("POST /rest/api/3/group/user", authMw(chk.EnforceGlobalAdmin(http.HandlerFunc(groupH.AddUser))))
	mux.Handle("DELETE /rest/api/3/group/user", authMw(chk.EnforceGlobalAdmin(http.HandlerFunc(groupH.RemoveUser))))
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
