package board

import (
	"sort"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newConfigDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Board{}, &BoardColumn{}, &BoardColumnStatus{}, &BoardQuickFilter{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSaveAndGetConfig(t *testing.T) {
	svc := NewService(newConfigDB(t))
	b, err := svc.Create("proj-1", "Board A", "scrum", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	in := BoardConfigInput{Swimlane: "assignee"}
	in.Columns = append(in.Columns, struct {
		Name      string   `json:"name"`
		StatusIDs []string `json:"statusIds"`
	}{Name: "To Do & Doing", StatusIDs: []string{"s1", "s2"}})
	in.Columns = append(in.Columns, struct {
		Name      string   `json:"name"`
		StatusIDs []string `json:"statusIds"`
	}{Name: "Done", StatusIDs: []string{"s3"}})
	in.QuickFilters = append(in.QuickFilters, struct {
		Name string `json:"name"`
		JQL  string `json:"jql"`
	}{Name: "Mine", JQL: "assignee = currentUser()"})

	if err := svc.SaveConfig(b.ID, in); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	cfg, err := svc.GetConfig(b.ID, nil)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}

	if cfg.Swimlane != "assignee" {
		t.Errorf("swimlane atteso 'assignee', got %q", cfg.Swimlane)
	}
	if len(cfg.Columns) != 2 {
		t.Fatalf("attese 2 colonne, got %d", len(cfg.Columns))
	}
	// order by position
	if cfg.Columns[0].Name != "To Do & Doing" || cfg.Columns[0].Position != 0 {
		t.Errorf("colonna 0 inattesa: %+v", cfg.Columns[0])
	}
	if cfg.Columns[1].Name != "Done" || cfg.Columns[1].Position != 1 {
		t.Errorf("colonna 1 inattesa: %+v", cfg.Columns[1])
	}
	if got := sortedCopy(cfg.Columns[0].StatusIDs); !equalStrs(got, []string{"s1", "s2"}) {
		t.Errorf("status set colonna 0 atteso [s1 s2], got %v", got)
	}
	if got := sortedCopy(cfg.Columns[1].StatusIDs); !equalStrs(got, []string{"s3"}) {
		t.Errorf("status set colonna 1 atteso [s3], got %v", got)
	}
	if len(cfg.QuickFilters) != 1 {
		t.Fatalf("atteso 1 quick filter, got %d", len(cfg.QuickFilters))
	}
	if cfg.QuickFilters[0].Name != "Mine" || cfg.QuickFilters[0].JQL != "assignee = currentUser()" {
		t.Errorf("quick filter inatteso: %+v", cfg.QuickFilters[0])
	}
}

func TestGetConfig_FallbackWhenUnconfigured(t *testing.T) {
	svc := NewService(newConfigDB(t))
	b, err := svc.Create("proj-1", "Board B", "scrum", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	fallback := []FallbackStatus{
		{ID: "st-todo", Name: "To Do"},
		{ID: "st-prog", Name: "In Progress"},
		{ID: "st-done", Name: "Done"},
	}
	cfg, err := svc.GetConfig(b.ID, fallback)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}

	if cfg.Swimlane != "none" {
		t.Errorf("swimlane di default atteso 'none', got %q", cfg.Swimlane)
	}
	if len(cfg.QuickFilters) != 0 {
		t.Errorf("nessun quick filter atteso, got %d", len(cfg.QuickFilters))
	}
	if len(cfg.Columns) != 3 {
		t.Fatalf("attese 3 colonne di fallback, got %d", len(cfg.Columns))
	}
	for i, st := range fallback {
		c := cfg.Columns[i]
		if c.Name != st.Name || c.Position != i {
			t.Errorf("colonna %d inattesa: %+v", i, c)
		}
		if !equalStrs(c.StatusIDs, []string{st.ID}) {
			t.Errorf("colonna %d status set atteso [%s], got %v", i, st.ID, c.StatusIDs)
		}
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func equalStrs(a, b []string) bool {
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
