# Round 24 — Gate reale (schema vero nei test, matrice CI, notification coperto) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the test suite exercise the schema and the drivers that production actually uses, so the class of bug that shipped three times in July — correct against SQLite, broken against Postgres or MySQL — can no longer reach a release undetected.

**Architecture:** Today every Go test builds its schema with `db.AutoMigrate(&Model{})` plus hand-written `CREATE TABLE` stubs that carry **no foreign keys and no real column types**, and the whole suite runs on SQLite only. That is why an empty string written into a nullable FK, and an `int64` written into a `TIMESTAMP`, both passed every gate and failed in production. Round 23 made the enabler available: `RunMigrations` now takes an already-open `*Store`, so a test can say "migrate *this* database". This round adds a shared helper that applies the **real** `migrations/*.sql` to a per-test database on whichever driver is selected, converts the domains that write nullable foreign keys, writes the missing tests for `notification` (which has none), and turns the single `backend` CI job into a matrix over SQLite, Postgres and MySQL.

**Tech Stack:** Go 1.26 (`golang-migrate/migrate/v4` v4.19.1, GORM, `go-sql-driver/mysql`, `jackc/pgx` via `gorm.io/driver/postgres`), GitHub Actions service containers. No new dependency: every driver is already required by `go.mod`.

**Spec:** `docs/superpowers/specs/2026-09-21-direzione-prodotto-design.md` (section "R24 — Gate reale"), as amended by two scope decisions taken with the user on 2026-09-23:
- **Per-worker Playwright databases are OUT of this round.** Playwright's `webServer` starts one backend for the whole run, so per-worker isolation would mean N backends on N ports plus routing `next dev` and `NEXT_PUBLIC_API_URL` per worker — high cost for ~1.5 minutes on a 2.8-minute suite. e2e stays at `workers: 1`; the isolation idea becomes a follow-up.
- **Scope is the four FK-nullable domains plus the CI matrix.** Porting the remaining test files to the helper is opportunistic, done when a file is touched for other reasons. PR #44 is not a task: it becomes this round's final verification.

## Global Constraints

- **Module path `github.com/it4nodummies/heureum`** — verbatim in imports.
- **No new dependency.** `go-sql-driver/mysql` is already a direct requirement (Round 23); `gorm.io/driver/postgres` and `gorm.io/driver/mysql` are already required. Adding a module to `go.mod` is a plan violation — if you believe one is needed, stop and report.
- **Do not touch `/rest/api/3` or `/rest/agile/1.0` route shapes.** ADR 0001 freezes that surface; this round adds no route. `go run ./cmd/gapreport` must produce no diff.
- **Production code paths must not learn about tests.** The helper lives in its own package and is imported only by `_test.go` files. Do not add test-only branches to `internal/store/store.go` or to any domain service.
- **A converted test that fails is a finding, not an obstacle.** The whole point of applying the real schema is that it enforces constraints the stubs did not. When a test goes red after conversion, diagnose it: if it is a genuine defect in production code, fix it and say so in your report; if the fix is larger than the task, record it and report it rather than weakening the test. **Never** relax a constraint, drop a foreign key, or switch a test back to `AutoMigrate` to make it pass.
- Conventional Commits. Branch: `feat/round-24-gate-reale` off `main`.
- **Three-level gate before the round is done:** (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test`; (3) `go run ./cmd/gapreport` with no diff.

---

### Task 1: Let callers choose the migration source

`RunMigrations` resolves the migrations directory from a hardcoded list — `file:///migrations`, `file://migrations`, `file://../../migrations` — which reaches at most two levels up. A test in `internal/domain/issue` sits **three** levels down, where `../../migrations` does not exist (verified). The helper in Task 2 therefore cannot use `RunMigrations` as it stands.

**Files:**
- Modify: `internal/store/migrate.go`
- Test: `internal/store/migrate_test.go`

**Interfaces:**
- Consumes: `RunMigrations(s *Store) error` and the unexported `driverFor`, both from Round 23.
- Produces: `func RunMigrationsFrom(s *Store, sourceURL string) error` — applies migrations from an explicit `file://` source. `RunMigrations(s)` keeps its signature and behaviour, now implemented on top of it. Task 2 calls `RunMigrationsFrom`.

- [ ] **Step 1: Write the failing test**

Append to `internal/store/migrate_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test ./internal/store/ -run 'TestRunMigrationsFrom' -v
```

Expected: FAIL to compile — `RunMigrationsFrom` is undefined.

- [ ] **Step 3: Implement it**

In `internal/store/migrate.go`, replace the body of `RunMigrations` and add the new function. Keep `migrationPaths`, `driverFor` and the `m.Close()` warning comment exactly as they are:

```go
// RunMigrations applies every pending migration to the store's own connection,
// looking for the migration files in the standard deployment and dev locations.
func RunMigrations(s *Store) error {
	var err error
	for _, path := range migrationPaths {
		if err = RunMigrationsFrom(s, path); err == nil {
			return nil
		}
	}
	return err
}

// RunMigrationsFrom applies every pending migration from an explicit source URL
// (e.g. "file:///abs/path/migrations") to the store's own connection. Callers that
// know where the migrations are — tests in packages nested deeper than the paths
// RunMigrations guesses — should use this directly.
func RunMigrationsFrom(s *Store, sourceURL string) error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	drv, name, err := driverFor(s.Driver, sqlDB)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance(sourceURL, name, drv)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	// Do NOT Close m: every golang-migrate driver's Close() closes the *sql.DB we
	// passed in, which is the application's own pool. (Keep the existing wording of
	// this comment if it is longer — do not shorten it.)
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
```

Note the behavioural detail you must preserve: `RunMigrations` returns the error of the **last** attempted path when all fail, exactly as before.

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/store/... -v && go build ./... && go vet ./...
```

Expected: all PASS, including the Round 23 tests `TestRunMigrationsAppliesToTheOpenConnection` and `TestNewSQLiteApp`.

- [ ] **Step 5: Commit**

```bash
git add internal/store/migrate.go internal/store/migrate_test.go
git commit -m "feat(store): add RunMigrationsFrom for callers that know the source"
```

---

### Task 2: A shared test-database helper that applies the real schema

**Files:**
- Create: `internal/storetest/storetest.go`
- Test: `internal/storetest/storetest_test.go`

**Interfaces:**
- Consumes: `store.New`, `store.RunMigrationsFrom` (Task 1), `config.DBConfig{Driver, DSN}`.
- Produces: `func storetest.New(t *testing.T) *gorm.DB` — a migrated, isolated database for one test, on the driver named by `TEST_DB_DRIVER` (default `sqlite`), cleaned up automatically. Tasks 3 and 4 use exactly this.

This is a normal (non-`_test.go`) package so other packages' tests can import it. It must never be imported by production code.

- [ ] **Step 1: Write the failing test**

Create `internal/storetest/storetest_test.go`:

```go
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

	if err := a.Exec("INSERT INTO projects (id, seq_id, `key`, name) VALUES (?, ?, ?, ?)",
		"p-isolation", 99001, "ISO", "Isolation").Error; err != nil {
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
```

If the backtick quoting around `key` turns out to be wrong for the driver under test, quote it in whatever way that driver accepts, or pick a column that needs no quoting — say so in your report.

- [ ] **Step 2: Run it to verify it fails**

```bash
go test ./internal/storetest/ -v
```

Expected: FAIL to compile — the package does not exist yet.

- [ ] **Step 3: Implement the helper**

Create `internal/storetest/storetest.go`:

```go
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
	"fmt"
	"os"
	"path/filepath"
	"testing"

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
```

Then add `dsnFor` in the same file. It must give **each test its own database**, because tests run in parallel within a package and share nothing:

- **sqlite**: a file inside `t.TempDir()` (not `:memory:` — `store.New` configures a 25-connection pool and every connection to `:memory:` is a separate database). Cleanup is handled by `t.TempDir()`, so return a no-op.
- **postgres**: connect to the server named by `TEST_POSTGRES_DSN` (default `postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable`), `CREATE DATABASE heureum_test_<unique>`, return a DSN pointing at it, and `DROP DATABASE` in the cleanup. Build the unique suffix from something collision-free across parallel tests.
- **mysql**: the same shape against `TEST_MYSQL_DSN` (default `root:root@tcp(127.0.0.1:3306)/`), `CREATE DATABASE` / `DROP DATABASE`. **The DSN you return must keep `multiStatements=true`** — `store.New` normalises it via `ensureMultiStatements`, so returning a plain DSN is fine, but do not bypass `store.New`.

Use `database/sql` with the driver already imported transitively for the admin connection, and fail the test with a clear message naming the env var when the server is unreachable — a developer running `TEST_DB_DRIVER=postgres` without a server should get "set TEST_POSTGRES_DSN or start a server", not a raw connection error.

- [ ] **Step 4: Run the helper's own tests on SQLite**

```bash
go test ./internal/storetest/ -v -count=1
```

Expected: both PASS.

- [ ] **Step 5: Run them against a real Postgres and a real MySQL**

```bash
docker run -d --name r24-pg  -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:16
docker run -d --name r24-my  -e MYSQL_ROOT_PASSWORD=root   -p 3306:3306 mysql:8
# wait for both to accept connections, then:
TEST_DB_DRIVER=postgres go test ./internal/storetest/ -v -count=1
TEST_DB_DRIVER=mysql    go test ./internal/storetest/ -v -count=1
```

Expected: PASS on all three drivers. **This is the first time in this project's history that the migrations have been applied to MySQL by anything automated** — if migration files turn out to contain SQL that MySQL rejects, that is a real finding: record it precisely (which migration, which statement, which error) and report it. Do not edit a migration that has already shipped without saying so prominently.

Port 3306 or 5432 may be taken on the dev machine (an unrelated stack runs there). If so, publish on a different host port and point the `TEST_*_DSN` env var at it; do not stop anyone else's containers. Clean up your own containers when done (`docker rm -f r24-pg r24-my`).

- [ ] **Step 6: Commit**

```bash
git add internal/storetest/
git commit -m "feat(storetest): shared helper applying the real migrations per test"
```

---

### Task 3: Convert the issue domain to the real schema

`internal/domain/issue` holds the code for issues, comments and issue history — three of the four areas that write nullable foreign keys. Its nine test files build their schema with `AutoMigrate` plus stubs.

**Files:**
- Modify: every `*_test.go` in `internal/domain/issue/` (currently `service_test.go`, `comment_service_test.go`, `worklog_service_test.go`, `attachment_service_test.go`, `remotelink_service_test.go`, `resolution_test.go`, `rank_test.go`, `vote_service_test.go`, `export_test.go` — confirm the list with `ls` before starting)

**Interfaces:**
- Consumes: `storetest.New(t)` from Task 2.
- Produces: nothing other tasks depend on.

- [ ] **Step 1: Inventory what the current helpers fabricate**

```bash
grep -n 'AutoMigrate\|CREATE TABLE' internal/domain/issue/*_test.go
```

Write the list into your report. Every `CREATE TABLE` stub you are about to delete is a place where a test has been running against a table that does not match production.

- [ ] **Step 2: Replace the schema setup, one file at a time**

In each file, replace the local `newTestDB`/`newIssueTestDB`-style helper body with `return storetest.New(t)` (import `github.com/it4nodummies/heureum/internal/storetest`), and delete the `AutoMigrate` call and every hand-written `CREATE TABLE`. Keep each helper's name and signature so the test bodies do not change.

Where a test previously relied on a stub table having no constraints — inserting an issue with a `project_id` that does not exist, for instance — it will now fail against the real schema. That failure is the finding this round exists to produce: create the parent row the production code would require, and note in your report which test needed it and why.

- [ ] **Step 3: Run the domain's tests after each file**

```bash
go test ./internal/domain/issue/ -count=1
```

Convert the next file only once the previous one is green. Record every test that needed a change beyond the helper swap, with the reason.

- [ ] **Step 4: Run them on all three drivers**

```bash
go test ./internal/domain/issue/ -count=1
TEST_DB_DRIVER=postgres go test ./internal/domain/issue/ -count=1
TEST_DB_DRIVER=mysql    go test ./internal/domain/issue/ -count=1
```

Expected: green on all three. A failure that appears **only** on Postgres or MySQL is precisely the bug class this round targets — diagnose it, and if it is a defect in production code, fix it and say so prominently in your report.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/issue/
git commit -m "test(issue): run the issue domain against the real migrated schema"
```

---

### Task 4: Write the missing tests for the notification domain

`internal/domain/notification` has **0.0% coverage and no test file at all** — measured, not estimated. It is also where a production-only defect shipped in July: `Notification.CreatedAt` was declared `int64` with `gorm:"autoCreateTime"` while the column is `TIMESTAMP`, so every insert failed on Postgres and succeeded on loosely-typed SQLite. The fix landed; nothing guards it.

**Files:**
- Create: `internal/domain/notification/service_test.go`

**Interfaces:**
- Consumes: `storetest.New(t)` from Task 2; the service constructor and methods in `internal/domain/notification/service.go` — read that file for the exact signatures rather than trusting this list: `Create(userID, notifType, title, body, link string) error`, `CreateBatch([]string, ...) error`, `ListByUser(userID string, unreadOnly bool) ([]Notification, error)`, `MarkRead(id string) error`, `MarkAllRead(userID string) error`, `GetUnreadCount(userID string) (int64, error)`.
- Produces: nothing other tasks depend on.

- [ ] **Step 1: Read the service and the schema**

```bash
sed -n '1,60p' internal/domain/notification/service.go
grep -rn 'notifications' migrations/*.up.sql | head
```

Note the real column types — particularly `created_at` — and which columns are nullable. Write what you found into your report before writing tests.

- [ ] **Step 2: Write the tests**

Cover, at minimum:

1. **`Create` persists and reads back** — including that `CreatedAt` round-trips as a time, not as an integer. This is the regression guard for the July defect; name it so that intent is obvious, e.g. `TestCreateStoresCreatedAtAsATimestamp`.
2. **`ListByUser` scopes to the user** — a notification for user A is not returned for user B.
3. **`ListByUser(unreadOnly=true)`** returns only unread ones.
4. **`GetUnreadCount`** matches the number of unread rows.
5. **`MarkRead`** flips exactly one notification, and **`MarkAllRead`** flips only that user's.
6. **`CreateBatch`** creates one row per user id.

Each test starts from `storetest.New(t)`. Create any parent rows the real foreign keys require — read the migration to see which they are; do not guess, and do not work around a constraint.

- [ ] **Step 3: Run them, on all three drivers**

```bash
go test ./internal/domain/notification/ -v -count=1
TEST_DB_DRIVER=postgres go test ./internal/domain/notification/ -count=1
TEST_DB_DRIVER=mysql    go test ./internal/domain/notification/ -count=1
go test -cover ./internal/domain/notification/
```

Expected: green on all three, and coverage well above 0.0%. Report the number.

- [ ] **Step 4: Commit**

```bash
git add internal/domain/notification/
git commit -m "test(notification): cover the domain that shipped a Postgres-only defect with none"
```

---

### Task 5: Turn the backend CI job into a driver matrix

**Files:**
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `TEST_DB_DRIVER`, `TEST_POSTGRES_DSN`, `TEST_MYSQL_DSN` from Task 2.
- Produces: a `backend` job that runs three times.

- [ ] **Step 1: Read the current workflow**

```bash
sed -n '1,30p' .github/workflows/ci.yml
sed -n '74,110p' .github/workflows/ci.yml
```

The existing `postgres-smoke` job already shows this repository's house style for a service container — follow it rather than inventing your own.

- [ ] **Step 2: Give `backend` a matrix**

Add `strategy.matrix.driver: [sqlite, postgres, mysql]` with `fail-fast: false` (a MySQL failure must not hide a Postgres one), attach `postgres:16` and `mysql:8` service containers with health checks, set `TEST_DB_DRIVER: ${{ matrix.driver }}` plus the two DSN env vars on the `go test` step, and keep `go build` and `go vet` running once per matrix leg (they are cheap).

Services are started for every leg regardless of driver; that is acceptable and simpler than conditioning them. If you prefer to condition them, do not use an expression that silently evaluates to an empty service block.

- [ ] **Step 3: Decide what happens to `postgres-smoke`**

It runs `cmd/seed` against a real Postgres and was added in July as the first Postgres coverage this project ever had. The matrix now covers far more. Judge whether it is still earning its place: it exercises the **seed binary** end to end, which the unit tests do not. **Recommendation: keep it**, and add one line to its comment saying the matrix now covers the domain tests so a future reader knows why both exist. If you decide to remove it, justify that in your report.

- [ ] **Step 4: Validate the workflow file**

```bash
python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); print('YAML ok')"
```

You cannot run GitHub Actions locally; CI on the pull request is the verification, and the round is not done until all matrix legs are green there.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: run the backend suite against sqlite, postgres and mysql"
```

---

### Task 6: Close the loop on PR #44

PR #44 (`github.com/go-sql-driver/mysql` 1.8.1 → 1.10.1) was deliberately left open at the end of Round 23 with this reasoning, recorded in `docs/superpowers/STATE.md`: it bumps the very library that `ensureMultiStatements` depends on, and **no CI job exercised MySQL**, so a green CI said almost nothing about it. This round removes that excuse.

**Files:** none in this repository — this is a verification step plus a STATE.md note.

**Interfaces:**
- Consumes: Task 5's matrix, which must be on `main` before PR #44's CI can exercise it.
- Produces: the round's falsifiable claim, tested.

- [ ] **Step 1: Get this round onto `main` first**

Open a pull request for `feat/round-24-gate-reale`, confirm every matrix leg is green, and merge it. **The merge is an outward-facing action: ask the user and proceed only on an explicit yes.**

- [ ] **Step 2: Rebase PR #44 and read the MySQL leg**

```bash
gh pr comment 44 --body "@dependabot rebase"
# wait for the rebase and the run, then:
gh pr checks 44
```

Expected: a `backend (mysql)` leg exists and is green. That green now means something: the bumped driver's `ParseDSN`/`FormatDSN` were exercised by `ensureMultiStatements`, and the migrations ran against a real MySQL server through it.

- [ ] **Step 3: Record the outcome either way**

Update the `[APERTO]` follow-up in `docs/superpowers/STATE.md` that says MySQL is structurally unverified: mark it `[CHIUSO — verificato <date>: matrice CI backend (mysql), Round 24]` if the leg is green, using the same format as the other verified entries, and keep the section's count line consistent with the file.

If the MySQL leg is **red**, that is the round paying for itself on its first day: do not merge #44, record precisely what failed, and treat it as a new finding.

- [ ] **Step 4: Ask before merging #44**

Merging is the user's call. Present the CI result and stop.

---

## Self-review notes

- **Spec coverage:** the spec's four R24 bullets map to Tasks 2 (shared helper on real migrations), 3 and 4 (the FK-nullable domains — note that `comment` and `issue_history` live inside `internal/domain/issue`, so Task 3 covers three of the four areas, and Task 4 covers the fourth), and 5 (the CI matrix, extended to MySQL per the Round 23 finding). The fifth bullet, per-worker Playwright databases, is explicitly out of scope by the user's decision recorded above; it must be added to the follow-up list in Task 6 Step 3 so it is not silently lost. Task 1 is an enabler discovered while planning: the existing `RunMigrations` cannot find the migrations from a package three levels deep.
- **Measured, not assumed:** 23 migration files; a full apply on in-memory SQLite takes 10–20 ms, so per-test migration is affordable for the ~23 tests in `internal/domain/issue`; the suite has ~380 test functions in total, which is why this round converts two packages rather than all 33 files.
- **The prediction this plan makes:** converting to the real schema will surface at least one test that was passing only because the stub had no constraints. If nothing at all turns red in Task 3, be suspicious that the helper is not actually being used, and verify before reporting success.

---

### Task 7: Make the migrations run on MySQL

Added on 2026-09-24 after the repository owner chose to make MySQL support real rather than withdraw the claim. `README.md:33` advertises "PostgreSQL or MySQL/MariaDB in production" and MySQL has never been able to apply migration 000001. Because it has never succeeded, **no MySQL deployment of Heureum can exist**, so there is no installed base to break; golang-migrate does not checksum migration files, so editing them does not disturb SQLite or Postgres installations that have already applied them.

**Files:**
- Modify: the 14 `migrations/*.up.sql` files listed below
- Test: none new — the proof is that the migrations apply on a real MySQL server

**Interfaces:**
- Consumes: `storetest.New(t)` with `TEST_DB_DRIVER=mysql`, which already exists and is already wired for MySQL.
- Produces: migrations that apply on MySQL 8. Task 8 depends on this landing first.

- [ ] **Step 1: Confirm the inventory before changing anything**

```bash
grep -cE '(TEXT|JSON|BLOB)[[:space:]]+(NOT NULL[[:space:]]+)?DEFAULT[[:space:]]+'"'" migrations/*.up.sql | grep -v ':0'
grep -ohE '(TEXT|JSON|BLOB)[[:space:]]+(NOT NULL[[:space:]]+)?DEFAULT[[:space:]]+'"'"'[^'"'"']*'"'" migrations/*.up.sql | sed -E "s/'[^']*'/'X'/" | sort | uniq -c
```

Expected, and already measured: **69 occurrences across 14 files**, in exactly two syntactic forms — `TEXT DEFAULT 'X'` (49) and `TEXT NOT NULL DEFAULT 'X'` (20). Per file: `000001` 46, `000002` 3, `000005` 3, `000010` 5, `000014` 2, `000021` 2, and one each in `000004`, `000008`, `000011`, `000012`, `000013`, `000015`, `000018`, `000023`. If your counts differ, stop and report — the tree has changed under this plan.

- [ ] **Step 2: Apply the transformation**

Wrap the literal in parentheses: `TEXT DEFAULT '{}'` becomes `TEXT DEFAULT ('{}')`, and `TEXT NOT NULL DEFAULT ''` becomes `TEXT NOT NULL DEFAULT ('')`. Nothing else changes — not the column type, not the value, not `NOT NULL`, not the ordering.

Facts already verified on real containers, so you need not re-derive them: MySQL 8 **rejects** the bare form with Error 1101 and **accepts** the parenthesised form; SQLite and Postgres 16 accept **both** and produce an identical default; and all three accept the parenthesised form inside `ALTER TABLE ... ADD COLUMN` as well as `CREATE TABLE`, with and without `NOT NULL` — which matters because 10 of the 69 are in `ADD COLUMN` statements.

Leave the `.down.sql` files alone: they contain no such defaults (verified).

Do not touch anything else in these files. A migration file that has shipped is not a refactoring opportunity.

- [ ] **Step 3: Prove the migrations now apply on MySQL**

```bash
docker run -d --name r24-mysql -e MYSQL_ROOT_PASSWORD=root -p 3306:3306 mysql:8
# wait for mysqladmin ping to answer, then:
TEST_DB_DRIVER=mysql go test ./internal/storetest/ -v -count=1
```

**Expect this to fail at least once, and treat each failure as the next discovery.** The 69 defaults are the only *known* blocker, and they are known only because they are the first statement MySQL rejects — the first table of the first migration. Fixing them may simply reveal the next incompatibility behind it. Iterate: read the error, identify which migration and which statement, verify the portable form on all three engines before adopting it, and record every additional change in your report with the same evidence. Note that `CREATE TABLE IF NOT EXISTS` (4 occurrences) is accepted by MySQL and needs nothing; `CREATE INDEX IF NOT EXISTS` is not, but R22 already removed that form.

- [ ] **Step 4: Prove nothing regressed on the other two**

```bash
go test ./... -count=1
TEST_DB_DRIVER=postgres go test ./internal/storetest/... ./internal/domain/issue/... ./internal/domain/notification/... -count=1
```

Both must be green. A behavioural change on SQLite or Postgres means the transformation was not value-preserving somewhere — find it, do not paper over it.

- [ ] **Step 5: Commit**

```bash
git add migrations/
git commit -m "fix(migrations): parenthesise text defaults so MySQL can apply the schema"
```

---

### Task 8: Put MySQL behind the gate

Making MySQL work is worth nothing if nothing keeps it working — that is the whole thesis of this round.

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `internal/domain/issue/service_test.go`
- Modify: `internal/storetest/storetest.go`
- Modify: `docs/superpowers/STATE.md`

**Interfaces:**
- Consumes: Task 7's migrations.
- Produces: a third green matrix leg.

- [ ] **Step 1: Make the issue-domain fixtures dialect-portable**

`internal/domain/issue/service_test.go`'s `createTestProject`/`createTestUser` use raw SQL with two MySQL problems in one statement: `ON CONFLICT (id) DO NOTHING` is Postgres/SQLite-only, and `key` is a MySQL reserved word that is unquoted there. This was deferred twice as blocked on the MySQL decision; the decision has landed.

Replace the raw SQL with GORM's dialect-aware API — `db.Table("projects").Create(map[string]any{...})` with `clause.OnConflict{DoNothing: true}` — exactly as `internal/storetest/storetest_test.go` already does. GORM quotes identifiers per dialect and emits the right upsert syntax for each engine.

- [ ] **Step 2: Close the quoting asymmetry in the helper**

`internal/storetest/storetest.go` quotes Postgres identifiers through a proper helper that doubles embedded quotes, but interpolates the MySQL identifier directly into backticks. Safe today because the generated names are alphanumerics and underscores only, but asymmetric and a trap for whoever next changes the name generator. Give MySQL an equivalent quoting helper.

- [ ] **Step 3: Add the MySQL matrix leg**

Add `mysql` to `strategy.matrix.driver`, attach a `mysql:8` service container with a health check in the same style as the existing `postgres:16` one, and set `TEST_MYSQL_DSN` on the `go test` step alongside the existing variables. Read `internal/storetest/storetest.go` for the exact env var name and its default rather than trusting this text. Keep `fail-fast: false`.

Note the detail that makes MySQL slower to start than Postgres: `mysql:8` needs a longer health-check start period before it accepts connections. Give it enough retries that the leg does not fail intermittently on a cold runner — an intermittently red leg is worse than no leg.

- [ ] **Step 4: Verify all three legs locally**

```bash
go test ./... -count=1
TEST_DB_DRIVER=postgres go test ./internal/storetest/... ./internal/domain/issue/... ./internal/domain/notification/... -count=1
TEST_DB_DRIVER=mysql    go test ./internal/storetest/... ./internal/domain/issue/... ./internal/domain/notification/... -count=1
```

All three green. Validate the workflow parses. Remove any container you started; touch none you did not.

- [ ] **Step 5: Tell the truth in STATE.md**

The `[APERTO]` entry recording that MySQL has never worked is now closed. Mark it `[CHIUSO — verificato 2026-09-24: <the commit that fixed it>, matrice CI backend (mysql)]` using the file's existing convention, keep the section's count line consistent, and extend the Round 24 entry to cover Tasks 7 and 8. State plainly what is now true: MySQL applies the schema and is covered by CI. Do **not** claim more than that — no MySQL deployment has ever run this application, so "the migrations apply and the domain tests pass" is the honest scope of the claim.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/ci.yml internal/domain/issue/service_test.go internal/storetest/storetest.go docs/superpowers/STATE.md
git commit -m "ci: add the mysql matrix leg and make test fixtures dialect-portable"
```
