package v3

import (
	"encoding/json"
	"testing"

	"github.com/open-jira/open-jira/internal/domain/user"
)

func TestJiraVotes(t *testing.T) {
	u := user.User{ID: "u1", Email: "u1@example.com", DisplayName: "User One", IsActive: true}

	vs := JiraVotes("DEMO-1", "https://example.com", 3, true, []user.User{u})

	if vs.Self != "https://example.com/rest/api/3/issue/DEMO-1/votes" {
		t.Errorf("Self = %q", vs.Self)
	}
	if vs.Votes != 3 {
		t.Errorf("Votes = %d, want 3", vs.Votes)
	}
	if !vs.HasVoted {
		t.Error("HasVoted = false, want true")
	}
	if len(vs.Voters) != 1 || vs.Voters[0].AccountID != "u1" {
		t.Errorf("Voters = %+v", vs.Voters)
	}
}

func TestJiraVotes_EmptyVotersIsNotNull(t *testing.T) {
	vs := JiraVotes("DEMO-1", "https://example.com", 0, false, nil)

	b, err := json.Marshal(vs.Voters)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "[]" {
		t.Errorf("marshaled Voters = %s, want []", b)
	}
}

func TestJiraWatchers(t *testing.T) {
	u := user.User{ID: "u1", Email: "u1@example.com", DisplayName: "User One", IsActive: true}

	ws := JiraWatchers("DEMO-1", "https://example.com", true, []user.User{u})

	if ws.Self != "https://example.com/rest/api/3/issue/DEMO-1/watchers" {
		t.Errorf("Self = %q", ws.Self)
	}
	if !ws.IsWatching {
		t.Error("IsWatching = false, want true")
	}
	if ws.WatchCount != 1 {
		t.Errorf("WatchCount = %d, want 1", ws.WatchCount)
	}
	if len(ws.Watchers) != 1 || ws.Watchers[0].AccountID != "u1" {
		t.Errorf("Watchers = %+v", ws.Watchers)
	}
}

func TestJiraWatchers_EmptyWatchersIsNotNull(t *testing.T) {
	ws := JiraWatchers("DEMO-1", "https://example.com", false, nil)

	b, err := json.Marshal(ws.Watchers)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "[]" {
		t.Errorf("marshaled Watchers = %s, want []", b)
	}
}
