package user

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func mk(db *gorm.DB, id, email, name string) {
	db.Create(&User{ID: id, Email: email, Username: id, DisplayName: name, IsActive: true})
}

func TestSearch(t *testing.T) {
	db := newDB(t)
	mk(db, "u1", "ada@x.io", "Ada Admin")
	mk(db, "u2", "dev@x.io", "Devi Dev")
	svc := NewService(db)
	res, err := svc.Search("ada", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 || res[0].ID != "u1" {
		t.Errorf("search per nome errata: %+v", res)
	}
	res, _ = svc.Search("x.io", 10)
	if len(res) != 2 {
		t.Errorf("search per email deve trovare 2, %d", len(res))
	}
}

func TestUpdateProfile(t *testing.T) {
	db := newDB(t)
	mk(db, "u1", "ada@x.io", "Ada")
	svc := NewService(db)
	dn, tz := "Ada Lovelace", "Europe/Rome"
	u, err := svc.UpdateProfile("u1", &dn, &tz, nil, nil)
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if u.DisplayName != "Ada Lovelace" || u.TimeZone != "Europe/Rome" {
		t.Errorf("profilo non aggiornato: %+v", u)
	}
}
