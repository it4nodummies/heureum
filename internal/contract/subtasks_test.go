package contract

import (
	"net/http"
	"testing"
)

// TestCreateIssue_ResolvesTypeByName copre la regressione notata in Round 13
// Task 1: IssueHandler.Create risolveva TypeID solo da fields.issuetype.id,
// mai da .name (il caso comune: la UI e Jira reale mandano il nome). Senza
// risoluzione, buildIssueInput trovava iss.TypeID nil e v3.JiraIssue applicava
// un fallback fisso "Task" (subtask:false) — mascherando silenziosamente
// qualunque altro tipo richiesto, incluso "Subtask" per le sottotask create
// dalla sezione Subtasks del Task 2. Verifica che una issue creata con solo
// issuetype.name riceva davvero quel tipo (nome + subtask flag corretti).
func TestCreateIssue_ResolvesTypeByName(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "TYP", "Type Resolution Proj")

	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/issue", map[string]any{
		"fields": map[string]any{
			"project":   map[string]any{"key": "TYP"},
			"summary":   "A subtask created by name only",
			"issuetype": map[string]any{"name": "Subtask"},
		},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create issue: %d", resp.StatusCode)
	}
	created, _ := decodeBody(t, resp)
	key, _ := created["key"].(string)

	getResp := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/issue/"+key, nil)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get issue: %d", getResp.StatusCode)
	}
	got, _ := decodeBody(t, getResp)
	fields, _ := got["fields"].(map[string]any)
	issuetype, _ := fields["issuetype"].(map[string]any)
	if issuetype == nil {
		t.Fatalf("fields.issuetype assente nella risposta")
	}
	if name, _ := issuetype["name"].(string); name != "Subtask" {
		t.Errorf("issuetype.name = %q, atteso %q (fallback a Task = risoluzione per nome rotta)", name, "Subtask")
	}
	if subtask, _ := issuetype["subtask"].(bool); !subtask {
		t.Errorf("issuetype.subtask = %v, atteso true", subtask)
	}
}

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
