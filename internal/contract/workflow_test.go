package contract

import (
	"net/http"
	"strings"
	"testing"
)

// TestStatusCategory_Conformant valida GET /statuscategory (elenco) e
// GET /statuscategory/{idOrKey} (per key "done") contro lo schema ufficiale.
func TestStatusCategory_Conformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	v := MustLoad(t, specPath)

	// GET /statuscategory restituisce un array JSON top-level: passiamo
	// direttamente res.Body a ValidateResponse (che lo legge e lo
	// ribufferizza internamente), come in TestPriorities_ConformsToContract.
	res := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/statuscategory", nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /statuscategory status = %d, want 200", res.StatusCode)
	}
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/statuscategory", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /statuscategory non conforme: %v", err)
	}

	res2 := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/statuscategory/done", nil)
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("GET /statuscategory/done status = %d, want 200", res2.StatusCode)
	}
	body2, raw2 := decodeBody(t, res2)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/statuscategory/{idOrKey}", res2.StatusCode, res2.Header, strings.NewReader(string(raw2))); err != nil {
		t.Errorf("GET /statuscategory/{idOrKey} non conforme: %v", err)
	}
	if body2["key"] != "done" {
		t.Errorf("statuscategory/done key = %v, want done", body2["key"])
	}
}

// TestIssueTransitions_Conformant esercita il ciclo GET disponibili ->
// POST transizione -> GET di nuovo, validando ogni risposta contro lo
// schema ufficiale e asserendo il cambio di stato reale dell'issue.
func TestIssueTransitions_Conformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "WF", "Workflow Proj")
	// createProjectViaAPI crea via HTTP il progetto e il suo default
	// workflow (TO DO/IN PROGRESS/DONE + transizioni); una issue appena
	// creata riceve lo status "TO DO" (categoria todo), che ha una
	// transizione uscente ("Start Progress" -> IN PROGRESS): le transizioni
	// disponibili non sono quindi vuote fin da subito.
	key := createIssueViaAPI(t, srv, tok, "WF", "Transition me")
	v := MustLoad(t, specPath)

	// --- GET transizioni disponibili dallo stato iniziale ---
	res := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/issue/"+key+"/transitions", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET transitions status = %d, want 200", res.StatusCode)
	}
	body, raw := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/issue/{issueIdOrKey}/transitions", res.StatusCode, res.Header, strings.NewReader(string(raw))); err != nil {
		t.Errorf("GET transitions non conforme: %v", err)
	}
	trs, _ := body["transitions"].([]any)
	if len(trs) == 0 {
		t.Fatal("attese transizioni disponibili dallo stato iniziale (TO DO)")
	}
	first, ok := trs[0].(map[string]any)
	if !ok {
		t.Fatalf("transizione non è un oggetto: %#v", trs[0])
	}
	trID, _ := first["id"].(string)
	if trID == "" {
		t.Fatal("transizione senza id")
	}
	if _, ok := first["to"].(map[string]any); !ok {
		t.Errorf("transizione senza campo to: %#v", first)
	}

	// --- POST esegue la transizione (shape Jira {transition:{id}}) ---
	res2 := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/issue/"+key+"/transitions", map[string]any{
		"transition": map[string]any{"id": trID},
	})
	res2.Body.Close()
	if res2.StatusCode != http.StatusNoContent {
		t.Fatalf("POST transitions status = %d, want 204", res2.StatusCode)
	}

	// --- GET di nuovo: lo stato è cambiato, quindi anche le transizioni
	// disponibili devono conformarsi (nuovo insieme, dallo stato di
	// destinazione IN PROGRESS: "Stop Progress" e "Done").
	res3 := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/issue/"+key+"/transitions", nil)
	if res3.StatusCode != http.StatusOK {
		t.Fatalf("GET transitions #2 status = %d, want 200", res3.StatusCode)
	}
	body3, raw3 := decodeBody(t, res3)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/issue/{issueIdOrKey}/transitions", res3.StatusCode, res3.Header, strings.NewReader(string(raw3))); err != nil {
		t.Errorf("GET transitions #2 non conforme: %v", err)
	}
	trs3, _ := body3["transitions"].([]any)
	for _, raw := range trs3 {
		tr, _ := raw.(map[string]any)
		if tr["id"] == trID {
			t.Errorf("transizione %q eseguita non dovrebbe più essere disponibile dal nuovo stato", trID)
		}
	}

	// --- conferma via GET issue che lo status sia effettivamente cambiato ---
	res4 := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/issue/"+key, nil)
	issueBody, _ := decodeBody(t, res4)
	fields, _ := issueBody["fields"].(map[string]any)
	status, _ := fields["status"].(map[string]any)
	if status["name"] != "IN PROGRESS" {
		t.Errorf("issue status dopo la transizione = %v, want IN PROGRESS", status["name"])
	}
}
