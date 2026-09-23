package storetest

import "testing"

func TestNewAppliesTheRealSchema(t *testing.T) {
	db := New(t)

	// A table only the real migrations create — AutoMigrate on a domain model
	// would never produce it.
	if !db.Migrator().HasTable("schema_migrations") {
		t.Error("schema_migrations missing: the real migrations did not run")
	}
	for _, table := range []string{"projects", "issues", "notifications", "issue_history"} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("table %q missing", table)
		}
	}
}

func TestNewGivesEachTestItsOwnDatabase(t *testing.T) {
	a := New(t)
	b := New(t)

	// Go through GORM rather than raw SQL: "key" needs backticks on MySQL but
	// double quotes on Postgres, and projects.key is TEXT NOT NULL UNIQUE so it
	// cannot be omitted. GORM's Table().Create() quotes identifiers per dialect,
	// keeping this insert portable across all three drivers.
	if err := a.Table("projects").Create(map[string]any{
		"id":     "p-isolation",
		"seq_id": 99001,
		"key":    "ISO",
		"name":   "Isolation",
	}).Error; err != nil {
		t.Fatalf("insert into a: %v", err)
	}
	var n int64
	if err := b.Raw("SELECT count(*) FROM projects WHERE id = ?", "p-isolation").Scan(&n).Error; err != nil {
		t.Fatalf("count in b: %v", err)
	}
	if n != 0 {
		t.Errorf("row leaked between databases: got %d, want 0", n)
	}
}
