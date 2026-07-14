package v3

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

func TestJiraComment(t *testing.T) {
	c := issue.Comment{ID: "c1", IssueID: "i1", BodyJSON: `{"content":"Hello"}`, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	author := &user.User{ID: "u1", DisplayName: "Ada", IsActive: true}
	jc := JiraComment(c, author, author, "http://h")
	if jc.ID != "c1" {
		t.Errorf("id = %q", jc.ID)
	}
	if jc.Self != "http://h/rest/api/3/issue/i1/comment/c1" {
		t.Errorf("self = %q", jc.Self)
	}
	if jc.Author == nil || jc.Author.AccountID != "u1" {
		t.Errorf("author = %+v", jc.Author)
	}
	raw, _ := json.Marshal(jc.Body)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	if doc["type"] != "doc" {
		t.Errorf("body not ADF: %s", raw)
	}
	if jc.Created == "" {
		t.Error("created empty")
	}
}

func TestExtractMentions(t *testing.T) {
	body := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
		{"type":"text","text":"ciao "},
		{"type":"mention","attrs":{"id":"acc-42","text":"@Bob"}},
		{"type":"text","text":" e "},
		{"type":"mention","attrs":{"id":"acc-99"}}
	]}]}`
	ids := ExtractMentions(body)
	if len(ids) != 2 || ids[0] != "acc-42" || ids[1] != "acc-99" {
		t.Errorf("mentions = %v", ids)
	}
	if len(ExtractMentions(`{"content":"plain"}`)) != 0 {
		t.Error("expected no mentions")
	}
}
