# Round 26 — Workflow QA state + real Kanban/Scrum — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to
> implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** New software projects get a 4-state default workflow (TO DO → IN PROGRESS → TO BE TESTED →
DONE); Kanban and Scrum boards behave differently (Scrum has sprints; Kanban has a sprint-less flat
backlog and rejects sprint creation).

**Architecture:** Backend change to the default-workflow factory + a board-type gate on sprint
creation; frontend reads `board.type` to show/hide the sprint/backlog experience. No new routes, no
new DTO fields, no schema migration.

**Tech Stack:** Go (GORM, net/http ServeMux), Next.js App Router + TanStack Query, Playwright.

**Spec:** `docs/superpowers/specs/2026-10-27-round-26-workflow-kanban-scrum-design.md`

## Global Constraints

- Branch off `origin/main` (has R25). Conventional Commits; **no `Co-Authored-By`** trailer; never
  commit on `main`.
- Three-level gate must pass: `go build ./... && go vet ./... && go test ./...`; `cd frontend-next
  && npm run build && npx playwright test`; `go run ./cmd/gapreport` with **no diff**.
- SQLite in tests hides Postgres-only defects; no schema change here, but keep that in mind.
- The review state reuses the existing `in-progress` status category — no new category.
- R26 changes only the workflow **created at project creation**; existing projects are untouched.

## Review Focus

- A **business** project still gets exactly 3 statuses (no review state) — Task 1 test.
- A new **scrum/kanban** project's board renders **4 columns** and an issue can move TO DO →
  IN PROGRESS → TO BE TESTED → DONE end-to-end — Task 4 e2e.
- A **kanban** board's `/backlog` shows a **flat backlog** (to-do issues, rank-order, "Send to
  board") and **no sprint UI** — Tasks 3 & 4.
- **Creating a sprint on a kanban board is rejected by the API** (HTTP 400), not merely hidden —
  Task 2 test.
- **Existing pre-R26 projects/boards** are unaffected (no migration; default-only change) — asserted
  by Task 1 (signature is opt-in via flag; `includeReview=false` reproduces today's 3-state).

---

### Task 1: Software default workflow gains a "TO BE TESTED" state

**Files:**
- Modify: `internal/domain/workflow/service.go:182` (`CreateDefaultWorkflow`)
- Modify: `internal/api/handlers/project_handler.go:133` (caller)
- Modify: `cmd/seed/main.go:174` (caller)
- Test: `internal/domain/workflow/service_test.go`

**Interfaces:**
- Produces: `CreateDefaultWorkflow(projectID string, includeReview bool) (*Workflow, error)`.
  When `includeReview` is true the workflow has statuses TO DO (todo), IN PROGRESS (in-progress),
  TO BE TESTED (in-progress), DONE (done) and transitions *Start Progress* (TODO→INPROG), *Stop
  Progress* (INPROG→TODO), *Ready to Test* (INPROG→TOBETESTED), *Back to In Progress*
  (TOBETESTED→INPROG), *Done* (TOBETESTED→DONE, setResolution=true). When false, the current 3-state
  workflow is produced unchanged.
- Consumes: `project.Type` in the handler to compute the flag.

- [ ] **Step 1: Write failing tests**
  - `TestCreateDefaultWorkflow_WithReview_HasFourStatesAndTransitions`: call with
    `includeReview=true`; assert statuses in order `["TO DO","IN PROGRESS","TO BE TESTED","DONE"]`,
    `TO BE TESTED` category == `CategoryInProgress`, and the 5 transitions above exist.
  - `TestCreateDefaultWorkflow_WithoutReview_IsThreeState`: call with `includeReview=false`; assert
    exactly 3 statuses and the original 3 transitions (reproduces today's behavior).
- [ ] **Step 2: Run tests, verify they fail** — `go test ./internal/domain/workflow/... -run CreateDefaultWorkflow -v`
- [ ] **Step 3: Implement** the new signature in `service.go`: keep the 3-state path; when
  `includeReview`, insert the TO BE TESTED status (color e.g. `#8B5CF6`) and the two extra
  transitions, and point *Done* at TOBETESTED→DONE instead of INPROG→DONE.
- [ ] **Step 4: Update both callers** — `project_handler.go:133` →
  `h.wfSvc.CreateDefaultWorkflow(p.ID, p.Type != project.TypeBusiness)`; `cmd/seed/main.go:174` →
  `CreateDefaultWorkflow(demo.ID, true)` (demo is a software project).
- [ ] **Step 5: Run** `go build ./... && go test ./internal/domain/workflow/... ./internal/api/handlers/...` — PASS
- [ ] **Step 6: Commit** `feat(workflow): software default workflow adds an IN REVIEW state`

### Task 2: Kanban boards reject sprint creation

**Files:**
- Modify: `internal/api/handlers/agile_sprint_handler.go:83` (`Create`, where board `b` is resolved)
- Modify: `internal/domain/sprint/service.go` (add a sentinel error if a service-level guard is
  preferred; otherwise gate in the handler using `b.Type`)
- Test: `internal/api/handlers/agile_sprint_handler_test.go` (or the existing agile sprint test file)

**Interfaces:**
- Consumes: `board.Board.Type` (already resolved as `b` in the agile handler before `CreateFull`).
- Produces: sprint creation on a board whose `Type != "scrum"` returns HTTP 400 with a clear message
  (e.g. `"Sprints are only available on scrum boards."`); scrum boards unchanged.

- [ ] **Step 1: Write failing tests**
  - `TestAgileSprintCreate_RejectedOnKanbanBoard`: create a kanban board, POST
    `/rest/agile/1.0/sprint` with `originBoardId` = that board → assert 400 and no sprint row.
  - `TestAgileSprintCreate_AllowedOnScrumBoard`: same against a scrum board → 201.
- [ ] **Step 2: Run, verify fail** — `go test ./internal/api/handlers/... -run AgileSprintCreate -v`
- [ ] **Step 3: Implement** the guard in `agile_sprint_handler.Create`: after resolving board `b`,
  if `b.Type != "scrum"` call `v3.WriteError(w, 400, ...)` and return before `CreateFull`.
- [ ] **Step 4: Keep contract tests green** — if an `internal/contract` test exercises sprint create,
  ensure it uses a scrum board; add/adjust as needed.
- [ ] **Step 5: Run** `go test ./internal/... && go run ./cmd/gapreport` (no diff) — PASS
- [ ] **Step 6: Commit** `feat(agile): reject sprint creation on kanban boards`

### Task 3: Frontend — scrum sprints vs kanban flat backlog

**Files:**
- Modify: board page `frontend-next/app/app/boards/[boardId]/page.tsx` (Backlog link/entry — label
  it "Backlog" for both; it's always present now)
- Modify: backlog page `frontend-next/app/app/boards/[boardId]/backlog/page.tsx` (branch on type)
- Reference: `frontend-next/lib/api.ts` — `boardsApi.get` returns `type`; existing issue-list,
  `PUT /issue/rank` and transition wrappers are reused (no new API wrappers expected).
- Test: covered by Task 4 e2e; this task is the UI wiring.

**Interfaces:**
- Consumes: `board.type` (`"scrum" | "kanban"`) from `boardsApi.get`.
- Produces:
  - **Scrum board**: backlog page unchanged — sprint sections + backlog↔sprint DnD.
  - **Kanban board**: backlog page renders a **flat backlog** — one rank-ordered list of the
    board's **to-do-category** issues, drag-to-reorder (`PUT /issue/rank`), each row with a **"Send
    to board"** action that transitions the issue to the first in-progress status. **No sprint
    controls** anywhere (no create-sprint, no sprint sections).

- [ ] **Step 1: Board page** — keep the Backlog link for all boards (scrum + kanban both have a
  backlog now); no sprint-specific controls on the board page for kanban.
- [ ] **Step 2: Backlog page — branch on `board.type`.** Scrum: current sprint behavior. Kanban:
  render the flat backlog list (to-do-category issues, ranked), DnD reorder via `PUT /issue/rank`,
  and a "Send to board" action per row (transition to the first `in-progress` status); render **no**
  sprint sections/controls.
- [ ] **Step 3: Run** `cd frontend-next && npm run build` — PASS (lint/types clean).
- [ ] **Step 4: Commit** `feat(board): kanban flat backlog; scrum keeps sprints`

### Task 4: End-to-end acceptance

**Files:**
- Test: `frontend-next/e2e/round-26-workflow-kanban-scrum.spec.ts` (new)

**Interfaces:**
- Consumes: project-create flow (`CreateProjectModal`, scrum/kanban/business templates), board +
  backlog pages.

- [ ] **Step 1: e2e — TO BE TESTED flow (scrum project):** create a scrum project; open its board;
  assert **4 columns** including "TO BE TESTED"; create an issue; drag it TO DO → IN PROGRESS →
  TO BE TESTED → DONE and assert it lands in each.
- [ ] **Step 2: e2e — kanban flat backlog, no sprints:** create a kanban project; open
  `/boards/{id}/backlog`; assert the **flat backlog** (a to-do issue appears, no sprint sections,
  no create-sprint control); use **"Send to board"** on an issue and assert it leaves the backlog
  and appears on the board.
- [ ] **Step 3: e2e — business stays lean:** create a business project; assert its board has
  **3 columns** (no TO BE TESTED).
- [ ] **Step 4: Run** `cd frontend-next && npx playwright test e2e/round-26-workflow-kanban-scrum.spec.ts` — PASS
- [ ] **Step 5: Commit** `test(e2e): R26 TO BE TESTED flow, kanban flat backlog, business lean`

---

## Final

- [ ] Run the full three-level gate (backend build/vet/test; frontend build + full playwright;
  gapreport no-diff).
- [ ] Whole-branch review, then `superpowers:finishing-a-development-branch`.
- [ ] Update `docs/superpowers/STATE.md` with the R26 entry (what changed, that Kanban/Scrum is now
  behavioral, the review state is a software default; note WIP limits / workflow templates as
  deferred follow-ups).
