package v3

import (
	"testing"

	"github.com/open-jira/open-jira/internal/domain/user"
)

func TestJiraUser(t *testing.T) {
	u := user.User{ID: "u1", Email: "alice@example.com", DisplayName: "Alice",
		AvatarURL: "http://x/a.png", IsActive: true}
	ju := JiraUser(u, "http://localhost:8080")

	if ju.AccountID != "u1" || ju.DisplayName != "Alice" || !ju.Active {
		t.Errorf("unexpected: %+v", ju)
	}
	if ju.Self != "http://localhost:8080/rest/api/3/user?accountId=u1" {
		t.Errorf("self = %q", ju.Self)
	}
	if ju.AccountType != "atlassian" {
		t.Errorf("accountType = %q", ju.AccountType)
	}
	if ju.AvatarUrls["48x48"] != "http://x/a.png" {
		t.Errorf("avatarUrls = %v", ju.AvatarUrls)
	}
}

func TestJiraUser_EmptyAvatar(t *testing.T) {
	ju := JiraUser(user.User{ID: "u2", IsActive: true}, "http://h")
	// Jira serializza sempre avatarUrls con le 4 taglie.
	for _, size := range []string{"16x16", "24x24", "32x32", "48x48"} {
		if _, ok := ju.AvatarUrls[size]; !ok {
			t.Errorf("missing avatar size %s", size)
		}
	}
}
