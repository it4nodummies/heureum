package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestCreateIssue_UnknownParentIs400: before Round 28 an unknown
// fields.parent.key was silently dropped (201, issue created without a
// parent). Now it is a client error: 400 errors.parent and nothing written.
func TestCreateIssue_UnknownParentIs400(t *testing.T) {
	f := newIssueCreateFixture(t)
	for name, parent := range map[string]any{
		"unknown key": map[string]any{"key": "FUP-9999"},
		"unknown id":  map[string]any{"id": "99999999"},
		"numeric id":  map[string]any{"id": 99999999},
		"empty spec":  map[string]any{},
	} {
		w := f.post(t, map[string]any{"summary": "orphan " + name, "parent": parent})
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400 (body %s)", name, w.Code, w.Body.String())
			continue
		}
		var body struct {
			Errors map[string]string `json:"errors"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: decode: %v", name, err)
		}
		if body.Errors["parent"] != "Parent issue not found." {
			t.Errorf("%s: errors.parent = %q", name, body.Errors["parent"])
		}
	}
	if n := f.issueCount(t); n != 0 {
		t.Errorf("issues created = %d, want 0", n)
	}
}

// TestCreateIssue_ParentByNumericID: fields.parent.id (the public seq id,
// string or number) resolves like fields.parent.key.
func TestCreateIssue_ParentByNumericID(t *testing.T) {
	f := newIssueCreateFixture(t)
	epicType, err := f.h.svc.TypeIDByName(f.proj.ID, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.svc.TypeIDByName(f.proj.ID, "Story"); err != nil {
		t.Fatal(err)
	}
	epic, err := f.h.svc.Create(f.proj.Key, f.proj.ID, "The epic", "", "medium", nil, &epicType)
	if err != nil {
		t.Fatal(err)
	}
	w := f.post(t, map[string]any{
		"summary":   "story under epic by id",
		"issuetype": map[string]any{"name": "Story"},
		"parent":    map[string]any{"id": epic.SeqID},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var created struct{ Key string }
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	story, err := f.h.svc.GetByKey(created.Key)
	if err != nil {
		t.Fatal(err)
	}
	if story.ParentID == nil || *story.ParentID != epic.ID {
		t.Errorf("story.ParentID = %v, want %s", story.ParentID, epic.ID)
	}
}
