package issue

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/storetest"
	"gorm.io/gorm"
)

func resDB(t *testing.T) *gorm.DB {
	return storetest.New(t)
}

// TestEnsureResolutionIdempotent runs against the real schema: the global
// `resolutions` table comes from migrations/000002_jira_alignment.up.sql (there
// is no Go model struct for it — it's accessed via db.Table("resolutions"),
// mirroring ResolutionIDByName and cmd/seed), so the test exercises the actual
// column set and constraints rather than a hand-written stub.
func TestEnsureResolutionIdempotent(t *testing.T) {
	db := resDB(t)
	svc := NewService(db)

	id, err := svc.EnsureResolution("Done", "Work is complete")
	if err != nil {
		t.Fatalf("EnsureResolution: %v", err)
	}
	if id == "" {
		t.Fatal("EnsureResolution returned empty id")
	}

	var count int64
	if err := db.Table("resolutions").Where("LOWER(name) = LOWER(?)", "Done").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("resolutions row count = %d, want 1", count)
	}

	// Second call must return the SAME id and not duplicate.
	id2, err := svc.EnsureResolution("Done", "Work is complete")
	if err != nil {
		t.Fatalf("EnsureResolution (second call): %v", err)
	}
	if id2 != id {
		t.Errorf("second id = %q, want %q (same resolution)", id2, id)
	}
	if err := db.Table("resolutions").Where("LOWER(name) = LOWER(?)", "Done").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("after re-run resolutions row count = %d, want 1 (no duplicate)", count)
	}
}

func TestSetResolution(t *testing.T) {
	db := resDB(t)
	createTestProject(t, db, "p")
	svc := NewService(db)
	iss := &Issue{ID: "i1", ProjectID: "p", Key: "P-1", Title: "x", SeqID: 1}
	db.Create(iss)

	// issues.resolution_id is a real FK to resolutions(id): the table is
	// otherwise empty (production seeds no default resolutions — see
	// internal/api/handlers/reference_handler.go's defaultResolutions
	// fallback), so the row this test assigns must actually exist.
	if err := db.Exec(`INSERT INTO resolutions (id, name) VALUES (?, ?)`, "res-done", "Done").Error; err != nil {
		t.Fatalf("seed resolution: %v", err)
	}

	res := "res-done"
	if err := svc.SetResolution("P-1", &res); err != nil {
		t.Fatalf("SetResolution: %v", err)
	}
	var got Issue
	db.First(&got, "id = ?", "i1")
	if got.ResolutionID == nil || *got.ResolutionID != "res-done" {
		t.Errorf("resolution non settata: %v", got.ResolutionID)
	}

	// clear
	if err := svc.SetResolution("P-1", nil); err != nil {
		t.Fatalf("SetResolution clear: %v", err)
	}
	db.First(&got, "id = ?", "i1")
	if got.ResolutionID != nil {
		t.Errorf("resolution doveva essere azzerata, got %v", *got.ResolutionID)
	}
}
