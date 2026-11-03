package notification

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/storetest"
)

// TestNotificationTexts_PinnedForTheWebUI pins the exact English title/body
// sentences the backend stores. The web UI parses these bodies to re-render
// them in the user's language (frontend-next/components/notifications/
// NotificationBell.tsx, notificationBody: "You were assigned to KEY: …",
// "Someone mentioned you in the comments of KEY: …", `Sprint "…" has started`,
// `Sprint "…" has been completed`, "… status changed to …"). A wording change
// here must fail this test — not silently leave the Italian UI in English.
func TestNotificationTexts_PinnedForTheWebUI(t *testing.T) {
	type text struct{ title, body string }
	pair := func(title, body string) text { return text{title, body} }

	cases := []struct {
		name      string
		got, want text
	}{
		{"assignment", pair(assignmentText("DEMO-1", "Fix login")),
			text{"You were assigned to an issue", "You were assigned to DEMO-1: Fix login"}},
		{"comment", pair(commentText("DEMO-1", "Fix login", "looks good")),
			text{"New comment on DEMO-1", "Fix login: looks good"}},
		{"mention", pair(mentionText("DEMO-1", "Fix login")),
			text{"You were mentioned in DEMO-1", "Someone mentioned you in the comments of DEMO-1: Fix login"}},
		{"sprint_started", pair(sprintStartedText("Sprint 1")),
			text{"Sprint started", `Sprint "Sprint 1" has started`}},
		{"sprint_completed", pair(sprintCompletedText("Sprint 1")),
			text{"Sprint completed", `Sprint "Sprint 1" has been completed`}},
		{"status_change", pair(statusChangedText("DEMO-1", "Fix login", "DONE")),
			text{"Status changed on DEMO-1", "Fix login status changed to DONE"}},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %+v, want %+v", c.name, c.got, c.want)
		}
	}
}

// TestNotifyStoresThePinnedTexts checks the Notify* methods actually store
// those sentences (the ones that need no issue/project fixtures).
func TestNotifyStoresThePinnedTexts(t *testing.T) {
	db := storetest.New(t)
	createTestUser(t, db, "u-texts-1", "texts-user-1")
	svc := NewService(db)

	if err := svc.NotifyIssueAssigned("issue-1", "u-texts-1", "DEMO-7", "Fix login"); err != nil {
		t.Fatalf("NotifyIssueAssigned: %v", err)
	}
	if err := svc.NotifyUsersMentionedByIDs([]string{"u-texts-1"}, "", "DEMO-7", "Fix login"); err != nil {
		t.Fatalf("NotifyUsersMentionedByIDs: %v", err)
	}
	notifs, err := svc.ListByUser("u-texts-1", false)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	want := map[string][2]string{
		"assignment": {"You were assigned to an issue", "You were assigned to DEMO-7: Fix login"},
		"mention":    {"You were mentioned in DEMO-7", "Someone mentioned you in the comments of DEMO-7: Fix login"},
	}
	if len(notifs) != len(want) {
		t.Fatalf("got %d notifications, want %d", len(notifs), len(want))
	}
	for _, n := range notifs {
		w, ok := want[n.Type]
		if !ok {
			t.Errorf("unexpected type %q", n.Type)
			continue
		}
		if n.Title != w[0] || n.Body != w[1] {
			t.Errorf("%s stored (%q, %q), want (%q, %q)", n.Type, n.Title, n.Body, w[0], w[1])
		}
	}
}
