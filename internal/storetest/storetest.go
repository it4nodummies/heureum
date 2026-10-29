// Package storetest gives tests a database with the REAL schema — the migrations
// in migrations/*.sql, applied by golang-migrate — instead of a GORM AutoMigrate
// of a handful of models plus hand-written CREATE TABLE stubs.
//
// That distinction is the reason this package exists. The stubs carry no foreign
// keys and no real column types, so production-only defects passed every gate in
// July 2026: an empty string written into a nullable FK, and an int64 written
// into a TIMESTAMP column.
//
// What this actually guarantees, per driver:
//
//   - Real column types and NOT NULL/UNIQUE constraints, on every driver, simply
//     because the schema comes from the real migrations rather than AutoMigrate —
//     this alone is what makes the int64-into-TIMESTAMP defect above impossible
//     here, on every driver, with or without any pragma.
//   - Referential integrity (foreign keys), on Postgres and MySQL always, and on
//     SQLite ONLY because of that pragma: SQLite defaults PRAGMA foreign_keys to
//     OFF, and neither store.New nor gorm.io/driver/sqlite ever turns it on, so
//     without the pragma an insert referencing a nonexistent parent row would
//     silently succeed here — exactly the class of bug the empty-string-FK
//     defect above was. Verified directly: with the pragma off, that read is 0
//     and such an insert succeeds; with it on (what New actually uses), the read
//     is 1 and the same insert fails with "FOREIGN KEY constraint failed".
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

// Driver returns the driver name this package's New will open — the value of
// TEST_DB_DRIVER, or "sqlite" when it is unset. It is the single source of
// truth for that default: tests that need to branch on the driver in effect
// (e.g. to compute an expected value that only differs on one of them) should
// call this instead of re-reading TEST_DB_DRIVER and re-deriving the "sqlite"
// default themselves.
func Driver() string {
	driver := os.Getenv("TEST_DB_DRIVER")
	if driver == "" {
		driver = "sqlite"
	}
	return driver
}

// New returns a migrated, isolated database for this test, on the driver named by
// TEST_DB_DRIVER ("sqlite" when unset). It registers its own cleanup.
func New(t *testing.T) *gorm.DB {
	t.Helper()

	driver := Driver()

	dsn := dsnFor(t, driver)

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
// the given driver, and registers its own removal via t.Cleanup before
// returning. Tests run in parallel within a package and share nothing, so
// every call must produce a database no other test can see.
//
// Each driver's helper registers its drop-cleanup itself, immediately after
// the CREATE DATABASE that makes it necessary succeeds — not by returning a
// closure for the caller to register later. Anything that can still fail
// between then and this function returning (DSN rewriting, DSN parsing for
// the caller's own connection, ...) must not be able to leak the database by
// t.Fatalf-ing (which calls runtime.Goexit) before a cleanup was ever
// registered. New() then registers the application connection's Close() after
// this returns, so Go's LIFO cleanup order still closes that connection
// before dropping the database out from under it.
func dsnFor(t *testing.T, driver string) string {
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
		return ""
	}
}

// sqliteDSN returns a path inside t.TempDir(), never ":memory:" — store.New
// configures a 25-connection pool, and every connection to ":memory:" is a
// separate, independent database, which would make the schema invisible to
// all but the first connection to grab it. t.TempDir() cleans itself up, so
// there is nothing for this to register.
//
// "?_foreign_keys=on" is load-bearing, not decoration: neither store.New nor
// gorm.io/driver/sqlite ever sets PRAGMA foreign_keys, and SQLite defaults it
// OFF. Verified directly against this DSN: without the pragma,
// "PRAGMA foreign_keys" reads 0 and an insert referencing a nonexistent
// parent row SUCCEEDS; with it, the same read is 1 and the same insert fails
// with "FOREIGN KEY constraint failed" — exactly the constraint class the
// empty-string-FK defect (see the package doc) violated. Without this, the
// helper would deliver real column types/NOT NULL/UNIQUE from the migrations
// but silently skip referential integrity, on the one driver everybody runs
// locally. mattn/go-sqlite3 opens with SQLITE_OPEN_URI, so the "file:" prefix
// makes it parse the query string as PRAGMA-setting options rather than part
// of the filename (see its Open() doc comment for the full "_foreign_keys"
// option).
func sqliteDSN(t *testing.T) string {
	t.Helper()
	return "file:" + filepath.Join(t.TempDir(), "test.db") + "?_foreign_keys=on"
}

// postgresDSN creates a fresh database on the Postgres server named by
// TEST_POSTGRES_DSN (an admin/superuser DSN, defaulting to a local dev
// Postgres), registers its removal, and returns a DSN pointing at it.
func postgresDSN(t *testing.T) string {
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

	// Register the drop *now*, before anything else in this function can fail:
	// the database above already exists, so from this point on only a
	// t.Cleanup registered before any subsequent t.Fatalf can prevent a leak.
	t.Cleanup(func() {
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
	})

	testDSN, err := postgresDSNWithDatabase(adminDSN, name)
	if err != nil {
		t.Fatalf("storetest: build postgres test DSN from TEST_POSTGRES_DSN %q: %v", adminDSN, err)
	}
	return testDSN
}

// postgresDSNWithDatabase rewrites dsn to point at database name, accepting
// either form Postgres/pgx accept: a URL ("postgres://user:pass@host:port/db
// ?opts", "postgresql://...") or libpq keyword/value pairs ("host=... dbname=
// ... ..."). net/url.Parse "succeeds" on a keyword/value string too — it just
// silently produces a URL with only a Path (host, port and credentials
// dropped) — so the two forms must be told apart before doing any rewriting,
// not just fed to url.Parse and hoped for.
func postgresDSNWithDatabase(dsn, name string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("parse %q as a postgres URL: %w", dsn, err)
		}
		u.Path = "/" + name
		return u.String(), nil
	}
	return replacePostgresKeywordValueDBName(dsn, name), nil
}

// replacePostgresKeywordValueDBName replaces the dbname= token in a libpq
// keyword/value DSN with name, appending one if the DSN doesn't already have
// one. Tokens are whitespace-separated, but a value may be single-quoted to
// contain whitespace (with \' and \\ as its only escapes) — see
// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
// — so this can't just strings.Fields the input.
func replacePostgresKeywordValueDBName(dsn, name string) string {
	tokens := splitKeywordValue(dsn)
	replaced := false
	for i, tok := range tokens {
		if key, _, ok := strings.Cut(tok, "="); ok && key == "dbname" {
			tokens[i] = "dbname=" + quoteKeywordValue(name)
			replaced = true
		}
	}
	if !replaced {
		tokens = append(tokens, "dbname="+quoteKeywordValue(name))
	}
	return strings.Join(tokens, " ")
}

// splitKeywordValue splits a libpq keyword/value DSN into its "key=value"
// tokens, treating whitespace inside a single-quoted value as part of the
// token rather than a separator.
func splitKeywordValue(dsn string) []string {
	var tokens []string
	var cur strings.Builder
	inQuotes := false
	escaped := false
	for _, r := range dsn {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case inQuotes && r == '\\':
			cur.WriteRune(r)
			escaped = true
		case r == '\'':
			inQuotes = !inQuotes
			cur.WriteRune(r)
		case !inQuotes && (r == ' ' || r == '\t' || r == '\n'):
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// quoteKeywordValue quotes v for use as a libpq keyword/value token if it
// contains characters that would otherwise be misread (whitespace, quotes,
// backslashes, or emptiness); otherwise it is returned unquoted.
func quoteKeywordValue(v string) string {
	if v == "" {
		return "''"
	}
	if !strings.ContainsAny(v, " \t\n'\\") {
		return v
	}
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `'`, `\'`)
	return "'" + v + "'"
}

// pgQuoteIdent quotes a Postgres identifier, doubling any embedded quotes.
func pgQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// mysqlDSN creates a fresh database on the MySQL/MariaDB server named by
// TEST_MYSQL_DSN (an admin DSN, defaulting to a local dev server), registers
// its removal, and returns a DSN pointing at it. The returned DSN keeps
// whatever multiStatements setting the admin DSN had (store.New normalises it
// to true via ensureMultiStatements regardless, so this never needs to set it
// itself).
func mysqlDSN(t *testing.T) string {
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

	// Register the drop *now*, before anything else in this function can fail —
	// same reasoning as postgresDSN above.
	t.Cleanup(func() {
		admin, err := sql.Open("mysql", adminDSN)
		if err != nil {
			return
		}
		defer func() { _ = admin.Close() }()
		_, _ = admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name))
	})

	cfg, err := gomysql.ParseDSN(adminDSN)
	if err != nil {
		t.Fatalf("storetest: parse TEST_MYSQL_DSN %q: %v", adminDSN, err)
	}
	cfg.DBName = name
	return cfg.FormatDSN()
}
