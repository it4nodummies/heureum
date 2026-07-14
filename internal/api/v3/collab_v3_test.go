package v3

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
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

func TestJiraLinkType(t *testing.T) {
	lt := JiraLinkType("blocks", "https://example.com")

	if lt.Name != "Blocks" {
		t.Errorf("Name = %q, want Blocks", lt.Name)
	}
	if lt.Inward != "is blocked by" {
		t.Errorf("Inward = %q, want %q", lt.Inward, "is blocked by")
	}
	if lt.Outward != "blocks" {
		t.Errorf("Outward = %q, want blocks", lt.Outward)
	}
	if lt.Self != "https://example.com/rest/api/3/issueLinkType/1" {
		t.Errorf("Self = %q", lt.Self)
	}
}

func TestLinkTypeForName_RoundTrips(t *testing.T) {
	cases := map[string]string{
		"Blocks":    "blocks",
		"Duplicate": "duplicates",
		"Relates":   "relates",
	}
	for name, internal := range cases {
		if got := LinkTypeForName(name); got != internal {
			t.Errorf("LinkTypeForName(%q) = %q, want %q", name, got, internal)
		}
		lt := JiraLinkType(internal, "https://example.com")
		if lt.Name != name {
			t.Errorf("JiraLinkType(%q).Name = %q, want %q", internal, lt.Name, name)
		}
	}
}

func TestJiraRemoteLink(t *testing.T) {
	rl := issue.RemoteLink{
		ID:           "link-1",
		IssueID:      "issue-1",
		GlobalID:     "system=http://acme.com&id=1",
		URL:          "https://acme.com/ticket/1",
		Title:        "Ticket 1",
		Summary:      "Support ticket",
		Relationship: "causes",
	}

	out := JiraRemoteLink(rl, "https://example.com")

	if out.Self != "https://example.com/rest/api/3/issue/issue-1/remotelink/link-1" {
		t.Errorf("Self = %q", out.Self)
	}
	if out.GlobalID != "system=http://acme.com&id=1" {
		t.Errorf("GlobalID = %q", out.GlobalID)
	}
	if out.Relationship != "causes" {
		t.Errorf("Relationship = %q", out.Relationship)
	}
	if out.Object.URL != "https://acme.com/ticket/1" {
		t.Errorf("Object.URL = %q", out.Object.URL)
	}
	if out.Object.Title != "Ticket 1" {
		t.Errorf("Object.Title = %q", out.Object.Title)
	}
	if out.Object.Summary != "Support ticket" {
		t.Errorf("Object.Summary = %q", out.Object.Summary)
	}
}

func TestJiraRemoteLink_OmitsIDField(t *testing.T) {
	rl := issue.RemoteLink{ID: "link-1", IssueID: "issue-1", URL: "https://acme.com", Title: "T"}

	b, err := json.Marshal(JiraRemoteLink(rl, "https://example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"id"`) {
		t.Errorf("marshaled RemoteLink contains an \"id\" field, want it omitted: %s", b)
	}
}
