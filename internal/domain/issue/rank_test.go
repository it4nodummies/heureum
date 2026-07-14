package issue

import (
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// rankSeqCounter genera seq_id univoci per gli helper di test: usare
// int64(len(id)) collide quando gli id di test hanno la stessa lunghezza
// (es. "a", "b", "x"), violando l'unique index su issues.seq_id.
var rankSeqCounter int64

func rankDB(t *testing.T) *gorm.DB {
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

func mk(t *testing.T, db *gorm.DB, id string, pos float64) {
	t.Helper()
	seqID := atomic.AddInt64(&rankSeqCounter, 1)
	if err := db.Create(&Issue{ID: id, ProjectID: "p", Key: id, Title: id, Position: pos, SeqID: seqID}).Error; err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

func pos(t *testing.T, db *gorm.DB, id string) float64 {
	t.Helper()
	var iss Issue
	db.First(&iss, "id = ?", id)
	return iss.Position
}

func TestRank_Between(t *testing.T) {
	db := rankDB(t)
	mk(t, db, "a", 100)
	mk(t, db, "b", 200)
	mk(t, db, "x", 0)
	svc := NewService(db)
	after, before := "a", "b"
	if err := svc.Rank([]string{"x"}, &before, &after); err != nil {
		t.Fatalf("Rank: %v", err)
	}
	p := pos(t, db, "x")
	if p <= 100 || p >= 200 {
		t.Errorf("posizione di x deve stare tra 100 e 200, got %v", p)
	}
}

func TestRank_AfterOnly(t *testing.T) {
	db := rankDB(t)
	mk(t, db, "a", 100)
	mk(t, db, "x", 0)
	svc := NewService(db)
	after := "a"
	if err := svc.Rank([]string{"x"}, nil, &after); err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if pos(t, db, "x") <= 100 {
		t.Errorf("x deve stare dopo a (pos>100), got %v", pos(t, db, "x"))
	}
}

func TestRank_EndWhenNoNeighbors(t *testing.T) {
	db := rankDB(t)
	mk(t, db, "a", 100)
	mk(t, db, "x", 0)
	svc := NewService(db)
	if err := svc.Rank([]string{"x"}, nil, nil); err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if pos(t, db, "x") <= 100 {
		t.Errorf("x deve finire in coda (pos>max=100), got %v", pos(t, db, "x"))
	}
}
