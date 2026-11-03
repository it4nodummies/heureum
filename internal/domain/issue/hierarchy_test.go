package issue

import "testing"

// TestLevelForTypeName_Italian pins that the Italian standard type names
// (sottotask/sotto-attività → subtask, epica → epic) map to the same hierarchy
// levels as their English counterparts, case- and whitespace-insensitively.
func TestLevelForTypeName_Italian(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{"Sottotask", -1},
		{"sotto-task", -1},
		{"Sotto-attività", -1},
		{"sottoattività", -1},
		{"Epica", 1},
		{"Storia", 0},
		{"Subtask", -1},
		{"Epic", 1},
		{"  Epic  ", 1},
	}
	for _, c := range cases {
		if got := levelForTypeName(c.name); got != c.want {
			t.Errorf("levelForTypeName(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestTypeIDByName_ItalianSubtask verifies that TypeIDByName creates an
// Italian-named subtask type as a real subtask (IsSubtask, subtask icon,
// level -1) and an Italian-named epic at level 1 with the epic icon.
func TestTypeIDByName_ItalianSubtask(t *testing.T) {
	db := newIssueTestDB(t)
	// issue_types.project_id is a NOT NULL FK to projects(id).
	createTestProject(t, db, "p1")
	svc := NewService(db)

	subID, err := svc.TypeIDByName("p1", "Sottotask")
	if err != nil {
		t.Fatalf("TypeIDByName(Sottotask): %v", err)
	}
	var sub IssueType
	if err := db.Where("id = ?", subID).First(&sub).Error; err != nil {
		t.Fatal(err)
	}
	if !sub.IsSubtask {
		t.Errorf("Sottotask IsSubtask = false, want true")
	}
	if sub.Icon != "subtask" {
		t.Errorf("Sottotask Icon = %q, want %q", sub.Icon, "subtask")
	}
	if sub.HierarchyLevel != -1 {
		t.Errorf("Sottotask HierarchyLevel = %d, want -1", sub.HierarchyLevel)
	}

	epicID, err := svc.TypeIDByName("p1", "Epica")
	if err != nil {
		t.Fatalf("TypeIDByName(Epica): %v", err)
	}
	var epic IssueType
	if err := db.Where("id = ?", epicID).First(&epic).Error; err != nil {
		t.Fatal(err)
	}
	if epic.HierarchyLevel != 1 {
		t.Errorf("Epica HierarchyLevel = %d, want 1", epic.HierarchyLevel)
	}
	if epic.Icon != "epic" {
		t.Errorf("Epica Icon = %q, want %q", epic.Icon, "epic")
	}
	if epic.IsSubtask {
		t.Errorf("Epica IsSubtask = true, want false")
	}
}
