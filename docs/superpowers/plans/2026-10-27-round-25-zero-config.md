# Round 25 — Zero-config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** `POST /rest/api/3/project` makes a project **immediately usable** — it also creates the default workflow (loudly, not best-effort-logged), a set of default issue types, a global "Done" resolution, and a default board — and the scrum/kanban/business templates produce genuinely different configs. A user can create a project and drag an issue on its board without ever opening Settings.

**Architecture:** The project-create HTTP handler orchestrates the scaffold after the project row is created (workflow → issue types → resolution → board), failing loudly and deleting the half-created project on any error. Board columns already render 1:1 from workflow statuses, so no column seeding is needed. Per-template differences are driven by `project.Type`.

**Tech Stack:** Go 1.25 (GORM, net/http ServeMux), Next.js App Router + React 19 + @dnd-kit + Playwright.

**Spec:** `docs/superpowers/specs/2026-09-21-direzione-prodotto-design.md` § "R25 — Zero-config".

## Global Constraints

- Branch off `main`; Conventional Commits; **no `Co-Authored-By` trailer** (see CLAUDE.md § Git & commit).
- Keep the Jira contract intact: `POST /project` request/response shape unchanged (still returns `{self,id,key}`), contract tests in `internal/contract` must stay green.
- Don't key routes on internal UUID; wire new mutating routes through authz if any (none new here — `POST /project` already `authMw`).
- Nullable FK columns get `NULL`, never `""`. Migrations `NNNNNN_name.up/down.sql`, no `IF [NOT] EXISTS`.
- Gate (all three green before done): `go build ./... && go vet ./... && go test ./...`; `cd frontend-next && npm run build && npx playwright test`; `go run ./cmd/gapreport` (no drift).

### Product decisions (made here — the plan's "decided" choices)
- **Default issue types**: software templates (scrum, kanban) → `Epic, Story, Task, Bug, Subtask`; business → `Task, Bug, Subtask`.
- **Board type**: scrum→`scrum`, kanban→`kanban`, business→`kanban`.
- **Workflow**: uniform `TO DO / IN PROGRESS / DONE` for every template (differing workflows per template is out of scope for R25).
- **Resolution**: ensure a single **global** `Done` resolution exists (resolutions are a global table today; keep it global — no schema change).
- **Atomicity**: scaffold runs after the `projects` row is created; on ANY scaffold error, delete the project (`svc.Delete`) and return 500. No half-created projects.

## Review Focus

- **Idempotency / re-create**: creating a project whose scaffold partially ran before (e.g. a global "Done" already exists, or an issue type name already present) must not error — `EnsureResolution` and `SeedStandardTypes` must be idempotent. → covered in Task 1 & 2 tests.
- **Scaffold failure leaves no half-project**: if the board (or any step) fails, the project row + members must be gone. → covered in Task 4 test (`TestCreate_ScaffoldFailureRollsBack`).
- **Contract shape unchanged**: the richer create must still return exactly `{self,id,key}` and not break `internal/contract`. → Task 4 keeps the response writer untouched; run contract tests.
- **Template actually differs**: a kanban project gets a kanban board + software issue types; a business project gets a kanban board + the business issue-type set. → Task 4 test asserts board type + type set per template.
- **A brand-new project is usable with zero Settings visits**: board resolves immediately, has columns, and a dragged issue transitions. → Task 5 acceptance E2E.

---

## Task 1 — Default issue types (`issue.Service.SeedStandardTypes`)

**Files:**
- Modify: `internal/domain/issue/service.go`
- Test: `internal/domain/issue/service_test.go` (or the existing issue-type test file)

**Interfaces:**
- Consumes: existing `TypeIDByName(projectID, name string) (string, error)` (get-or-create, infers icon/isSubtask from the name).
- Produces: `func (s *Service) SeedStandardTypes(projectID string, names []string) error` — idempotent; creates each named type via `TypeIDByName` (so re-running is a no-op for existing names). Returns the first error.

- [ ] **Step 1: failing test** `TestSeedStandardTypesCreatesAndIsIdempotent` — call `SeedStandardTypes(p.ID, []string{"Epic","Story","Task","Bug","Subtask"})`; assert 5 types exist for the project (query `issue_types`), that "Subtask" has `IsSubtask=true` and "Bug" icon "bug"; call it again → still 5 (no duplicates, no error).
- [ ] **Step 2: run → FAIL** `go test ./internal/domain/issue/ -run TestSeedStandardTypes -v`
- [ ] **Step 3: implement** `SeedStandardTypes` looping `TypeIDByName` over `names`, returning on first error.
- [ ] **Step 4: run → PASS**
- [ ] **Step 5: commit** `feat(issue): SeedStandardTypes (idempotent default issue types per project)`

## Task 2 — Resolution writer (`issue.Service.EnsureResolution`)

**Files:**
- Modify: `internal/domain/issue/service.go`
- Test: `internal/domain/issue/service_test.go`

**Interfaces:**
- Consumes: existing `ResolutionIDByName(name string) (string, bool)` (read-only, global `resolutions` table).
- Produces: `func (s *Service) EnsureResolution(name, description string) (string, error)` — idempotent: if a resolution with that name exists, return its id; else insert a row (`id uuid, name, description`) into `resolutions` and return the new id. Mirrors `cmd/seed/main.go` "Done" insert.

- [ ] **Step 1: failing test** `TestEnsureResolutionIdempotent` — `EnsureResolution("Done","Work is complete")` returns a non-empty id and a `resolutions` row exists; calling again returns the SAME id and does not duplicate.
- [ ] **Step 2: run → FAIL**
- [ ] **Step 3: implement** `EnsureResolution` (lookup via `ResolutionIDByName`; on miss insert via `s.db.Table("resolutions").Create(...)` with a `uuid.New().String()`).
- [ ] **Step 4: run → PASS**
- [ ] **Step 5: commit** `feat(issue): EnsureResolution (idempotent global resolution writer)`

## Task 3 — `CreateDefaultWorkflow` fails loudly

**Files:**
- Modify: `internal/domain/workflow/service.go`
- Test: `internal/domain/workflow/service_test.go`

**Interfaces:**
- `CreateDefaultWorkflow(projectID string) (*Workflow, error)` — unchanged signature, but the inner `AddStatus`/`AddTransition` calls (currently `x, _ := ...`) must check and return their errors instead of ignoring them, so a partial workflow can't be created silently.

- [ ] **Step 1: failing test** `TestCreateDefaultWorkflowPropagatesStatusError` — using the package's test DB, induce an `AddStatus`/`AddTransition` failure (e.g. call `CreateDefaultWorkflow` twice for the same project so the second hits a uniqueness/duplicate, OR assert that a wrapped error from a stubbed failure surfaces). If a clean failure is hard to induce, instead assert the positive path still returns a fully-formed workflow AND change the impl to check each inner error (the behavior change is "no `_`-discarded errors"); verify by reading the code in Step 3. Keep the existing `TestCreateDefaultWorkflowCreatesStatusesAndTransitions` green.
- [ ] **Step 2: run → FAIL (or N/A if asserting code shape)**
- [ ] **Step 3: implement** replace each `x, _ := s.AddStatus(...)` / `AddTransition(...)` with error-checked versions that `return nil, err` on failure.
- [ ] **Step 4: run → PASS** (`go test ./internal/domain/workflow/...`)
- [ ] **Step 5: commit** `fix(workflow): CreateDefaultWorkflow stops ignoring inner status/transition errors`

## Task 4 — Scaffold orchestration in the create handler (+ per-template)

**Files:**
- Modify: `internal/api/handlers/project_handler.go` (struct + `NewProjectHandler` + `Create`)
- Modify: `internal/api/router.go` (one call site, line ~64 — pass `boardSvc`, `issueSvc`, both already in scope)
- Test: `internal/api/handlers/project_handler_test.go`

**Interfaces:**
- Consumes: Task 1 `issueSvc.SeedStandardTypes`, Task 2 `issueSvc.EnsureResolution`, Task 3 `wfSvc.CreateDefaultWorkflow`, existing `boardSvc.Create(projectID, name, boardType string, filterID *string)`, existing `svc.Delete(key)` for cleanup.
- Produces: `NewProjectHandler(svc *project.Service, wfSvc *workflow.Service, boardSvc *board.Service, issueSvc *issue.Service, chk *authz.Checker, baseURL string)` — two new params.

- [ ] **Step 1: failing test(s)** in `project_handler_test.go`:
  - Extend `setupProjectTestDB` to `AutoMigrate` board, issue-type, and resolution tables (and whatever the sub-services need).
  - Update the `NewProjectHandler(...)` call with `board.NewService(db)` and the issue service.
  - `TestCreate_ScaffoldsBoardTypesResolution`: POST a scrum project → 201; assert a board exists for the project (via `boardSvc.ListByProject`), the 5 software issue types exist, and the global "Done" resolution exists, and a default workflow exists.
  - `TestCreate_TemplateDiffers`: POST a kanban template → board type `kanban`; POST a business template → board type `kanban` + issue-type set `{Task,Bug,Subtask}` (no Epic/Story).
  - `TestCreate_ScaffoldFailureRollsBack`: force a scaffold step to fail (e.g. inject a failing stub or a closed DB for one sub-service) → handler returns 500 AND no `projects` row remains for that key.
- [ ] **Step 2: run → FAIL**
- [ ] **Step 3: implement** — add `boardSvc`/`issueSvc` to the struct + constructor; in `Create`, after `h.svc.CreateProject(in)` succeeds, run the scaffold in order and on ANY error `h.svc.Delete(p.Key)` then `v3.WriteError(w, 500, ...)` and return:
  1. `wfSvc.CreateDefaultWorkflow(p.ID)` (no more `log.Printf` swallow).
  2. `issueSvc.SeedStandardTypes(p.ID, typeSetFor(p.Type))` where `typeSetFor` returns `{Epic,Story,Task,Bug,Subtask}` for scrum/kanban and `{Task,Bug,Subtask}` for business.
  3. `issueSvc.EnsureResolution("Done","Work is complete")`.
  4. `boardSvc.Create(p.ID, p.Name+" board", boardTypeFor(p.Type), nil)` where `boardTypeFor` returns `scrum`/`kanban`/`kanban`.
  Keep the 201 response writer exactly as is.
- [ ] **Step 4: run → PASS** `go test ./internal/api/handlers/... ./internal/contract/...` (contract shape unchanged).
- [ ] **Step 5: build+vet** `go build ./... && go vet ./...`
- [ ] **Step 6: commit** `feat(api): POST /project scaffolds workflow+types+resolution+board (per-template); fail-loud with cleanup`

## Task 5 — Frontend + acceptance E2E + round closure

**Files:**
- Modify: `frontend-next/components/projects/ProjectOverview.tsx` (the now-vestigial "Create board" prompt)
- Create/Modify: `frontend-next/e2e/board.spec.ts` (acceptance test)
- Modify: `docs/superpowers/STATE.md`, `CHANGELOG.md`
- Modify: `docs/contracts/gap-report.md` (only if gapreport drifts)

**Interfaces:**
- Consumes: a project created via `CreateProjectModal` now already has a board (Task 4), so `boardsApi.list()` finds it and the Board/Backlog tabs resolve.

- [ ] **Step 1: neutralize the vestigial prompt** — in `ProjectOverview.tsx`, since a new project now always has a board, the "This project doesn't have a board yet / Create board" block (lines ~65-91) is dead for new projects. Keep it ONLY as a fallback for legacy board-less projects (render it when `boards.list` is empty), otherwise show the board link. Do not break the existing "legacy project without board" path.
- [ ] **Step 2: acceptance E2E** in `e2e/board.spec.ts` — `test("new project is usable without opening Settings")`: log in; create a project via `CreateProjectModal` (scrum); from the project page the **Board tab is already a link** (no "Create board" click); open the board; create an issue in the project (via the create-issue modal or API helper); then `dragBetween` the card from one column to a valid next column and assert it moved — all WITHOUT navigating to any `/settings` route. Reuse the existing `dragBetween` helper.
- [ ] **Step 3: three-level gate** — `go build ./... && go vet ./... && go test ./...`; `cd frontend-next && npm run build && npx playwright test`; `go run ./cmd/gapreport && git diff --exit-code docs/contracts/gap-report.md`.
- [ ] **Step 4: docs** — `CHANGELOG.md` `### Added` under `[Unreleased]` (zero-config project creation: default workflow/issue-types/resolution/board, per-template differences); `STATE.md` new round entry (what changed, the handler orchestration + per-template decisions, that templates now differ by board type + issue-type set, workflow still uniform).
- [ ] **Step 5: commit** `feat(projects): zero-config — new project usable immediately; acceptance e2e; docs`

---

## Self-review
- **Spec coverage** (R25 four points): default board+types+resolution on create (T1,T2,T4) ✓; templates genuinely differ (T4 board type + issue-type set) ✓; `CreateDefaultWorkflow` no longer silent (T3 + T4 handler) ✓; acceptance E2E create→drag without Settings (T5) ✓.
- **Review Focus** each has an owning test (idempotency T1/T2, rollback T4, contract T4, template-differs T4, usable-without-settings T5).
- **Types consistent**: `SeedStandardTypes(projectID, names)`, `EnsureResolution(name,desc)→(id,err)`, `NewProjectHandler(...boardSvc,issueSvc...)`, `boardSvc.Create(projectID,name,type,filterID)` — all as used in Task 4.
- **No placeholders**; product decisions explicit in Global Constraints.
- **Out of scope (YAGNI)**: per-template different workflows; project-scoped (vs global) resolutions; first-run wizard (separate, post-R25).
