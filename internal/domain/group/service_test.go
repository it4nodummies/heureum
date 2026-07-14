package group

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
	if err := db.AutoMigrate(&Group{}, &GroupMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateAndFind(t *testing.T) {
	svc := NewService(newDB(t))
	g, err := svc.Create("developers")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if g.ID == "" || g.Name != "developers" {
		t.Errorf("gruppo errato: %+v", g)
	}
	got, err := svc.FindByName("developers")
	if err != nil {
		t.Fatalf("FindByName: %v", err)
	}
	if got.ID != g.ID {
		t.Error("id mismatch")
	}
	if _, err := svc.Create("developers"); err == nil {
		t.Error("atteso errore per nome duplicato")
	}
}

func TestMembers(t *testing.T) {
	svc := NewService(newDB(t))
	g, _ := svc.Create("qa")
	if err := svc.AddUser(g.ID, "user-1"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	svc.AddUser(g.ID, "user-2")
	svc.AddUser(g.ID, "user-1") // idempotente: nessun errore/duplicato
	ids, total, err := svc.MemberIDs(g.ID, 0, 50)
	if err != nil {
		t.Fatalf("MemberIDs: %v", err)
	}
	if total != 2 || len(ids) != 2 {
		t.Errorf("attesi 2 membri, total=%d len=%d", total, len(ids))
	}
	if err := svc.RemoveUser(g.ID, "user-1"); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
	_, total, _ = svc.MemberIDs(g.ID, 0, 50)
	if total != 1 {
		t.Errorf("atteso 1 membro dopo rimozione, %d", total)
	}
}

func TestSearchAndDelete(t *testing.T) {
	svc := NewService(newDB(t))
	svc.Create("developers")
	svc.Create("designers")
	svc.Create("qa")
	found, err := svc.Search("des", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(found) != 1 || found[0].Name != "designers" {
		t.Errorf("search errata: %+v", found)
	}
	g, _ := svc.FindByName("qa")
	if err := svc.Delete(g.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.FindByName("qa"); err == nil {
		t.Error("gruppo dovrebbe essere eliminato")
	}
}
