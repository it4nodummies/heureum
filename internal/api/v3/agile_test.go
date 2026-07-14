package v3

import (
	"testing"
	"time"
)

func TestAgileBoard_Shape(t *testing.T) {
	b := AgileBoard(BoardInput{
		SeqID: 3, Name: "Scrum Board", Type: "scrum",
		ProjectID: 10000, ProjectKey: "DEMO", ProjectName: "Demo", ProjectTypeKey: "software",
		BaseURL: "http://x",
	})
	if b.ID != 3 || b.Name != "Scrum Board" || b.Type != "scrum" {
		t.Errorf("campi base errati: %+v", b)
	}
	if b.Self == "" {
		t.Error("self mancante")
	}
	if b.Location == nil || b.Location.ProjectKey != "DEMO" || b.Location.ProjectID != 10000 {
		t.Errorf("location errata: %+v", b.Location)
	}
}

func TestAgileSprint_Shape(t *testing.T) {
	start := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	board := int64(3)
	sp := AgileSprint(SprintInput{
		SeqID: 5, Name: "Sprint 1", State: "active", Goal: "ship",
		OriginBoardID: &board, StartDate: &start, BaseURL: "http://x",
	})
	if sp.ID != 5 || sp.State != "active" || sp.Goal != "ship" {
		t.Errorf("campi base errati: %+v", sp)
	}
	if sp.OriginBoardID != 3 {
		t.Errorf("originBoardId atteso 3, got %d", sp.OriginBoardID)
	}
	if sp.StartDate == "" {
		t.Error("startDate deve essere formattata (non vuota)")
	}
	if sp.Self == "" {
		t.Error("self mancante")
	}
}

func TestAgileSprint_OmitsEmptyDates(t *testing.T) {
	sp := AgileSprint(SprintInput{SeqID: 1, Name: "S", State: "future", BaseURL: "http://x"})
	if sp.StartDate != "" || sp.EndDate != "" || sp.CompleteDate != "" {
		t.Error("date non impostate devono restare stringhe vuote (omitempty)")
	}
}
