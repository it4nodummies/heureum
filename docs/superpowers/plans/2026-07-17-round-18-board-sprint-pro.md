# Round 18 — Board & Sprint pro (sprint goal/dates, board columns/swimlanes/quick filters) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Give sprints a full create/edit form (goal + start/end dates) and a complete-sprint dialog that moves incomplete issues to the backlog OR the next sprint; and make the board configurable — persisted columns that map a set of statuses, swimlanes (by assignee/epic/none), and quick-filter chips — with the board rendering that config instead of a fixed 1:1-status layout.

**Architecture:** Sprints are nearly ready server-side (model has Goal/StartDate/EndDate; the seq-keyed agile endpoint `POST /rest/agile/1.0/sprint/{sprintId}` already accepts name/goal/state/dates). Board config is greenfield: a new migration + board-config domain persists columns (name + ordered status-id set + position), a swimlane mode, and quick filters (name + JQL). Persisted columns are emitted through the EXISTING contract-shaped `GET /board/{boardId}/configuration` (its `columnConfig.columns[].statuses[]` already models a multi-status column), defaulting to 1:1 workflow statuses when unconfigured — so existing boards keep working. Swimlanes and quick filters live on a Heureum-custom config endpoint (never extend the contract-validated `BoardConfig` — same reasoning as R16's `sharePermissions`).

**Tech Stack:** Go 1.25 (`net/http` ServeMux, GORM, golang-migrate, in-memory SQLite tests), Next.js 16 App Router + React 19 + TanStack Query + Tailwind + @dnd-kit, Playwright.

## Global Constraints

- **Module path `github.com/it4nodummies/heureum`** — verbatim in imports.
- **Use seq-keyed sprint routes, NOT the UUID-keyed api/3 family.** The `PATCH /rest/api/3/project/{key}/sprints/{id}` route keys `{id}` on the sprint **UUID** (never exposed) and only updates name+goal — do NOT build on it. Use `POST /rest/agile/1.0/sprint/{sprintId}` (seq-keyed, accepts name/goal/state/startDate/endDate → `UpdateFull`).
- **Do not break the Agile contract.** `v3.BoardConfig` is validated by `internal/contract` against the official agile 1.0 spec. Persisted columns go into the existing `columnConfig.columns[].statuses[]` (already multi-status, contract-safe). Swimlane mode + quick filters go on a SEPARATE Heureum-custom endpoint — never add fields to `v3.BoardConfig`. After any backend change run `go run ./cmd/gapreport`: a NEW custom route legitimately appears as an extension (commit the regenerated report); the contract-validated routes must still pass `go test ./internal/contract/...`.
- **Permission gating:** board-config writes require `permission.AdministerProjects` (or `ManageSprints` if you prefer parity with sprint mgmt — pick AdministerProjects); sprint mutations `ManageSprints`; reads `BrowseProjects`. Use seq-id resolvers (`ByBoardSeq`, `BySprintSeq`) or `ByKey`.
- **Backward compatibility:** a board with no persisted column config must render exactly as today (1:1 workflow statuses). The default swimlane is "none"; default quick filters empty.
- UI accent `#0052cc`; UI under `/app`; single typed client `frontend-next/lib/api.ts`; board drag uses `@dnd-kit` (see existing `BoardColumns.tsx`).
- Three-level gate before done: (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test --workers=1`; (3) `go run ./cmd/gapreport` → commit if a new extension route changed it, and it must be clean on a second run.
- Conventional Commits; branch `feat/frontend-next`. E2E inline the login preamble from `e2e/export.spec.ts`; full suite `--workers=1`. Seed board id `1` = DEMO.
- **Next migration number: `000018`.**

---

### Task 1: Sprint complete → move incomplete to backlog OR next sprint (backend)

**Problem:** `sprint.Service.Complete(sprintID, moveOpenToBacklog bool)` only nulls `sprint_id` (→ backlog). B.2 needs the option to move incomplete issues into another sprint. Add that, and expose it on a seq-keyed complete endpoint the frontend can call.

**Files:**
- Modify: `internal/domain/sprint/service.go` (`Complete` gains an optional target sprint)
- Modify: `internal/api/handlers/agile_sprint_handler.go` (+ a complete action) and/or `router.go`
- Test: `internal/domain/sprint/service_test.go` (extend/create)

**Interfaces:**
- Produces: `Complete(sprintID string, moveOpenToBacklog bool, moveToSprintID *string) (*Sprint, error)` — when `moveToSprintID != nil`, reassign incomplete (non-`done`) issues to that sprint id; else keep the backlog behavior when `moveOpenToBacklog` is true; else leave them. A seq-keyed complete path: `POST /rest/agile/1.0/sprint/{sprintId}/complete` with body `{ "moveToSprintId": <seqId|null>, "moveOpenToBacklog": bool }` (resolve target seq→UUID in-handler).

- [ ] **Step 1: Write the failing test** — in the sprint domain test (mirror the existing test harness / in-memory SQLite setup), seed a sprint with two issues (one in a `done`-category status, one not), plus a second target sprint. Call `Complete(s1, false, &s2.ID)` and assert: the incomplete issue's `SprintID == s2.ID`, the done issue stays on s1 (or is untouched), s1 state is `closed`. Then a second case `Complete(s3, true, nil)` asserts incomplete → `sprint_id NULL` (backlog).

> Read the real `Complete` signature + how issues/status categories are queried; adapt.

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/domain/sprint/ -run Complete -v` → FAIL (arity/behavior).

- [ ] **Step 3: Implement** — change `Complete` to the 3-arg signature; when `moveToSprintID != nil`, `UPDATE issues SET sprint_id = ? WHERE sprint_id = ? AND status_id NOT IN (SELECT id FROM workflow_statuses WHERE category = 'done')`; else if `moveOpenToBacklog`, the existing NULL update. Update ALL existing callers of `Complete` (the agile Update path calls `Complete(sp.ID, true)` → change to `Complete(sp.ID, true, nil)`; the api/3 sprint handler complete → `Complete(sp.ID, body.MoveOpenToBacklog, nil)`). Add the seq-keyed `Complete` action: extend `agile_sprint_handler.go` with a `CompleteSprint` handler that parses `{moveToSprintId, moveOpenToBacklog}` (resolve `moveToSprintId` seq→UUID via `sprintSvc.GetBySeqID`), and register `POST /rest/agile/1.0/sprint/{sprintId}/complete` (gate `ManageSprints`, `BySprintSeq`).

- [ ] **Step 4: Verify** — `go test ./internal/domain/sprint/ -v && go build ./... && go vet ./... && go test ./... && go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md` (the new `/complete` path is an agile extension → commit the regenerated report if it changed; contract tests stay green).

- [ ] **Step 5: Commit**
```bash
git add internal/domain/sprint/service.go internal/api/handlers/agile_sprint_handler.go internal/api/router.go internal/domain/sprint/service_test.go docs/contracts/gap-report.md
git commit -m "feat(sprint): complete-sprint can move incomplete issues to another sprint (seq-keyed endpoint)"
```

---

### Task 2: Sprint goal/dates form + complete dialog (frontend)

**Files:**
- Modify: `frontend-next/lib/api.ts` (`sprints.update`, `sprints.complete`)
- Modify: `frontend-next/app/app/boards/[boardId]/backlog/page.tsx` (create/edit form + complete dialog + goal on header)
- Test: `frontend-next/e2e/backlog-sprint.spec.ts` (create)

**Interfaces:**
- `sprints.update(sprintId, { name?, goal?, startDate?, endDate? })` → `POST /rest/agile/1.0/sprint/${sprintId}` (the existing Update; sends the fields it supports).
- `sprints.complete(sprintId, { moveToSprintId?: number|null; moveOpenToBacklog?: boolean })` → `POST /rest/agile/1.0/sprint/${sprintId}/complete` (Task 1). Keep `sprints.setState` for Start.
- `sprints.create` already takes `(name, originBoardId, goal?)`; add optional dates if the create body supports them (agile Create accepts startDate/endDate) — if the api/3 create used by the page doesn't, switch the create call to the agile create or extend it.

- [ ] **Step 1: Write the failing test** — `e2e/backlog-sprint.spec.ts` (inline login): go to `/app/boards/1/backlog`, create a sprint with a name AND a goal, assert the sprint header shows the goal (`data-testid="sprint-goal"`). If seeding a completable sprint is awkward, at minimum cover create-with-goal + goal rendered.

- [ ] **Step 2: Run to verify it fails** — the create form has no goal field / header doesn't show goal.

- [ ] **Step 3: Implement** — read `backlog/page.tsx` first. Extend the sprint create form with goal + start/end date inputs; add an "Edit sprint" affordance opening a form (name/goal/dates → `sprints.update`); render the goal (and dates) on the sprint header (`data-testid="sprint-goal"`). Replace the plain "Complete" button with a dialog: shows "N incomplete issues" and offers "Move to Backlog" or "Move to <next sprint>" (a select of other open sprints), calling `sprints.complete(sprintId, {...})`. Invalidate the sprints/backlog queries on each mutation.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/backlog-sprint.spec.ts e2e/board.spec.ts --workers=1` (new PASS; board.spec — which covers backlog sprint create/start — still passes; adapt board.spec only if the create form's selectors changed).

- [ ] **Step 5: Commit**
```bash
git add frontend-next/lib/api.ts "frontend-next/app/app/boards/[boardId]/backlog/page.tsx" frontend-next/e2e/backlog-sprint.spec.ts
git commit -m "feat(frontend): sprint goal + start/end dates form and complete-sprint move dialog"
```

---

### Task 3: Board config persistence (migration + domain)

**Files:**
- Create: `migrations/000018_board_config.up.sql` / `.down.sql`
- Create/Modify: `internal/domain/board/config.go` (config model + service methods) and `internal/domain/board/service.go`
- Test: `internal/domain/board/config_test.go` (create)

**Interfaces:**
- Tables: `board_columns(id TEXT PK, board_id TEXT, name TEXT, position INT)`; `board_column_statuses(column_id TEXT, status_id TEXT, PRIMARY KEY(column_id,status_id))`; add to `boards`: `swimlane_mode TEXT NOT NULL DEFAULT 'none'`; `board_quick_filters(id TEXT PK, board_id TEXT, name TEXT, jql TEXT, position INT)`.
- Produces on `board.Service`:
  - `GetConfig(boardID string, wfStatuses []struct{ID,Name string}) (BoardConfig, error)` — returns persisted columns (+ their status ids), swimlane mode, quick filters. If no columns persisted, return a default derived 1:1 from `wfStatuses` (name+[id]). (Pass the workflow statuses in, or have the service accept a resolver — keep it simple: caller passes the fallback status list.)
  - `SaveConfig(boardID string, cfg BoardConfigInput) error` — replaces columns (+status links), swimlane mode, quick filters transactionally.
  - Go types `BoardColumn{ID,Name string; Position int; StatusIDs []string}`, `BoardQuickFilter{ID,Name,JQL string; Position int}`, `BoardConfig{Columns []BoardColumn; Swimlane string; QuickFilters []BoardQuickFilter}`, `BoardConfigInput{Columns []{Name string; StatusIDs []string}; Swimlane string; QuickFilters []{Name,JQL string}}`.

- [ ] **Step 1: Write the failing test** — `config_test.go` (in-memory SQLite, AutoMigrate the new structs): `SaveConfig(board, {Columns:[{"To Do & Doing",[s1,s2]},{"Done",[s3]}], Swimlane:"assignee", QuickFilters:[{"Mine","assignee = currentUser()"}]})` then `GetConfig(board, fallback)` returns 2 columns with the right status sets, swimlane "assignee", 1 quick filter. A second board with no SaveConfig → `GetConfig` returns the fallback 1:1 columns, swimlane "none", no filters.

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/domain/board/ -v` → FAIL.

- [ ] **Step 3: Implement** — write the migration (up creates the 3 tables + alters boards; down drops them). Add GORM models (with table names) and the service methods (SaveConfig in a transaction: delete existing columns/statuses/filters for the board, insert new, set swimlane_mode). GetConfig loads columns ordered by position with their status ids, quick filters ordered by position, and the swimlane_mode; fallback to 1:1 when no columns.

- [ ] **Step 4: Verify** — `go test ./internal/domain/board/ -v && go build ./... && go vet ./...` (migrations run on server start; tests AutoMigrate directly).

- [ ] **Step 5: Commit**
```bash
git add migrations/000018_board_config.up.sql migrations/000018_board_config.down.sql internal/domain/board/config.go internal/domain/board/service.go internal/domain/board/config_test.go
git commit -m "feat(board): persist board column/swimlane/quick-filter configuration (migration 000018)"
```

---

### Task 4: Board config endpoints (emit persisted columns + custom config read/write)

**Files:**
- Modify: `internal/api/handlers/agile_board_handler.go` (`Configuration` emits persisted columns, fallback 1:1)
- Create/Modify: a board-config handler + routes for `GET`/`PUT` the full editable config (columns + swimlane + quickFilters) — Heureum-custom path
- Modify: `internal/api/router.go`
- Test: `internal/api/handlers/board_config_test.go` (create) + confirm `internal/contract` agile board configuration test still passes

**Interfaces:**
- `GET /rest/agile/1.0/board/{boardId}/configuration` (existing, contract-validated) now builds `columnConfig.columns` from `boardSvc.GetConfig(...)` (persisted, or 1:1 fallback). **Do not add swimlane/quickFilter fields here** — keep the shape exactly `v3.BoardConfig`.
- NEW Heureum-custom: `GET /rest/api/3/project/{key}/board/{boardId}/config` and `PUT .../config` (or seq-scoped `/rest/agile/1.0/board/{boardId}/config` — pick a clear path; gate read `BrowseProjects`, write `AdministerProjects`, resolver `ByBoardSeq`/`ByKey`). Returns/accepts `{ columns:[{name, statusIds:[...]}], swimlane, quickFilters:[{name, jql}] }`. On PUT call `boardSvc.SaveConfig`.

- [ ] **Step 1: Write the failing test** — handler test: seed a board + workflow (2-3 statuses); PUT the custom config with a merged column; GET the custom config → reflects it; GET the agile `/configuration` → its `columnConfig.columns` reflects the merged column (multi-status). Confirm `go test ./internal/contract/ -run Board` still passes (the agile configuration stays schema-valid).

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/api/handlers/ -run BoardConfig -v` → FAIL.

- [ ] **Step 3: Implement** — in `Configuration`, replace the live 1:1 build with `boardSvc.GetConfig(b.ID, <workflow statuses>)` and map each `BoardColumn` → `v3.BoardColumnConfig{Name, Statuses: [{ID} per statusID]}`. Add the custom config handler (GET builds `{columns, swimlane, quickFilters}` from `GetConfig`; PUT decodes and calls `SaveConfig`). Register routes. Thread `boardSvc` into the handler if not present.

- [ ] **Step 4: Verify** — `go test ./internal/api/handlers/ -run BoardConfig -v && go test ./internal/contract/ -run 'Board|Agile' -v && go build ./... && go test ./... && go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md` (custom config route is a new extension → commit regenerated report; contract green).

- [ ] **Step 5: Commit**
```bash
git add internal/api/handlers/agile_board_handler.go internal/api/handlers/board_config_handler.go internal/api/router.go internal/api/handlers/board_config_test.go docs/contracts/gap-report.md
git commit -m "feat(board): emit persisted columns via agile configuration + custom board-config read/write endpoints"
```

---

### Task 5: Board settings editor (frontend)

**Files:**
- Modify: `frontend-next/lib/api.ts` (board config client)
- Create: `frontend-next/app/app/boards/[boardId]/settings/page.tsx` (or a `BoardSettings` component)
- Modify: board navigation to reach it (a "Board settings" link near the board)
- Test: `frontend-next/e2e/board-settings.spec.ts` (create)

**Interfaces:** `boards.config(boardId)` (GET) + `boards.saveConfig(boardId, {columns, swimlane, quickFilters})` (PUT) from Task 4; `workflow.get(projectKey)` for the status palette.

- [ ] **Step 1: Write the failing test** — `e2e/board-settings.spec.ts` (inline login): open the board settings for board 1, assert `data-testid="board-settings"` visible with the current columns listed; add a column (or rename one) and save; reload and assert it persisted.

- [ ] **Step 2: Run to verify it fails** — no settings page.

- [ ] **Step 3: Implement** — a Board settings page: list configured columns (each: name + the statuses mapped to it), allow add/remove/rename columns and assign statuses to columns (a simple multi-select or checkboxes of the project's workflow statuses per column — full drag-mapping is nice-to-have; a checkbox matrix is acceptable), a swimlane `<select>` (none/assignee/epic), and a quick-filter editor (list + add {name, jql} + remove). Save → `boards.saveConfig`. Provide a link to this page from the board (e.g. a gear/"Board settings" button on `boards/[boardId]/page.tsx`).

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/board-settings.spec.ts e2e/board.spec.ts --workers=1`.

- [ ] **Step 5: Commit**
```bash
git add frontend-next/lib/api.ts "frontend-next/app/app/boards/[boardId]/settings/page.tsx" "frontend-next/app/app/boards/[boardId]/page.tsx" frontend-next/e2e/board-settings.spec.ts
git commit -m "feat(frontend): board settings editor (columns/status mapping, swimlane, quick filters)"
```

---

### Task 6: Board renders configured columns + swimlanes + quick filters (frontend)

**Files:**
- Modify: `frontend-next/app/app/boards/[boardId]/page.tsx` and `frontend-next/components/board/BoardColumns.tsx`
- Test: extend `frontend-next/e2e/board.spec.ts` or `e2e/board-settings.spec.ts`

**Interfaces:** consumes the agile `configuration` (now persisted columns, multi-status) + the custom `boards.config` (swimlane + quickFilters).

- [ ] **Step 1: Write the failing test** — after Task 5 merges two statuses into one column (or seed a config), the board page renders that single merged column containing issues from BOTH statuses; and a swimlane grouping renders `data-testid="swimlane-*"` when swimlane != none. Pick a deterministic assertion against the DEMO seed.

- [ ] **Step 2: Run to verify it fails** — board still buckets by `statuses[0]`/name.

- [ ] **Step 3: Implement** — change the column derivation to use ALL of `column.statuses` (a status-SET): bucket an issue into the column whose status set contains the issue's `status.id` (match by id, not name). Drag-drop onto a column transitions to `column.statuses[0]` (the column's primary status). Render swimlanes: when swimlane is `assignee` or `epic`, group rows into `data-testid="swimlane-<key>"` bands (each band shows the column layout for that group); `none` = flat as today. Render quick-filter chips from `boards.config`; clicking a chip runs `search.jql(chip.jql)` and intersects the board issues by key (active chip filters the visible cards). Keep the existing dnd + card rendering.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/board.spec.ts e2e/board-settings.spec.ts --workers=1` (all pass; existing board dnd/columns still work with the default 1:1 config).

- [ ] **Step 5: Commit**
```bash
git add "frontend-next/app/app/boards/[boardId]/page.tsx" frontend-next/components/board/BoardColumns.tsx frontend-next/e2e/
git commit -m "feat(frontend): board renders configured columns (status sets), swimlanes, quick-filter chips"
```

---

### Task 7: Round close — gate, docs

- [ ] **Step 1: Full three-level gate**
```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test --workers=1 && cd ..
go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md
```
- [ ] **Step 2: CHANGELOG.md** — Added: sprint goal/dates form + complete-move dialog; configurable board columns (status sets), swimlanes, quick filters + migration 000018 + config endpoints. Note deferred: card layout, per-column min/max.
- [ ] **Step 3: STATE.md** — Round 18 entry (what shipped, the sprint UUID-route avoided in favor of seq-keyed, board config greenfield + contract-safe column emission) + follow-ups (card layout, min/max, quick-filter client-side intersection is approximate, swimlane by epic uses parent_id). Set "Prossimo: Round 19".
- [ ] **Step 4: Commit** docs + plan.
- [ ] **Step 5: Update auto-memory** (controller action).

---

## Self-Review

**Spec coverage:** B.2 goal/dates form → T2; complete-move dialog (backlog or next sprint) → T1 (backend) + T2 (UI); goal on header → T2. B.3 column config (status sets) → T3/T4/T6; swimlanes → T3/T4/T5/T6; quick filters → T3/T4/T5/T6; board renders config → T6; board settings UI → T5. Deferred: card layout, per-column min/max (noted). ✅

**Placeholder scan:** migration/table shapes, service signatures, endpoint shapes given; UI specified by behavior + testids + components to reuse. No TBD.

**Type consistency:** `Complete(sprintID, moveOpenToBacklog bool, moveToSprintID *string)` matches all updated callers; `BoardConfig`/`BoardColumn`/`BoardQuickFilter` Go types match the custom endpoint JSON and the client; the agile `configuration` continues to emit exactly `v3.BoardConfig` (no new fields → contract-safe); `boards.saveConfig` body `{columns:[{name,statusIds}],swimlane,quickFilters:[{name,jql}]}` matches the PUT handler.

**Cross-cutting risks:** (1) contract — never add fields to `v3.BoardConfig`; multi-status columns already fit its schema; swimlane/quickfilter only on the custom route; run the contract tests in T4. (2) Backward compat — unconfigured boards must fall back to 1:1 (test in T3). (3) drag onto a multi-status column targets `statuses[0]`. (4) quick-filter application is server-JQL + client key-intersection (note as approximate). (5) update ALL existing `Complete(` callers when changing its arity. (6) full suite `--workers=1`.
