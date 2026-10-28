package git

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"strings"
)

type GitLabProvider struct{}

func NewGitLabProvider() *GitLabProvider { return &GitLabProvider{} }

func (p *GitLabProvider) GetName() ProviderType { return ProviderGitLab }

// ValidateWebhook checks the shared token GitLab sends in X-Gitlab-Token.
// Constant-time compare (a plain == leaks the matching prefix length via
// timing); empty on either side is a hard reject.
func (p *GitLabProvider) ValidateWebhook(payload []byte, secret string, signature string) (bool, error) {
	if signature == "" {
		return false, fmt.Errorf("missing token")
	}
	if secret == "" {
		return false, fmt.Errorf("webhook secret not configured")
	}
	return subtle.ConstantTimeCompare([]byte(signature), []byte(secret)) == 1, nil
}

func (p *GitLabProvider) ParseWebhook(payload []byte) (*WebhookEvent, error) {
	var envelope struct {
		ObjectKind string `json:"object_kind"`
		Ref        string `json:"ref"`
		Repository struct {
			Homepage string `json:"homepage"`
		} `json:"repository"`
		Commits []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commits"`
		EventType        string `json:"event_type"`
		ObjectAttributes *struct {
			IID          int     `json:"iid"`
			Title        string  `json:"title"`
			URL          string  `json:"url"`
			State        string  `json:"state"`
			Action       string  `json:"action"`
			MergedAt     *string `json:"merged_at"`
			SourceBranch string  `json:"source_branch"`
		} `json:"object_attributes"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse gitlab webhook: %w", err)
	}

	event := &WebhookEvent{
		Provider:   ProviderGitLab,
		RawPayload: payload,
	}

	kind := envelope.ObjectKind
	if kind == "" {
		kind = envelope.EventType
	}

	if kind == "push" && envelope.Ref != "" {
		event.EventType = "push"
		event.RepoURL = envelope.Repository.Homepage
		event.Branch = strings.TrimPrefix(envelope.Ref, "refs/heads/")
		for _, c := range envelope.Commits {
			event.Commits = append(event.Commits, CommitInfo{
				SHA:     c.ID,
				Message: c.Message,
				Author:  c.Author.Name,
				Branch:  event.Branch,
				RepoURL: event.RepoURL,
			})
		}
	}

	if (kind == "merge_request" || kind == "") && envelope.ObjectAttributes != nil {
		event.EventType = "pull_request"
		event.RepoURL = envelope.Repository.Homepage
		oa := envelope.ObjectAttributes
		state := oa.State
		if oa.Action == "merge" || (oa.State == "merged") {
			state = "merged"
		}
		mergedAt := ""
		if oa.MergedAt != nil {
			mergedAt = *oa.MergedAt
		}
		event.PullRequest = &PRInfo{
			Number:     oa.IID,
			Title:      oa.Title,
			URL:        oa.URL,
			State:      state,
			MergedAt:   mergedAt,
			BranchName: oa.SourceBranch,
		}
	}

	return event, nil
}

var _ GitProvider = (*GitLabProvider)(nil)
