package store

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/config"
)

// A plain ":memory:" DSN is the sharpest form of the bug: EVERY connection to
// ":memory:" is its own separate database, so the old URL-based RunMigrations —
// which opened a second sql.DB of its own — applied the migrations somewhere the
// application never sees.
//
// The pool is pinned to a single connection on purpose. store.New configures 25,
// and with ":memory:" each pooled connection would be a different database, which
// would make this test flaky in BOTH directions. One connection makes the
// assertion be about the defect and nothing else.
func TestRunMigrationsAppliesToTheOpenConnection(t *testing.T) {
	cfg := config.DBConfig{Driver: "sqlite", DSN: ":memory:"}
	s, err := New(cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	sqlDB, err := s.DB.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := RunMigrations(s); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// The migrations create `projects` (migration 000001). If migrations ran
	// against a different connection, this table does not exist here.
	if !s.DB.Migrator().HasTable("projects") {
		t.Error("projects table missing: migrations did not apply to the store's own connection")
	}
}

// RunMigrationsFrom lets a caller that knows where the migrations live say so,
// instead of relying on the relative-path guesses that only work two levels deep.
func TestRunMigrationsFromExplicitSource(t *testing.T) {
	cfg := config.DBConfig{Driver: "sqlite", DSN: ":memory:"}
	s, err := New(cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()
	sqlDB, err := s.DB.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// internal/store is two levels below the repository root.
	if err := RunMigrationsFrom(s, "file://../../migrations"); err != nil {
		t.Fatalf("RunMigrationsFrom() error = %v", err)
	}
	if !s.DB.Migrator().HasTable("projects") {
		t.Error("projects table missing: migrations did not apply")
	}
}

func TestRunMigrationsFromRejectsMissingSource(t *testing.T) {
	cfg := config.DBConfig{Driver: "sqlite", DSN: ":memory:"}
	s, err := New(cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	if err := RunMigrationsFrom(s, "file://./definitely-not-a-migrations-dir"); err == nil {
		t.Error("expected an error for a source directory that does not exist, got nil")
	}
}
