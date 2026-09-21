# Round 23 — Riapertura cantiere (e2e workers, migrate instance, dependabot, v1.1.1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clear two months of accumulated debt so the next rounds start from a clean base: unblock PR #24 by fixing the real cause of its red e2e job, unblock the Go dependabot backlog by fixing a latent migration bug, merge the 11 dependency PRs, ship v1.1.1, and make the documentation honest (stability promise, backup procedure, follow-up list).

**Architecture:** Two genuine code defects were diagnosed before this plan was written, and neither is in PR #24. (1) `playwright.config.ts` sets no `workers`, so CI runs Playwright's default — **2 workers on a 4-vCPU runner** — against a *single shared SQLite backend*, which the repo documents everywhere as requiring `--workers=1`. PR #24 adds one spec file (75 tests instead of 74), which redistributed files across those two workers and tipped the documented write-contention race into a failure. (2) `store.RunMigrations` builds a database URL by string concatenation (`"sqlite3://" + DSN`), which produces the malformed URL `sqlite3://file::memory:?cache=shared`; Go 1.26.0's stricter `net/url` rejects `::memory:` as an invalid port, so every dependabot PR that raises the `go` directive fails the backend job. Both are fixed at the root: pin one worker, and run migrations against the already-open `*sql.DB` via each driver's `WithInstance` instead of parsing a URL.

**Tech Stack:** Go 1.25 (`golang-migrate/migrate/v4` v4.19.1, GORM, in-memory SQLite tests), Playwright, GitHub Actions. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-09-21-direzione-prodotto-design.md` (section "R23 — Riapertura cantiere"). Decisions this plan is downstream of: `docs/adr/0001-superficie-jira-compat-congelata.md`, `docs/adr/0003-promessa-di-stabilita-selettiva.md`.

## Global Constraints

- **Module path `github.com/it4nodummies/heureum`** — verbatim in imports.
- **No new dependency.** Every driver needed (`sqlite3`, `postgres`, `mysql`) is already imported in `internal/store/migrate.go`.
- **Do not touch `/rest/api/3` or `/rest/agile/1.0` route shapes.** ADR 0001 freezes that surface; this round adds no route.
- **Do not bump the `go` directive in `go.mod` yourself.** Task 2 must make the migration code correct *at the current* `go 1.25.0`; the directive moves only when a dependabot PR that raises it is merged in Task 6.
- **Conventional Commits.** Branch: `fix/round-23-riapertura-cantiere` off `main`.
- **Three-level gate before the round is done:** (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test`; (3) `go run ./cmd/gapreport` — no route changes are expected, so it must produce no diff.
- **External actions (Task 6, Task 7) require explicit user approval each time.** Merging a PR, closing a PR and pushing a tag are irreversible and outward-facing: ask, do not assume, and never push a tag yourself.

---

### Task 1: Pin Playwright to a single worker

The e2e suite shares one SQLite-backed server across all specs (`scripts/e2e-backend.sh`). Running spec files in parallel against it is the write-contention flakiness recorded in STATE.md since R13. Every doc in the repo already assumes `--workers=1` — including the comment inside `scripts/e2e-backend.sh` — but nothing enforces it.

**Files:**
- Modify: `frontend-next/playwright.config.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: a deterministic single-worker e2e run. Task 6 relies on this to re-run PR #24's CI green.

- [ ] **Step 1: Observe the current (wrong) worker count**

```bash
cd frontend-next && npx playwright test 2>&1 | head -3
```

Expected: a line reading `Running 74 tests using N workers` with **N greater than 1** on any multi-core machine. This is the defect: N > 1 against one shared database. (CI evidence: run 29730464667 on `main` logged `Running 74 tests using 2 workers`; run 29736654784 on PR #24 logged `Running 75 tests using 2 workers` and failed one test.)

- [ ] **Step 2: Pin the worker count**

In `frontend-next/playwright.config.ts`, add `workers: 1` immediately after `retries`:

```typescript
export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  retries: process.env.CI ? 2 : 0,
  // Every spec runs against ONE shared backend with ONE SQLite file
  // (scripts/e2e-backend.sh). Playwright's default is half the CPU count, which
  // puts two spec files in write contention on that single database — the
  // flakiness recorded in STATE.md since R13, and the real cause of PR #24's red
  // e2e job (adding one spec file redistributed the load across two workers).
  // Round 24 gives each worker its own database and earns the parallelism back;
  // until then this must stay 1.
  workers: 1,
  use: {
```

- [ ] **Step 3: Verify the worker count is now 1 and the suite is green**

```bash
cd frontend-next && npx playwright test 2>&1 | tail -20
```

Expected: the header reads `Running 74 tests using 1 worker`, and the run ends with all tests passed. Note the run takes roughly twice as long as before (~5 minutes instead of ~2.5); the CI job's `timeout-minutes: 15` accommodates this.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/playwright.config.ts
git commit -m "fix(e2e): pin Playwright to one worker against the shared SQLite backend"
```

---

### Task 2: Run migrations against the open database instead of a hand-built URL

`RunMigrations` currently ignores the connection the caller already opened and reconstructs a URL from the DSN. That is wrong twice over: the URL is malformed for any DSN that is not a bare path (`sqlite3://file::memory:?cache=shared` parses `file` as host and `:memory:` as an invalid port, which Go 1.26.0 rejects outright), and even when it parses, it opens a *second, different* database — so with a plain `:memory:` DSN the migrations run somewhere the application never sees.

All three callers already call `store.New` before `RunMigrations`, so passing the open store is a mechanical change.

**Files:**
- Modify: `internal/store/migrate.go`
- Modify: `internal/app/app.go:18`
- Modify: `cmd/server/main.go:26`
- Modify: `cmd/seed/main.go:42`
- Test: `internal/store/migrate_test.go` (create)

**Interfaces:**
- Consumes: `store.New(cfg config.DBConfig, env string) (*Store, error)` — already returns `*Store{DB *gorm.DB, Driver string}`.
- Produces: `func RunMigrations(s *Store) error` — replaces `RunMigrations(cfg config.DBConfig) error`. Task 6 relies on this compiling under a `go` directive of 1.26.0.

- [ ] **Step 1: Write the failing test**

Create `internal/store/migrate_test.go`:

```go
package store

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/config"
)

// A plain ":memory:" DSN is the sharpest form of the bug: the old URL-based
// RunMigrations opened its own second connection, so migrations landed in a
// different in-memory database and the store's own connection stayed empty.
func TestRunMigrationsAppliesToTheOpenConnection(t *testing.T) {
	cfg := config.DBConfig{Driver: "sqlite", DSN: ":memory:"}
	s, err := New(cfg, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	if err := RunMigrations(s); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	// The migrations create `projects` (migration 000001). If migrations ran
	// against a different connection, this table does not exist here.
	if !s.DB.Migrator().HasTable("projects") {
		t.Error("projects table missing: migrations did not apply to the store's own connection")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/store/... -run TestRunMigrationsAppliesToTheOpenConnection -v
```

Expected: FAIL. It will not even compile at first (`RunMigrations` still takes `config.DBConfig`); after you adjust the call it fails on the missing `projects` table, or on the malformed-URL error. Either failure is the red you want — record which one you saw.

- [ ] **Step 3: Rewrite `internal/store/migrate.go`**

```go
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
// any DSN that is not a bare path — "sqlite3://file::memory:?cache=shared"
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
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
```

Note the import change: the three database drivers move from blank imports (`_ "..."`) to named imports, because `WithInstance` is called directly. The `source/file` driver stays a blank import — it is still resolved by URL scheme.

- [ ] **Step 4: Update the three call sites**

`internal/app/app.go` line 18 — `store.RunMigrations(cfg.DB)` becomes `store.RunMigrations(s)`:

```go
	s, err := store.New(cfg.DB, cfg.Env)
	if err != nil {
		return nil, err
	}
	if err := store.RunMigrations(s); err != nil {
		return nil, err
	}
```

`cmd/server/main.go` line 26 — same substitution, the local variable is also called `s`:

```go
	if err := store.RunMigrations(s); err != nil {
		logger.Error("failed to run migrations", "error", err)
		log.Fatal(err)
	}
```

`cmd/seed/main.go` line 42 — check the local variable's name first with `sed -n '35,45p' cmd/seed/main.go` and pass that store value; it is `s` there too, so:

```go
	if err := store.RunMigrations(s); err != nil {
```

- [ ] **Step 5: Run the new test and the whole backend suite**

```bash
go build ./... && go vet ./... && go test ./internal/store/... -v
go test ./...
```

Expected: the new test PASSES, `TestNewSQLiteApp` still passes, and the full suite is green.

- [ ] **Step 6: Verify the fix holds under the Go directive that broke CI**

```bash
S=$(mktemp -d)
sed 's/^go 1\.25\.0$/go 1.26.0/' go.mod > "$S/go126.mod"
cp go.sum "$S/go126.sum"
go test -modfile="$S/go126.mod" ./internal/app/... ./internal/store/...
```

Expected: PASS. This is the configuration in which dependabot PR #41 failed CI (`setup-go` installs exactly the version in the `go` directive, and that PR raised it to 1.26.0). Do **not** commit the modified go.mod — it is a scratch file.

- [ ] **Step 7: Commit**

```bash
git add internal/store/migrate.go internal/store/migrate_test.go internal/app/app.go cmd/server/main.go cmd/seed/main.go
git commit -m "fix(store): run migrations on the open connection instead of a hand-built URL"
```

---

### Task 3: README stability promise and .gitignore for local uploads

ADR 0003 has no effect until the README states the promise. And `data/uploads/` — 140 real attachment files produced by local runs — is untracked but not ignored: one `git add -A` commits user attachments.

**Files:**
- Modify: `README.md` (section "API compatibility & gap report", line 160)
- Modify: `.gitignore`

**Interfaces:**
- Consumes: the four stable areas named in `docs/adr/0003-promessa-di-stabilita-selettiva.md`.
- Produces: nothing other tasks depend on.

- [ ] **Step 1: Confirm the uploads directory is untracked and unignored**

```bash
git status --short | grep data/ ; git check-ignore -v data/uploads || echo "NOT ignored"
```

Expected: `?? data/` in the status output, and `NOT ignored`.

- [ ] **Step 2: Add the ignore rule**

Append to `.gitignore`, after the `/tmp/` line:

```gitignore
# Local attachment/avatar storage (APP_UPLOADS_DIR default). In Docker this is
# a named volume; locally it is real user data and must never be committed.
/data/
```

- [ ] **Step 3: Verify the rule works**

```bash
git check-ignore -v data/uploads && git status --short | grep -c '^?? data/'
```

Expected: `check-ignore` prints the matching rule, and the count of `?? data/` lines is `0`.

- [ ] **Step 4: Add the stability promise to the README**

In `README.md`, inside the "API compatibility & gap report" section, append this after the existing text:

```markdown
### What is stable

Heureum implements a subset of the Jira Cloud v3 surface deliberately, as a migration bridge
rather than a full re-implementation — see
[ADR 0001](docs/adr/0001-superficie-jira-compat-congelata.md). Within that subset, four areas are
**stable within a major version**, because they are what a migration from Jira actually goes
through:

1. **Issues** — CRUD, fields, transitions, comments, worklogs, links, watchers
2. **Projects** — CRUD, search, categories
3. **Search / JQL** — `/search/jql` and the language the parser accepts
4. **Agile** — boards, sprints, backlog (`/rest/agile/1.0/*`)

An incompatible change in those four areas — removing a route, changing a response shape,
narrowing a field — requires a major version and an announced deprecation period. Everything else
in the surface is best-effort: consult the gap report above for what exists today, and expect it
to change in a minor release. See
[ADR 0003](docs/adr/0003-promessa-di-stabilita-selettiva.md) for the reasoning.
```

- [ ] **Step 5: Commit**

```bash
git add README.md .gitignore
git commit -m "docs(readme): declare the selective stability promise; ignore local uploads"
```

---

### Task 4: Backup and restore, documented and proven

The spec's "minimo operativo" is the one prerequisite before real data goes into an instance. Nothing about backup exists in the repo today (`grep -rliE 'backup|restore|pg_dump' README.md docs/ deploy/` returns no operational document). Two things must be backed up: the Postgres database and the `uploads` volume.

**Files:**
- Create: `docs/OPERATIONS.md`
- Modify: `README.md` (add a link to it from the "Docker" section)

**Interfaces:**
- Consumes: the service and volume names in `deploy/docker/docker-compose.yml` (`db`, `pgdata`, `uploads`).
- Produces: nothing other tasks depend on.

- [ ] **Step 1: Read the compose file to get the exact service and volume names**

```bash
grep -nE '^\s{2}[a-z-]+:|volumes:|image:|POSTGRES_|container_name' deploy/docker/docker-compose.yml | head -30
```

Record the database service name, the database user and name, and the two volume names. Use those exact values in the next step — do not guess them.

- [ ] **Step 2: Write `docs/OPERATIONS.md`**

Write the document with these sections, filling the placeholders with the values you just read:

- **What to back up** — the Postgres database and the `uploads` volume. Both, or a restore produces issues whose attachments 404.
- **Backup** — a `docker compose exec` + `pg_dump` command writing a timestamped dump, and a `docker run --rm -v <uploads volume>:/data -v $(pwd):/backup alpine tar czf ...` for the volume.
- **Restore** — bring the stack down, restore the volume, `psql` the dump into a fresh database, bring it up. State explicitly that restoring into a non-empty database is not supported.
- **Verify a backup** — restore into a throwaway stack and check that the project list and one issue's attachment both load. A backup that has never been restored is not a backup.
- **What is NOT backed up** — nothing else holds state; the app is stateless besides those two.

- [ ] **Step 3: Prove the procedure end-to-end**

Run the backup commands against a local Docker stack, then restore into a fresh one and confirm the demo project and an attachment survive:

```bash
docker compose -f deploy/docker/docker-compose.yml up -d
# ... run the documented backup commands ...
docker compose -f deploy/docker/docker-compose.yml down -v
# ... run the documented restore commands ...
docker compose -f deploy/docker/docker-compose.yml up -d
curl -s -u admin@example.com:admin-demo-123 http://localhost:8080/rest/api/3/project | head -c 300
```

Expected: the project list comes back with the demo project. **If a documented command fails, fix the document, not the terminal history** — the point of this task is that the commands in the file are the ones that were actually run. Note in the document that Docker Desktop in this environment tends to stop on its own (`open -a Docker` to restart it).

- [ ] **Step 4: Link it from the README**

In the "Docker" section of `README.md`, add one line:

```markdown
For backup, restore and day-2 operations, see [docs/OPERATIONS.md](docs/OPERATIONS.md).
```

- [ ] **Step 5: Commit**

```bash
git add docs/OPERATIONS.md README.md
git commit -m "docs(ops): add a verified backup and restore procedure"
```

---

### Task 5: Verify the follow-up list in STATE.md

The list has roughly 35 entries accumulated across rounds and is partly fiction: the entry claiming "no ownership check on `GET/PUT/DELETE /filter/{id}`, any authenticated user can modify them" is **already closed** — `FilterHandler.requireOwner` returns 403 to a non-owner non-admin, and `Get` checks owner/shared/admin. Consuming that list to plan work, or handing the product to colleagues without knowing which known defects are real, is what this task prevents.

**Files:**
- Modify: `docs/superpowers/STATE.md` (the "Follow-up aperti (non bloccanti)" section)

**Interfaces:**
- Consumes: nothing.
- Produces: a trustworthy defect list for R24/R25 planning.

- [ ] **Step 1: Extract the list**

```bash
sed -n "$(grep -n '^## Follow-up aperti' docs/superpowers/STATE.md | cut -d: -f1),\$p" docs/superpowers/STATE.md | grep -n '^- ' | cut -c1-160
```

- [ ] **Step 2: Verify each entry against the code**

For every entry, find the symbol it names and read it. Do not trust the entry's own description — three of the four already sampled were accurate and one was stale. Known results, do not re-verify these four:

- Filter ownership → **already closed** (`FilterHandler.requireOwner`, 403 on non-owner non-admin).
- `WorklogService.Delete` does not decrement `TimeSpent` → **still open**; the method is a single `s.db.Delete(...)` line.
- `WorkflowHandler.ListStatuses` dead code → **still open**; the handler exists, the router does not reference it.
- History logs unchanged fields → **still open**; the guard is `title != nil`, not `*title != issue.Title`, and the frontend form resends every field on every save.

- [ ] **Step 3: Rewrite the section with a verdict per entry**

Replace each bullet with one of three prefixes, keeping the original text so the history stays readable:

```markdown
- **[APERTO — verificato 2026-09-21]** <testo originale>
- **[CHIUSO — verificato 2026-09-21: <symbol that closes it>]** ~~<testo originale>~~
- **[ARCHIVIATO — 2026-09-21: <one-line reason>]** ~~<testo originale>~~
```

Use ARCHIVIATO only for entries that are no longer worth doing under the direction in the spec — for example follow-ups that propose growing the `/rest/api/3` surface, which ADR 0001 now forbids. Say so in the reason.

- [ ] **Step 4: Add a count at the top of the section**

```markdown
> Verificato integralmente il 2026-09-21 (Round 23): N aperti, M chiusi, K archiviati su T voci.
> Una voce senza verdetto è una voce mai verificata: aggiungerne di nuove con il prefisso [APERTO].
```

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/STATE.md
git commit -m "docs(state): verify every follow-up entry against the code"
```

---

### Task 6: Merge PR #24 and the dependency backlog

**Every step in this task is an outward-facing, irreversible action. Ask the user before each group and proceed only on an explicit yes.** Do not merge anything whose CI is not green.

**Files:** none in this repository — these are GitHub operations.

**Interfaces:**
- Consumes: Task 1 (single worker) and Task 2 (migrations under a 1.26 `go` directive), both of which must be on `main` first.
- Produces: a `main` with PR #24 and the 11 dependency PRs merged.

- [ ] **Step 1: Get this round's branch onto `main` first**

Open a PR for `fix/round-23-riapertura-cantiere`, wait for all five CI jobs to be green, and merge it. PR #24 and the dependabot PRs all need these fixes underneath them.

- [ ] **Step 2: Re-run PR #24's CI on top of the fixes**

```bash
gh pr comment 24 --body "Rebasing onto main: the red e2e job was caused by Playwright running 2 workers against one shared SQLite backend (fixed in R23), not by this branch."
gh pr merge 24 --help   # do NOT merge yet — first update the branch
```

Update the branch from `main` through the GitHub UI or `gh pr update-branch 24`, then wait and check:

```bash
gh pr checks 24
```

Expected: all five jobs green. **If `e2e` is still red, stop and diagnose — do not merge.** The diagnosis in this plan predicts it will pass; if it does not, the prediction was wrong and the cause is elsewhere.

- [ ] **Step 3: Ask the user, then merge PR #24**

- [ ] **Step 4: Lot A — GitHub Actions bumps (low risk)**

PRs #25 (`actions/setup-go` 5→7) and #26 (`docker/setup-buildx-action` 3→4). Check each with `gh pr checks <n>`, ask, merge.

- [ ] **Step 5: Lot B — Go modules**

PRs #34 (`gorm.io/driver/postgres` 1.6.0→1.6.2), #38 (`kin-openapi` 0.142.0→0.149.0), #39 (`x/oauth2` 0.36.0→0.37.0), #41 (`x/crypto` 0.54.0→0.57.0). **#41 also raises the `go` directive to 1.26.0** — this is the PR whose backend job failed on `TestNewSQLiteApp`; Task 2 is what makes it pass. Re-run its checks first and confirm the backend job is green before touching the others. Note that #38 bumps the OpenAPI validation library used by every contract test: read its release notes for validation behaviour changes if `internal/contract` goes red.

- [ ] **Step 6: Lot C — frontend, non-major**

PRs #27 (`tailwindcss` 4.3.2→4.3.3), #29 (`react` + `@types/react`), #31 (`react-dom` 19.2.7→19.2.8), #32 (`@tailwindcss/postcss` 4.3.0→4.3.3). React and react-dom must go in together or the versions skew — if #29 and #31 land separately, run `npm run build` between them and fix the lockfile.

- [ ] **Step 7: Lot D — `@dnd-kit/sortable` 8→10, alone and last**

PR #30 is a two-major jump in the library behind the board and backlog drag & drop — the most timing-sensitive code in the repository, and the subject of two dnd-kit bugs already documented in `docs/superpowers/plans/2026-07-16-backlog-sprint-dnd.md`. Read that plan before reviewing this bump. Run the full e2e suite locally against the branch, not just CI, and pay attention to `board.spec.ts` and `backlog-sprint.spec.ts`. If it needs code changes, that is a separate branch and a separate PR — do not fix it inside a dependabot branch.

---

### Task 7: Release v1.1.1

**The tag push is the user's action.** Prepare everything, then hand it over.

**Files:**
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: everything merged in Task 6.
- Produces: a release-ready `main`.

- [ ] **Step 1: Move `[Unreleased]` to `[1.1.1]`**

The section already holds three user-facing Postgres fixes (unassigned-issue save returning 500, notification inserts failing, the `postgres-smoke` job) plus the pencil-edit UX change. Add this round's entries: the single-worker e2e fix, the migration-on-open-connection fix, the ignored uploads directory, the stability promise, and the operations document. Follow the existing Keep a Changelog structure and date it with the day you do it.

- [ ] **Step 2: Verify the full gate on `main`**

```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test
cd .. && go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md
```

Expected: everything green, and **no diff** in the gap report.

- [ ] **Step 3: Commit and open the PR**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): finalize 1.1.1"
```

- [ ] **Step 4: Hand the tag to the user**

Tell the user, in one message: `main` is release-ready, the tag to push is `v1.1.1`, pushing it triggers `release.yml` which builds and publishes the GHCR images, and the procedure is in `docs/RELEASE.md`. Then stop. Do not create or push the tag.

---

## Self-review notes

- **Spec coverage:** every bullet of the spec's "R23 — Riapertura cantiere" maps to a task — PR #24 diagnosis and merge (Tasks 1, 6), dependabot in lots with dnd-kit last (Task 6), v1.1.1 (Task 7), `.gitignore` (Task 3), backup/restore (Task 4), follow-up verification (Task 5), README stability promise (Task 3). Task 2 is additional: it was discovered while diagnosing and it blocks the dependabot backlog, so the round cannot complete without it.
- **Known stale artifact:** `.claude/worktrees/wizardly-mendel-195094/` holds an old copy of the tree, which makes repo-wide greps return duplicate hits. Exclude it when searching; leave it alone otherwise.
- **The prediction this plan makes:** PR #24's e2e job goes green once `workers: 1` is on `main`. Task 6 Step 2 is the point where that prediction is tested. If it fails there, stop and re-diagnose rather than working around it.
