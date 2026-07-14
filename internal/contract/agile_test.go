package contract

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const agileSpecPath = "../../docs/contracts/jira-agile-1.0.json"

// agilePath costruisce un path /rest/agile/1.0/... interpolando un id
// numerico tra un prefisso e un suffisso (es. itoaPath nel piano).
func agilePath(prefix string, id int64, suffix string) string {
	return fmt.Sprintf("%s%d%s", prefix, id, suffix)
}

// TestAgileBoardAndSprint esercita l'intero ciclo di vita agile end-to-end:
// board -> backlog -> sprint -> spostamento issue -> rank -> start/complete
// sprint, validando status HTTP, campi chiave del body e conformità OpenAPI
// (spec jira-agile-1.0) dove lo spec definisce uno schema esplicito.
func TestAgileBoardAndSprint(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "AG", "Agile Proj")
	issueKey := createIssueViaAPI(t, srv, jwt, "AG", "Backlog item")

	v := MustLoad(t, agileSpecPath)

	// --- create board ---
	res := doJSON(t, srv, http.MethodPost, jwt, "/rest/agile/1.0/board", map[string]any{
		"name": "AG Board", "type": "scrum", "projectKeyOrId": "AG",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST /board status = %d, want 201", res.StatusCode)
	}
	board, rawBoard := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodPost, "/rest/agile/1.0/board", res.StatusCode, res.Header, strings.NewReader(string(rawBoard))); err != nil {
		t.Errorf("POST /board non conforme: %v", err)
	}
	boardID, ok := board["id"].(float64)
	if !ok || boardID == 0 {
		t.Fatal("board id mancante")
	}
	if loc, ok := board["location"].(map[string]any); !ok || loc["projectKey"] != "AG" {
		t.Errorf("board.location.projectKey = %v, want AG", board["location"])
	}

	// --- list boards ---
	res = doJSON(t, srv, http.MethodGet, jwt, "/rest/agile/1.0/board", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /board status = %d, want 200", res.StatusCode)
	}
	_, rawList := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/agile/1.0/board", res.StatusCode, res.Header, strings.NewReader(string(rawList))); err != nil {
		t.Errorf("GET /board non conforme: %v", err)
	}

	// --- board backlog (deve contenere l'unica issue creata) ---
	res = doJSON(t, srv, http.MethodGet, jwt, agilePath("/rest/agile/1.0/board/", int64(boardID), "/backlog"), nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /board/{id}/backlog status = %d, want 200", res.StatusCode)
	}
	backlog, rawBacklog := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/agile/1.0/board/{boardId}/backlog", res.StatusCode, res.Header, strings.NewReader(string(rawBacklog))); err != nil {
		t.Errorf("GET /board/{id}/backlog non conforme: %v", err)
	}
	issues, _ := backlog["issues"].([]any)
	if len(issues) != 1 {
		t.Errorf("backlog deve contenere 1 issue, got %d", len(issues))
	}

	// --- board configuration ---
	res = doJSON(t, srv, http.MethodGet, jwt, agilePath("/rest/agile/1.0/board/", int64(boardID), "/configuration"), nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /board/{id}/configuration status = %d, want 200", res.StatusCode)
	}
	_, rawCfg := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/agile/1.0/board/{boardId}/configuration", res.StatusCode, res.Header, strings.NewReader(string(rawCfg))); err != nil {
		t.Errorf("GET /board/{id}/configuration non conforme: %v", err)
	}

	// --- create sprint on the board ---
	res = doJSON(t, srv, http.MethodPost, jwt, "/rest/agile/1.0/sprint", map[string]any{
		"name": "Sprint 1", "originBoardId": int64(boardID), "goal": "ship",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST /sprint status = %d, want 201", res.StatusCode)
	}
	sp, rawSprint := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodPost, "/rest/agile/1.0/sprint", res.StatusCode, res.Header, strings.NewReader(string(rawSprint))); err != nil {
		t.Errorf("POST /sprint non conforme: %v", err)
	}
	sprintID, ok := sp["id"].(float64)
	if !ok || sprintID == 0 {
		t.Fatal("sprint id mancante")
	}
	if sp["state"] != "future" && sp["state"] != "" {
		t.Errorf("sprint appena creato deve essere future, got %v", sp["state"])
	}

	// --- move issue to sprint ---
	res = doJSON(t, srv, http.MethodPost, jwt, agilePath("/rest/agile/1.0/sprint/", int64(sprintID), "/issue"), map[string]any{
		"issues": []string{issueKey},
	})
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /sprint/{id}/issue status = %d, want 204", res.StatusCode)
	}
	res.Body.Close()

	// --- sprint issues now contains it ---
	res = doJSON(t, srv, http.MethodGet, jwt, agilePath("/rest/agile/1.0/sprint/", int64(sprintID), "/issue"), nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /sprint/{id}/issue status = %d, want 200", res.StatusCode)
	}
	sissues, rawSIssues := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/agile/1.0/sprint/{sprintId}/issue", res.StatusCode, res.Header, strings.NewReader(string(rawSIssues))); err != nil {
		t.Errorf("GET /sprint/{id}/issue non conforme: %v", err)
	}
	arr, _ := sissues["issues"].([]any)
	if len(arr) != 1 {
		t.Errorf("sprint deve contenere 1 issue, got %d", len(arr))
	}

	// --- move back to backlog ---
	res = doJSON(t, srv, http.MethodPost, jwt, "/rest/agile/1.0/backlog/issue", map[string]any{
		"issues": []string{issueKey},
	})
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /backlog/issue status = %d, want 204", res.StatusCode)
	}
	res.Body.Close()

	// --- rank ---
	res = doJSON(t, srv, http.MethodPut, jwt, "/rest/agile/1.0/issue/rank", map[string]any{
		"issues": []string{issueKey},
	})
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT /issue/rank status = %d, want 204", res.StatusCode)
	}
	res.Body.Close()

	// --- start sprint ---
	res = doJSON(t, srv, http.MethodPost, jwt, agilePath("/rest/agile/1.0/sprint/", int64(sprintID), ""), map[string]any{"state": "active"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /sprint/{id} (active) status = %d, want 200", res.StatusCode)
	}
	started, rawStarted := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodPost, "/rest/agile/1.0/sprint/{sprintId}", res.StatusCode, res.Header, strings.NewReader(string(rawStarted))); err != nil {
		t.Errorf("POST /sprint/{id} (active) non conforme: %v", err)
	}
	if started["state"] != "active" {
		t.Errorf("sprint deve essere active, got %v", started["state"])
	}

	// --- complete sprint ---
	res = doJSON(t, srv, http.MethodPost, jwt, agilePath("/rest/agile/1.0/sprint/", int64(sprintID), ""), map[string]any{"state": "closed"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /sprint/{id} (closed) status = %d, want 200", res.StatusCode)
	}
	closed, rawClosed := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodPost, "/rest/agile/1.0/sprint/{sprintId}", res.StatusCode, res.Header, strings.NewReader(string(rawClosed))); err != nil {
		t.Errorf("POST /sprint/{id} (closed) non conforme: %v", err)
	}
	if closed["state"] != "closed" {
		t.Errorf("sprint deve essere closed, got %v", closed["state"])
	}
}
