package issue

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func resDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Issue{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSetResolution(t *testing.T) {
	db := resDB(t)
	svc := NewService(db)
	iss := &Issue{ID: "i1", ProjectID: "p", Key: "P-1", Title: "x", SeqID: 1}
	db.Create(iss)

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
