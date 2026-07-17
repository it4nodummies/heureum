# Round 16 — Amministrazione & condivisione (members/roles, groups, shared filters, dashboard gadgets) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Surface the ready-but-headless administration surfaces — project members/roles, global groups, filter sharing, and dashboard gadgets — in the UI, closing the two small backend gaps (filter `is_shared` is hardcoded off at the HTTP layer; `ListMembers` returns no user info) that block a usable admin experience.

**Architecture:** All Round-16 routes are already frontend-reachable (members/invites are `{key}`-keyed; groups use `?groupname=` query params; filters/dashboards use their own UUIDs returned by list endpoints with owner checks inside the handlers) — so unlike R14/R15 there is no UUID-routing blocker. The work is: two thin backend fixes (parse/expose `is_shared`; hydrate member user info), a batch of typed client additions, and four UI surfaces (an Access/People settings tab, a global Groups admin page, filter-sharing controls, and dashboard gadget add/remove).

**Tech Stack:** Go 1.25 (`net/http` ServeMux, GORM, in-memory SQLite tests), Next.js 16 App Router + React 19 + TanStack Query + Tailwind, Playwright.

## Global Constraints

- **Module path `github.com/it4nodummies/heureum`** — verbatim in every Go import.
- **Two-ID rule:** frontend-facing project routes key on `{key}`; these Round-16 routes already comply (members `{key}`, groups `?groupname=`, filters/dashboards own-UUID). Do not introduce a `{projectID}`-UUID route.
- **Permission gating stays intact:** members list `EnforceNotFound(BrowseProjects, ByKey)`, member mutations `Enforce(AdministerProjects, ByKey)`; group mutations `EnforceGlobalAdmin`; filter/dashboard mutations owner-checked in-handler. Never weaken a gate.
- **`ListMembers` hydration must not leak email** beyond the existing rule: email is visible only to the user themselves and to global admins (mirror how `v3.JiraUser`/the group `Members` handler already gates `emailAddress` via omitempty). Reuse the existing user→`v3.JiraUser` mapping; do not hand-roll a new one that always includes email.
- **Groups & filter/dashboard routes are Jira-shaped or Heureum-custom** — none are absent from the contract in a way this round changes; run `cmd/gapreport` and expect no diff.
- UI accent `#0052cc`; UI under `/app`; single typed client `frontend-next/lib/api.ts`.
- Three-level gate before done: (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test --workers=1`; (3) `go run ./cmd/gapreport` → no diff.
- Conventional Commits; branch `feat/frontend-next`. E2E specs inline the login preamble from `e2e/export.spec.ts` (no `e2e/helpers.ts`). The demo admin (`admin@example.com`) is a global admin, so group-admin and member-admin flows are exercisable in E2E.

---

### Task 1: Backend gaps — filter `is_shared` + member user hydration

**Problem A (filters):** `filter_handler.go` parses a `filterBody{Name,Description,JQL}` with no `is_shared`, calls `Create(..., false)` and `Update(..., nil)`, and `toFilter` omits the flag from the response. The service already supports `isShared`.
**Problem B (members):** `ListMembers` returns raw `[]ProjectMember` (`{project_id,user_id,role}`) — no display name/email/avatar, so the UI can't render a people list without N lookups. Hydrate each member into a shape carrying the user's `v3.JiraUser` fields + role, mirroring the group `Members` handler.

**Files:**
- Modify: `internal/api/handlers/filter_handler.go` (parse + pass + expose `is_shared`)
- Modify: `internal/api/v3/*` filter DTO (add `isShared` to the filter wire shape) — find the type behind `toFilter`
- Modify: `internal/api/handlers/project_handler.go` (`ListMembers` hydrates user info)
- Test: `internal/api/handlers/filter_share_test.go` (create) and a member-hydration assertion (extend an existing project/member handler test if one exists, else add `project_members_hydrate_test.go`)

**Interfaces:**
- Consumes: `search.FilterService.Create(ownerID, projectID, name, desc, jql string, isShared bool)`, `Update(id, name, desc, jql string, isShared *bool)`; the existing user→`v3.JiraUser` mapper used by the group `Members` handler; `project.Service.ListMembers(projectID)`.
- Produces: filter create/update accept+echo `is_shared`; `GET /project/{key}/members` returns `[]{ ...JiraUser fields, "role": "admin"|"member"|"viewer" }`.

- [ ] **Step 1: Write the failing tests**

Read `internal/api/handlers/filter_handler.go` (the `filterBody`, `Create`, `Update`, `toFilter`) and how existing handler tests spin up a server (look for `newTestServer`/`newTestServerDB`/`promoteAdmin` helpers in `internal/api/handlers/*_test.go` or `internal/contract`). Write a handler-level test that POSTs a filter with `{"is_shared": true}` and asserts the response JSON has `isShared: true` (or the exact wire key `toFilter` uses), and that a GET reflects it. If a full HTTP test harness isn't readily reusable, instead write a **service-plus-mapping** test: call `FilterService.Create(..., true)` then map via the same `toFilter` helper and assert the DTO exposes the flag.

For members: write a test that seeds a project with a member user (with a display name), calls the `ListMembers` HTTP handler (or the response-building helper), and asserts the returned JSON includes the member's `displayName` and `role`.

> Because the exact test harness varies, the implementer MUST read a sibling `*_handler` test first and match its setup; do not invent a new harness.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/api/handlers/ -run 'Filter|Member' -v`
Expected: FAIL — `is_shared` not parsed/exposed; member response lacks `displayName`.

- [ ] **Step 3: Implement**

Filters: add `IsShared *bool `json:"is_shared"`` to `filterBody`; in `Create` pass `b.IsShared != nil && *b.IsShared` (or default false); in `Update` pass `b.IsShared` through as the `*bool`; add the `isShared` field to the v3 filter DTO and set it in `toFilter`.

Members: in `ListMembers`, for each `ProjectMember` look up the user and build a response element that embeds the user's `v3.JiraUser` mapping plus `role` (reuse the same user-lookup + mapper the group `Members` handler uses — find it in `group_handler.go` and share/copy the mapping call, honoring the email-visibility rule). Return the hydrated slice.

- [ ] **Step 4: Verify**

Run: `go test ./internal/api/handlers/ -run 'Filter|Member' -v && go build ./... && go vet ./... && go test ./... && go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md`
Expected: new tests PASS, full suite green, no gapreport diff.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/filter_handler.go internal/api/handlers/project_handler.go internal/api/v3/ internal/api/handlers/filter_share_test.go
git commit -m "feat(admin): expose filter is_shared at the HTTP layer and hydrate project member user info"
```

---

### Task 2: Admin clients (frontend)

**Files:** Modify `frontend-next/lib/api.ts`.

**Interfaces / Produces:**
- `projects.members`: `list(key) → ProjectMember[]` (hydrated `{accountId, displayName, emailAddress?, avatarUrls?, role}`), `add(key, {user_id, role})`, `remove(key, userId)`, `invite(key, {email, role}) → {token}`. Add a `ProjectMember` interface.
- `groups`: add `get(groupname)`, `del(groupname)`, `members(groupname) → PageBean<JiraUser>`, `addUser(groupname, accountId)`, `removeUser(groupname, accountId)` (keep existing `picker`, `create`).
- `filters`: add `update(id, {name?, jql?, description?, is_shared?})`; add optional `is_shared` to `create`.
- `dashboards`: add `addWidget(id, {widget_type, config_json})`, `removeWidget(id, widgetId)`.

- [ ] **Step 1: Implement** — add the interfaces + methods, matching the exact routes/params from the scout:
  - members: `GET/POST /rest/api/3/project/${key}/members`, `DELETE /rest/api/3/project/${key}/members/${userId}`, `POST /rest/api/3/project/${key}/invites`.
  - groups: `GET /rest/api/3/group?groupname=`, `DELETE /rest/api/3/group?groupname=`, `GET /rest/api/3/group/member?groupname=`, `POST /rest/api/3/group/user?groupname=` (body `{accountId}`), `DELETE /rest/api/3/group/user?groupname=&accountId=`.
  - filters: `PUT /rest/api/3/filter/${id}` (body `{name, jql, description, is_shared}`); `create` adds `is_shared` to its body.
  - dashboards: `POST /rest/api/3/dashboards/${id}/widgets` (body `{widget_type, config_json}`), `DELETE /rest/api/3/dashboards/${id}/widgets/${widgetId}`.
  - For the add-member user picker, confirm `profile.searchUsers(query)` (global `/user/search`) exists and export it conveniently if needed (do NOT use `users.assignableSearch` — it only returns existing members).

- [ ] **Step 2: Verify** — `cd frontend-next && npx tsc --noEmit` clean.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): admin clients (members, groups, filter update/share, dashboard gadgets)"
```

---

### Task 3: Access / People tab in Project Settings

**Files:**
- Create: `frontend-next/components/projects/AccessTab.tsx`
- Modify: `frontend-next/components/projects/ProjectSettings.tsx` (add "Access" tab)
- Test: `frontend-next/e2e/access.spec.ts` (create)

**Interfaces:** consumes `projects.members.*` (Task 2) + a global user search (`profile.searchUsers`).

- [ ] **Step 1: Write the failing test**

`e2e/access.spec.ts` (inline login): go to `/app/projects/DEMO/settings`, click "Access", assert `data-testid="access-tab"` visible and that at least one member row renders (the DEMO admin is a member).

```ts
test("access tab lists project members", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Access" }).click();
  await expect(page.getByTestId("access-tab")).toBeVisible();
  await expect(page.getByTestId("member-row").first()).toBeVisible();
});
```

- [ ] **Step 2: Run to verify it fails** — no Access tab.

- [ ] **Step 3: Implement**

`AccessTab.tsx` (root `data-testid="access-tab"`): list members (`projects.members.list(projectKey)`) each row `data-testid="member-row"` showing displayName (fallback accountId), email if present, and a role `<select>` (admin/member/viewer) that calls `projects.members.add(projectKey, {user_id, role})` (upsert = change-role) on change, plus a Remove button (`projects.members.remove`). An "Add member" control: a search input over `profile.searchUsers(query)` (debounced, like `UserPicker`) to pick a non-member, a role select, and Add → `projects.members.add`. Invalidate `["members", projectKey]` on every mutation.

In `ProjectSettings.tsx`: extend the tab union with `"access"`, add an "Access" button, render `<AccessTab projectKey={projectKey} />`.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/access.spec.ts --workers=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/components/projects/AccessTab.tsx frontend-next/components/projects/ProjectSettings.tsx frontend-next/e2e/access.spec.ts
git commit -m "feat(frontend): Access/People tab (project members + roles) in settings"
```

---

### Task 4: Groups admin page

**Files:**
- Create: `frontend-next/app/app/groups/page.tsx` (+ a component if you prefer, e.g. `components/admin/GroupsAdmin.tsx`)
- Modify: sidebar/nav so the page is reachable (find the sidebar component; if "Groups" isn't there, add a link — gate it visually for admins if the nav supports it, else just add the route)
- Test: `frontend-next/e2e/groups.spec.ts` (create)

**Interfaces:** consumes `groups.*` (Task 2). There is no "list all groups" endpoint — drive the list off `groups.picker("")` (empty query returns all/most) or the search route.

- [ ] **Step 1: Write the failing test**

`e2e/groups.spec.ts` (inline login): navigate to `/app/groups`, create a group, assert it appears; add/remove flows optional but assert create+list at minimum.

```ts
test("create a group and see it listed", async ({ page }) => {
  await login(page);
  await page.goto("/app/groups");
  await expect(page.getByTestId("groups-admin")).toBeVisible();
  await page.getByLabel(/group name/i).fill("qa-team");
  await page.getByRole("button", { name: /create group/i }).click();
  await expect(page.getByText("qa-team")).toBeVisible();
});
```

- [ ] **Step 2: Run to verify it fails** — no `/app/groups` route.

- [ ] **Step 3: Implement**

Create the page (root `data-testid="groups-admin"`): list groups via `groups.picker("")`; a create form (name `aria-label="Group name"` → `groups.create`); per-group expand to view members (`groups.members(name)`), add user (global user search → `groups.addUser(name, accountId)`), remove user (`groups.removeUser`), and delete group (`groups.del(name)`). Invalidate the groups query on mutations. Add a "Groups" entry to the app sidebar/nav linking `/app/groups` (read the existing sidebar component first; match its link pattern — several not-yet-built entries are rendered as disabled "Coming soon", so replace/enable appropriately or add a new admin link).

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/groups.spec.ts --workers=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add "frontend-next/app/app/groups/page.tsx" frontend-next/e2e/groups.spec.ts
git commit -m "feat(frontend): global Groups admin page (create/list/members)"
```

---

### Task 5: Shared filters UI

**Files:**
- Modify: `frontend-next/app/app/filters/page.tsx` (share toggle on save, shared/private indicator, edit/delete)
- Test: `frontend-next/e2e/search.spec.ts` (extend) or a new `e2e/filters-share.spec.ts`

**Interfaces:** consumes `filters.create(name, jql, {description?, is_shared?})`, `filters.update`, `filters.del` (Task 2).

- [ ] **Step 1: Write the failing test** — add to `e2e/search.spec.ts` (it already has a save-filter test) or create `filters-share.spec.ts`:

```ts
test("saves a shared filter and shows a shared indicator", async ({ page }) => {
  await login(page);
  await page.goto("/app/filters");
  await page.getByRole("textbox", { name: /jql|query/i }).first().fill("project = DEMO");
  await page.getByRole("button", { name: /save/i }).click();
  // a save dialog with name + share toggle
  await page.getByLabel(/filter name/i).fill("Shared DEMO");
  await page.getByLabel(/share with (the )?team/i).check();
  await page.getByRole("button", { name: /^save$|create/i }).click();
  await expect(page.getByText("Shared DEMO")).toBeVisible();
  await expect(page.getByTestId("filter-shared-badge").first()).toBeVisible();
});
```

> Read `app/app/filters/page.tsx` first — the current Save uses `prompt()`. Replace the `prompt` with a small inline save form (name input `aria-label="Filter name"` + a "Share with team" checkbox) so the test can drive it. Align the test selectors with what you build.

- [ ] **Step 2: Run to verify it fails** — no share toggle / badge.

- [ ] **Step 3: Implement** — replace the `prompt()`-based save with an inline form (name + "Share with team" checkbox) calling `filters.create(name, jql, { is_shared })`. In the saved-filters sidebar list, render a `data-testid="filter-shared-badge"` next to shared filters, and add edit (rename / toggle share via `filters.update`) and delete (`filters.del`) controls. Invalidate the filters list on mutations.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/search.spec.ts --workers=1` (and the new test) → all pass.

- [ ] **Step 5: Commit**

```bash
git add "frontend-next/app/app/filters/page.tsx" frontend-next/e2e/
git commit -m "feat(frontend): shared filters — share toggle, shared badge, edit/delete"
```

---

### Task 6: Dashboard gadgets add/remove

**Files:**
- Modify: `frontend-next/app/app/dashboards/[id]/page.tsx`
- Test: `frontend-next/e2e/dashboards.spec.ts` (extend the existing dashboards test, or the reports/dashboards spec)

**Interfaces:** consumes `dashboards.get(id)`, `dashboards.addWidget(id, {widget_type, config_json})`, `dashboards.removeWidget(id, widgetId)` (Task 2). Supported/hydrated gadget types are `assigned_to_me` and `activity_stream` — the "catalog" is these two (others would render as raw JSON, so don't offer them).

- [ ] **Step 1: Write the failing test** — extend the dashboards E2E: open a dashboard, click "Add gadget", pick "Assigned to me", assert a gadget card appears; then remove it.

```ts
test("adds and removes a dashboard gadget", async ({ page }) => {
  await login(page);
  await page.goto("/app/dashboards");
  // open or create a dashboard, then:
  await page.getByRole("button", { name: /add gadget/i }).click();
  await page.getByRole("option", { name: /assigned to me/i }).click(); // or a select
  await expect(page.getByTestId("gadget").filter({ hasText: /assigned to me/i }).first()).toBeVisible();
});
```

> Read the current `dashboards/[id]/page.tsx` and the dashboards list page first; align selectors and the navigate-to-a-dashboard flow with what exists (the existing `e2e/reports.spec.ts` already covers "dashboards page lists and creates a dashboard" — reuse that entry path).

- [ ] **Step 2: Run to verify it fails** — no Add gadget control.

- [ ] **Step 3: Implement** — on the dashboard detail page add an "Add gadget" control offering the two supported types (`assigned_to_me` → "Assigned to me", `activity_stream` → "Activity stream"); on select call `dashboards.addWidget(id, {widget_type, config_json: "{}"})` and invalidate the dashboard query. Add a remove control (`data-testid="gadget"` on each card + a Remove button → `dashboards.removeWidget`). Keep the existing typed rendering for the two hydrated types.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/reports.spec.ts e2e/dashboards.spec.ts --workers=1` (whichever exists) → pass.

- [ ] **Step 5: Commit**

```bash
git add "frontend-next/app/app/dashboards/[id]/page.tsx" frontend-next/e2e/
git commit -m "feat(frontend): dashboard gadget add/remove from the supported catalog"
```

---

### Task 7: Round close — gate, docs

**Files:** `CHANGELOG.md`, `docs/superpowers/STATE.md`.

- [ ] **Step 1: Full three-level gate**

```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test --workers=1 && cd ..
go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md
```
Expected: all green; no gapreport diff.

- [ ] **Step 2: Update CHANGELOG.md** — Unreleased/Added: Access/People tab (members + roles), Groups admin page, shared filters (toggle + badge + edit/delete), dashboard gadget add/remove. Fixed: filter `is_shared` now settable via the API (was hardcoded off); project member list now returns user display info.

- [ ] **Step 3: Update STATE.md** — Round 16 completed entry (what shipped + the two backend gaps closed); note remaining follow-ups (no accept-invite route; no "list all groups" endpoint — UI uses picker; only two gadget types hydrated; dashboard `position_json` unused). Set "Prossimo: Round 17".

- [ ] **Step 4: Commit**

```bash
git add CHANGELOG.md docs/superpowers/STATE.md docs/superpowers/plans/2026-07-17-round-16-amministrazione-condivisione.md
git commit -m "docs: record Round 16 (members/roles, groups, shared filters, dashboard gadgets)"
```

- [ ] **Step 5: Update auto-memory** — note Round 16 complete (controller action).

---

## Self-Review

**Spec coverage:** A.8 members/roles → T1 (hydration) + T2 (client) + T3 (Access tab); A.8 groups → T2 (client) + T4 (Groups page); A.9 shared filters → T1 (backend is_shared) + T2 (client) + T5 (UI); A.10 dashboard gadgets → T2 (client) + T6 (UI). ✅

**Placeholder scan:** backend changes and client signatures are exact; UI components specified by behavior + testids + the sibling components to mirror. No TBD.

**Type consistency:** `ProjectMember` (hydrated) shape is produced by T1 and consumed by T2/T3; `filters.update`/`create` `is_shared` param matches the T1 handler body key `is_shared`; `dashboards.addWidget` body `{widget_type, config_json}` matches the handler; group client params (`?groupname=`, body `{accountId}`) match the routes.

**Cross-cutting risks:** the demo admin is a global admin, so group-admin + member-admin E2E flows work; `ListMembers` hydration must respect email-visibility (reuse the existing mapper); the add-member picker must use the GLOBAL user search, not `assignableSearch` (members-only); the only "list groups" path is `picker("")`; run the full suite `--workers=1`.
