# Round 17 — List produttiva (bulk edit, inline edit, hierarchy, pagination) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Turn the read-only issue List (`SearchResults`) into a productive work surface: inline-edit priority/assignee/status per row, multi-select rows with a bulk action bar backed by a NEW `POST /issues/bulk` endpoint, real cursor pagination, and lightweight epic→children indentation.

**Architecture:** The list is `SearchResults.tsx` (read-only) fed by `search.jql` on the filters page. The only new backend is a bulk-update endpoint; everything else is a frontend rework plus richer `fields` requested from the existing search. The bulk endpoint spans issues across projects, so it is registered undecorated and enforces per-issue in-handler (`RequireProject(uid, iss.ProjectID, EditIssues)` — `TransitionIssues` for status) with partial-failure reporting, mirroring `IssueLinkHandler.Create` / `/issues/rank`.

**Tech Stack:** Go 1.25 (`net/http` ServeMux, GORM, in-memory SQLite tests), Next.js 16 App Router + React 19 + TanStack Query + Tailwind, Playwright.

## Global Constraints

- **Module path `github.com/it4nodummies/heureum`** — verbatim in imports.
- **Two-ID rule:** frontend deals in keys/seq_ids; the bulk endpoint accepts issue **keys** (or seq ids) in its body and resolves them server-side.
- **Bulk authorization is per-issue, in-handler:** for each target issue resolve its project and call `chk.RequireProject(uid, iss.ProjectID, permission.EditIssues)` (and `permission.TransitionIssues` when changing status). A forbidden/unknown key is reported as a per-key failure, not a 403 for the whole request. The route is registered WITHOUT the `Enforce` decorator (it can't gate a body list) — exactly like `POST /rest/api/3/issues/rank`.
- **Status is not settable via `PUT /issue`** — it goes through transitions. Inline status edit fetches that issue's available transitions (`GET /issue/{key}/transitions`) lazily and calls `issues.transition(key, statusId)`. Bulk status change is OUT OF SCOPE this round (cross-project workflow validation) — note it as a follow-up.
- **Sprint bulk change is OUT OF SCOPE** (agile move) — note as follow-up.
- UI accent `#0052cc`; UI under `/app`; single typed client `frontend-next/lib/api.ts`.
- Three-level gate before done: (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test --workers=1`; (3) `go run ./cmd/gapreport` → no diff.
- Conventional Commits; branch `feat/frontend-next`. E2E inline the login preamble from `e2e/export.spec.ts`. Full suite runs `--workers=1`. Seed project is `DEMO`.

---

### Task 1: Bulk update endpoint (backend)

**New:** `POST /rest/api/3/issues/bulk` applies a partial field set to a list of issue keys, per-issue authz, partial results.

**Files:**
- Create: `internal/api/handlers/bulk_handler.go` (or add to `issue_handler.go`)
- Modify: `internal/api/router.go` (register the undecorated route)
- Test: `internal/api/handlers/bulk_handler_test.go` (create)

**Interfaces:**
- Consumes: `issue.Service.GetByKey`/`GetBySeqID`, `Update(key, title, descJSON *string, priority *Priority, assigneeID, statusID *string, storyPoints *int)`, `SetLabels(issueID, projectID, names)`, `Delete(key)`; `authz.Checker.RequireProject(uid, projectID, permKey)`; `middleware.UserIDFromContext`.
- Produces: `POST /rest/api/3/issues/bulk` with body:
  ```json
  { "keys": ["DEMO-1","DEMO-2"],
    "fields": { "assignee": {"accountId": "..."}|null, "priority": {"id":"3"}, "labels": ["a","b"] },
    "delete": false }
  ```
  Response `200`: `{ "results": [ {"key":"DEMO-1","ok":true}, {"key":"DEMO-2","ok":false,"error":"forbidden"} ] }`.

- [ ] **Step 1: Write the failing test**

Read a sibling handler test that builds an HTTP server with an in-memory DB and an authenticated context (look for `newTestServer*`/`promoteAdmin` in `internal/api/handlers/*_test.go`; the R16 `filter_share_test.go` uses `httptest` + `middleware.UserIDKey` context injection — mirror it). Write `bulk_handler_test.go`:
- Seed a project + two issues (as the demo admin, who is global admin → passes `RequireProject`).
- POST `/issues/bulk` with `{keys:["<k1>","<k2>"], fields:{priority:{id:"1"}}}`; assert `200`, both results `ok:true`, and that reloading the issues shows priority `highest`.
- POST with a non-existent key mixed in; assert that key's result has `ok:false` and a non-empty `error`, while the valid key succeeds (partial failure).

> Match the exact harness the sibling test uses; do not invent a new one.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/api/handlers/ -run Bulk -v`
Expected: FAIL — handler/route not defined.

- [ ] **Step 3: Implement**

Handler `BulkUpdate`:
```go
type bulkBody struct {
	Keys   []string `json:"keys"`
	Fields struct {
		Assignee *struct{ AccountID string `json:"accountId"` } `json:"assignee"`
		Priority *struct{ ID string `json:"id"` }              `json:"priority"`
		Labels   *[]string                                     `json:"labels"`
	} `json:"fields"`
	Delete bool `json:"delete"`
}
type bulkResult struct {
	Key   string `json:"key"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}
```
For each key: resolve the issue (numeric → `GetBySeqID`, else `GetByKey`); if not found → result `{ok:false,error:"not found"}`. Enforce: `perm := permission.EditIssues`; `if err := h.chk.RequireProject(uid, iss.ProjectID, perm); err != nil { result forbidden; continue }`. Apply:
- delete → `h.svc.Delete(iss.Key)`;
- else build `*Priority` from `priorityEnumForID(Fields.Priority.ID)` (reuse the helper in `issue_handler.go`), `*string` assignee (nil-able: an explicit `assignee:null` clears — represent "unassign" as a pointer to empty string per the existing `Update` semantics; if the existing Update can't clear, document that clearing is not supported and only set when accountId non-empty), and if `Fields.Labels != nil` call `SetLabels(iss.ID, iss.ProjectID, *Fields.Labels)`; call `h.svc.Update(iss.Key, nil, nil, priority, assigneeID, nil, nil)`.
- Collect `{key, ok:true}` or `{ok:false,error: err.Error()}`.
Return `{results: [...]}`.

Register in `router.go` next to `/issues/rank` (undecorated, `authMw` only):
```go
mux.Handle("POST /rest/api/3/issues/bulk", authMw(http.HandlerFunc(issueH.BulkUpdate)))
```
Ensure the handler has access to `chk` (the `IssueHandler` already holds a `*authz.Checker` — verify; if not, thread it in).

- [ ] **Step 4: Verify**

Run: `go test ./internal/api/handlers/ -run Bulk -v && go build ./... && go vet ./... && go test ./... && go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md`
Expected: tests PASS, full suite green, no gapreport diff (bulk is a Heureum extension, not in the Jira spec).

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/bulk_handler.go internal/api/router.go internal/api/handlers/bulk_handler_test.go
git commit -m "feat(issues): POST /issues/bulk — per-issue-authorized bulk field update with partial results"
```

---

### Task 2: List clients + richer search fields

**Files:** Modify `frontend-next/lib/api.ts`.

**Interfaces / Produces:**
- `issues.bulk(body: { keys: string[]; fields?: { assignee?: {accountId:string}|null; priority?: {id:string}; labels?: string[] }; delete?: boolean }) → { results: {key:string; ok:boolean; error?:string}[] }`.
- `issues.transitions(key: string) → { transitions: {id:string; name:string; to:{id:string; name:string}}[] }` (GET `/rest/api/3/issue/${key}/transitions`) — read the exact response shape from the workflow transitions handler and match it.
- Expand `SearchIssue.fields` to include `issuetype?: {name:string; iconUrl?:string}`, `priority?: {id?:string; name:string}`, `status?: {id?:string; name; statusCategory?}`, `assignee?: {accountId?:string; displayName:string}|null`, `parent?: {key:string} | null`, `customfield_10016?: number`.
- A default richer fields list for the list view: export a constant `LIST_FIELDS = ["summary","status","priority","assignee","updated","issuetype","parent","customfield_10016"]` and have the filters page pass it to `search.jql`.

- [ ] **Step 1: Implement** — add the methods/types; verify the transitions response shape against `internal/api/handlers/workflow_handler.go` (the `ListTransitions`/`DoTransition` GET) and the priority-id mapping (1..5). Keep existing `issues.update`/`issues.transition`.

- [ ] **Step 2: Verify** — `cd frontend-next && npx tsc --noEmit` clean.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): list clients (issues.bulk, issues.transitions) + richer search fields"
```

---

### Task 3: Row selection + bulk action bar

**Files:**
- Modify: `frontend-next/components/search/SearchResults.tsx` (selection + bulk bar)
- Modify: `frontend-next/app/app/filters/page.tsx` (wire bulk mutation + refetch)
- Test: `frontend-next/e2e/list-bulk.spec.ts` (create)

**Interfaces:** consumes `issues.bulk` (Task 2); `SearchResults` gains an `onChanged?: () => void` callback (or the page passes a mutation) to refetch after bulk.

- [ ] **Step 1: Write the failing test**

`e2e/list-bulk.spec.ts` (inline login): go to `/app/filters`, run `project = DEMO`, check the select-all checkbox (or two row checkboxes), assert a bulk bar `data-testid="bulk-bar"` shows "N selected", pick a priority in the bulk bar, Apply, and assert the change reflects (e.g., a priority cell updates after refetch).

- [ ] **Step 2: Run to verify it fails** — no selection UI.

- [ ] **Step 3: Implement**

In `SearchResults.tsx`: add a leading checkbox column (header = select-all across the currently shown rows; per-row `data-testid="row-select"`), selection state `Set<string>` of keys. When ≥1 selected, render a bulk action bar (`data-testid="bulk-bar"`) showing "N selected" with: a priority `<select>` (5 priorities), an assignee picker (reuse `UserPicker` — needs a projectKey; since results may span projects, use the first selected issue's project derived from its key prefix, or make assignee bulk use a plain accountId search via `profile.searchUsers`), an "Add label" input, and a Delete button. "Apply" calls `issues.bulk({keys:[...selected], fields:{...}})` (or `{delete:true}`), then invalidates/refetches and clears selection. Surface any per-key failures from `results` as a small warning.

In `filters/page.tsx`: pass an `onChanged` that re-runs the current query (re-invoke `search.jql` with the current jql), and pass `LIST_FIELDS`.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/list-bulk.spec.ts --workers=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/components/search/SearchResults.tsx "frontend-next/app/app/filters/page.tsx" frontend-next/e2e/list-bulk.spec.ts
git commit -m "feat(frontend): list row selection + bulk action bar (priority/assignee/label/delete)"
```

---

### Task 4: Inline cell edit (priority / assignee / status)

**Files:**
- Modify: `frontend-next/components/search/SearchResults.tsx` (editable cells)
- Test: `frontend-next/e2e/list-inline.spec.ts` (create)

**Interfaces:** consumes `issues.update` (priority/assignee), `issues.transitions` + `issues.transition` (status).

- [ ] **Step 1: Write the failing test**

`e2e/list-inline.spec.ts` (inline login): run `project = DEMO`, click a priority cell → a dropdown appears → pick a different priority → assert the cell shows the new value after the update (refetch). (Keep the assertion tolerant to which issue; target DEMO-1's row.)

- [ ] **Step 2: Run to verify it fails** — cells are static text.

- [ ] **Step 3: Implement**

Make the priority, assignee, and status cells clickable-to-edit:
- **priority:** click → `<select>` of the 5 priorities → on change `issues.update(key, { priority: { id } })` → refetch/optimistic.
- **assignee:** click → `UserPicker` (or `profile.searchUsers`) → `issues.update(key, { assignee: { accountId } })`.
- **status:** click → lazily `issues.transitions(key)` → `<select>` of available target statuses → on change `issues.transition(key, targetStatusId)` → refetch. If no transitions available, show the status read-only.
Keep cells read-only until clicked (avoid turning every cell into an input). Editing must not trigger the row's key-link navigation.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/list-inline.spec.ts e2e/search.spec.ts --workers=1` → all pass (search.spec must still pass — the column-toggle/render tests).

- [ ] **Step 5: Commit**

```bash
git add frontend-next/components/search/SearchResults.tsx frontend-next/e2e/list-inline.spec.ts
git commit -m "feat(frontend): inline edit of priority/assignee/status in the issue list"
```

---

### Task 5: Real pagination

**Files:**
- Modify: `frontend-next/app/app/filters/page.tsx` (thread nextPageToken/isLast + controls)
- Test: extend `frontend-next/e2e/search.spec.ts` or `e2e/list-bulk.spec.ts`

**Interfaces:** `search.jql(jql, { fields, nextPageToken, maxResults })` returns `{ issues, nextPageToken?, isLast }` (no total).

- [ ] **Step 1: Write the failing test** — a test that runs a broad query and, if `isLast` is false, clicks "Next" and asserts the page advanced (different first row or a page indicator increments). If the DEMO seed has ≤ maxResults issues, assert the "Next" control is disabled/absent and a "showing N" count renders — pick whichever is deterministic against the seed (prefer asserting the count label `data-testid="list-count"` exists).

- [ ] **Step 2: Run to verify it fails** — no pagination controls.

- [ ] **Step 3: Implement** — in `filters/page.tsx`, keep a `maxResults` (e.g. 25), track `nextPageToken` and a page stack for Prev; render Prev/Next buttons (Next disabled when `isLast`), and a `data-testid="list-count"` showing how many are loaded on the current page. Re-run `search.jql` with the token on Next; push/pop tokens for Prev.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/search.spec.ts --workers=1` → pass.

- [ ] **Step 5: Commit**

```bash
git add "frontend-next/app/app/filters/page.tsx" frontend-next/e2e/
git commit -m "feat(frontend): real cursor pagination + result count on the issue list"
```

---

### Task 6: Lightweight epic→children hierarchy (client-side)

**Files:**
- Modify: `frontend-next/components/search/SearchResults.tsx` (indent children under parents present in the result set)
- Test: extend `e2e/list-inline.spec.ts` or a small `e2e/list-hierarchy.spec.ts`

**Interfaces:** uses `SearchIssue.fields.parent?.key` (requested via `LIST_FIELDS` in Task 2). No new backend.

- [ ] **Step 1: Write the failing test** — seed dependency: DEMO-1 has a subtask (seeded in R13). Run a query returning both DEMO-1 and its child; assert the child row renders indented (a `data-testid="child-row"` or an indentation marker) beneath its parent. If ordering makes this flaky, assert only that a child row carries the `data-testid="child-row"` when its parent key is also in the results.

- [ ] **Step 2: Run to verify it fails** — no hierarchy rendering.

- [ ] **Step 3: Implement** — after fetching, group rows so that any issue whose `fields.parent?.key` is also present in the current result set renders directly under its parent with a visual indent and `data-testid="child-row"`; issues whose parent is absent render at top level as normal. This is purely presentational within the current page (no cross-page/epic fetch). Keep selection/inline-edit working on child rows.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/ --workers=1` (the list specs) → pass.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/components/search/SearchResults.tsx frontend-next/e2e/
git commit -m "feat(frontend): indent epic/parent children in the issue list"
```

---

### Task 7: Round close — gate, docs

- [ ] **Step 1: Full three-level gate**
```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test --workers=1 && cd ..
go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md
```
- [ ] **Step 2: CHANGELOG.md** — Added: bulk edit (row select + action bar), inline edit (priority/assignee/status), cursor pagination + count, parent/child indentation; new `POST /issues/bulk` endpoint. Note deferred: bulk status & sprint change.
- [ ] **Step 3: STATE.md** — Round 17 entry + follow-ups (bulk status cross-project; sprint bulk; true epic-link field vs parent_id; numbered pagination needs total which /search/jql lacks). Set "Prossimo: Round 18".
- [ ] **Step 4: Commit** docs + plan.
- [ ] **Step 5: Update auto-memory** (controller action).

---

## Self-Review

**Spec coverage (B.1):** inline edit → T4; row select + bulk bar → T3; bulk endpoint → T1; hierarchy → T6 (lightweight, client-side); pagination → T5. Columns configurable already exist (kept). Bulk status & sprint explicitly deferred. ✅

**Placeholder scan:** backend endpoint has exact body/response + code; UI tasks specify testids + behavior + components to reuse. No TBD.

**Type consistency:** `issues.bulk` body/response match the Go `bulkBody`/`bulkResult` JSON; `SearchIssue.fields` additions match the v3 issue field JSON keys; `issues.transitions` shape must be matched to the workflow handler (verify in T2).

**Cross-cutting risks:** the bulk endpoint MUST enforce per-issue in-handler (never rely on a router decorator) and report partial failures — this is the security-critical part; the T1 test must cover a forbidden/unknown key. Assignee clearing semantics: confirm whether `issue.Service.Update` can clear an assignee (pointer-to-empty) — if not, only support setting, and note it. Inline edit must not trigger row navigation. Full suite `--workers=1`.
