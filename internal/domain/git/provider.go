package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

type ProviderType string

const (
	ProviderForgejo ProviderType = "forgejo"
	ProviderGitLab  ProviderType = "gitlab"
	ProviderGitHub  ProviderType = "github"
	ProviderGitea   ProviderType = "gitea"
)

type CommitInfo struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Branch  string `json:"branch"`
	RepoURL string `json:"repo_url"`
}

type PRInfo struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	State      string `json:"state"`
	MergedAt   string `json:"merged_at,omitempty"`
	BranchName string `json:"branch_name"`
}

type WebhookEvent struct {
	Provider    ProviderType `json:"provider"`
	EventType   string       `json:"event_type"`
	RepoURL     string       `json:"repo_url"`
	Branch      string       `json:"branch"`
	Commits     []CommitInfo `json:"commits,omitempty"`
	PullRequest *PRInfo      `json:"pull_request,omitempty"`
	RawPayload  []byte       `json:"-"`
}

type GitProvider interface {
	GetName() ProviderType
	ValidateWebhook(payload []byte, secret string, signature string) (bool, error)
	ParseWebhook(payload []byte) (*WebhookEvent, error)
}

var issueKeyRegex = regexp.MustCompile(`[A-Z]+-\d+`)

func ValidateHMACSHA256(payload []byte, secret, signature string) bool {
	sig := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

func ExtractIssueKeys(text string) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, match := range issueKeyRegex.FindAllString(text, -1) {
		if !seen[match] {
			seen[match] = true
			keys = append(keys, match)
		}
	}
	return keys
}

type noopProvider struct{}

func (p *noopProvider) GetName() ProviderType                          { return "" }
func (p *noopProvider) ValidateWebhook(payload []byte, secret string, signature string) (bool, error) {
	return true, nil
}
func (p *noopProvider) ParseWebhook(payload []byte) (*WebhookEvent, error) {
	return &WebhookEvent{}, nil
}

var _ GitProvider = (*noopProvider)(nil)
