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
// such file or directory" for a missing migrations path) clobbered the real
// failure. This is exactly the production shape: three candidates
// (file:///migrations, file://migrations, file://../../migrations), where the
// Dockerfile's layout makes candidate 1 fail to open, candidate 2 open and
// run, and candidate 3 fail to open — and it was candidate 3's open-error
// that masked a real SQL failure from candidate 2 in the operator's log.
//
// Deliberately THREE sources, not two: with only two (an open-failure
// followed by the Up()-failing source, which is also the last source), the
// pre-fix and post-fix code return the IDENTICAL error, because there is no
// later source left to overwrite it — the test would pass against either
// implementation and prove nothing. A trailing third source that also fails
// to open is what makes the two control flows diverge: the pre-fix code
// `continue`s past the Up() failure and returns candidate 3's open-error
// instead; the fixed code returns candidate 2's Up() error immediately and
// never even looks at candidate 3.
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
		"file:///definitely-not-a-real-path-abc", // candidate 3: fails to OPEN — must never be reached
	)
	if err == nil {
		t.Fatal("migrateUp() error = nil, want the second source's Up() error")
	}
	if !strings.Contains(err.Error(), "migrate up") {
		t.Errorf("migrateUp() error = %q, want it to be a \"migrate up\" (SQL) error from candidate 2, not a source-open error from candidate 1 or 3", err.Error())
	}
	if strings.Contains(err.Error(), "definitely-not-a-real-path-xyz") || strings.Contains(err.Error(), "definitely-not-a-real-path-abc") {
		t.Errorf("migrateUp() error = %q, want candidate 2's Up() error, not a trailing/leading candidate's open-error path leaking through", err.Error())
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
