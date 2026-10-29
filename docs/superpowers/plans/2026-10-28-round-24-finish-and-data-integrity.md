# Round 24 — Finish the real-schema gate & fix the data-integrity bugs it exposes — Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to
> implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land R24's real-schema test gate (storetest + sqlite/postgres CI matrix) on `main` on top of
R25–R27, and close the data-integrity defects the audit found that only a real schema catches: invite
`""` into a nullable FK, issue descriptions stored as broken JSON, non-atomic issue-key allocation,
multi-step writes without transactions (incl. a nil deref), sprint completion skipping NULL-status
issues, and a 400-for-500 mapping.

**Architecture:** Go backend only (`internal/`). Tests for the fixes use `internal/storetest` (real
migrations, per-test DB, `TEST_DB_DRIVER=sqlite|postgres`, SQLite with FKs enforced) so they run on both
CI legs. No new routes, no DTO changes, no frontend except one e2e assertion tightened.

**Scope decisions (2026-10-28):** the MySQL work (R24 Tasks 7–8, PR #44) is **deferred**: the
uncommitted migration rewrite is parked on branch `wip/r24-mysql-migrations` (commit 16632a2),
unverified; MySQL is not on the reference team's path (Postgres). The CI matrix stays `[sqlite,
postgres]`.

**Spec:** the audit findings recorded in `docs/superpowers/STATE.md` (R27 follow-ups, "dati") and the
R24 plan `docs/superpowers/plans/2026-09-23-round-24-gate-reale.md` (Tasks 1–5 delivered on this branch).

## Global Constraints

- Branch `feat/round-24-finish` (worktree `/Users/n0r41n/Development/heureum-r24`) = R24 commits +
  merge of `origin/main` (7b388ca). Conventional Commits; **no `Co-Authored-By`**.
- Every new/changed test for a data-integrity fix MUST use `storetest.New(t)` (real schema), not
  `sqlite.Open(":memory:")`+`AutoMigrate`, so the postgres CI leg exercises it. Red first.
- Nullable FK/string columns are pointers; never write `""` or zero `time.Time{}` (anti-pattern #2/#3).
- Three-level gate + `go run ./cmd/gapreport` no-diff (no routes). Locally the suite runs on SQLite
  (`TEST_DB_DRIVER` unset); the Postgres leg is CI (`TEST_POSTGRES_DSN`, `TZ=Europe/Rome`). If a local
  Postgres is available (`TEST_POSTGRES_DSN`), run the touched packages with `TEST_DB_DRIVER=postgres` too.

## Review Focus

- The invite test is RED on the merged tree (FK violation now enforced on SQLite too) — Task 2.
- A description typed in the create modal renders as rich text, not raw JSON — Task 3 (+ e2e exact text).
- Two concurrent creates in one project never produce the same key — Task 4.
- A failure after the issue row is created leaves no half-initialized issue and never panics — Task 5.
- Completing a sprint moves/keeps every open issue, including one whose `status_id` is NULL — Task 5.

---

### Task 1: Make the merged tree green on both drivers

**Files:**
- Modify: `internal/domain/issue/resolution_test.go` (R25 file; uses `sqlite.Open` with no import → convert to `storetest.New(t)` like the other issue tests on this branch)
- Modify: `internal/domain/notification/service_test.go:177` (R27 changed `MarkRead` to `(userID, id)`; fix the call and assert ownership semantics)
- Verify: `.github/workflows/ci.yml` matrix `[sqlite, postgres]` intact after the merge; `internal/storetest` applies migrations `000024`/`000025` (added by R27) without error.

- [ ] **Step 1:** `go vet ./...` shows exactly the two errors above; fix them.
- [ ] **Step 2:** `go build ./... && go vet ./... && go test ./... -count=1` green on SQLite; if `TEST_POSTGRES_DSN` is set, also `TEST_DB_DRIVER=postgres go test ./internal/domain/issue/... ./internal/domain/notification/... ./internal/storetest/...`.
- [ ] **Step 3: Commit** `test: adapt R25/R27 tests to the real-schema helper after merging main`

### Task 2: Project invites — `AcceptedBy` must be NULL, not `""`

**Files:**
- Modify: `internal/domain/project/invite.go` (~:19 `AcceptedBy string` → `*string`; ~:29-37 create path; also stop ignoring `rand.Read`'s error)
- Modify: callers/readers of `AcceptedBy` (grep), v3/handler mapping if it serializes it
- Test: `internal/domain/project/invite_test.go` → convert to `storetest.New(t)`

- [ ] **Step 1: Failing test** `TestCreateInvite_PersistsOnRealSchema` (storetest): create an invite → no error, row readable, `accepted_by IS NULL`. Expected RED on the merged tree (FK violation `accepted_by → users(id)` with `""`).
- [ ] **Step 2:** fix (`*string`, nil until accepted; `Accept` sets the pointer); `TestAcceptInvite_SetsAcceptedBy`.
- [ ] **Step 3:** gate green. **Commit** `fix(project): invite accepted_by is NULL until accepted (FK violation on real schema)`

### Task 3: Issue description at creation is stored as broken JSON

**Files:**
- Modify: `internal/domain/issue/service.go` (~:63 `fmt.Sprintf(`{"content":"%s"}`, description)` wraps already-marshaled ADF without escaping → store the ADF JSON as-is when valid JSON, else wrap via `json.Marshal`)
- Reference: `internal/api/handlers/issue_handler.go:240-243` (marshals ADF), `internal/api/v3/issue.go:105` (`descriptionADF` fallback `FromText(raw)`)
- Test: `internal/domain/issue/service_test.go` (storetest) + `frontend-next/e2e/issue-detail.spec.ts:265` (tighten `getByText` substring to the exact rendered paragraph)

- [ ] **Step 1: Failing test** `TestCreate_StoresValidADFDescription`: create with an ADF doc → stored `description` is valid JSON equal to the input doc; and the v3 mapping renders the paragraph text (no raw `{"content":...` string). Expected RED.
- [ ] **Step 2:** fix; keep plain-text descriptions working (`FromText`).
- [ ] **Step 3:** gate green incl. `npx playwright test e2e/issue-detail.spec.ts`. **Commit** `fix(issue): store create-time ADF description as valid JSON`

### Task 4: Atomic issue-key allocation

**Files:**
- Modify: `internal/domain/issue/service.go` (~:46-92: next key = "latest created_at row's key + 1" with the error ignored and a `-1` fallback; seq id = `MAX+1`; neither in a transaction)
- Create: migration `migrations/000026_project_issue_counter.up/down.sql` — `projects.issue_counter INTEGER NOT NULL DEFAULT 0` (no `IF NOT EXISTS`); backfill `UPDATE projects SET issue_counter = (SELECT COALESCE(MAX(seq_key),0) …)` in the migration if a numeric key column exists, else in Go at startup (document the choice)
- Test: `internal/domain/issue/service_test.go` (storetest): `TestCreate_AssignsSequentialKeysAtomically` — N goroutines create in the same project → N distinct keys, counter == N; `TestCreate_PropagatesCounterError`.

- [ ] **Step 1: Failing test** (concurrency on SQLite is single-writer, so also assert the counter column exists and increments; the race proper is pinned by the transactional `UPDATE … RETURNING`/`SELECT … FOR UPDATE` path on Postgres in CI).
- [ ] **Step 2:** implement inside ONE transaction: increment counter (`UPDATE projects SET issue_counter = issue_counter + 1 WHERE id = ? RETURNING issue_counter` on Postgres; on SQLite `UPDATE` then `SELECT` inside the same tx) → key `KEY-N`; propagate errors (no `-1` fallback); keep `seq_id` allocation in the same tx.
- [ ] **Step 3:** gate green. **Commit** `fix(issue): allocate issue keys atomically from a per-project counter`

### Task 5: Transactions around multi-step writes; sprint completion with NULL status; error mapping

**Files:**
- Modify: `internal/api/handlers/issue_handler.go` (~:269-302 create → status/assignee/labels/fixVersions all `_ =`; ~:278 `iss, _ = h.svc.Update(...)` can nil-deref) → do the follow-up writes in the domain inside the create transaction, or at least check errors and roll back/return 500; never dereference a nil `iss`.
- Modify: `internal/domain/sprint/service.go` (~:116-124 `Complete` ignores `Exec` errors; ~:150,152 `status_id NOT IN (done)` skips NULL → `(status_id IS NULL OR status_id NOT IN (...))`; wrap in a transaction)
- Modify: `internal/domain/workflow/service.go` `RemoveStatus` (~:74-76): refuse while issues reference the status (count > 0 → sentinel → handler 409 with a clear message)
- Modify: `internal/api/handlers/project_handler.go:148` (`_ = h.svc.Delete(p.Key)` rollback → log the error)
- Modify: `internal/api/handlers/agile_sprint_handler.go` (~:175-189 `CompleteSprint` maps real DB errors to 400 → 500 like `writeSprintErr`)
- Tests (storetest): `TestCreateIssue_RollsBackOnStatusAssignFailure` (inject failure → no issue row, 500, no panic), `TestComplete_MovesNullStatusIssues`, `TestRemoveStatus_RefusedWhileInUse`, `TestCompleteSprint_DBErrorIs500`.

- [ ] **Step 1:** failing tests (each RED for the right reason — the NULL-status one and RemoveStatus-in-use are RED on current code).
- [ ] **Step 2:** implement. **Step 3:** gate green. **Commit** `fix(issue,sprint,workflow): transactional multi-step writes, NULL-status sprint completion, status-in-use guard, 500 for DB errors`

### Task 6: Postgres-only assertions that need the real driver

**Files:**
- Modify: `internal/api/handlers/auth_handler_test.go` `TestCreateAPIToken_ExpiryIsUTC` (R27) → add a `storetest`-based round-trip: store an API token with expiry via the real schema and read it back; on the postgres leg (`TZ=Europe/Rome` in CI) assert the instant is unchanged (pins the `discardTimeZone` class the R24 notification test already pins).
- Modify: `docs/superpowers/STATE.md`: R24 entry → CHIUSO (Tasks 1–5 + this round), MySQL/PR #44 deferred with the `wip/r24-mysql-migrations` pointer; close the "merge R24" follow-up and the data-integrity follow-ups fixed here; keep the MySQL `RowsAffected`/index notes.

- [ ] **Step 1:** test + docs. **Step 2:** gate. **Commit** `test(auth): pin API-token expiry round-trip on the real schema; docs: close R24`

---

## Final

- [ ] Full gate (backend; frontend build + full Playwright; gapreport no-diff). CI must be green on BOTH matrix legs before merge.
- [ ] Whole-branch review (fresh reviewer): every fix has a storetest-based test that was red; no `""`/zero-time writes; transactions really roll back (inject failures).
- [ ] PR → merge → Forgejo sync → CD (migration `000026` runs on the homelab Postgres at startup — confirm the counter backfill matches `MAX(key number)` per project before declaring done).
