package issue

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/storetest"
	"gorm.io/gorm"
)

func resDB(t *testing.T) *gorm.DB {
	return storetest.New(t)
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
