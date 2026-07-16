package contract

import (
	"net/http"
	"testing"
)

// TestSubtasks_ListChildren copre GET /rest/api/3/issue/{issueIdOrKey}/subtasks:
// crea un'issue padre e una figlia (fields.parent.key), poi verifica che la
// lista sottotask del padre contenga esattamente la figlia.
func TestSubtasks_ListChildren(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "SUB", "Subtask Proj")
	parent := createIssueViaAPI(t, srv, tok, "SUB", "Parent story")
	// crea un figlio con fields.parent.key = parent
	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/issue", map[string]any{
		"fields": map[string]any{
			"project":   map[string]any{"key": "SUB"},
			"summary":   "Child subtask",
			"issuetype": map[string]any{"name": "Subtask"},
			"parent":    map[string]any{"key": parent},
		},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create subtask: %d", resp.StatusCode)
	}
	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/issue/"+parent+"/subtasks", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list subtasks: %d", resp.StatusCode)
	}
	body, _ := decodeBody(t, resp)
	vals, _ := body["values"].([]any)
	if len(vals) != 1 {
		t.Fatalf("atteso 1 subtask, ottenuti %d", len(vals))
	}
}
