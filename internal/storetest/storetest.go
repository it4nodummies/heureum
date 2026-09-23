// Package storetest gives tests a database with the REAL schema — the migrations
// in migrations/*.sql, applied by golang-migrate — instead of a GORM AutoMigrate
// of a handful of models plus hand-written CREATE TABLE stubs.
//
// That distinction is the reason this package exists. The stubs carry no foreign
// keys and no real column types, so three production-only defects passed every
// gate in July 2026: an empty string written into a nullable FK, and an int64
// written into a TIMESTAMP column. Both are impossible to write here.
//
// This package is imported only by _test.go files. Never import it from
// production code.
package storetest

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/config"
	"github.com/it4nodummies/heureum/internal/store"
)

// New returns a migrated, isolated database for this test, on the driver named by
// TEST_DB_DRIVER ("sqlite" when unset). It registers its own cleanup.
func New(t *testing.T) *gorm.DB {
	t.Helper()

	driver := os.Getenv("TEST_DB_DRIVER")
	if driver == "" {
		driver = "sqlite"
	}

	dsn, cleanup := dsnFor(t, driver)
	t.Cleanup(cleanup)

	s, err := store.New(config.DBConfig{Driver: driver, DSN: dsn}, "test")
	if err != nil {
		t.Fatalf("storetest: open %s: %v", driver, err)
	}
	t.Cleanup(func() { _ = s.Close() })

	src, err := migrationsSource()
	if err != nil {
		t.Fatalf("storetest: %v", err)
	}
	if err := store.RunMigrationsFrom(s, src); err != nil {
		t.Fatalf("storetest: apply migrations on %s: %v", driver, err)
	}
	return s.DB
}

// migrationsSource walks up from the working directory until it finds the
// migrations directory, and returns it as a file:// URL. Walking up is deliberate:
// test packages sit at different depths (internal/store is two levels down,
// internal/domain/issue is three), so any fixed relative path is wrong somewhere.
func migrationsSource() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for {
		candidate := filepath.Join(dir, "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return "file://" + filepath.ToSlash(candidate), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no migrations directory found walking up from the test's working directory")
		}
		dir = parent
	}
}

// dbNameCounter makes generated database names collision-free across tests
// running in parallel within the same process; combined with the OS pid it is
// also collision-free across separate `go test` processes (e.g. different
// packages under `go test ./...`).
var dbNameCounter int64

// uniqueDBName returns a database name derived from the test name, the process
// id and a monotonic counter — safe to use unquoted in CREATE/DROP DATABASE
// statements on both Postgres and MySQL.
func uniqueDBName(t *testing.T) string {
	t.Helper()
	n := atomic.AddInt64(&dbNameCounter, 1)
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, t.Name())
	if len(safe) > 32 {
		safe = safe[:32]
	}
	return fmt.Sprintf("heureum_test_%s_%d_%d", safe, os.Getpid(), n)
}

// dsnFor returns a DSN pointing at a fresh, isolated database for this test on
// the given driver, plus a cleanup function to remove it. Tests run in parallel
// within a package and share nothing, so every call must produce a database no
// other test can see.
func dsnFor(t *testing.T, driver string) (string, func()) {
	t.Helper()

	switch driver {
	case "sqlite":
		return sqliteDSN(t)
	case "postgres":
		return postgresDSN(t)
	case "mysql", "mariadb":
		return mysqlDSN(t)
	default:
		t.Fatalf("storetest: unsupported TEST_DB_DRIVER %q", driver)
		return "", func() {}
	}
}

// sqliteDSN returns a path inside t.TempDir(), never ":memory:" — store.New
// configures a 25-connection pool, and every connection to ":memory:" is a
// separate, independent database, which would make the schema invisible to
// all but the first connection to grab it.
func sqliteDSN(t *testing.T) (string, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	// t.TempDir() cleans itself up; nothing else to do.
	return path, func() {}
}

// postgresDSN creates a fresh database on the Postgres server named by
// TEST_POSTGRES_DSN (an admin/superuser DSN, defaulting to a local dev
// Postgres) and returns a DSN pointing at it plus a cleanup that drops it.
func postgresDSN(t *testing.T) (string, func()) {
	t.Helper()

	adminDSN := os.Getenv("TEST_POSTGRES_DSN")
	if adminDSN == "" {
		adminDSN = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}

	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("storetest: parse TEST_POSTGRES_DSN %q: %v", adminDSN, err)
	}
	if err := admin.Ping(); err != nil {
		_ = admin.Close()
		t.Fatalf("storetest: postgres unreachable at %q: set TEST_POSTGRES_DSN or start a server: %v", adminDSN, err)
	}

	name := uniqueDBName(t)
	if _, err := admin.Exec(fmt.Sprintf("CREATE DATABASE %s", pgQuoteIdent(name))); err != nil {
		_ = admin.Close()
		t.Fatalf("storetest: create postgres database %s: %v", name, err)
	}
	_ = admin.Close()

	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("storetest: parse TEST_POSTGRES_DSN %q as URL: %v", adminDSN, err)
	}
	u.Path = "/" + name
	testDSN := u.String()

	cleanup := func() {
		admin, err := sql.Open("pgx", adminDSN)
		if err != nil {
			return
		}
		defer func() { _ = admin.Close() }()
		// Terminate any lingering connections (e.g. a leaked pool member) so DROP
		// DATABASE doesn't fail with "database is being accessed by other users".
		_, _ = admin.Exec(
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()",
			name,
		)
		_, _ = admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", pgQuoteIdent(name)))
	}
	return testDSN, cleanup
}

// pgQuoteIdent quotes a Postgres identifier, doubling any embedded quotes.
func pgQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// mysqlDSN creates a fresh database on the MySQL/MariaDB server named by
// TEST_MYSQL_DSN (an admin DSN, defaulting to a local dev server) and returns a
// DSN pointing at it plus a cleanup that drops it. The returned DSN keeps
// whatever multiStatements setting the admin DSN had (store.New normalises it
// to true via ensureMultiStatements regardless, so this never needs to set it
// itself).
func mysqlDSN(t *testing.T) (string, func()) {
	t.Helper()

	adminDSN := os.Getenv("TEST_MYSQL_DSN")
	if adminDSN == "" {
		adminDSN = "root:root@tcp(127.0.0.1:3306)/"
	}

	admin, err := sql.Open("mysql", adminDSN)
	if err != nil {
		t.Fatalf("storetest: parse TEST_MYSQL_DSN %q: %v", adminDSN, err)
	}
	if err := admin.Ping(); err != nil {
		_ = admin.Close()
		t.Fatalf("storetest: mysql unreachable at %q: set TEST_MYSQL_DSN or start a server: %v", adminDSN, err)
	}

	name := uniqueDBName(t)
	if _, err := admin.Exec(fmt.Sprintf("CREATE DATABASE `%s`", name)); err != nil {
		_ = admin.Close()
		t.Fatalf("storetest: create mysql database %s: %v", name, err)
	}
	_ = admin.Close()

	cfg, err := gomysql.ParseDSN(adminDSN)
	if err != nil {
		t.Fatalf("storetest: parse TEST_MYSQL_DSN %q: %v", adminDSN, err)
	}
	cfg.DBName = name
	testDSN := cfg.FormatDSN()

	cleanup := func() {
		admin, err := sql.Open("mysql", adminDSN)
		if err != nil {
			return
		}
		defer func() { _ = admin.Close() }()
		_, _ = admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name))
	}
	return testDSN, cleanup
}
