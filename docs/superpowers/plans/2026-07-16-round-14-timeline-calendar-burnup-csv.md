# Round 14 — Timeline, Calendar, Burnup, CSV Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire four already-backed-by-the-backend Jira-parity features into usable UI — a project Timeline (Gantt), a Calendar, a Burnup report, and a working CSV export — fixing the three backend defects that currently make them unusable (Timeline/Calendar require an internal UUID the frontend never has; CSV writes raw UUIDs instead of names; the reports page hardcodes board `1`).

**Architecture:** These are "cablaggio" (wiring) features — the domain services (`internal/domain/timeline`, `internal/domain/calendar`, `internal/domain/report`) already exist. The work is: (1) fix the route/param and name-resolution defects in Go, TDD'd at the domain-service level against in-memory SQLite; (2) add typed clients in `frontend-next/lib/api.ts`; (3) build dependency-free SVG UI pages under the project-key route space and hang new tabs off the shared `ProjectHeader`. No new charting library, no new migration.

**Tech Stack:** Go 1.25 (`net/http` ServeMux, GORM, in-memory SQLite tests), Next.js 16 App Router + React 19 + TanStack Query + Tailwind, hand-rolled SVG charts, Playwright E2E.

## Global Constraints

- **Module path is `github.com/it4nodummies/heureum`** — use it verbatim in every Go import.
- **Two-ID rule:** the v3 API exposes the project **seq_id** as `id` (`internal/api/v3/project.go:52`), never the internal UUID. Frontend-facing routes MUST key on `{key}` (project key) and resolve to the UUID server-side via `project.Service.GetByKey(...).ID` — never expect the frontend to send a UUID.
- **Every project-scoped route stays permission-gated** through `chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, ...)` (404 for non-members, no existence leak). Never register an unguarded route.
- **UI accent color is `#0052cc`**; UI lives under the `/app` path prefix; the single typed client is `frontend-next/lib/api.ts` (no ad-hoc `fetch` in components — the one exception is the bearer-auth blob-download helper, which must live *inside* `lib/api.ts`).
- **Timeline/Calendar are Heureum-custom routes, NOT in the Jira OpenAPI spec** — changing their path param does not affect contract tests or `cmd/gapreport`.
- **Three-level quality gate must pass before the round is done:** (1) `go build ./... && go vet ./... && go test ./...`, (2) `cd frontend-next && npm run build && npx playwright test --workers=1`, (3) `go run ./cmd/gapreport` produces NO diff.
- Conventional Commits (`feat(scope): …`, `fix(scope): …`, `docs: …`); branch is `feat/frontend-next`.

---

### Task 1: CSV export — resolve names instead of UUIDs (backend)

**Problem:** `IssueHandler.ExportCSV` (`internal/api/handlers/issue_handler.go:129-166`) writes `*iss.StatusID`, `*iss.TypeID`, `*iss.AssigneeID` — raw UUIDs — into the Status/Type/Assignee columns. Fix at the domain layer with a name-resolving query so the handler stays thin and the logic is unit-testable.

**Files:**
- Modify: `internal/domain/issue/service.go` (add `ExportRow` type + `ExportRows` method)
- Modify: `internal/api/handlers/issue_handler.go:129-166` (use `ExportRows`)
- Test: `internal/domain/issue/export_test.go` (create)

**Interfaces:**
- Produces: `issue.ExportRow{Key, Title, Priority, Status, Type, Assignee string; StoryPoints int; Created, Updated time.Time}` and `func (s *Service) ExportRows(projectID string) ([]ExportRow, error)`.
- Consumes (Task 2 relies on): the CSV wire columns stay exactly `Key,Title,Priority,Status,Type,Assignee,Story Points,Created,Updated`.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/issue/export_test.go`. The `issue` package test DB only migrates issue tables, so seed the joined tables (`workflow_statuses`, `issue_types`, `users`) with raw SQL:

```go
package issue

import (
	"testing"
)

func TestExportRowsResolvesNames(t *testing.T) {
	db := newIssueTestDB(t)
	// Joined tables live in other domains; create minimal shapes via raw SQL.
	db.Exec(`CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, name TEXT)`)
	db.Exec(`CREATE TABLE issue_types (id TEXT PRIMARY KEY, name TEXT)`)
	db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, display_name TEXT, email TEXT)`)
	db.Exec(`INSERT INTO workflow_statuses (id,name) VALUES ('st-1','In Progress')`)
	db.Exec(`INSERT INTO issue_types (id,name) VALUES ('ty-1','Bug')`)
	db.Exec(`INSERT INTO users (id,display_name,email) VALUES ('u-1','Ada Lovelace','ada@example.com')`)

	svc := NewService(db)
	iss, err := svc.Create("DEMO", "proj-1", "Broken login", "", PriorityHigh, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	st, ty, as := "st-1", "ty-1", "u-1"
	db.Model(&Issue{}).Where("id = ?", iss.ID).
		Updates(map[string]any{"status_id": &st, "type_id": &ty, "assignee_id": &as, "story_points": 5})

	rows, err := svc.ExportRows("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.Status != "In Progress" || r.Type != "Bug" || r.Assignee != "Ada Lovelace" {
		t.Errorf("names not resolved: status=%q type=%q assignee=%q", r.Status, r.Type, r.Assignee)
	}
	if r.Priority != "high" || r.StoryPoints != 5 || r.Key == "" {
		t.Errorf("scalar fields wrong: %+v", r)
	}
}

func TestExportRowsUnassignedIsBlank(t *testing.T) {
	db := newIssueTestDB(t)
	db.Exec(`CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, name TEXT)`)
	db.Exec(`CREATE TABLE issue_types (id TEXT PRIMARY KEY, name TEXT)`)
	db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, display_name TEXT, email TEXT)`)
	svc := NewService(db)
	if _, err := svc.Create("DEMO", "proj-1", "No metadata", "", PriorityMedium, nil, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ExportRows("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Status != "" || rows[0].Type != "" || rows[0].Assignee != "" {
		t.Errorf("nil FKs should resolve to empty strings, got %+v", rows[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/issue/ -run TestExportRows -v`
Expected: FAIL — `svc.ExportRows undefined (type *Service has no field or method ExportRows)`.

- [ ] **Step 3: Write minimal implementation**

Add to `internal/domain/issue/service.go` (ensure `time` is imported — it already is for other methods):

```go
// ExportRow is a flattened, human-readable issue row for CSV export: the
// status/type/assignee foreign keys are resolved to their display names here
// (via LEFT JOINs) so the handler never leaks raw UUIDs. Unassigned or
// unresolved FKs come back as empty strings.
type ExportRow struct {
	Key         string
	Title       string
	Priority    string
	Status      string
	Type        string
	Assignee    string
	StoryPoints int
	Created     time.Time
	Updated     time.Time
}

// ExportRows returns every issue in the project (ordered by seq_id) with FK
// names resolved. Assignee falls back to email when display_name is blank.
func (s *Service) ExportRows(projectID string) ([]ExportRow, error) {
	var rows []ExportRow
	err := s.db.Raw(`
		SELECT i.key AS "key", i.title AS title, i.priority AS priority,
			COALESCE(ws.name, '') AS status,
			COALESCE(it.name, '') AS type,
			COALESCE(NULLIF(u.display_name, ''), u.email, '') AS assignee,
			i.story_points AS story_points,
			i.created_at AS created, i.updated_at AS updated
		FROM issues i
		LEFT JOIN workflow_statuses ws ON i.status_id = ws.id
		LEFT JOIN issue_types it ON i.type_id = it.id
		LEFT JOIN users u ON i.assignee_id = u.id
		WHERE i.project_id = ?
		ORDER BY i.seq_id ASC
	`, projectID).Scan(&rows).Error
	return rows, err
}
```

Then replace the body of `ExportCSV` in `internal/api/handlers/issue_handler.go:129-166` with:

```go
func (h *IssueHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	rows, err := h.svc.ExportRows(p.ID)
	if err != nil {
		http.Error(w, `{"error":"export failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-issues.csv", p.Key))
	wr := csv.NewWriter(w)
	wr.Write([]string{"Key", "Title", "Priority", "Status", "Type", "Assignee", "Story Points", "Created", "Updated"})
	for _, row := range rows {
		wr.Write([]string{
			row.Key,
			row.Title,
			row.Priority,
			row.Status,
			row.Type,
			row.Assignee,
			fmt.Sprintf("%d", row.StoryPoints),
			row.Created.Format("2006-01-02"),
			row.Updated.Format("2006-01-02"),
		})
	}
	wr.Flush()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/domain/issue/ -run TestExportRows -v && go build ./...`
Expected: PASS (both tests) and a clean build. If the build complains about an unused import in `issue_handler.go`, remove it — the old body may have used helpers now gone.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/issue/service.go internal/domain/issue/export_test.go internal/api/handlers/issue_handler.go
git commit -m "fix(export): resolve status/type/assignee names in CSV export instead of UUIDs"
```

---

### Task 2: CSV export button (frontend)

**Files:**
- Modify: `frontend-next/lib/api.ts` (add `issues.exportCsv`)
- Modify: `frontend-next/components/projects/ProjectOverview.tsx` (add the button)
- Test: `frontend-next/e2e/export.spec.ts` (create)

**Interfaces:**
- Consumes: `GET /rest/api/3/project/{key}/issues/export` (Task 1), which returns `text/csv` with `Content-Disposition: attachment`.
- Produces: `issues.exportCsv(key: string): Promise<void>` — fetches with the bearer token and triggers a browser download.

- [ ] **Step 1: Write the failing test**

Create `frontend-next/e2e/export.spec.ts`. It logs in, opens the DEMO project overview, clicks Export CSV, and asserts a download fires (bearer-auth blob path, so a naked `<a href>` would 401 — the test proves the helper works):

```ts
import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test("exports the project issues as CSV", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO");
  await expect(page.getByRole("heading", { name: "Recent issues" })).toBeVisible();
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: "Export CSV" }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe("DEMO-issues.csv");
});
```

> If `./helpers` has no `login` export, mirror the login flow used at the top of an existing spec (e.g. `e2e/board.spec.ts`) inline instead of importing.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend-next && npx playwright test e2e/export.spec.ts --workers=1`
Expected: FAIL — no "Export CSV" button exists yet (`getByRole('button', { name: 'Export CSV' })` times out).

- [ ] **Step 3: Write minimal implementation**

Add a method to the `issues` object in `frontend-next/lib/api.ts` (the object starts at line 263; add alongside the others, following the blob pattern already used by `attachments.contentBlobUrl` at line 1124). `BASE_URL`, `authHeaders`, and `handleUnauthorized` are already in scope in this file:

```ts
  // CSV export must go through fetch + bearer header (auth is header-based, so a
  // plain <a href> would 401). Mirrors attachments.contentBlobUrl: fetch bytes,
  // wrap in an object URL, click a synthetic anchor, then revoke.
  exportCsv: async (key: string): Promise<void> => {
    const res = await fetch(`${BASE_URL}/rest/api/3/project/${key}/issues/export`, {
      headers: authHeaders(),
    });
    handleUnauthorized(res);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${key}-issues.csv`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },
```

In `frontend-next/components/projects/ProjectOverview.tsx`, import `issues` and change the "Recent issues" heading (currently `<h2 ...>Recent issues</h2>` at line 93) into a flex row with the button. Replace line 93 with:

```tsx
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-[#1a1f36]">Recent issues</h2>
          <button
            onClick={() => issues.exportCsv(projectKey)}
            className="rounded border border-slate-300 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50"
          >
            Export CSV
          </button>
        </div>
```

And update the import at line 5 to include `issues`:

```tsx
import { projects as projectsApi, boards as boardsApi, search as searchApi, issues } from "@/lib/api";
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd frontend-next && npx playwright test e2e/export.spec.ts --workers=1`
Expected: PASS — the download fires with filename `DEMO-issues.csv`.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/lib/api.ts frontend-next/components/projects/ProjectOverview.tsx frontend-next/e2e/export.spec.ts
git commit -m "feat(frontend): Export CSV button on the project overview"
```

---

### Task 3: Timeline & Calendar routes accept `{key}` (backend)

**Problem (blocking):** Both routes use `{projectID}` = internal UUID via `chk.ByProjectID`→`GetByID`. The frontend only ever has the seq_id/key. Re-key both routes on `{key}` + `chk.ByKey`, resolve to the UUID in the handler, and pass it to the (unchanged) domain services.

**Files:**
- Modify: `internal/api/handlers/timeline_handler.go` (inject `projectSvc`, resolve key)
- Modify: `internal/api/handlers/calendar_handler.go` (inject `projectSvc`, resolve key)
- Modify: `internal/api/router.go:105,107` (constructor calls) and `:399-400` (routes)
- Test: `internal/api/handlers/timeline_calendar_route_test.go` (create — a light route wiring smoke test)

**Interfaces:**
- Consumes: `project.Service.GetByKey(key string) (*project.Project, error)` returning `.ID` (UUID); `timeline.Service.GetTimelineData(projectID, zoom string)`, `calendar.Service.GetCalendarData(projectID string, year, month int)` — both unchanged, still take the UUID.
- Produces: `handlers.NewTimelineHandler(svc *timeline.Service, projectSvc *project.Service)` and `handlers.NewCalendarHandler(svc *calendar.Service, projectSvc *project.Service)`; routes `GET /rest/api/3/project/{key}/timeline` and `.../calendar`.

- [ ] **Step 1: Write the failing test**

Create `internal/api/handlers/timeline_calendar_route_test.go`. Assert the constructors take a `*project.Service` (compile-time contract) — this fails to compile until Step 3:

```go
package handlers_test

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/api/handlers"
	"github.com/it4nodummies/heureum/internal/domain/calendar"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/timeline"
)

// Compile-time guard: both handlers must accept a project service so they can
// resolve {key} -> UUID (the domain services key on the internal UUID).
func TestTimelineCalendarHandlersTakeProjectSvc(t *testing.T) {
	var ts *timeline.Service
	var cs *calendar.Service
	var ps *project.Service
	_ = handlers.NewTimelineHandler(ts, ps)
	_ = handlers.NewCalendarHandler(cs, ps)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handlers/ -run TestTimelineCalendarHandlersTakeProjectSvc -v`
Expected: FAIL to compile — `not enough arguments in call to handlers.NewTimelineHandler`.

- [ ] **Step 3: Write minimal implementation**

Replace `internal/api/handlers/timeline_handler.go` entirely with:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/timeline"
)

type TimelineHandler struct {
	svc        *timeline.Service
	projectSvc *project.Service
}

func NewTimelineHandler(svc *timeline.Service, projectSvc *project.Service) *TimelineHandler {
	return &TimelineHandler{svc: svc, projectSvc: projectSvc}
}

// GetTimeline serves GET /rest/api/3/project/{key}/timeline. {key} is the
// public project key/seq_id; the domain service keys on the internal UUID, so
// resolve it here. zoom is one of weeks|months|quarters (default weeks).
func (h *TimelineHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	zoom := r.URL.Query().Get("zoom")
	if zoom == "" {
		zoom = "weeks"
	}
	data, err := h.svc.GetTimelineData(p.ID, zoom)
	if err != nil {
		http.Error(w, `{"error":"timeline failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
```

Replace `internal/api/handlers/calendar_handler.go` entirely with:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/domain/calendar"
	"github.com/it4nodummies/heureum/internal/domain/project"
)

type CalendarHandler struct {
	svc        *calendar.Service
	projectSvc *project.Service
}

func NewCalendarHandler(svc *calendar.Service, projectSvc *project.Service) *CalendarHandler {
	return &CalendarHandler{svc: svc, projectSvc: projectSvc}
}

// GetCalendar serves GET /rest/api/3/project/{key}/calendar?year=&month=.
// {key} is resolved to the internal UUID the domain service expects.
func (h *CalendarHandler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	if year == 0 || month == 0 {
		http.Error(w, `{"error":"year and month query params required"}`, http.StatusBadRequest)
		return
	}
	data, err := h.svc.GetCalendarData(p.ID, year, month)
	if err != nil {
		http.Error(w, `{"error":"calendar failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
```

In `internal/api/router.go`, update the two constructor calls (lines 105 and 107):

```go
	timelineH := handlers.NewTimelineHandler(timelineSvc, projectSvc)
	calendarH := handlers.NewCalendarHandler(calendarSvc, projectSvc)
```

And update the two routes (lines 399-400) — change `{projectID}` → `{key}` and `chk.ByProjectID` → `chk.ByKey`:

```go
	mux.Handle("GET /rest/api/3/project/{key}/timeline", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(timelineH.GetTimeline))))
	mux.Handle("GET /rest/api/3/project/{key}/calendar", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(calendarH.GetCalendar))))
```

- [ ] **Step 4: Run test + build + gapreport**

Run: `go test ./internal/api/handlers/ -run TestTimelineCalendarHandlersTakeProjectSvc -v && go build ./... && go vet ./... && go run ./cmd/gapreport`
Expected: test PASS, clean build/vet, and gapreport shows **no diff** (these routes aren't in the OpenAPI spec, so re-keying them doesn't change coverage). If `git status` shows `docs/contracts/gap-report.md` changed, inspect why before continuing.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/timeline_handler.go internal/api/handlers/calendar_handler.go internal/api/handlers/timeline_calendar_route_test.go internal/api/router.go
git commit -m "fix(timeline,calendar): key routes on project key and resolve to UUID server-side"
```

---

### Task 4: Timeline UI page + tab (frontend)

**Files:**
- Modify: `frontend-next/lib/api.ts` (add `TimelineData`/`TimelineBar` types + `timeline` client)
- Modify: `frontend-next/components/projects/ProjectHeader.tsx` (add Timeline tab)
- Create: `frontend-next/app/app/projects/[key]/timeline/page.tsx`
- Test: `frontend-next/e2e/timeline.spec.ts` (create)

**Interfaces:**
- Consumes: `GET /rest/api/3/project/{key}/timeline?zoom=weeks|months|quarters` (Task 3), returning `TimelineData`.
- Produces: `timeline.get(key, zoom)`; a Timeline tab in `ProjectHeader` linking `/app/projects/${key}/timeline`.

- [ ] **Step 1: Write the failing test**

Create `frontend-next/e2e/timeline.spec.ts`:

```ts
import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test("shows the project timeline with bars", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO");
  await page.getByRole("link", { name: "Timeline" }).click();
  await expect(page).toHaveURL(/\/app\/projects\/DEMO\/timeline/);
  await expect(page.getByTestId("timeline-chart")).toBeVisible();
  // DEMO has at least one sprint (seed), so at least one bar renders.
  await expect(page.getByTestId("timeline-bar").first()).toBeVisible();
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend-next && npx playwright test e2e/timeline.spec.ts --workers=1`
Expected: FAIL — no Timeline tab/page yet.

- [ ] **Step 3: Write minimal implementation**

Add to `frontend-next/lib/api.ts` (near the other resource clients; place the types just above the `export const timeline`):

```ts
export interface TimelineBar {
  id: string;
  name: string;
  type: string; // "epic" | "sprint"
  start_date: string | null;
  end_date: string | null;
  progress: number; // 0..100
  parent_id?: string;
  color: string; // hex
}
export interface TimelineData {
  project_id: string;
  zoom: string;
  start_date: string;
  end_date: string;
  bars: TimelineBar[];
  headers: string[];
}
export const timeline = {
  get: (key: string, zoom: "weeks" | "months" | "quarters" = "weeks") =>
    apiFetch<TimelineData>(`/rest/api/3/project/${key}/timeline?zoom=${zoom}`),
};
```

Extend `ProjectHeader.tsx`: add `"timeline"` and `"calendar"` (Calendar is added in Task 6 — add both now to avoid touching the union twice) to the `ActiveTab` union at line 10:

```tsx
type ActiveTab = "overview" | "board" | "backlog" | "timeline" | "calendar" | "reports" | "settings";
```

Then insert two `<TabLink>`s in the `<nav>` (after the Backlog tab at line 147, before Reports at line 148):

```tsx
          <TabLink href={`/app/projects/${projectKey}/timeline`} active={active === "timeline"}>
            Timeline
          </TabLink>
          <TabLink href={`/app/projects/${projectKey}/calendar`} active={active === "calendar"}>
            Calendar
          </TabLink>
```

Create `frontend-next/app/app/projects/[key]/timeline/page.tsx` — a dependency-free horizontal-bar Gantt. Bars are positioned by mapping their start/end onto the `[start_date, end_date]` window:

```tsx
"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { timeline, type TimelineBar } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

type Zoom = "weeks" | "months" | "quarters";

function pct(value: number, min: number, max: number): number {
  if (max <= min) return 0;
  return ((value - min) / (max - min)) * 100;
}

export default function TimelinePage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);
  const [zoom, setZoom] = useState<Zoom>("weeks");
  const q = useQuery({
    queryKey: ["timeline", key, zoom],
    queryFn: () => timeline.get(key, zoom),
  });

  return (
    <div>
      <ProjectHeader projectKey={key} active="timeline" />
      <div className="mx-auto max-w-5xl p-6">
        <div className="mb-4 flex items-center gap-2">
          <label className="text-sm text-slate-500">Zoom</label>
          <select
            aria-label="Timeline zoom"
            value={zoom}
            onChange={(e) => setZoom(e.target.value as Zoom)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            <option value="weeks">Weeks</option>
            <option value="months">Months</option>
            <option value="quarters">Quarters</option>
          </select>
        </div>

        {q.isLoading && <p className="text-sm text-slate-400">Loading timeline…</p>}
        {q.data && q.data.bars.length === 0 && (
          <p className="text-sm text-slate-400">No epics or sprints with dates yet.</p>
        )}
        {q.data && q.data.bars.length > 0 && (
          <TimelineChart data={q.data} />
        )}
      </div>
    </div>
  );
}

function TimelineChart({ data }: { data: { start_date: string; end_date: string; bars: TimelineBar[]; headers: string[] } }) {
  const min = new Date(data.start_date).getTime();
  const max = new Date(data.end_date).getTime();

  return (
    <div data-testid="timeline-chart" className="rounded border border-slate-200 bg-white">
      {/* Header ruler */}
      <div className="flex border-b border-slate-100 px-3 py-2 text-[10px] text-slate-400">
        {data.headers.map((h, i) => (
          <div key={i} className="flex-1 text-center">{h}</div>
        ))}
      </div>
      {/* Bars */}
      <div className="space-y-2 p-3">
        {data.bars.map((bar) => {
          const s = bar.start_date ? new Date(bar.start_date).getTime() : min;
          const e = bar.end_date ? new Date(bar.end_date).getTime() : max;
          const left = pct(s, min, max);
          const width = Math.max(pct(e, min, max) - left, 1.5);
          return (
            <div key={bar.id} className="flex items-center gap-2">
              <div className="w-32 shrink-0 truncate text-xs text-[#1a1f36]" title={bar.name}>
                {bar.name}
              </div>
              <div className="relative h-5 flex-1 rounded bg-slate-50">
                <div
                  data-testid="timeline-bar"
                  className="absolute top-0 flex h-5 items-center rounded px-1.5 text-[10px] font-medium text-white"
                  style={{ left: `${left}%`, width: `${width}%`, backgroundColor: bar.color }}
                  title={`${bar.name} · ${bar.type}${bar.type === "sprint" ? ` · ${Math.round(bar.progress)}%` : ""}`}
                >
                  {bar.type === "sprint" ? `${Math.round(bar.progress)}%` : ""}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run test + build to verify pass**

Run: `cd frontend-next && npm run build && npx playwright test e2e/timeline.spec.ts --workers=1`
Expected: build clean; test PASS (Timeline tab → page → chart + at least one bar visible).

- [ ] **Step 5: Commit**

```bash
git add frontend-next/lib/api.ts frontend-next/components/projects/ProjectHeader.tsx "frontend-next/app/app/projects/[key]/timeline/page.tsx" frontend-next/e2e/timeline.spec.ts
git commit -m "feat(frontend): project Timeline (Gantt) view + header tab"
```

---

### Task 5: Calendar start-date bucketing fix (backend)

**Problem:** `calendar.Service.GetCalendarData` (`internal/domain/calendar/service.go:30-36`) fetches issues whose **due_date OR start_date** falls in the month, but buckets *only* by `DueDate.Day()`. An issue with a start_date in the month but a null/out-of-month due_date is fetched and then silently dropped. Bucket by both dates so the calendar shows every fetched issue.

**Files:**
- Modify: `internal/domain/calendar/service.go:30-36`
- Test: `internal/domain/calendar/service_test.go` (create)

**Interfaces:**
- Consumes/Produces: `calendar.Service.GetCalendarData(projectID string, year, month int) (*calendar.CalendarData, error)` — signature unchanged; behavior fixed so start-date-only issues land on their start day.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/calendar/service_test.go`. The service uses raw SQL against `issues`/`workflow_statuses`, so create those tables via raw SQL and insert one due-date issue and one start-date-only issue:

```go
package calendar

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE issues (
		id TEXT PRIMARY KEY, key TEXT, title TEXT, priority TEXT,
		status_id TEXT, project_id TEXT, is_archived BOOLEAN DEFAULT 0,
		due_date DATETIME, start_date DATETIME)`)
	db.Exec(`CREATE TABLE workflow_statuses (id TEXT PRIMARY KEY, name TEXT)`)
	return db
}

func findDay(data *CalendarData, day int) *CalendarDay {
	for i := range data.Days {
		if data.Days[i].Day == day {
			return &data.Days[i]
		}
	}
	return nil
}

func TestCalendarBucketsStartDateOnlyIssues(t *testing.T) {
	db := newCalTestDB(t)
	// Due-date issue on the 10th.
	db.Exec(`INSERT INTO issues (id,key,title,priority,project_id,is_archived,due_date)
		VALUES ('i1','DEMO-1','Due one','high','proj-1',0,'2026-03-10 00:00:00')`)
	// Start-date-only issue on the 5th (no due_date).
	db.Exec(`INSERT INTO issues (id,key,title,priority,project_id,is_archived,start_date)
		VALUES ('i2','DEMO-2','Start one','low','proj-1',0,'2026-03-05 00:00:00')`)

	svc := NewService(db)
	data, err := svc.GetCalendarData("proj-1", 2026, 3)
	if err != nil {
		t.Fatal(err)
	}
	if d := findDay(data, 10); d == nil || len(d.Issues) != 1 || d.Issues[0].Key != "DEMO-1" {
		t.Errorf("day 10 = %+v, want DEMO-1", d)
	}
	if d := findDay(data, 5); d == nil || len(d.Issues) != 1 || d.Issues[0].Key != "DEMO-2" {
		t.Errorf("day 5 = %+v, want DEMO-2 (start-date bucketing broken)", d)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/calendar/ -run TestCalendarBucketsStartDateOnlyIssues -v`
Expected: FAIL — day 5 has 0 issues (DEMO-2 fetched but never bucketed).

- [ ] **Step 3: Write minimal implementation**

Replace the bucketing loop in `internal/domain/calendar/service.go:30-36` with one that buckets by due_date and, when there's no due_date, by start_date (guarding both against months other than the requested one):

```go
	issueMap := make(map[int][]CalendarIssue)
	for _, iss := range issues {
		// Prefer the due date; fall back to start date for issues that only
		// have a start_date in this month (otherwise they'd be fetched but
		// never placed on a day).
		var day int
		switch {
		case iss.DueDate != nil && int(iss.DueDate.Month()) == month && iss.DueDate.Year() == year:
			day = iss.DueDate.Day()
		case iss.StartDate != nil && int(iss.StartDate.Month()) == month && iss.StartDate.Year() == year:
			day = iss.StartDate.Day()
		default:
			continue
		}
		issueMap[day] = append(issueMap[day], iss)
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/calendar/ -v && go build ./...`
Expected: PASS (both days bucketed) and clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/calendar/service.go internal/domain/calendar/service_test.go
git commit -m "fix(calendar): bucket start-date-only issues onto their start day"
```

---

### Task 6: Calendar UI page (frontend)

**Files:**
- Modify: `frontend-next/lib/api.ts` (add `CalendarData`/`CalendarDay`/`CalendarIssue` types + `calendar` client)
- Create: `frontend-next/app/app/projects/[key]/calendar/page.tsx`
- Test: `frontend-next/e2e/calendar.spec.ts` (create)

> The Calendar tab was already added to `ProjectHeader` in Task 4.

**Interfaces:**
- Consumes: `GET /rest/api/3/project/{key}/calendar?year=&month=` (Tasks 3 & 5), returning `CalendarData`.
- Produces: `calendar.get(key, year, month)`; page at `/app/projects/${key}/calendar`.

- [ ] **Step 1: Write the failing test**

Create `frontend-next/e2e/calendar.spec.ts`:

```ts
import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test("renders a month grid and navigates months", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO");
  await page.getByRole("link", { name: "Calendar" }).click();
  await expect(page).toHaveURL(/\/app\/projects\/DEMO\/calendar/);
  await expect(page.getByTestId("calendar-grid")).toBeVisible();
  // 28..31 day cells depending on month; assert at least 28 rendered.
  const cells = page.getByTestId("calendar-day");
  await expect(cells.nth(27)).toBeVisible();

  const title = page.getByTestId("calendar-title");
  const before = await title.textContent();
  await page.getByRole("button", { name: "Previous month" }).click();
  await expect(title).not.toHaveText(before ?? "");
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend-next && npx playwright test e2e/calendar.spec.ts --workers=1`
Expected: FAIL — no Calendar page yet.

- [ ] **Step 3: Write minimal implementation**

Add to `frontend-next/lib/api.ts` (near the `timeline` client):

```ts
export interface CalendarIssue {
  id: string;
  key: string;
  title: string;
  priority: string;
  status: string;
  due_date: string | null;
  start_date: string | null;
}
export interface CalendarDay {
  date: string; // "YYYY-MM-DD"
  day: number;
  issues: CalendarIssue[];
}
export interface CalendarData {
  year: number;
  month: number;
  days: CalendarDay[];
  total_days: number;
}
export const calendar = {
  get: (key: string, year: number, month: number) =>
    apiFetch<CalendarData>(`/rest/api/3/project/${key}/calendar?year=${year}&month=${month}`),
};
```

Create `frontend-next/app/app/projects/[key]/calendar/page.tsx`. Month state is initialized to a fixed date literal to keep the E2E deterministic-ish, then the user navigates; the grid pads leading blanks so the 1st lands on the right weekday:

```tsx
"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { calendar } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

const MONTHS = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];
const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

export default function CalendarPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);
  const now = new Date();
  const [ym, setYm] = useState<{ year: number; month: number }>({
    year: now.getFullYear(),
    month: now.getMonth() + 1, // 1-based
  });

  const q = useQuery({
    queryKey: ["calendar", key, ym.year, ym.month],
    queryFn: () => calendar.get(key, ym.year, ym.month),
  });

  function shift(delta: number) {
    setYm((prev) => {
      const zero = prev.month - 1 + delta;
      const year = prev.year + Math.floor(zero / 12);
      const month = ((zero % 12) + 12) % 12 + 1;
      return { year, month };
    });
  }

  // Leading blank cells so day 1 aligns to its weekday.
  const firstWeekday = new Date(ym.year, ym.month - 1, 1).getDay();
  const blanks = Array.from({ length: firstWeekday });

  return (
    <div>
      <ProjectHeader projectKey={key} active="calendar" />
      <div className="mx-auto max-w-5xl p-6">
        <div className="mb-4 flex items-center gap-3">
          <button
            aria-label="Previous month"
            onClick={() => shift(-1)}
            className="rounded border border-slate-300 px-2 py-1 text-sm hover:bg-slate-50"
          >
            ‹
          </button>
          <h2 data-testid="calendar-title" className="text-sm font-semibold text-[#1a1f36]">
            {MONTHS[ym.month - 1]} {ym.year}
          </h2>
          <button
            aria-label="Next month"
            onClick={() => shift(1)}
            className="rounded border border-slate-300 px-2 py-1 text-sm hover:bg-slate-50"
          >
            ›
          </button>
        </div>

        <div data-testid="calendar-grid" className="grid grid-cols-7 gap-1">
          {WEEKDAYS.map((d) => (
            <div key={d} className="pb-1 text-center text-[10px] font-medium uppercase text-slate-400">
              {d}
            </div>
          ))}
          {blanks.map((_, i) => (
            <div key={`b${i}`} className="min-h-24 rounded bg-slate-50/50" />
          ))}
          {(q.data?.days ?? []).map((day) => (
            <div
              key={day.date}
              data-testid="calendar-day"
              className="min-h-24 rounded border border-slate-100 p-1"
            >
              <div className="mb-1 text-[10px] text-slate-400">{day.day}</div>
              <div className="space-y-1">
                {day.issues.map((iss) => (
                  <div
                    key={iss.id}
                    className="truncate rounded bg-[#0052cc]/10 px-1 py-0.5 text-[10px] text-[#0052cc]"
                    title={`${iss.key} · ${iss.title}`}
                  >
                    {iss.key}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run test + build to verify pass**

Run: `cd frontend-next && npm run build && npx playwright test e2e/calendar.spec.ts --workers=1`
Expected: build clean; test PASS (grid renders ≥28 day cells; month nav changes the title).

- [ ] **Step 5: Commit**

```bash
git add frontend-next/lib/api.ts "frontend-next/app/app/projects/[key]/calendar/page.tsx" frontend-next/e2e/calendar.spec.ts
git commit -m "feat(frontend): project Calendar month view + header tab"
```

---

### Task 7: Burnup report + sprint selector fix (frontend)

**Problem:** The reports page hardcodes `boards.sprints(1)` (`reports/page.tsx:26`), so burndown works only for projects on board `1` and burnup isn't wired at all. Resolve the project's own board, add a sprint `<select>`, and add a Burnup card that reuses `BurndownData`/`LineChart`.

**Files:**
- Modify: `frontend-next/lib/api.ts` (add `reports.burnup`)
- Modify: `frontend-next/app/app/projects/[key]/reports/page.tsx` (sprint selector + Burnup card)
- Test: `frontend-next/e2e/reports.spec.ts` (extend if it exists; otherwise create)

**Interfaces:**
- Consumes: `GET /rest/api/3/project/{key}/reports/burnup?sprintId=` (already routed at `router.go:337`), returning `BurndownData{labels,ideal,actual}`; `boards.list()` (`{values:[{id, location:{projectKey}}]}`) and `boards.sprints(boardId)` (`{values:[{id, name}]}`).
- Produces: `reports.burnup(key, sprintId)`.

- [ ] **Step 1: Write the failing test**

If `frontend-next/e2e/reports.spec.ts` exists, add this test to it; otherwise create the file with a login helper import:

```ts
import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test("shows a Burnup card and a sprint selector", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/reports");
  await expect(page.getByRole("heading", { name: "Burnup" })).toBeVisible();
  await expect(page.getByTestId("reports-sprint-select")).toBeVisible();
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend-next && npx playwright test e2e/reports.spec.ts --workers=1 -g "Burnup"`
Expected: FAIL — no Burnup heading / sprint select yet.

- [ ] **Step 3: Write minimal implementation**

Add to the `reports` object in `frontend-next/lib/api.ts` (after `burndown`, line ~715):

```ts
  burnup: (key: string, sprintId: string) =>
    apiFetch<BurndownData>(`/rest/api/3/project/${key}/reports/burnup?sprintId=${sprintId}`),
```

Rewrite the sprint-source and burndown section of `frontend-next/app/app/projects/[key]/reports/page.tsx`. Replace lines 25-33 (the hardcoded `boards.sprints(1)` block + the `burndown` query) with a project-board-resolving version plus explicit sprint state:

```tsx
  // Resolve THIS project's board (not the hardcoded board 1), then its sprints.
  const boardsList = useQuery({ queryKey: ["boards"], queryFn: () => boards.list() });
  const board = boardsList.data?.values.find((b) => b.location?.projectKey === key);
  const sprints = useQuery({
    queryKey: ["reports", key, "sprints", board?.id],
    queryFn: () => boards.sprints(board!.id),
    enabled: !!board,
  });
  const [sprintId, setSprintId] = useState<string>("");
  const activeSprintId = sprintId || sprints.data?.values[0]?.id?.toString() || "";

  const burndown = useQuery({
    queryKey: ["reports", key, "burndown", activeSprintId],
    queryFn: () => reports.burndown(key, activeSprintId),
    enabled: !!activeSprintId,
  });
  const burnup = useQuery({
    queryKey: ["reports", key, "burnup", activeSprintId],
    queryFn: () => reports.burnup(key, activeSprintId),
    enabled: !!activeSprintId,
  });
```

Ensure the import at line 5 includes `boards` (it already does: `import { reports, boards } from "@/lib/api";`).

Add a sprint selector just inside the `max-w-3xl` container, before the Burndown card (before line 43's `<Card title="Burndown">`):

```tsx
        <div className="mb-4 flex items-center gap-2">
          <label className="text-sm text-slate-500">Sprint</label>
          <select
            data-testid="reports-sprint-select"
            aria-label="Sprint"
            value={activeSprintId}
            onChange={(e) => setSprintId(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            {(sprints.data?.values ?? []).map((s) => (
              <option key={s.id} value={String(s.id)}>
                {s.name}
              </option>
            ))}
          </select>
        </div>
```

Add a Burnup card right after the Burndown card (after line 55's closing `</Card>`):

```tsx
        <Card title="Burnup">
          {burnup.data ? (
            <LineChart
              labels={burnup.data.labels}
              series={[
                { name: "Scope", color: "#8993a4", values: burnup.data.ideal },
                { name: "Completed", color: "#00875a", values: burnup.data.actual },
              ]}
            />
          ) : (
            <p className="text-sm text-slate-400">No active sprint</p>
          )}
        </Card>
```

- [ ] **Step 4: Run test + build to verify pass**

Run: `cd frontend-next && npm run build && npx playwright test e2e/reports.spec.ts --workers=1`
Expected: build clean; the new Burnup test PASSES and the pre-existing reports tests still pass.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/lib/api.ts "frontend-next/app/app/projects/[key]/reports/page.tsx" frontend-next/e2e/reports.spec.ts
git commit -m "feat(frontend): Burnup report + project-scoped sprint selector (fix hardcoded board 1)"
```

---

### Task 8: Round close — docs, gate, memory

**Files:**
- Modify: `CHANGELOG.md` (Unreleased → add R14 entries)
- Modify: `docs/superpowers/STATE.md` (record Round 14, set next round)
- Modify: `docs/superpowers/specs/2026-07-16-jira-cloud-parity-requirements-v2.md` (tick §A.2/A.3/A.6/A.7 as done — optional if the file tracks status)

**Interfaces:** none (documentation + verification only).

- [ ] **Step 1: Run the full three-level gate**

```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test --workers=1 && cd ..
go run ./cmd/gapreport
```

Expected: all Go tests green; full Playwright suite green at `--workers=1`; gapreport produces **no diff** (`git status` shows `docs/contracts/gap-report.md` unchanged). If gapreport did change the file, commit the regenerated report.

- [ ] **Step 2: Update CHANGELOG.md**

Under `## [Unreleased]` → `### Added`, append:

```markdown
- **Project Timeline (Gantt)**: a horizontal-bar timeline of epics and sprints per project, with
  weeks/months/quarters zoom (`GET /project/{key}/timeline`). Sprint bars show completion %.
- **Project Calendar**: a month grid placing issues on their due date (or start date when there's
  no due date) with month navigation (`GET /project/{key}/calendar`).
- **Burnup report** on the project Reports page, alongside a **sprint selector** that resolves the
  project's own board (previously the page was hardcoded to board `1`).
```

Under `### Fixed`, append:

```markdown
- CSV export (`GET /project/{key}/issues/export`) now writes resolved **Status/Type/Assignee names**
  instead of raw UUIDs, and the frontend gained an **Export CSV** button on the project overview
  (bearer-auth blob download).
- Timeline and Calendar routes now key on the project **key** (resolved to the internal UUID
  server-side) — previously they required the internal UUID the frontend never receives, making
  both views uncallable.
- The Calendar service dropped issues that had a start_date in the month but no due_date; they're
  now bucketed onto their start day.
```

- [ ] **Step 3: Update STATE.md**

Add a Round 14 section (mirror the format of the Round 13 entry) summarizing: what shipped (Timeline/Calendar UI + routes re-keyed, Burnup + sprint selector, CSV name resolution), the defects fixed, and set "Prossimo" to the next requirements-v2 round (per the doc, **R15**). Record any new follow-ups discovered during execution.

- [ ] **Step 4: Commit docs**

```bash
git add CHANGELOG.md docs/superpowers/STATE.md docs/superpowers/specs/2026-07-16-jira-cloud-parity-requirements-v2.md
git commit -m "docs: record Round 14 (timeline, calendar, burnup, CSV export)"
```

- [ ] **Step 5: Update auto-memory**

Update `~/.claude/projects/-Users-n0r41n-Development-open-jira/memory/jira-parity-rounds.md` to note Round 14 complete (Timeline/Calendar/Burnup/CSV; still dev-only, not released; last public tag remains v1.0.2). Keep the `MEMORY.md` index line accurate. This is a controller action after the final gate — not a subagent task.

---

## Self-Review

**1. Spec coverage (requirements v2 §A.2/A.3/A.6/A.7):**
- §A.2 Timeline → Tasks 3 (route fix) + 4 (UI). ✅
- §A.3 Calendar → Tasks 3 (route fix) + 5 (bucketing fix) + 6 (UI). ✅
- §A.6 Burnup → Task 7 (client + card + sprint selector fix). ✅
- §A.7 CSV export → Tasks 1 (name resolution) + 2 (button). ✅

**2. Placeholder scan:** No TBD/TODO; every code step shows complete code; every test step shows the assertion and the exact run command with expected output.

**3. Type consistency:** `ExportRow` fields (Task 1) match the CSV columns written in the handler and the domain query aliases. `NewTimelineHandler(svc, projectSvc)` / `NewCalendarHandler(svc, projectSvc)` signatures (Task 3) match the compile-time test and the router constructor calls. `TimelineData`/`TimelineBar` (Task 4), `CalendarData`/`CalendarDay`/`CalendarIssue` (Task 6) mirror the Go JSON tags verbatim (snake_case). `reports.burnup` returns `BurndownData` (Task 7), consistent with the existing `reports.burndown`. The `ActiveTab` union is extended once (Task 4) to include both `"timeline"` and `"calendar"`, so Task 6 needs no further union edit.

**Cross-cutting risks flagged for the executor:**
- The `login` import in the E2E specs assumes `e2e/helpers.ts` exports `login`. If it doesn't, copy the login preamble from an existing spec (`e2e/board.spec.ts`) inline — do not invent a helper.
- Task 1's domain test creates `workflow_statuses`/`issue_types`/`users` via raw SQL because they belong to other domain packages; confirm the `users` table's display-name column is `display_name` (the COALESCE already falls back to `email`).
- Run the full Playwright suite with `--workers=1` (known SQLite write-contention flakiness above one worker — a recorded R13 follow-up).
