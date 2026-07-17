package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/board"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

// setupBoardConfigTest crea un DB in-memory con board + workflow (3 stati) e
// restituisce l'handler agile, la board e gli status id ordinati per position.
func setupBoardConfigTest(t *testing.T) (*AgileBoardHandler, *board.Board, []string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&board.Board{}, &board.BoardColumn{}, &board.BoardColumnStatus{}, &board.BoardQuickFilter{},
		&workflow.Workflow{}, &workflow.WorkflowStatus{}, &workflow.WorkflowTransition{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	boardSvc := board.NewService(db)
	wfSvc := workflow.NewService(db)

	projectID := uuid.NewString()
	if _, err := wfSvc.CreateDefaultWorkflow(projectID); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	wf, err := wfSvc.GetWorkflow(projectID)
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	statusIDs := make([]string, 0, len(wf.Statuses))
	for _, st := range wf.Statuses {
		statusIDs = append(statusIDs, st.ID)
	}
	if len(statusIDs) < 3 {
		t.Fatalf("expected >=3 statuses, got %d", len(statusIDs))
	}

	b, err := boardSvc.Create(projectID, "Board", "scrum", nil)
	if err != nil {
		t.Fatalf("create board: %v", err)
	}

	h := NewAgileBoardHandler(boardSvc, nil, nil, nil, wfSvc, nil, nil, "http://localhost:8080")
	return h, b, statusIDs
}

// TestBoardConfig_CustomAndAgile verifica il flusso completo Task 4: PUT del
// config custom (colonna multi-stato + swimlane + quick filter), GET custom che
// lo riflette, e GET /configuration agile che espone la colonna multi-stato.
func TestBoardConfig_CustomAndAgile(t *testing.T) {
	h, b, sids := setupBoardConfigTest(t)
	s1, s2, s3 := sids[0], sids[1], sids[2]

	// --- PUT custom config: merge s1+s2 into one column ---
	putBody := map[string]any{
		"columns": []map[string]any{
			{"name": "To Do & Doing", "statusIds": []string{s1, s2}},
			{"name": "Done", "statusIds": []string{s3}},
		},
		"swimlane": "assignee",
		"quickFilters": []map[string]any{
			{"name": "Mine", "jql": "assignee = currentUser()"},
		},
	}
	pb, _ := json.Marshal(putBody)
	preq := httptest.NewRequest(http.MethodPut, "/rest/agile/1.0/board/1/config", bytes.NewReader(pb))
	preq.SetPathValue("boardId", "1")
	preq.Header.Set("Content-Type", "application/json")
	pw := httptest.NewRecorder()
	h.SaveCustomConfig(pw, preq)
	if pw.Code != http.StatusOK && pw.Code != http.StatusNoContent {
		t.Fatalf("PUT config status = %d, want 200/204: %s", pw.Code, pw.Body.String())
	}

	// --- GET custom config reflects it ---
	greq := httptest.NewRequest(http.MethodGet, "/rest/agile/1.0/board/1/config", nil)
	greq.SetPathValue("boardId", "1")
	gw := httptest.NewRecorder()
	h.GetCustomConfig(gw, greq)
	if gw.Code != http.StatusOK {
		t.Fatalf("GET config status = %d, want 200: %s", gw.Code, gw.Body.String())
	}
	var cfg struct {
		Columns []struct {
			Name      string   `json:"name"`
			StatusIDs []string `json:"statusIds"`
		} `json:"columns"`
		Swimlane     string `json:"swimlane"`
		QuickFilters []struct {
			Name string `json:"name"`
			JQL  string `json:"jql"`
		} `json:"quickFilters"`
	}
	if err := json.NewDecoder(gw.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode custom config: %v", err)
	}
	if len(cfg.Columns) != 2 {
		t.Fatalf("custom columns = %d, want 2: %+v", len(cfg.Columns), cfg)
	}
	if got := sortedCopy(cfg.Columns[0].StatusIDs); !equalStrings(got, sortedCopy([]string{s1, s2})) {
		t.Errorf("column[0] statusIds = %v, want set {%s,%s}", cfg.Columns[0].StatusIDs, s1, s2)
	}
	if cfg.Columns[0].Name != "To Do & Doing" {
		t.Errorf("column[0] name = %q, want %q", cfg.Columns[0].Name, "To Do & Doing")
	}
	if cfg.Swimlane != "assignee" {
		t.Errorf("swimlane = %q, want assignee", cfg.Swimlane)
	}
	if len(cfg.QuickFilters) != 1 || cfg.QuickFilters[0].Name != "Mine" {
		t.Errorf("quickFilters = %+v, want one {Mine}", cfg.QuickFilters)
	}

	// --- GET agile /configuration exposes the merged multi-status column ---
	creq := httptest.NewRequest(http.MethodGet, "/rest/agile/1.0/board/1/configuration", nil)
	creq.SetPathValue("boardId", "1")
	cw := httptest.NewRecorder()
	h.Configuration(cw, creq)
	if cw.Code != http.StatusOK {
		t.Fatalf("GET configuration status = %d, want 200: %s", cw.Code, cw.Body.String())
	}
	var ac struct {
		ColumnConfig struct {
			Columns []struct {
				Name     string `json:"name"`
				Statuses []struct {
					ID string `json:"id"`
				} `json:"statuses"`
			} `json:"columns"`
		} `json:"columnConfig"`
	}
	if err := json.NewDecoder(cw.Body).Decode(&ac); err != nil {
		t.Fatalf("decode agile config: %v", err)
	}
	if len(ac.ColumnConfig.Columns) != 2 {
		t.Fatalf("agile columns = %d, want 2: %+v", len(ac.ColumnConfig.Columns), ac)
	}
	merged := ac.ColumnConfig.Columns[0]
	if merged.Name != "To Do & Doing" {
		t.Errorf("agile column[0] name = %q, want %q", merged.Name, "To Do & Doing")
	}
	if len(merged.Statuses) != 2 {
		t.Errorf("agile column[0] statuses = %d, want 2 (multi-status): %+v", len(merged.Statuses), merged.Statuses)
	}
	got := []string{merged.Statuses[0].ID, merged.Statuses[1].ID}
	if !equalStrings(sortedCopy(got), sortedCopy([]string{s1, s2})) {
		t.Errorf("agile column[0] statuses = %v, want set {%s,%s}", got, s1, s2)
	}

	_ = b
}

// TestBoardConfig_FallbackWhenUnconfigured verifica che una board senza config
// persistita restituisca le colonne 1:1 dal workflow (swimlane none, no filtri).
func TestBoardConfig_FallbackWhenUnconfigured(t *testing.T) {
	h, _, sids := setupBoardConfigTest(t)

	greq := httptest.NewRequest(http.MethodGet, "/rest/agile/1.0/board/1/config", nil)
	greq.SetPathValue("boardId", "1")
	gw := httptest.NewRecorder()
	h.GetCustomConfig(gw, greq)
	if gw.Code != http.StatusOK {
		t.Fatalf("GET config status = %d, want 200: %s", gw.Code, gw.Body.String())
	}
	var cfg struct {
		Columns []struct {
			StatusIDs []string `json:"statusIds"`
		} `json:"columns"`
		Swimlane     string `json:"swimlane"`
		QuickFilters []any  `json:"quickFilters"`
	}
	json.NewDecoder(gw.Body).Decode(&cfg)
	if len(cfg.Columns) != len(sids) {
		t.Errorf("fallback columns = %d, want %d (1:1)", len(cfg.Columns), len(sids))
	}
	if cfg.Swimlane != "none" {
		t.Errorf("fallback swimlane = %q, want none", cfg.Swimlane)
	}
	if len(cfg.QuickFilters) != 0 {
		t.Errorf("fallback quickFilters = %d, want 0", len(cfg.QuickFilters))
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
