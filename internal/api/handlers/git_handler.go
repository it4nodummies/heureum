package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/git"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
)

type GitHandler struct {
	gitConfigSvc *git.ConfigService
	issueSvc     *issue.Service
	projectSvc   *project.Service
	commentSvc   *issue.CommentService
	baseURL      string
}

func NewGitHandler(gitConfigSvc *git.ConfigService, issueSvc *issue.Service, projectSvc *project.Service, commentSvc *issue.CommentService, baseURL string) *GitHandler {
	return &GitHandler{
		gitConfigSvc: gitConfigSvc,
		issueSvc:     issueSvc,
		projectSvc:   projectSvc,
		commentSvc:   commentSvc,
		baseURL:      strings.TrimRight(baseURL, "/"),
	}
}

// gitProviderResponse is the only shape the provider config is ever
// serialized in. It carries no secret: the API token is reduced to
// has_token, the HMAC secret is omitted, and the URL token appears solely
// inside webhook_url (the admin needs that to paste into the provider).
type gitProviderResponse struct {
	ID           string    `json:"id"`
	ProviderType string    `json:"provider_type"`
	BaseURL      string    `json:"base_url"`
	HasToken     bool      `json:"has_token"`
	WebhookURL   string    `json:"webhook_url"`
	CreatedAt    time.Time `json:"created_at"`
}

func (h *GitHandler) toResponse(cfg *git.GitProviderConfig) gitProviderResponse {
	out := gitProviderResponse{
		ID:           cfg.ID,
		ProviderType: string(cfg.ProviderType),
		BaseURL:      cfg.BaseURL,
		HasToken:     cfg.TokenEncrypted != "",
		CreatedAt:    cfg.CreatedAt,
	}
	if cfg.WebhookURLToken != nil && *cfg.WebhookURLToken != "" {
		out.WebhookURL = h.baseURL + "/rest/api/3/webhooks/git/" + *cfg.WebhookURLToken
	}
	return out
}

func (h *GitHandler) ConfigureProvider(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	p, err := h.projectSvc.GetByKey(projectKey)
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	var req struct {
		ProviderType  string `json:"provider_type"`
		BaseURL       string `json:"base_url"`
		Token         string `json:"token"`
		WebhookSecret string `json:"webhook_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.ProviderType == "" || req.BaseURL == "" {
		http.Error(w, `{"error":"provider_type and base_url are required"}`, http.StatusBadRequest)
		return
	}

	cfg, err := h.gitConfigSvc.CreateProvider(p.ID, req.ProviderType, req.BaseURL, req.Token, req.WebhookSecret)
	if err != nil {
		switch {
		case errors.Is(err, git.ErrProviderTypeUnsupported):
			v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"provider_type": err.Error()})
		case errors.Is(err, git.ErrWebhookSecretRequired), errors.Is(err, git.ErrWebhookSecretTooShort):
			v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"webhook_secret": err.Error()})
		default:
			http.Error(w, `{"error":"failed to create provider"}`, http.StatusConflict)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(h.toResponse(cfg))
}

func (h *GitHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	p, err := h.projectSvc.GetByKey(projectKey)
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	cfg, err := h.gitConfigSvc.GetProvider(p.ID)
	if err != nil {
		http.Error(w, `{"error":"no git provider configured"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.toResponse(cfg))
}

func (h *GitHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	p, err := h.projectSvc.GetByKey(projectKey)
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	cfg, err := h.gitConfigSvc.GetProvider(p.ID)
	if err != nil {
		http.Error(w, `{"error":"no git provider configured"}`, http.StatusNotFound)
		return
	}

	if err := h.gitConfigSvc.DeleteProvider(cfg.ID); err != nil {
		http.Error(w, `{"error":"failed to delete provider"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *GitHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	cfg, err := h.gitConfigSvc.FindByWebhookURLToken(token)
	if err != nil {
		http.Error(w, `{"error":"invalid webhook token"}`, http.StatusNotFound)
		return
	}

	// The body is capped by the global middleware.MaxBodyBytes (router.go).
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if middleware.IsBodyTooLarge(err) {
			http.Error(w, `{"error":"request body too large"}`, http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	provider := git.ProviderForType(cfg.ProviderType)

	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		signature = r.Header.Get("X-Forgejo-Signature")
	}
	if signature == "" {
		signature = r.Header.Get("X-Gitea-Signature")
	}
	if signature == "" {
		signature = r.Header.Get("X-Gitlab-Token")
	}

	valid, err := provider.ValidateWebhook(body, cfg.WebhookSecret, signature)
	if err != nil || !valid {
		http.Error(w, `{"error":"invalid signature"}`, http.StatusUnauthorized)
		return
	}

	event, err := provider.ParseWebhook(body)
	if err != nil {
		http.Error(w, `{"error":"failed to parse webhook"}`, http.StatusBadRequest)
		return
	}

	h.processWebhookEvent(cfg, event)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *GitHandler) processWebhookEvent(cfg *git.GitProviderConfig, event *git.WebhookEvent) {
	if event.EventType == "push" {
		h.processPushEvent(cfg, event)
	} else if event.EventType == "pull_request" {
		h.processPullRequestEvent(cfg, event)
	}
}

// projectIssue resolves an issue key referenced by an inbound event, but only
// within the provider's own project. The event is authenticated by THAT
// project's secret, and any authenticated user can create a project and
// configure a provider: without this scope, correctly signed events could
// comment on / link to / auto-transition any issue on the instance. A key
// from another project is treated exactly like an unknown one (nil, skipped
// silently) so a stray reference never fails the whole webhook.
func (h *GitHandler) projectIssue(cfg *git.GitProviderConfig, issueKey string) *issue.Issue {
	iss, err := h.issueSvc.GetByKey(issueKey)
	if err != nil || iss.ProjectID != cfg.ProjectID {
		return nil
	}
	return iss
}

func (h *GitHandler) processPushEvent(cfg *git.GitProviderConfig, event *git.WebhookEvent) {
	branchIssueKeys := git.ExtractIssueKeys(event.Branch)

	for _, commit := range event.Commits {
		commitKeys := git.ExtractIssueKeys(commit.Message)
		allKeys := uniqueKeys(append(branchIssueKeys, commitKeys...))

		for _, issueKey := range allKeys {
			iss := h.projectIssue(cfg, issueKey)
			if iss == nil {
				continue
			}
			created, err := h.gitConfigSvc.LinkCommit(iss.ID, cfg.ID, commit.SHA, commit.Message, commit.Author)
			if err == nil && created {
				h.commentCommitReference(iss.ID, commit.SHA, commit.Message)
			}
		}
	}

	for _, issueKey := range branchIssueKeys {
		iss := h.projectIssue(cfg, issueKey)
		if iss == nil {
			continue
		}
		h.gitConfigSvc.LinkBranch(iss.ID, cfg.ID, event.Branch, event.RepoURL)
	}
}

func (h *GitHandler) processPullRequestEvent(cfg *git.GitProviderConfig, event *git.WebhookEvent) {
	pr := event.PullRequest
	if pr == nil {
		return
	}

	branchKeys := git.ExtractIssueKeys(pr.BranchName)
	titleKeys := git.ExtractIssueKeys(pr.Title)
	allKeys := uniqueKeys(append(branchKeys, titleKeys...))

	for _, issueKey := range allKeys {
		iss := h.projectIssue(cfg, issueKey)
		if iss == nil {
			continue
		}

		existingPR, err := h.gitConfigSvc.FindPullRequestByNumber(cfg.ID, pr.Number)
		if err == nil {
			if pr.State == "merged" {
				now := time.Now()
				h.gitConfigSvc.UpdatePullRequestState(existingPR.ID, "merged", &now)
				h.autoTransitionIssue(iss, cfg.ProjectID)
			} else if pr.State == "closed" {
				h.gitConfigSvc.UpdatePullRequestState(existingPR.ID, "closed", nil)
			}
		} else {
			h.gitConfigSvc.LinkPullRequest(iss.ID, cfg.ID, pr.Number, pr.Title, pr.URL, pr.State)
			if pr.State == "merged" {
				h.autoTransitionIssue(iss, cfg.ProjectID)
			}
		}
	}
}

// autoTransitionIssue moves iss to the first "done" status of projectID's
// workflow. The issue must belong to that project: a status id from another
// project's workflow would corrupt the issue (callers already scope lookups
// with projectIssue; this is the last line of defence).
func (h *GitHandler) autoTransitionIssue(iss *issue.Issue, projectID string) {
	if iss == nil || iss.ProjectID != projectID {
		return
	}
	db := h.gitConfigSvc.DB()
	var statuses []struct {
		ID       string `gorm:"column:id"`
		Category string `gorm:"column:category"`
	}
	db.Table("workflow_statuses").
		Select("workflow_statuses.id, workflow_statuses.category").
		Joins("JOIN workflows ON workflows.id = workflow_statuses.workflow_id").
		Where("workflows.project_id = ?", projectID).
		Scan(&statuses)

	var doneStatusID string
	for _, s := range statuses {
		if s.Category == "done" && iss.StatusID != nil && *iss.StatusID != s.ID {
			doneStatusID = s.ID
			break
		}
	}

	if doneStatusID != "" {
		h.issueSvc.Update(iss.Key, nil, nil, nil, nil, &doneStatusID, nil)
	}
}

func (h *GitHandler) commentCommitReference(issueID, sha, message string) {
	if h.commentSvc == nil {
		return
	}
	short := sha
	if len(short) > 8 {
		short = short[:8]
	}
	body, err := buildCommitCommentADF(short, message)
	if err != nil {
		return
	}
	_, _ = h.commentSvc.AddComment(issueID, "", body)
}

// buildCommitCommentADF builds the ADF (Atlassian Document Format) JSON body
// for an auto-generated "commit referenced this issue" comment. It marshals
// via encoding/json rather than string interpolation so that commit messages
// containing control bytes or invalid UTF-8 still produce valid JSON.
func buildCommitCommentADF(shortSHA, message string) (string, error) {
	text := fmt.Sprintf("Commit %s referenced this issue: %s", shortSHA, message)
	doc := map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": text},
				},
			},
		},
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func uniqueKeys(keys []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, k := range keys {
		if !seen[k] {
			seen[k] = true
			result = append(result, k)
		}
	}
	return result
}

func (h *GitHandler) GetIssueGitInfo(w http.ResponseWriter, r *http.Request) {
	issueKey := r.PathValue("issueKey")
	iss, err := h.issueSvc.GetByKey(issueKey)
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}

	commits, _ := h.gitConfigSvc.GetIssueCommits(iss.ID)
	branches, _ := h.gitConfigSvc.GetIssueBranches(iss.ID)
	prs, _ := h.gitConfigSvc.GetIssuePullRequests(iss.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"commits":       commits,
		"branches":      branches,
		"pull_requests": prs,
	})
}
