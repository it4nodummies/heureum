package v3

import (
	"testing"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

func TestJiraWorklog(t *testing.T) {
	started := time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC)
	wl := issue.Worklog{
		ID:               "wl-1",
		IssueID:          "issue-1",
		CommentJSON:      `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"worked"}]}]}`,
		TimeSpentSeconds: 3600,
		Started:          &started,
		CreatedAt:        started,
		UpdatedAt:        started,
	}
	author := &user.User{ID: "user-1", DisplayName: "Alice", Email: "alice@example.com"}

	got := JiraWorklog(wl, author, "http://localhost:8080")

	if got.TimeSpent != "1h" {
		t.Errorf("TimeSpent = %q, want 1h", got.TimeSpent)
	}
	if got.TimeSpentSeconds != 3600 {
		t.Errorf("TimeSpentSeconds = %d, want 3600", got.TimeSpentSeconds)
	}
	if got.Author == nil || got.Author.AccountID != "user-1" {
		t.Errorf("Author = %+v, want AccountID user-1", got.Author)
	}
	if got.UpdateAuthor == nil || got.UpdateAuthor.AccountID != "user-1" {
		t.Errorf("UpdateAuthor = %+v, want AccountID user-1", got.UpdateAuthor)
	}
	if got.Self == "" {
		t.Error("expected non-empty Self")
	}
	if got.Started == "" {
		t.Error("expected non-empty Started")
	}
	if got.Comment == nil {
		t.Error("expected non-nil Comment")
	}
	if got.ID != "wl-1" || got.IssueID != "issue-1" {
		t.Errorf("ID/IssueID = %q/%q", got.ID, got.IssueID)
	}
}

func TestFormatSeconds(t *testing.T) {
	cases := []struct {
		sec  int
		want string
	}{
		{0, "0m"},
		{60, "1m"},
		{3600, "1h"},
		{3660, "1h 1m"},
		{7200, "2h"},
	}
	for _, c := range cases {
		if got := formatSeconds(c.sec); got != c.want {
			t.Errorf("formatSeconds(%d) = %q, want %q", c.sec, got, c.want)
		}
	}
}
