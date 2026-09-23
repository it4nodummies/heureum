package store

import (
	"os"
	"path/filepath"
	"strings"
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

// TestMigrateUpSurfacesTheSecondSourcesUpError is the regression guard for the
// production-reachable defect this round fixed: migrateUp used to keep looping
// over candidate sources even after one of them had OPENED and then failed
// Up() with a genuine SQL error, so a later candidate's open-error (e.g. "no
// such file or directory" for the Docker image's missing file:///migrations)
// clobbered the real failure. Since the Dockerfile's first candidate path
// always fails to open and the second always opens, this is exactly the
// production shape: candidate 1 open-fails, candidate 2 opens and its Up()
// hits a real SQL error — the caller must see THAT error, not candidate 1's.
func TestMigrateUpSurfacesTheSecondSourcesUpError(t *testing.T) {
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

	drv, name, err := driverForStore(s)
	if err != nil {
		t.Fatalf("driverForStore() error = %v", err)
	}

	// A source whose only migration file is not valid SQL, so its Up() fails
	// with a genuine SQL error once golang-migrate opens it.
	badDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(badDir, "000001_bad.up.sql"), []byte("THIS IS NOT SQL;"), 0o644); err != nil {
		t.Fatalf("write bad migration up file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "000001_bad.down.sql"), []byte(""), 0o644); err != nil {
		t.Fatalf("write bad migration down file: %v", err)
	}

	err = migrateUp(drv, name,
		"file:///definitely-not-a-real-path-xyz", // candidate 1: fails to OPEN
		"file://"+filepath.ToSlash(badDir),       // candidate 2: opens, Up() fails
	)
	if err == nil {
		t.Fatal("migrateUp() error = nil, want the second source's Up() error")
	}
	if !strings.Contains(err.Error(), "migrate up") {
		t.Errorf("migrateUp() error = %q, want it to be a \"migrate up\" (SQL) error, not a source-open error", err.Error())
	}
	if strings.Contains(err.Error(), "definitely-not-a-real-path-xyz") {
		t.Errorf("migrateUp() error = %q, want the SECOND source's error, not the first (open-failed) candidate's path leaking through", err.Error())
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
