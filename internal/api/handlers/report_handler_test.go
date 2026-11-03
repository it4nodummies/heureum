package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/report"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
	"github.com/it4nodummies/heureum/internal/storetest"
)

// The reports page passes the PUBLIC sprint id (seq_id, as
// /rest/agile/1.0/board/{id}/sprint exposes it) as ?sprintId=. The handlers
// used to look it up as the internal UUID, so burndown/burnup answered
// 500 "sprint not found" for every sprint (visible as four HTTP 500s on the
// reports page).
func seedReportProject(t *testing.T, db *gorm.DB, key string, seq int64) *project.Project {
	t.Helper()
	p := &project.Project{ID: uuid.NewString(), Key: key, Name: key, SeqID: seq}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	return p
}

func seedReportSprint(t *testing.T, db *gorm.DB, projectID string, seq int64) *sprint.Sprint {
	t.Helper()
	start := time.Now().AddDate(0, 0, 30)
	end := start.AddDate(0, 0, 14)
	sp := &sprint.Sprint{ID: uuid.NewString(), ProjectID: projectID, Name: "Future", State: sprint.StateFuture, StartDate: &start, EndDate: &end, SeqID: seq}
	if err := db.Create(sp).Error; err != nil {
		t.Fatalf("create sprint: %v", err)
	}
	return sp
}

func TestReportHandler_BurndownBurnupBySprintPublicID(t *testing.T) {
	db := storetest.New(t)
	demo := seedReportProject(t, db, "DEMO", 10000)
	other := seedReportProject(t, db, "OTHER", 10001)
	seedReportSprint(t, db, demo.ID, 7)
	foreign := seedReportSprint(t, db, other.ID, 8)

	h := NewReportHandler(report.NewService(db), project.NewService(db, nil))
	call := func(fn http.HandlerFunc, sprintID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/rest/api/3/project/DEMO/reports/x?sprintId="+sprintID, nil)
		req.SetPathValue("key", "DEMO")
		w := httptest.NewRecorder()
		fn(w, req)
		return w
	}

	for name, fn := range map[string]http.HandlerFunc{"burndown": h.Burndown, "burnup": h.Burnup} {
		w := call(fn, "7")
		if w.Code != http.StatusOK {
			t.Fatalf("%s?sprintId=7 (future sprint of DEMO) = %d, want 200: %s", name, w.Code, w.Body.String())
		}
		var data report.BurndownData
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || len(data.Labels) == 0 {
			t.Errorf("%s: want a non-empty series, got %s (err %v)", name, w.Body.String(), err)
		}
		for _, ref := range []string{"999", "8", foreign.ID, "abc"} {
			if w := call(fn, ref); w.Code != http.StatusNotFound {
				t.Errorf("%s?sprintId=%s = %d, want 404 (unknown, other project's, or internal id)", name, ref, w.Code)
			}
		}
	}
}
