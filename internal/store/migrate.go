package store

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// migrationPaths covers the standard Docker layout first, then the two local
// dev layouts (repo root, and a package two levels down).
var migrationPaths = []string{
	"file:///migrations",
	"file://migrations",
	"file://../../migrations",
}

// driverFor wraps the ALREADY-OPEN *sql.DB in the golang-migrate driver for
// this database. Building a URL from the DSN instead (the previous approach)
// both opened a second, unrelated connection and produced malformed URLs for
// any DSN that is not a bare path - "sqlite3://file::memory:?cache=shared"
// parses ":memory:" as a port, which Go 1.26's net/url rejects.
func driverFor(driverName string, db *sql.DB) (database.Driver, string, error) {
	switch driverName {
	case "sqlite":
		d, err := sqlite3.WithInstance(db, &sqlite3.Config{})
		return d, "sqlite3", err
	case "postgres":
		d, err := postgres.WithInstance(db, &postgres.Config{})
		return d, "postgres", err
	case "mysql", "mariadb":
		// mysql.WithInstance requires the *sql.DB to have been opened with
		// multiStatements=true (it runs each migration file as one ExecContext
		// covering many statements) — see store.ensureMultiStatements, applied
		// to the DSN in store.New before this connection was ever opened.
		d, err := mysql.WithInstance(db, &mysql.Config{})
		return d, "mysql", err
	default:
		return nil, "", fmt.Errorf("unsupported DB driver: %s", driverName)
	}
}

// RunMigrations applies every pending migration to the store's own connection.
func RunMigrations(s *Store) error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	drv, name, err := driverFor(s.Driver, sqlDB)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}

	var m *migrate.Migrate
	for _, path := range migrationPaths {
		m, err = migrate.NewWithDatabaseInstance(path, name, drv)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	// Deliberately no `defer m.Close()` here, and none should ever be added: m
	// wraps the caller-owned *sql.DB (via driverFor/WithInstance above), not a
	// connection it opened itself. All three drivers' Close() closes that
	// *sql.DB (sqlite3.go: `return m.db.Close()`; postgres.go and mysql.go close
	// both their internal *sql.Conn and the *sql.DB) — calling it here would
	// close the application's own connection pool right after a successful
	// migration, invisible to the compiler, go vet, and every existing test.
	// Related fact for postgres/mysql: WithInstance checks out one *sql.Conn from
	// the pool for its own use and never returns it, permanently costing 1 of the
	// 25 connections configured in store.New — not a regression, just something a
	// reader of this function should know.
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
