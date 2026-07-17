# Round 15 — Configurabilità (Automation UI, Custom fields UI, Workflow rules) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the ready-but-headless Automation-rule engine and Custom-field system into the UI, and close the workflow-transition-rule editing gap — fixing the `{projectID}`/`{issueID}`-UUID routing defect (which also silently bypasses authorization) that currently makes both features uncallable from the frontend.

**Architecture:** Automation (`internal/domain/automation`) and Custom-fields (`internal/domain/customfield`) domain services are complete. The blocking defect: their project-scoped routes key on `{projectID}` = internal UUID (and custom-values on `{issueID}` = issue UUID), which the v3 API never exposes (it emits seq_id/key). Worse, a seq_id path value makes the authz resolver return `ok=false`, and `Enforce` then passes through to the handler with NO permission check — so re-keying to `{key}`/`{issueIdOrKey}` + `chk.ByKey`/`chk.ByIssueParam` is both a functional and a security fix. Then add typed clients and UI (Project Settings tabs, dynamic custom-field rendering, workflow transition edit).

**Tech Stack:** Go 1.25 (`net/http` ServeMux, GORM, golang-migrate, in-memory SQLite tests), Next.js 16 App Router + React 19 + TanStack Query + Tailwind, Playwright.

## Global Constraints

- **Module path is `github.com/it4nodummies/heureum`** — verbatim in every Go import.
- **Two-ID rule:** the v3 API exposes seq_id/key, never the internal UUID (`internal/api/v3/project.go:52`, `internal/api/v3/issue.go:168`). Every frontend-facing project route MUST key on `{key}` and resolve to the UUID via `project.Service.GetByKey(...).ID`; issue-scoped routes MUST accept key-or-seqid via `chk.ByIssueParam` and resolve in-handler. Never expect the frontend to send a UUID.
- **Every mutating route stays permission-gated:** creates/updates/deletes go through `chk.Enforce(permission.AdministerProjects, resolver, ...)` (or `EditIssues` for custom-values); reads through `chk.EnforceNotFound(permission.BrowseProjects, resolver, ...)`. A resolver that returns `ok=false` makes `Enforce` pass through unchecked — so the resolver MUST resolve the real project for every legitimate request. Never register an unguarded route.
- **Automation & Custom-fields are Heureum-custom routes, NOT in the Jira OpenAPI spec** — changing their path params does not affect contract tests or `cmd/gapreport`.
- **Conditions/actions are freeform JSON strings** (`ConditionsJSON`/`ActionsJSON`), not typed structs. Supported condition keys: `priority`, `title_contains`. Supported action types: `set_assignee`, `add_label`, `transition_issue`, `add_comment`. Triggers: `issue_created`, `issue_updated`, `issue_transitioned`. The UI builder must only emit these.
- **Story points stay native:** `issues.story_points` surfaced via the compat alias `customfield_10016` — it is NOT a custom field. Never route `customfield_10016` through the new custom-field value APIs; the custom-field system is UUID-keyed and independent.
- UI accent `#0052cc`; UI under `/app`; single typed client `frontend-next/lib/api.ts`.
- Three-level gate before done: (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test --workers=1`; (3) `go run ./cmd/gapreport` → no diff.
- Conventional Commits; branch `feat/frontend-next`.

---

### Task 1: Re-key automation project routes to `{key}` (backend)

**Problem:** `GET/POST /rest/api/3/project/{projectID}/automation` use `{projectID}`=UUID via `chk.ByProjectID`. Re-key to `{key}` + `chk.ByKey`; resolve key→UUID in the handler. The rule-scoped routes (`/automation/{ruleID}`) already resolve project via the rule and are fine.

**Files:**
- Modify: `internal/api/handlers/automation_handler.go` (inject `*project.Service`; `ListRules`/`CreateRule` resolve `{key}`)
- Modify: `internal/api/router.go` (constructor call for `autoH`; routes at the two project-scoped automation lines)
- Test: `internal/api/handlers/automation_route_test.go` (create)

**Interfaces:**
- Consumes: `project.Service.GetByKey(key) (*project.Project, error)`; `automation.Service.ListRules(projectID)`, `CreateRule(projectID, name, triggerType, conditionsJSON, actionsJSON)`.
- Produces: `handlers.NewAutomationHandler(...)` gains a trailing `*project.Service` param; routes `GET/POST /rest/api/3/project/{key}/automation`.

- [ ] **Step 1: Write the failing compile-time wiring test**

Create `internal/api/handlers/automation_route_test.go`:

```go
package handlers_test

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/domain/automation"
	"github.com/it4nodummies/heureum/internal/domain/project"
)

// Compile-time guard: the automation handler must take a project service so it
// can resolve {key} -> UUID (the domain service keys on the project UUID).
func TestAutomationHandlerTakesProjectSvc(t *testing.T) {
	var as *automation.Service
	var ps *project.Service
	_ = mustNewAutomationHandler(as, ps)
}
```

Add a tiny local helper in the same file that calls the real constructor with whatever final signature Task expects, so the test fails to compile until the constructor is updated:

```go
func mustNewAutomationHandler(as *automation.Service, ps *project.Service) any {
	return newAutomationHandlerForTest(as, ps)
}
```

> Note: since `handlers.NewAutomationHandler` currently takes only `(*automation.Service)`, the cleanest failing test is to reference the NEW signature directly. Replace the two helpers above with a single direct call once you know the exact constructor name by reading `automation_handler.go`:
> ```go
> func TestAutomationHandlerTakesProjectSvc(t *testing.T) {
>     var as *automation.Service
>     var ps *project.Service
>     _ = handlers.NewAutomationHandler(as, ps) // must accept project svc
> }
> ```
> Use this direct form (delete the `mustNew…`/`newAutomationHandlerForTest` scaffolding). Add the `handlers` import.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handlers/ -run TestAutomationHandlerTakesProjectSvc`
Expected: FAIL to compile — `not enough arguments in call to handlers.NewAutomationHandler`.

- [ ] **Step 3: Implement**

In `internal/api/handlers/automation_handler.go`: add a `projectSvc *project.Service` field, update `NewAutomationHandler` to accept and store it (import `github.com/it4nodummies/heureum/internal/domain/project`). In `ListRules` and `CreateRule`, replace `projectID := r.PathValue("projectID")` with:

```go
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
```

then use `p.ID` everywhere the old `projectID` was used (the `svc.ListRules(p.ID)` / `svc.CreateRule(p.ID, ...)` calls).

In `internal/api/router.go`: update the `autoH := handlers.NewAutomationHandler(autoSvc)` call to pass `projectSvc`, and change the two routes:

```go
	mux.Handle("GET /rest/api/3/project/{key}/automation", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(autoH.ListRules))))
	mux.Handle("POST /rest/api/3/project/{key}/automation", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(autoH.CreateRule))))
```

- [ ] **Step 4: Verify**

Run: `go test ./internal/api/handlers/ -run TestAutomationHandlerTakesProjectSvc && go build ./... && go vet ./... && go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md`
Expected: test PASS, clean build/vet, and NO change to gap-report.md.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/automation_handler.go internal/api/router.go internal/api/handlers/automation_route_test.go
git commit -m "fix(automation): key project routes on project key, resolve UUID server-side (closes authz bypass)"
```

---

### Task 2: Automation client (frontend)

**Files:**
- Modify: `frontend-next/lib/api.ts` (add `AutomationRun` interface + automation methods)

**Interfaces:**
- Consumes: routes `GET/POST /project/{key}/automation`, `GET/PATCH/DELETE /automation/{ruleID}`, `POST /automation/{ruleID}/execute` (body `{issue_id}`), `GET /automation/{ruleID}/runs`.
- Produces: on the existing `integrations` object (which already has `automationRules`), add: `automationCreate(key, body)`, `automationGet(ruleId)`, `automationUpdate(ruleId, patch)`, `automationDelete(ruleId)`, `automationTest(ruleId, issueId)`, `automationRuns(ruleId)`. Fix `automationRules` to take `key` (not the UUID). Add `AutomationRun` interface `{id, rule_id, issue_id?, triggered_at, status, log}`.

- [ ] **Step 1: Write the failing test** — this is a pure client addition; its behavior is exercised by Task 3's E2E. Skip a unit test here; the deliverable is verified by `npm run build` (type-checks the new methods) and Task 3.

- [ ] **Step 2: Implement**

In `frontend-next/lib/api.ts`, add the interface near the existing `AutomationRule` (~line 1024):

```ts
export interface AutomationRun {
  id: string;
  rule_id: string;
  issue_id?: string;
  triggered_at: string;
  status: string; // success | skipped | error | test
  log: string;
}
```

In the `integrations` object, change `automationRules` to accept the project **key** and add the rest:

```ts
  automationRules: (key: string) =>
    apiFetch<AutomationRule[]>(`/rest/api/3/project/${key}/automation`),
  automationCreate: (
    key: string,
    body: { name: string; trigger_type: string; conditions_json: string; actions_json: string }
  ) =>
    apiFetch<AutomationRule>(`/rest/api/3/project/${key}/automation`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  automationGet: (ruleId: string) =>
    apiFetch<AutomationRule>(`/rest/api/3/automation/${ruleId}`),
  automationUpdate: (
    ruleId: string,
    patch: Partial<{ name: string; is_active: boolean; trigger_type: string; conditions_json: string; actions_json: string }>
  ) =>
    apiFetch<AutomationRule>(`/rest/api/3/automation/${ruleId}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  automationDelete: (ruleId: string) =>
    apiFetch<void>(`/rest/api/3/automation/${ruleId}`, { method: "DELETE" }),
  automationTest: (ruleId: string, issueId: string) =>
    apiFetch<AutomationRun>(`/rest/api/3/automation/${ruleId}/execute`, {
      method: "POST",
      body: JSON.stringify({ issue_id: issueId }),
    }),
  automationRuns: (ruleId: string) =>
    apiFetch<AutomationRun[]>(`/rest/api/3/automation/${ruleId}/runs`),
```

> If the existing `automationRules` had a different call site (e.g. `IntegrationsTab`), update that caller to pass the project key. Grep `automationRules(` and fix.

- [ ] **Step 3: Verify**

Run: `cd frontend-next && npx tsc --noEmit` (or `npm run build`) — clean.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): automation client (create/get/update/delete/test/runs)"
```

---

### Task 3: Automation tab in Project Settings (frontend)

**Files:**
- Create: `frontend-next/components/projects/AutomationTab.tsx`
- Modify: `frontend-next/components/projects/ProjectSettings.tsx` (add "Automation" tab)
- Test: `frontend-next/e2e/automation.spec.ts` (create)

**Interfaces:**
- Consumes: the automation client from Task 2; `ProjectSettings` passes `projectKey`.
- Produces: an Automation tab listing rules with an active toggle + delete, a create form (trigger select; conditions builder for `priority`/`title_contains`; actions builder for the 4 action types), and a per-rule runs view.

- [ ] **Step 1: Write the failing test**

Create `frontend-next/e2e/automation.spec.ts` (inline the login preamble from `e2e/export.spec.ts`):

```ts
import { test, expect } from "@playwright/test";
// inline login (no e2e/helpers.ts in repo) — copy the block from export.spec.ts

test("create and list an automation rule", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Automation" }).click();
  await expect(page.getByTestId("automation-tab")).toBeVisible();
  await page.getByRole("button", { name: /new rule|create rule/i }).click();
  await page.getByLabel(/rule name/i).fill("Auto-assign on create");
  // trigger select defaults to issue_created; submit
  await page.getByRole("button", { name: /^create$|save rule/i }).click();
  await expect(page.getByText("Auto-assign on create")).toBeVisible();
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd frontend-next && npx playwright test e2e/automation.spec.ts --workers=1`
Expected: FAIL — no Automation tab.

- [ ] **Step 3: Implement**

Create `frontend-next/components/projects/AutomationTab.tsx`. Requirements the component must satisfy (build it following the patterns in the existing `IntegrationsTab.tsx` and `WorkflowEditor.tsx`):
- Root element has `data-testid="automation-tab"`.
- `useQuery(["automation", projectKey], () => integrations.automationRules(projectKey))` — list each rule: name, `trigger_type`, an active toggle wired to `integrations.automationUpdate(rule.id, { is_active: !rule.is_active })`, and a delete button (`integrations.automationDelete`), each invalidating the list query.
- A "New rule" button reveals a create form: rule name input (`aria-label="Rule name"`); trigger `<select>` with the three triggers; a conditions section with add-condition rows (`<select>` field = `priority`|`title_contains`, value input) serialized to `conditions_json` (object keyed by field, e.g. `{"priority":"high"}`); an actions section with add-action rows (`<select>` type = the four actions, value input) serialized to `actions_json` (array of `{type,value}`). On submit call `integrations.automationCreate(projectKey, {name, trigger_type, conditions_json: JSON.stringify(condObj), actions_json: JSON.stringify(actArr)})`, invalidate, close form.
- Per-rule "View runs" expander: `useQuery` on `integrations.automationRuns(rule.id)`, render `triggered_at`, `status`, `log`.

In `ProjectSettings.tsx`: extend the tab union `"general" | "workflow" | "summary" | "integrations"` with `"automation"`, add a `<button>` labeled "Automation", and render `{tab === "automation" && <AutomationTab projectKey={projectKey} />}`.

- [ ] **Step 4: Verify**

Run: `cd frontend-next && npm run build && npx playwright test e2e/automation.spec.ts --workers=1`
Expected: build clean; test PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/components/projects/AutomationTab.tsx frontend-next/components/projects/ProjectSettings.tsx frontend-next/e2e/automation.spec.ts
git commit -m "feat(frontend): Automation rules tab in project settings"
```

---

### Task 4: Re-key custom-field routes to `{key}`/issue key + add `required` + broaden SetValue (backend)

**Problem:** custom-field project routes use `{projectID}`=UUID; custom-values use `{issueID}`=UUID. Re-key project routes to `{key}`+`chk.ByKey`, and custom-values routes to `{issueIdOrKey}`+`chk.ByIssueParam("issueIdOrKey")` (resolve the issue by key/seqid in-handler). Also add a `required bool` column to `custom_fields` (the UI needs it) and broaden `SetValue` to accept `date` (RFC3339 string → `ValueDate`) and `user` (accountId string → `ValueText`) so all six types have a write path.

**Files:**
- Create: `migrations/000017_custom_field_required.up.sql` / `.down.sql`
- Modify: `internal/domain/customfield/model.go` (add `Required bool`), `service.go` (CreateField signature + SetValue type handling)
- Modify: `internal/api/handlers/customfield_handler.go` (inject `*project.Service` + `*issue.Service`; resolve key/issue-key; parse `required`; date/user values)
- Modify: `internal/api/router.go` (constructor + routes)
- Test: `internal/domain/customfield/service_test.go` (create — required flag + date/user SetValue)

**Interfaces:**
- Consumes: `project.Service.GetByKey`, `issue.Service.GetByKey`/`GetBySeqID`.
- Produces: `customfield.Service.CreateField(projectID, name string, fieldType FieldType, required bool)`; `CustomField.Required bool`; `SetValue` handling for `date`/`user`; routes `GET/POST /project/{key}/custom-fields`, `GET /issue/{issueIdOrKey}/custom-values`, `PUT /issue/{issueIdOrKey}/custom-values/{fieldID}`.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/customfield/service_test.go`:

```go
package customfield

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&CustomField{}, &CustomFieldOption{}, &IssueCustomValue{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateFieldStoresRequired(t *testing.T) {
	svc := NewService(newDB(t))
	f, err := svc.CreateField("proj-1", "Team", FieldTypeText, true)
	if err != nil {
		t.Fatal(err)
	}
	if !f.Required {
		t.Errorf("Required = false, want true")
	}
	got, _ := svc.ListFields("proj-1")
	if len(got) != 1 || !got[0].Required {
		t.Errorf("ListFields lost Required: %+v", got)
	}
}

func TestSetValueDateAndUser(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	df, _ := svc.CreateField("proj-1", "Due", FieldTypeDate, false)
	uf, _ := svc.CreateField("proj-1", "Owner", FieldTypeUser, false)

	if err := svc.SetValue("iss-1", df.ID, "2026-03-10T00:00:00Z"); err != nil {
		t.Fatalf("date SetValue: %v", err)
	}
	if err := svc.SetValue("iss-1", uf.ID, "user-123"); err != nil {
		t.Fatalf("user SetValue: %v", err)
	}
	vals, _ := svc.GetValues("iss-1")
	byField := map[string]IssueCustomValue{}
	for _, v := range vals {
		byField[v.FieldID] = v
	}
	if byField[df.ID].ValueDate == nil || byField[df.ID].ValueDate.Format("2006-01-02") != "2026-03-10" {
		t.Errorf("date not stored: %+v", byField[df.ID])
	}
	if byField[uf.ID].ValueText != "user-123" {
		t.Errorf("user not stored: %+v", byField[uf.ID])
	}
	_ = time.Now
}
```

> Read the actual `SetValue`/`GetValues`/`SetOptionValue` signatures first; adapt the calls if they differ (e.g. `SetValue` may currently take `(issueID, fieldID string, value any)`). The test asserts the NEW date/user behavior.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/domain/customfield/ -v`
Expected: FAIL — `CreateField` arity / `Required` field / date-handling not present.

- [ ] **Step 3: Implement**

Add the migration `migrations/000017_custom_field_required.up.sql`:
```sql
ALTER TABLE custom_fields ADD COLUMN required BOOLEAN NOT NULL DEFAULT FALSE;
```
`.down.sql`:
```sql
ALTER TABLE custom_fields DROP COLUMN required;
```

In `model.go` add to `CustomField`: `Required bool `gorm:"not null;default:false" json:"required"``.

In `service.go`: change `CreateField` to accept `required bool` and set it; in `SetValue`, extend the type switch — for a field of type `date`, parse the string via `time.Parse(time.RFC3339, s)` into `ValueDate`; for type `user`, store the accountId string in `ValueText`. (Look up the field's type by `fieldID` inside `SetValue` to branch, mirroring the existing `SetOptionValue` path.) Keep `text`/`number`/`select`(option)/`multiselect`(option) working.

In `customfield_handler.go`: inject `*project.Service` and `*issue.Service`; in `ListFields`/`CreateField` resolve `{key}`→`p.ID`; parse `required` from the CreateField body (`{name, field_type, required}`); in `GetValues`/`SetValue` resolve `{issueIdOrKey}` to the issue UUID via `issueSvc.GetByKey` (fallback `GetBySeqID` if numeric) and pass that UUID to the service.

In `router.go`: update the `cfH := handlers.NewCustomFieldHandler(...)` constructor call to pass the project + issue services; change routes:
```go
	mux.Handle("GET /rest/api/3/project/{key}/custom-fields", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(cfH.ListFields))))
	mux.Handle("POST /rest/api/3/project/{key}/custom-fields", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(cfH.CreateField))))
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/custom-values", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(cfH.GetValues))))
	mux.Handle("PUT /rest/api/3/issue/{issueIdOrKey}/custom-values/{fieldID}", authMw(chk.Enforce(permission.EditIssues, chk.ByIssueParam("issueIdOrKey"), http.HandlerFunc(cfH.SetValue))))
```
Keep the field/option-scoped routes (`/custom-fields/{fieldID}...`) unchanged (they resolve via `ByCustomField`).

- [ ] **Step 4: Verify**

Run: `go test ./internal/domain/customfield/ -v && go build ./... && go vet ./... && go test ./... && go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md`
Expected: new tests PASS, full suite green, no gapreport diff. (Migrations run on server start; the in-memory tests AutoMigrate directly, so the new column is covered.)

- [ ] **Step 5: Commit**

```bash
git add migrations/000017_custom_field_required.up.sql migrations/000017_custom_field_required.down.sql internal/domain/customfield/model.go internal/domain/customfield/service.go internal/api/handlers/customfield_handler.go internal/api/router.go internal/domain/customfield/service_test.go
git commit -m "fix(customfield): key routes on project/issue key, add required flag, broaden SetValue (date/user)"
```

---

### Task 5: Custom-fields client (frontend)

**Files:**
- Modify: `frontend-next/lib/api.ts` (add `CustomField`/`CustomFieldOption`/`IssueCustomValue` interfaces + `customFields` client)

**Interfaces:**
- Produces: `customFields.list(key)`, `customFields.create(key, {name, field_type, required})`, `customFields.remove(fieldId)`, `customFields.options(fieldId)`, `customFields.addOption(fieldId, value)`, `customFields.removeOption(optionId)`, `customFields.values(issueKey)`, `customFields.setValue(issueKey, fieldId, {value?, option_id?})`.

- [ ] **Step 1: Implement**

Add to `frontend-next/lib/api.ts`:

```ts
export interface CustomField {
  id: string;
  project_id: string;
  name: string;
  field_type: "text" | "number" | "date" | "select" | "multiselect" | "user";
  required: boolean;
}
export interface CustomFieldOption { id: string; field_id: string; value: string; position: number; }
export interface IssueCustomValue {
  issue_id: string;
  field_id: string;
  value_text: string;
  value_number?: number;
  value_date?: string;
  option_id?: string;
}
export const customFields = {
  list: (key: string) => apiFetch<CustomField[]>(`/rest/api/3/project/${key}/custom-fields`),
  create: (key: string, body: { name: string; field_type: CustomField["field_type"]; required: boolean }) =>
    apiFetch<CustomField>(`/rest/api/3/project/${key}/custom-fields`, { method: "POST", body: JSON.stringify(body) }),
  remove: (fieldId: string) => apiFetch<void>(`/rest/api/3/custom-fields/${fieldId}`, { method: "DELETE" }),
  options: (fieldId: string) => apiFetch<CustomFieldOption[]>(`/rest/api/3/custom-fields/${fieldId}/options`),
  addOption: (fieldId: string, value: string) =>
    apiFetch<CustomFieldOption>(`/rest/api/3/custom-fields/${fieldId}/options`, { method: "POST", body: JSON.stringify({ value }) }),
  removeOption: (optionId: string) =>
    apiFetch<void>(`/rest/api/3/custom-fields/options/${optionId}`, { method: "DELETE" }),
  values: (issueKey: string) => apiFetch<IssueCustomValue[]>(`/rest/api/3/issue/${issueKey}/custom-values`),
  setValue: (issueKey: string, fieldId: string, body: { value?: unknown; option_id?: string }) =>
    apiFetch<void>(`/rest/api/3/issue/${issueKey}/custom-values/${fieldId}`, { method: "PUT", body: JSON.stringify(body) }),
};
```

- [ ] **Step 2: Verify** — `cd frontend-next && npx tsc --noEmit` clean.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): custom-fields client (fields/options/values)"
```

---

### Task 6: Custom fields settings tab (frontend)

**Files:**
- Create: `frontend-next/components/projects/CustomFieldsTab.tsx`
- Modify: `frontend-next/components/projects/ProjectSettings.tsx` (add "Fields" tab)
- Test: `frontend-next/e2e/customfields.spec.ts` (create)

**Interfaces:** consumes `customFields` client (Task 5); `ProjectSettings` passes `projectKey`.

- [ ] **Step 1: Write the failing test**

Create `frontend-next/e2e/customfields.spec.ts` (inline login):

```ts
test("create a custom field and see it listed", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Fields" }).click();
  await expect(page.getByTestId("custom-fields-tab")).toBeVisible();
  await page.getByLabel(/field name/i).fill("Team name");
  await page.getByRole("button", { name: /add field|create field/i }).click();
  await expect(page.getByText("Team name")).toBeVisible();
});
```

- [ ] **Step 2: Run to verify it fails** — `npx playwright test e2e/customfields.spec.ts --workers=1` → FAIL (no Fields tab).

- [ ] **Step 3: Implement**

Create `CustomFieldsTab.tsx` (root `data-testid="custom-fields-tab"`): list fields (`customFields.list(projectKey)`) with name, type, required badge, delete; a create form (name input `aria-label="Field name"`, type `<select>` of the 6 types, required checkbox → `customFields.create`); for `select`/`multiselect` fields, an inline options manager (list `customFields.options(fieldId)`, add via `addOption`, remove via `removeOption`). Invalidate the list query on each mutation.

In `ProjectSettings.tsx`: extend the tab union with `"fields"`, add a "Fields" button, render `{tab === "fields" && <CustomFieldsTab projectKey={projectKey} />}`.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/customfields.spec.ts --workers=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/components/projects/CustomFieldsTab.tsx frontend-next/components/projects/ProjectSettings.tsx frontend-next/e2e/customfields.spec.ts
git commit -m "feat(frontend): Custom fields tab in project settings"
```

---

### Task 7: Dynamic custom fields in issue create + detail (frontend)

**Files:**
- Modify: `frontend-next/components/issues/CreateIssueModal.tsx` (render project custom fields, required validation, submit values)
- Modify: `frontend-next/components/issues/IssueView.tsx` (Details: render + edit custom values)
- Test: `frontend-next/e2e/customfields.spec.ts` (extend with a create+set-value flow)

**Interfaces:** consumes `customFields.list/values/setValue` (Task 5). Note: `CreateIssueModal` gets `projectKey`; issue create returns a key usable to set values after creation. `IssueView` has `issueKey`.

- [ ] **Step 1: Write the failing test** — extend `customfields.spec.ts`:

```ts
test("issue detail shows and edits a custom field value", async ({ page }) => {
  await login(page);
  // assumes a text field exists on DEMO (create one via the Fields tab first, or seeded)
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Fields" }).click();
  await page.getByLabel(/field name/i).fill("Squad");
  await page.getByRole("button", { name: /add field|create field/i }).click();
  await expect(page.getByText("Squad")).toBeVisible();
  // open an issue and confirm the custom field renders in Details
  await page.goto("/app/projects/DEMO");
  await page.getByRole("cell", { name: /^DEMO-1$/ }).click();
  await expect(page.getByText("Squad")).toBeVisible();
});
```

- [ ] **Step 2: Run to verify it fails** — the DEMO-1 detail won't show "Squad" until IssueView renders custom fields.

- [ ] **Step 3: Implement**

`CreateIssueModal.tsx`: `useQuery(customFields.list(projectKey))`; render an input per field by type (text→text input, number→number input, date→date input, select→dropdown from `options(fieldId)`, multiselect→multi-select, user→reuse `UserPicker`), marking required fields with `*` and blocking submit if a required field is empty. After the issue is created (you have its key), for each filled field call `customFields.setValue(newIssueKey, fieldId, {value}|{option_id})`. Do NOT render or touch `customfield_10016` (story points stay native).

`IssueView.tsx`: `useQuery(customFields.list(issue.projectKey))` + `useQuery(customFields.values(issueKey))`; render each field in the Details panel with its current value; in Edit mode allow editing and persist via `customFields.setValue(issueKey, fieldId, ...)`. Keep the native story-points row (`customfield_10016`) exactly as is.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/customfields.spec.ts --workers=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/components/issues/CreateIssueModal.tsx frontend-next/components/issues/IssueView.tsx frontend-next/e2e/customfields.spec.ts
git commit -m "feat(frontend): dynamic custom fields in issue create + detail"
```

---

### Task 8: Workflow transition edit UI (B.7 gap)

**Problem:** The WorkflowEditor add-transition form already exposes `require_assignee`/`set_resolution` and the `workflow.updateTransition` client already sends them — but existing transitions can only be added/deleted, not edited. Add inline edit of a transition's name + the two rule flags.

**Files:**
- Modify: `frontend-next/components/workflow/WorkflowEditor.tsx`
- Test: `frontend-next/e2e/workflow.spec.ts` (extend)

**Interfaces:** consumes `workflow.updateTransition(projectKey, transitionId, {name?, require_assignee?, set_resolution?})` (already in `lib/api.ts`).

- [ ] **Step 1: Write the failing test** — add to `e2e/workflow.spec.ts`:

```ts
test("workflow editor edits a transition's require-assignee flag", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();
  // assumes at least one transition exists (seeded or created earlier in suite);
  // if none, create one first mirroring the existing "creates ... a transition" test
  await page.getByTestId("transition-edit").first().click();
  await page.getByLabel(/require assignee/i).check();
  await page.getByRole("button", { name: /save/i }).click();
  await expect(page.getByText(/require assignee/i).first()).toBeVisible();
});
```

> Read the current WorkflowEditor + workflow.spec.ts first; align testids/labels with what you implement.

- [ ] **Step 2: Run to verify it fails** — no `transition-edit` control yet.

- [ ] **Step 3: Implement** — in `WorkflowEditor.tsx`, add an "Edit" affordance (`data-testid="transition-edit"`) on each rendered transition that opens an inline form pre-filled with the transition's `name`/`require_assignee`/`set_resolution`; on save call `workflow.updateTransition(projectKey, t.id, {...})` and invalidate the workflow query. Reuse the existing add-form's checkbox markup.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/workflow.spec.ts --workers=1` → all pass.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/components/workflow/WorkflowEditor.tsx frontend-next/e2e/workflow.spec.ts
git commit -m "feat(frontend): edit workflow transition rules in-place (require_assignee/set_resolution)"
```

---

### Task 9: Round close — gate, docs, seed

**Files:** `CHANGELOG.md`, `docs/superpowers/STATE.md`; optionally `cmd/seed/main.go` (seed a demo automation rule + custom field on DEMO so the tabs aren't empty).

- [ ] **Step 1: (optional) Seed demo config** — in `cmd/seed/main.go`, idempotently create one automation rule (e.g. trigger `issue_created`, action `add_label`) and one text custom field on the DEMO project, so the new tabs render populated. Keep it idempotent (check-exists-first, like the R13 seed additions).

- [ ] **Step 2: Full three-level gate**

```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test --workers=1 && cd ..
go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md
```
Expected: all green; gap-report.md unchanged (or commit it if the seed/route changes altered counts — automation/customfield routes are extensions, so path-match count should be stable, but the `{key}` vs `{projectID}` rename does not change coverage).

- [ ] **Step 3: Update CHANGELOG.md** — under `## [Unreleased]`, Added: Automation rules tab, Custom fields (settings tab + dynamic rendering on create/detail), workflow transition in-place editing. Fixed: automation & custom-field routes re-keyed on project/issue key (closing an authz bypass where a seq_id path value skipped the permission check); custom-field `required` flag; `SetValue` date/user support.

- [ ] **Step 4: Update STATE.md** — add a Round 15 completed entry (mirror the R14 format): what shipped, the authz-bypass fix, new follow-ups (e.g. multiselect/date value UX limits, automation condition/action set still limited to the hardcoded keys, `ExecuteRule` handler actually runs `TestRule`). Set "Prossimo: Round 16".

- [ ] **Step 5: Commit**

```bash
git add CHANGELOG.md docs/superpowers/STATE.md cmd/seed/main.go docs/superpowers/plans/2026-07-17-round-15-configurabilita.md
git commit -m "docs: record Round 15 (automation UI, custom fields UI, workflow rules)"
```

- [ ] **Step 6: Update auto-memory** — note Round 15 complete in `~/.claude/projects/-Users-n0r41n-Development-open-jira/memory/jira-parity-rounds.md` (controller action).

---

## Self-Review

**Spec coverage:** A.4 Automation → T1 (route fix) + T2 (client) + T3 (UI). A.5 Custom fields → T4 (route fix + required + SetValue) + T5 (client) + T6 (settings tab) + T7 (dynamic render). B.7 workflow rules → T8 (edit UI; the add-form flags were already present). ✅

**Placeholder scan:** backend route/handler changes and client signatures are given exactly; UI components are specified by required behavior + testids + the existing components to mirror (the implementer reads those files) — acceptable given the component sizes; no TBD/TODO.

**Type consistency:** `CustomField.field_type` union matches the Go `FieldType` constants; `AutomationRun` matches the Go struct JSON tags; `customFields.setValue` body `{value?, option_id?}` matches the handler's `{value interface{}, option_id string}`; `NewAutomationHandler(as, ps)` / `NewCustomFieldHandler(..., projectSvc, issueSvc)` signatures match their wiring tests and router calls.

**Cross-cutting risks for the executor:**
- The authz-bypass note is the reason the backend re-keying is P0 — verify with a manual check that a POST with a valid key is gated (a viewer gets 403) if time permits; at minimum keep the wiring tests.
- E2E specs assume inline login (no `e2e/helpers.ts`); copy the block from `export.spec.ts`.
- Full Playwright suite must run `--workers=1`.
- Do not conflate `customfield_10016` (native story points) with the custom-field system.
