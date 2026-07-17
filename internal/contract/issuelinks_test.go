package contract

import (
	"net/http"
	"testing"
)

// TestListIssueLinks_ForIssue copre GET /rest/api/3/issue/{issueIdOrKey}/issuelinks
// (Round 13 Task 3): crea due issue A/B in progetto "LNK", collega A (outward)
// -> B (inward) con type "Blocks", poi verifica che GET /issue/A/issuelinks
// restituisca un link con type.name == "Blocks" e la issue collegata (B)
// esposta come inwardIssue (A è la sorgente/outward del link creato).
func TestListIssueLinks_ForIssue(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "LNK", "Link Proj")
	a := createIssueViaAPI(t, srv, tok, "LNK", "Issue A")
	b := createIssueViaAPI(t, srv, tok, "LNK", "Issue B")

	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/issueLink", map[string]any{
		"type":         map[string]any{"name": "Blocks"},
		"outwardIssue": map[string]any{"key": a},
		"inwardIssue":  map[string]any{"key": b},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create issueLink: %d", resp.StatusCode)
	}

	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/issue/"+a+"/issuelinks", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list issuelinks: %d", resp.StatusCode)
	}
	body, _ := decodeBody(t, resp)
	links, _ := body["issuelinks"].([]any)
	if len(links) != 1 {
		t.Fatalf("atteso 1 link, ottenuti %d", len(links))
	}
	link, _ := links[0].(map[string]any)
	typ, _ := link["type"].(map[string]any)
	if name, _ := typ["name"].(string); name != "Blocks" {
		t.Errorf("type.name = %q, atteso %q", name, "Blocks")
	}
	inward, _ := link["inwardIssue"].(map[string]any)
	if inward == nil {
		t.Fatalf("inwardIssue assente: A è outward, l'altro capo (B) deve comparire come inwardIssue")
	}
	if key, _ := inward["key"].(string); key != b {
		t.Errorf("inwardIssue.key = %q, atteso %q", key, b)
	}
	if _, present := link["outwardIssue"]; present && link["outwardIssue"] != nil {
		t.Errorf("outwardIssue presente e non nil = %v, atteso omesso (A è già il soggetto della lista)", link["outwardIssue"])
	}
}
