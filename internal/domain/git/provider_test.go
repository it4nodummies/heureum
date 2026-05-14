package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestValidateHMACSHA256(t *testing.T) {
	secret := "my-secret"
	payload := []byte("test payload")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !ValidateHMACSHA256(payload, secret, expected) {
		t.Error("expected valid HMAC")
	}

	if ValidateHMACSHA256(payload, "wrong-secret", expected) {
		t.Error("expected invalid HMAC with wrong secret")
	}
}

func TestForgejoProvider_PushEvent(t *testing.T) {
	payload := githubPushPayload("refs/heads/feature/PROJ-123-fix-bug", []commitData{
		{SHA: "abc123", Message: "PROJ-123: Fix the bug", Author: "dev1"},
		{SHA: "def456", Message: "Cleanup code", Author: "dev2"},
	})

	provider := NewForgejoProvider()
	event, err := provider.ParseWebhook(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.EventType != "push" {
		t.Errorf("expected push event, got %s", event.EventType)
	}
	if event.Branch != "feature/PROJ-123-fix-bug" {
		t.Errorf("expected branch 'feature/PROJ-123-fix-bug', got %s", event.Branch)
	}
	if len(event.Commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(event.Commits))
	}
	if event.Commits[0].SHA != "abc123" {
		t.Errorf("expected SHA abc123, got %s", event.Commits[0].SHA)
	}

	keys := ExtractIssueKeys(event.Branch)
	if len(keys) != 1 || keys[0] != "PROJ-123" {
		t.Errorf("expected branch keys [PROJ-123], got %v", keys)
	}

	keys = ExtractIssueKeys(event.Commits[0].Message)
	if len(keys) != 1 || keys[0] != "PROJ-123" {
		t.Errorf("expected commit keys [PROJ-123], got %v", keys)
	}

	keys = ExtractIssueKeys(event.Commits[1].Message)
	if len(keys) != 0 {
		t.Errorf("expected no keys from cleanup commit, got %v", keys)
	}
}

func TestForgejoProvider_PullRequestEvent(t *testing.T) {
	payload := githubPRPayload("closed", "PROJ-456", true, "feature/PROJ-456-add-feature")

	provider := NewForgejoProvider()
	event, err := provider.ParseWebhook(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.EventType != "pull_request" {
		t.Errorf("expected pull_request event, got %s", event.EventType)
	}
	if event.PullRequest == nil {
		t.Fatal("expected pull request info")
	}
	if event.PullRequest.State != "merged" {
		t.Errorf("expected state 'merged', got %s", event.PullRequest.State)
	}
	if event.PullRequest.Title != "PROJ-456" {
		t.Errorf("expected title 'PROJ-456', got %s", event.PullRequest.Title)
	}
	if event.PullRequest.Number != 42 {
		t.Errorf("expected PR number 42, got %d", event.PullRequest.Number)
	}
	if event.PullRequest.BranchName != "feature/PROJ-456-add-feature" {
		t.Errorf("expected branch 'feature/PROJ-456-add-feature', got %s", event.PullRequest.BranchName)
	}
}

func TestGitHubProvider_PushEvent(t *testing.T) {
	payload := githubPushPayload("refs/heads/main", []commitData{
		{SHA: "ghi789", Message: "TASK-001: Initial commit", Author: "user1"},
	})

	provider := NewGitHubProvider()
	event, err := provider.ParseWebhook(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.EventType != "push" {
		t.Errorf("expected push event, got %s", event.EventType)
	}
	if len(event.Commits) == 0 || event.Commits[0].SHA != "ghi789" {
		t.Errorf("unexpected commit data")
	}
}

func TestGitHubProvider_PullRequestMerged(t *testing.T) {
	payload := githubPRPayload("closed", "Merge feature", true, "feature/new-thing")

	provider := NewGitHubProvider()
	event, err := provider.ParseWebhook(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.PullRequest.State != "merged" {
		t.Errorf("expected merged state, got %s", event.PullRequest.State)
	}
}

func TestGitHubProvider_ValidateWebhook(t *testing.T) {
	secret := "test-secret"
	payload := []byte("hello")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	provider := NewGitHubProvider()
	valid, err := provider.ValidateWebhook(payload, secret, sig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Error("expected valid signature")
	}

	valid, err = provider.ValidateWebhook(payload, "wrong", sig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Error("expected invalid signature with wrong secret")
	}
}

func TestGitLabProvider_TokenValidation(t *testing.T) {
	provider := NewGitLabProvider()
	valid, err := provider.ValidateWebhook(nil, "my-token", "my-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Error("expected valid token match")
	}

	valid, err = provider.ValidateWebhook(nil, "my-token", "wrong-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Error("expected invalid token mismatch")
	}
}

func TestGitLabProvider_PushEvent(t *testing.T) {
	payload, err := json.Marshal(map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/FEAT-789-new-ui",
		"repository":  map[string]interface{}{"homepage": "https://gitlab.com/test/repo"},
		"commits": []map[string]interface{}{
			{
				"id":      "jkl012",
				"message": "FEAT-789: New UI component",
				"author":  map[string]interface{}{"name": "dev3"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	provider := NewGitLabProvider()
	event, err := provider.ParseWebhook(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.EventType != "push" {
		t.Errorf("expected push event, got %s", event.EventType)
	}
	if event.Branch != "FEAT-789-new-ui" {
		t.Errorf("expected branch 'FEAT-789-new-ui', got %s", event.Branch)
	}
}

func TestGitLabProvider_MergeRequest(t *testing.T) {
	mergedAt := "2024-01-15T10:00:00Z"
	payload, err := json.Marshal(map[string]interface{}{
		"object_kind": "merge_request",
		"event_type":  "merge_request",
		"repository":  map[string]interface{}{"homepage": "https://gitlab.com/test/repo"},
		"object_attributes": map[string]interface{}{
			"iid":            float64(10),
			"title":          "FEAT-789: New feature",
			"url":            "https://gitlab.com/test/repo/-/merge_requests/10",
			"state":          "merged",
			"action":         "merge",
			"merged_at":      mergedAt,
			"source_branch":  "FEAT-789-new-ui",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	provider := NewGitLabProvider()
	event, err := provider.ParseWebhook(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.EventType != "pull_request" {
		t.Errorf("expected pull_request event, got %s", event.EventType)
	}
	if event.PullRequest.State != "merged" {
		t.Errorf("expected merged state, got %s", event.PullRequest.State)
	}
	if event.PullRequest.Number != 10 {
		t.Errorf("expected PR number 10, got %d", event.PullRequest.Number)
	}
	if event.PullRequest.BranchName != "FEAT-789-new-ui" {
		t.Errorf("expected branch 'FEAT-789-new-ui', got %s", event.PullRequest.BranchName)
	}
}

func TestExtractIssueKeys(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"feature/PROJ-123-fix-bug", []string{"PROJ-123"}},
		{"PROJ-123: Fix bug\nPROJ-456: Another fix", []string{"PROJ-123", "PROJ-456"}},
		{"No issue key here", nil},
		{"ABC-1 XYZ-2", []string{"ABC-1", "XYZ-2"}},
		{"", nil},
		{"feature/PROJ-123-PROJ-456", []string{"PROJ-123", "PROJ-456"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			keys := ExtractIssueKeys(tt.input)
			if len(keys) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, keys)
				return
			}
			for i, k := range keys {
				if k != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, keys)
					return
				}
			}
		})
	}
}

func TestProviderForType(t *testing.T) {
	forgejo := ProviderForType(ProviderForgejo)
	if forgejo.GetName() != ProviderForgejo {
		t.Error("expected forgejo provider")
	}

	gitlab := ProviderForType(ProviderGitLab)
	if gitlab.GetName() != ProviderGitLab {
		t.Error("expected gitlab provider")
	}

	github := ProviderForType(ProviderGitHub)
	if github.GetName() != ProviderGitHub {
		t.Error("expected github provider")
	}

	gitea := ProviderForType(ProviderGitea)
	if gitea.GetName() != ProviderForgejo {
		t.Error("expected forgejo name for gitea type")
	}
}

type commitData struct {
	SHA     string
	Message string
	Author  string
}

func githubPushPayload(ref string, commits []commitData) []byte {
	commitsJSON := make([]map[string]interface{}, len(commits))
	for i, c := range commits {
		commitsJSON[i] = map[string]interface{}{
			"id":      c.SHA,
			"message": c.Message,
			"author": map[string]interface{}{
				"username": c.Author,
				"name":     c.Author,
			},
		}
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"ref": ref,
		"repository": map[string]interface{}{
			"html_url": "https://forgejo.example.com/test/repo",
		},
		"commits": commitsJSON,
	})
	return payload
}

func githubPRPayload(action, title string, merged bool, branchRef string) []byte {
	mergedAt := "2024-01-15T10:00:00Z"
	if !merged {
		mergedAt = ""
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"action": action,
		"repository": map[string]interface{}{
			"html_url": "https://forgejo.example.com/test/repo",
		},
		"pull_request": map[string]interface{}{
			"number":    float64(42),
			"title":     title,
			"html_url":  "https://forgejo.example.com/test/repo/pulls/42",
			"state":     action,
			"merged_at": mergedAt,
			"head": map[string]interface{}{
				"ref": branchRef,
			},
		},
	})
	return payload
}
