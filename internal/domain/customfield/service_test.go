package customfield

import (
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&CustomField{}, &CustomFieldOption{}, &IssueCustomValue{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateFieldStoresRequired(t *testing.T) {
	svc := NewService(newDB(t))
	f, err := svc.CreateField("proj-1", "Team", FieldTypeText, true)
	if err != nil {
		t.Fatal(err)
	}
	if !f.Required {
		t.Errorf("Required = false, want true")
	}
	got, _ := svc.ListFields("proj-1")
	if len(got) != 1 || !got[0].Required {
		t.Errorf("ListFields lost Required: %+v", got)
	}
}

func TestSetValueDateAndUser(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	df, _ := svc.CreateField("proj-1", "Due", FieldTypeDate, false)
	uf, _ := svc.CreateField("proj-1", "Owner", FieldTypeUser, false)

	if err := svc.SetValue("iss-1", df.ID, "2026-03-10T00:00:00Z"); err != nil {
		t.Fatalf("date SetValue: %v", err)
	}
	if err := svc.SetValue("iss-1", uf.ID, "user-123"); err != nil {
		t.Fatalf("user SetValue: %v", err)
	}
	vals, _ := svc.GetValues("iss-1")
	byField := map[string]IssueCustomValue{}
	for _, v := range vals {
		byField[v.FieldID] = v
	}
	if byField[df.ID].ValueDate == nil || byField[df.ID].ValueDate.Format("2006-01-02") != "2026-03-10" {
		t.Errorf("date not stored: %+v", byField[df.ID])
	}
	if byField[uf.ID].ValueText != "user-123" {
		t.Errorf("user not stored: %+v", byField[uf.ID])
	}
	_ = time.Now
}

func TestSetValueUnknownFieldRejected(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)

	err := svc.SetValue("iss-1", "does-not-exist", "whatever")
	if err == nil {
		t.Fatal("SetValue on unknown fieldID returned nil, want ErrFieldNotFound")
	}
	if !errors.Is(err, ErrFieldNotFound) {
		t.Errorf("error = %v, want ErrFieldNotFound", err)
	}
	// No orphaned row should have been written.
	vals, _ := svc.GetValues("iss-1")
	if len(vals) != 0 {
		t.Errorf("orphaned value row written: %+v", vals)
	}
}
