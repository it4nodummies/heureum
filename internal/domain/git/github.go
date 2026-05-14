package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type GitHubProvider struct{}

func NewGitHubProvider() *GitHubProvider { return &GitHubProvider{} }

func (p *GitHubProvider) GetName() ProviderType { return ProviderGitHub }

func (p *GitHubProvider) ValidateWebhook(payload []byte, secret string, signature string) (bool, error) {
	if signature == "" {
		return false, fmt.Errorf("missing signature")
	}
	sig := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected)), nil
}

func (p *GitHubProvider) ParseWebhook(payload []byte) (*WebhookEvent, error) {
	var envelope struct {
		Ref        string `json:"ref"`
		Repository struct {
			HTMLURL string `json:"html_url"`
		} `json:"repository"`
		Commits []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			Author  struct {
				Username string `json:"username"`
				Name     string `json:"name"`
			} `json:"author"`
		} `json:"commits"`
		Action      string `json:"action"`
		PullRequest *struct {
			Number   int    `json:"number"`
			Title    string `json:"title"`
			HTMLURL  string `json:"html_url"`
			State    string `json:"state"`
			Merged   bool   `json:"merged"`
			MergedAt *string `json:"merged_at"`
			Head     struct {
				Ref string `json:"ref"`
			} `json:"head"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse github webhook: %w", err)
	}

	event := &WebhookEvent{
		Provider:   ProviderGitHub,
		RawPayload: payload,
	}

	if envelope.Ref != "" && len(envelope.Commits) > 0 {
		event.EventType = "push"
		event.RepoURL = envelope.Repository.HTMLURL
		event.Branch = strings.TrimPrefix(envelope.Ref, "refs/heads/")
		for _, c := range envelope.Commits {
			author := c.Author.Username
			if author == "" {
				author = c.Author.Name
			}
			event.Commits = append(event.Commits, CommitInfo{
				SHA:     c.ID,
				Message: c.Message,
				Author:  author,
				Branch:  event.Branch,
				RepoURL: event.RepoURL,
			})
		}
	}

	if envelope.PullRequest != nil {
		event.EventType = "pull_request"
		event.RepoURL = envelope.Repository.HTMLURL
		pr := envelope.PullRequest
		state := pr.State
		if pr.Merged || (envelope.Action == "closed" && pr.MergedAt != nil) {
			state = "merged"
		}
		mergedAt := ""
		if pr.MergedAt != nil {
			mergedAt = *pr.MergedAt
		}
		event.PullRequest = &PRInfo{
			Number:     pr.Number,
			Title:      pr.Title,
			URL:        pr.HTMLURL,
			State:      state,
			MergedAt:   mergedAt,
			BranchName: pr.Head.Ref,
		}
	}

	return event, nil
}

var _ GitProvider = (*GitHubProvider)(nil)
