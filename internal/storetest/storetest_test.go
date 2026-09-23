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

// TestPostgresDSNWithDatabase covers both DSN forms Postgres/pgx accept.
// No database is needed: this only exercises the string transformation.
func TestPostgresDSNWithDatabase(t *testing.T) {
	cases := []struct {
		name   string
		dsn    string
		dbName string
		want   string
	}{
		{
			name:   "postgres URL scheme",
			dsn:    "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
			dbName: "heureum_test_x",
			want:   "postgres://postgres:postgres@localhost:5432/heureum_test_x?sslmode=disable",
		},
		{
			name:   "postgresql URL scheme",
			dsn:    "postgresql://user:pass@db.example.com:5433/appdb",
			dbName: "heureum_test_y",
			want:   "postgresql://user:pass@db.example.com:5433/heureum_test_y",
		},
		{
			// The regression this guards: url.Parse "succeeds" on this input
			// too, but silently drops host/port/user/password, leaving only a
			// path. Any correct rewrite must keep them.
			name:   "keyword/value form with an existing dbname",
			dsn:    "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable",
			dbName: "heureum_test_z",
			want:   "host=localhost port=5432 user=postgres password=postgres dbname=heureum_test_z sslmode=disable",
		},
		{
			name:   "keyword/value form with no dbname",
			dsn:    "host=localhost port=5432 user=postgres sslmode=disable",
			dbName: "heureum_test_w",
			want:   "host=localhost port=5432 user=postgres sslmode=disable dbname=heureum_test_w",
		},
		{
			name:   "keyword/value form with a quoted value containing a space",
			dsn:    "host=localhost dbname=postgres password='a b' sslmode=disable",
			dbName: "heureum_test_v",
			want:   "host=localhost dbname=heureum_test_v password='a b' sslmode=disable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := postgresDSNWithDatabase(tc.dsn, tc.dbName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
