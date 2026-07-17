# Round 19 — Releases / Versions (C.1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add project versions (releases): a `version` domain with CRUD + released flag + dates + progress, multi fix-versions on issues, Jira-conformant v3 endpoints, a Releases tab/page with progress bars, a Fix versions field on the issue panel + create modal, and a Releases lane in the Timeline.

**Architecture:** A `versions` table already exists (from `000001`) with id/project_id/name/description/release_date/released — but no `start_date`, no domain code, no UI. This round adds the missing `start_date`/`archived` columns + an `issue_versions` pivot (multi fix-versions, mirroring the labels many-to-many), a `version` domain service, Jira-conformant v3 mapping + endpoints (the official contract has a `Version` schema and `/project/{key}/version(s)` + `/version/{id}` paths, so this is contract-tested), issue fixVersions wiring, a Releases lane in the timeline, and the frontend surfaces.

**Tech Stack:** Go 1.25 (`net/http` ServeMux, GORM, golang-migrate, in-memory SQLite tests, `kin-openapi` contract harness), Next.js 16 App Router + React 19 + TanStack Query + Tailwind, Playwright.

## Global Constraints

- **Module path `github.com/it4nodummies/heureum`** — verbatim in imports.
- **Two-ID rule:** version list/create are keyed on the project `{key}` (resolve to `project.ID` server-side); version get/update/delete are keyed on the version's own `id` (a UUID string, exposed as the resource id — schema-valid since the contract's `Version.id` is a readOnly string). Never expose or require an internal project UUID in a path.
- **Contract conformance (this round IS contract-tested):** the official `Version` schema fields are `id`(string), `name`, `description`, `released`(bool), `archived`(bool), `startDate`(**string, format:date = YYYY-MM-DD**), `releaseDate`(**YYYY-MM-DD**), `projectId`(**integer int64 = the project seq_id**), `self`(uri), `overdue`(bool, readOnly). Emit EXACTLY these keys with these types. Version dates are **date-only `YYYY-MM-DD`**, NOT the RFC3339 `JiraTime` used elsewhere — do not reuse `JiraTime` for version dates. Add a contract test using the `internal/contract` harness for the version create/get/list responses. Run `go test ./internal/contract/...` after backend changes.
- **Permission gating:** version create/update/delete → `permission.AdministerProjects`; version/list reads → `permission.BrowseProjects`; assigning fix-versions to an issue → `permission.EditIssues`. Project-scoped routes use `chk.ByKey`; version-id routes need a NEW resolver `chk.ByVersion("id")` (mirror `chk.ByCustomField`) which requires mounting the version service on the `Checker` (`authz.New(...)`).
- **The dead `issue.VersionID` single FK stays dead** — multi fix-versions supersede it via the new `issue_versions` pivot. Do not wire `version_id`.
- **gapreport:** the new `/version*` paths ARE in the official spec, so implementing them should INCREASE the matched count (not add extensions) — commit the regenerated `docs/contracts/gap-report.md`.
- UI accent `#0052cc`; UI under `/app`; single typed client `frontend-next/lib/api.ts`.
- Three-level gate before done: (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test --workers=1`; (3) `go run ./cmd/gapreport` → commit regenerated report; clean on second run.
- Conventional Commits; branch `feat/frontend-next`. E2E inline the login preamble from `e2e/export.spec.ts`; full suite `--workers=1`. Seed project `DEMO`. **Next migration number: `000019`.**

---

### Task 1: Version migration + domain

**Files:**
- Create: `migrations/000019_versions.up.sql` / `.down.sql`
- Create: `internal/domain/version/model.go`, `internal/domain/version/service.go`
- Test: `internal/domain/version/service_test.go`

**Interfaces:**
- Migration: `ALTER TABLE versions ADD COLUMN start_date TIMESTAMP;` and `ALTER TABLE versions ADD COLUMN archived BOOLEAN NOT NULL DEFAULT FALSE;` + `CREATE TABLE issue_versions(issue_id TEXT NOT NULL, version_id TEXT NOT NULL, PRIMARY KEY(issue_id, version_id));`. Down: drop the pivot + the two columns.
- `version.Version` GORM model: `ID, ProjectID, Name, Description string; Released, Archived bool; StartDate, ReleaseDate *time.Time; CreatedAt time.Time`.
- `version.Service` (mirror `customfield.Service`): `NewService(db) *Service`; `Create(projectID, name, description string, startDate, releaseDate *time.Time) (*Version, error)`; `Get(id) (*Version, error)`; `ListByProject(projectID) ([]Version, error)`; `Update(id string, name, description *string, released, archived *bool, startDate, releaseDate *time.Time) (*Version, error)` (nil = unchanged); `Delete(id) error`; `SetFixVersions(issueID string, versionIDs []string) error` (reconcile pivot like `SetLabels`); `GetFixVersions(issueID) ([]Version, error)`; `ProgressCounts(versionID) (done, total int, error)` (count issues linked via pivot, done = status category 'done').

- [ ] **Step 1: Write the failing test** — `service_test.go` (in-memory SQLite, AutoMigrate Version + a pivot model + minimal issues/workflow_statuses via raw SQL where needed): create a version with dates; Update to released=true; ListByProject returns it; SetFixVersions(issue, [v1,v2]) then GetFixVersions returns 2; ProgressCounts with one done + one not-done issue linked → (1,2).

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/domain/version/ -v` → FAIL.

- [ ] **Step 3: Implement** — migration + models + service. `SetFixVersions` diffs existing pivot rows vs wanted (mirror `issue.Service.SetLabels` at service.go:228). `ProgressCounts` via a join `issue_versions`→`issues`→`workflow_statuses` counting total and `category='done'`.

- [ ] **Step 4: Verify** — `go test ./internal/domain/version/ -v && go build ./... && go vet ./...`.

- [ ] **Step 5: Commit**
```bash
git add migrations/000019_versions.up.sql migrations/000019_versions.down.sql internal/domain/version/ 
git commit -m "feat(version): version domain + issue_versions pivot (migration 000019)"
```

---

### Task 2: v3 Version mapping + endpoints + authz resolver (+ contract test)

**Files:**
- Create: `internal/api/v3/version.go` (`JiraVersion` + `VersionRef` + mapper)
- Create: `internal/api/handlers/version_handler.go`
- Modify: `internal/api/authz/*` (add `ByVersion` resolver + mount version service on `Checker`), `internal/api/router.go`
- Test: `internal/contract/version_test.go` (create) + a handler test

**Interfaces:**
- `v3.JiraVersion{ Self, ID, Name, Description string; Released, Archived, Overdue bool; StartDate, ReleaseDate string /*YYYY-MM-DD, omitempty*/; ProjectID int64 }` + `v3.VersionRef{ Self, ID, Name string }` + `func VersionFrom(v version.Version, projectSeqID int64, baseURL string) JiraVersion` (format dates via `t.Format("2006-01-02")`; empty when nil).
- Routes (follow the contract paths):
  - `GET /rest/api/3/project/{key}/versions` → list (array of JiraVersion) — `EnforceNotFound(BrowseProjects, ByKey)`.
  - `POST /rest/api/3/version` → create (body `{ name, description, startDate, releaseDate, projectId }` where projectId is the project seq_id OR accept `{project: "<key>"}`; resolve to project.ID; authz in-handler `RequireProject(uid, projectID, AdministerProjects)` since there's no path key) — return 201 JiraVersion.
  - `GET /rest/api/3/version/{id}` → `EnforceNotFound(BrowseProjects, ByVersion("id"))`.
  - `PUT /rest/api/3/version/{id}` → `Enforce(AdministerProjects, ByVersion("id"))` (body may include `released`, `archived`, `name`, `description`, `startDate`, `releaseDate`).
  - `DELETE /rest/api/3/version/{id}` → `Enforce(AdministerProjects, ByVersion("id"))`.
- `chk.ByVersion(param) Resolver` mirroring `ByCustomField` (`resolvers.go:124`): `versionSvc.Get(id)` → `.ProjectID`. Mount `versionSvc` on the Checker — extend `authz.New(...)` and its struct + the `router.go:59` call.

- [ ] **Step 1: Write the failing tests** — a contract test (mirror an existing `internal/contract/*_test.go`: boot the test server, create a version, GET it and the list, `ValidateResponse` against the `Version`/`PageBeanVersion` schema — or the array response); assert dates serialize as `YYYY-MM-DD` and `projectId` is the numeric seq id. A handler test for create→get→list→update(released)→delete happy path + a 403 for a non-admin.

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/contract/ -run Version -v` → FAIL.

- [ ] **Step 3: Implement** — v3 mapper, handlers (resolve project via `GetByKey` for list; via `projectId` seq→`GetBySeqID` or `{project:key}` for create), the `ByVersion` resolver + Checker mounting, routes. Emit `self = baseURL + "/rest/api/3/version/" + id`.

- [ ] **Step 4: Verify** — `go test ./internal/contract/ -run 'Version' -v && go test ./internal/api/handlers/ -run Version -v && go build ./... && go vet ./... && go test ./... && go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md` (matched count should rise; commit regenerated report).

- [ ] **Step 5: Commit**
```bash
git add internal/api/v3/version.go internal/api/handlers/version_handler.go internal/api/authz/ internal/api/router.go internal/contract/version_test.go docs/contracts/gap-report.md
git commit -m "feat(version): Jira-conformant v3 version endpoints + authz + contract test"
```

---

### Task 3: Issue fixVersions (multi) wiring

**Files:**
- Modify: `internal/api/v3/issue.go` (add `FixVersions []VersionRef` to `IssueFields` + `IssueInput`)
- Modify: `internal/api/handlers/issue_handler.go` (`buildIssueInput` populates fixVersions; `Update` + `Create` parse `fields.fixVersions` → `SetFixVersions`)
- Modify: whichever handler holds the issue deps to also hold `versionSvc`
- Test: extend an issue handler test or add `internal/api/handlers/issue_fixversions_test.go`

**Interfaces:**
- `IssueFields.FixVersions []VersionRef `json:"fixVersions"``; `IssueInput.FixVersions []v3.VersionRef` (or `[]version.Version` mapped in `JiraIssue`).
- `buildIssueInput`: `fvs, _ := h.versionSvc.GetFixVersions(iss.ID); in.FixVersions = map to refs`.
- PUT/Create body: parse `fields.fixVersions: [{id}]` → `h.versionSvc.SetFixVersions(iss.ID, ids)` (mirror the labels block at `issue_handler.go:359`).

- [ ] **Step 1: Write the failing test** — create an issue, PUT `fields.fixVersions:[{id:v1}]`, GET the issue → `fields.fixVersions` contains v1 (name+id); clearing with `[]` removes it.

- [ ] **Step 2: Run to verify it fails** — FAIL (field absent / not parsed).

- [ ] **Step 3: Implement** — add the struct field, thread `versionSvc` into `IssueHandler` (constructor + `router.go`), populate in `buildIssueInput`, parse+apply in `Update` and `Create`. Field projection needs no change (generic).

- [ ] **Step 4: Verify** — `go test ./internal/api/handlers/ -run 'Issue|FixVersion' -v && go test ./internal/contract/ -run Issue -v && go build ./... && go test ./...` (contract issue tests must still pass — fixVersions is a valid IssueBean field).

- [ ] **Step 5: Commit**
```bash
git add internal/api/v3/issue.go internal/api/handlers/issue_handler.go internal/api/router.go internal/api/handlers/issue_fixversions_test.go
git commit -m "feat(issue): fixVersions (multi) on issue read + create/update"
```

---

### Task 4: Timeline Releases lane

**Files:**
- Modify: `internal/domain/timeline/service.go` (add version bars)
- Test: `internal/domain/timeline/service_test.go` (create or extend)

**Interfaces:** `GetTimelineData` appends bars with `Type:"version"` for each version that has a start/release date, `Progress` from `version.ProgressCounts` (done/total), a distinct color; positioned by startDate→releaseDate.

- [ ] **Step 1: Write the failing test** — seed a project with a version (start+release dates) + a linked done issue; `GetTimelineData(projectID,"weeks")` includes a bar with `Type=="version"` named after the version with `Progress>0`.

- [ ] **Step 2: Run to verify it fails** — no version bars.

- [ ] **Step 3: Implement** — query versions for the project (with dates), compute progress via a join over `issue_versions`, append `TimelineBar{Type:"version", Name, StartDate, EndDate: releaseDate, Progress, Color:"#6554C0"}`. The frontend timeline page already renders any bar generically (color per bar), so version bars appear automatically; if a legend exists, add "Release".

- [ ] **Step 4: Verify** — `go test ./internal/domain/timeline/ -v && go build ./... && go test ./...`.

- [ ] **Step 5: Commit**
```bash
git add internal/domain/timeline/service.go internal/domain/timeline/service_test.go
git commit -m "feat(timeline): Releases lane (version bars with progress)"
```

---

### Task 5: Versions client + Releases tab/page (frontend)

**Files:**
- Modify: `frontend-next/lib/api.ts` (`versions` client + types)
- Modify: `frontend-next/components/projects/ProjectHeader.tsx` (Releases tab)
- Create: `frontend-next/app/app/projects/[key]/releases/page.tsx`
- Test: `frontend-next/e2e/releases.spec.ts` (create)

**Interfaces:**
- `Version` TS type `{ id, name, description, released, archived, startDate?, releaseDate?, projectId }`; `versions.list(key)`, `versions.create(key, {name,description?,startDate?,releaseDate?})` (POST `/version` with the resolved project — send `{project:key}` or the seq id per Task 2's create contract), `versions.update(id, {...})`, `versions.remove(id)`. Optionally `versions.progress(id)` if Task 2 exposes relatedIssueCounts; otherwise compute progress from a per-version issue search.
- ProjectHeader: add `"releases"` to `ActiveTab` + a `<TabLink href={`/app/projects/${projectKey}/releases`}>Releases</TabLink>`.

- [ ] **Step 1: Write the failing test** — `e2e/releases.spec.ts` (inline login): open `/app/projects/DEMO`, click "Releases", assert `data-testid="releases-page"`; create a release (name + optional release date), assert it appears in the table; toggle it released and assert the status shows Released; a released/unreleased filter.

- [ ] **Step 2: Run to verify it fails** — no Releases tab/page.

- [ ] **Step 3: Implement** — the client; the tab; the page: a table (name, status released/unreleased with a release toggle, a progress bar of done/total issues, start/release dates, description), a "Create release" form, and a released/unreleased filter. Progress per version: either from a Task-2 count endpoint or `search.jql(\`project = KEY AND fixVersion = <id>\`)` done vs total (if JQL supports fixVersion; if not, note it and show issue count only). Invalidate the versions query on mutations.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/releases.spec.ts --workers=1`.

- [ ] **Step 5: Commit**
```bash
git add frontend-next/lib/api.ts frontend-next/components/projects/ProjectHeader.tsx "frontend-next/app/app/projects/[key]/releases/page.tsx" frontend-next/e2e/releases.spec.ts
git commit -m "feat(frontend): Releases tab + versions page (create/release/progress)"
```

---

### Task 6: Fix versions field on issue (view/edit + create)

**Files:**
- Modify: `frontend-next/components/issues/IssueView.tsx` (Fix versions field view+edit)
- Modify: `frontend-next/components/issues/CreateIssueModal.tsx` (fix-versions picker)
- Test: `frontend-next/e2e/releases.spec.ts` (extend) or `e2e/issue-fixversions.spec.ts`

**Interfaces:** consumes `versions.list(projectKey)` for the option set; reads `issue.fields.fixVersions`; writes via `issues.update(key, { fixVersions: [{id}] })` (the PUT handler from Task 3).

- [ ] **Step 1: Write the failing test** — create a release for DEMO; open DEMO-1; in Edit mode assign the fix version; save; reload and assert the Fix versions field shows it (`data-testid="issue-fixversions"`).

- [ ] **Step 2: Run to verify it fails** — no fix-versions field.

- [ ] **Step 3: Implement** — IssueView: a "Fix versions" Details row (view = chips of `fields.fixVersions[].name`; edit = multi-select of the project's unreleased+released versions → `issues.update(key,{fixVersions:[{id}]})`), mirroring the labels/custom-field editing blocks. CreateIssueModal: a fix-versions multi-select over `versions.list(projectKey)`, sent in the create fields. Do not disturb the native story-points / custom-field handling.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/releases.spec.ts e2e/issues.spec.ts --workers=1` (issues.spec must still pass).

- [ ] **Step 5: Commit**
```bash
git add frontend-next/components/issues/IssueView.tsx frontend-next/components/issues/CreateIssueModal.tsx frontend-next/e2e/
git commit -m "feat(frontend): Fix versions field on issue detail + create modal"
```

---

### Task 7: Round close — seed, gate, docs

- [ ] **Step 1: Seed a demo version** — in `cmd/seed/main.go`, idempotently create a "v1.0" version on DEMO (with a release date) and assign it as a fix version on a demo issue, so the Releases page + Timeline lane render populated.
- [ ] **Step 2: Full three-level gate**
```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test --workers=1 && cd ..
go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md
```
- [ ] **Step 3: CHANGELOG.md** — Added: project Releases/Versions (Releases tab + page with progress, create/edit/release), multi Fix versions on issues, Timeline Releases lane, Jira-conformant `/version` + `/project/{key}/versions` endpoints (migration 000019). Note: the dead single `version_id` FK remains unused.
- [ ] **Step 4: STATE.md** — Round 19 entry (greenfield version domain on the pre-existing table; contract-conformant so it raised the matched count; multi fix-versions via pivot) + follow-ups (VersionMoveBean/relatedIssueCounts/unresolvedIssueCount not implemented; JQL `fixVersion` support if not added; archive UI). Set "Prossimo: Round 20".
- [ ] **Step 5: Commit** docs + plan + seed.
- [ ] **Step 6: Update auto-memory** (controller action).

---

## Self-Review

**Spec coverage (C.1):** version domain CRUD → T1/T2; issue↔version multi association → T1 (pivot) + T3 (wiring); v3 endpoints → T2; Releases tab/table/progress/create/release → T5; Fix versions field on issue + create → T6; Timeline Releases lane → T4. ✅

**Placeholder scan:** migration/model/service signatures, v3 shape (exact contract keys/types), routes + resolver given; UI specified by behavior + testids + components to mirror. No TBD.

**Type consistency:** `version.Service` signatures consumed by T2/T3/T4; `v3.JiraVersion` keys match the official `Version` schema (contract-tested); `VersionRef` used by `IssueFields.FixVersions` (T3) and the frontend chips (T6); `versions` client shapes match the handlers.

**Cross-cutting risks:** (1) contract — version dates are `YYYY-MM-DD` (NOT `JiraTime`); `projectId` is the integer seq id; emit exactly the schema keys; add the contract test in T2. (2) new `ByVersion` resolver requires mounting the version service on the Checker (`authz.New`) — update the struct + constructor + the router call. (3) fixVersions is multi (pivot) — the dead `version_id` FK stays unused. (4) gapreport matched count rises (these paths are in the spec) — commit the report. (5) `versionSvc` must be threaded into both the version handler and the issue handler. (6) full suite `--workers=1`.
