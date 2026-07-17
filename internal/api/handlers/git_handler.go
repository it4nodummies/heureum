package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/git"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
)

type GitHandler struct {
	gitConfigSvc *git.ConfigService
	issueSvc     *issue.Service
	projectSvc   *project.Service
	commentSvc   *issue.CommentService
}

func NewGitHandler(gitConfigSvc *git.ConfigService, issueSvc *issue.Service, projectSvc *project.Service, commentSvc *issue.CommentService) *GitHandler {
	return &GitHandler{
		gitConfigSvc: gitConfigSvc,
		issueSvc:     issueSvc,
		projectSvc:   projectSvc,
		commentSvc:   commentSvc,
	}
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
		http.Error(w, `{"error":"failed to create provider"}`, http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cfg)
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
	json.NewEncoder(w).Encode(cfg)
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
	cfg, err := h.gitConfigSvc.FindByWebhookToken(token)
	if err != nil {
		http.Error(w, `{"error":"invalid webhook token"}`, http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
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

func (h *GitHandler) processPushEvent(cfg *git.GitProviderConfig, event *git.WebhookEvent) {
	branchIssueKeys := git.ExtractIssueKeys(event.Branch)

	for _, commit := range event.Commits {
		commitKeys := git.ExtractIssueKeys(commit.Message)
		allKeys := uniqueKeys(append(branchIssueKeys, commitKeys...))

		for _, issueKey := range allKeys {
			iss, err := h.issueSvc.GetByKey(issueKey)
			if err != nil {
				continue
			}
			created, err := h.gitConfigSvc.LinkCommit(iss.ID, cfg.ID, commit.SHA, commit.Message, commit.Author)
			if err == nil && created {
				h.commentCommitReference(iss.ID, commit.SHA, commit.Message)
			}
		}
	}

	for _, issueKey := range branchIssueKeys {
		iss, err := h.issueSvc.GetByKey(issueKey)
		if err != nil {
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
		iss, err := h.issueSvc.GetByKey(issueKey)
		if err != nil {
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

func (h *GitHandler) autoTransitionIssue(iss *issue.Issue, projectID string) {
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
