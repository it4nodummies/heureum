package automation

import (
	"encoding/json"
	"testing"

	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/adf"
	"github.com/it4nodummies/heureum/internal/storetest"
)

func seedAutomationIssue(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, stmt := range []string{
		"INSERT INTO projects (id, name, key) VALUES ('p1', 'P1', 'P1')",
		"INSERT INTO issues (id, project_id, key, title, seq_id) VALUES ('i1', 'p1', 'P1-1', 'one', 90001)",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

// TestAutomation_AddCommentStoresValidADF: the add_comment action used to
// format the value into a JSON literal by hand, so a quote or a newline in
// the text produced an unparsable comments.body_json. The body must be a
// valid ADF document whose text round-trips.
func TestAutomation_AddCommentStoresValidADF(t *testing.T) {
	db := storetest.New(t)
	seedAutomationIssue(t, db)
	svc := NewService(db)

	const value = "She said \"ship it\"\nthen left \\ early"
	actions, _ := json.Marshal([]map[string]string{{"type": "add_comment", "value": value}})
	rule, err := svc.CreateRule("p1", "comment on create", string(TriggerIssueCreated), "{}", string(actions))
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	run, err := svc.ExecuteRule(rule.ID, "i1")
	if err != nil {
		t.Fatalf("ExecuteRule: %v", err)
	}
	if run.Status != "success" {
		t.Fatalf("run status = %q (%s), want success", run.Status, run.Log)
	}

	var body string
	if err := db.Raw("SELECT body_json FROM comments WHERE issue_id = 'i1'").Scan(&body).Error; err != nil {
		t.Fatalf("read comment: %v", err)
	}
	if body == "" {
		t.Fatal("no comment row was written")
	}
	if !json.Valid([]byte(body)) {
		t.Fatalf("comments.body_json is not valid JSON: %s", body)
	}
	var doc adf.Node
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("body_json is not an ADF document: %v (%s)", err, body)
	}
	if doc.Type != "doc" {
		t.Errorf("body_json type = %q, want \"doc\"", doc.Type)
	}
	if got := adf.PlainText(doc); got != value {
		t.Errorf("comment text round-trip: got %q, want %q", got, value)
	}
}
