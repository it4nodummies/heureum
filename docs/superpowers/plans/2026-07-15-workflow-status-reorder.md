# Riordino status/colonne board (drag & drop) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users reorder workflow statuses (= board columns) via drag & drop in the Project Settings → Workflow panel, fix the underlying status-ordering bug, and clarify the category combobox that users currently mistake for a column-order control.

**Architecture:** The backend already has a working `ReorderStatuses` service method and `PUT .../workflow/statuses/order` endpoint, plus a ready `workflow.reorderStatuses` API client — none of it is wired to the UI. This plan (1) fixes `GetWorkflow` to return statuses ordered by `position` (currently unordered), (2) replaces the static status `<ul>` in `WorkflowEditor.tsx` with a `@dnd-kit/sortable` list wired to the existing reorder endpoint, and (3) relabels the category `<select>` with clarifying text. No new backend endpoints, no changes to `AddStatus` (new statuses still append at the end; users drag them into place after creation, per user decision).

**Tech Stack:** Go + GORM + SQLite (backend, table-driven `testing`), Next.js + React + TanStack Query + `@dnd-kit/sortable` (already a dependency) + Playwright (frontend).

Reference spec: `docs/superpowers/specs/2026-07-15-workflow-status-reorder-design.md`

---

### Task 1: Fix `GetWorkflow` to return statuses ordered by position

**Files:**
- Modify: `internal/domain/workflow/service.go:28-34`
- Test: `internal/domain/workflow/service_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/domain/workflow/service_test.go`:

```go
func TestGetWorkflowReturnsStatusesOrderedByPosition(t *testing.T) {
	db := setupWorkflowTestDB(t)
	svc := NewService(db)

	wf, _ := svc.CreateWorkflow("proj-9", "WF")
	s1, _ := svc.AddStatus(wf.ID, "Todo", CategoryTodo, "#111")
	s2, _ := svc.AddStatus(wf.ID, "InProgress", CategoryInProgress, "#222")
	s3, _ := svc.AddStatus(wf.ID, "Done", CategoryDone, "#333")

	// Reorder to: Done, Todo, InProgress.
	if err := svc.ReorderStatuses(wf.ID, []string{s3.ID, s1.ID, s2.ID}); err != nil {
		t.Fatalf("ReorderStatuses() error = %v", err)
	}

	result, err := svc.GetWorkflow("proj-9")
	if err != nil {
		t.Fatalf("GetWorkflow() error = %v", err)
	}
	if len(result.Statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(result.Statuses))
	}
	got := []string{result.Statuses[0].Name, result.Statuses[1].Name, result.Statuses[2].Name}
	want := []string{"Done", "Todo", "InProgress"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Statuses order = %v, want %v", got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/workflow/... -run TestGetWorkflowReturnsStatusesOrderedByPosition -v`
Expected: FAIL — `Statuses order = [Todo InProgress Done], want [Done Todo InProgress]` (GORM's unordered `Preload` returns insertion order, not `position` order).

- [ ] **Step 3: Fix `GetWorkflow`**

Replace in `internal/domain/workflow/service.go`:

```go
func (s *Service) GetWorkflow(projectID string) (*Workflow, error) {
	var wf Workflow
	if err := s.db.Preload("Statuses").Preload("Transitions").Where("project_id = ?", projectID).First(&wf).Error; err != nil {
		return nil, errors.New("workflow not found")
	}
	return &wf, nil
}
```

with:

```go
func (s *Service) GetWorkflow(projectID string) (*Workflow, error) {
	var wf Workflow
	if err := s.db.
		Preload("Statuses", func(db *gorm.DB) *gorm.DB { return db.Order("position ASC") }).
		Preload("Transitions").
		Where("project_id = ?", projectID).First(&wf).Error; err != nil {
		return nil, errors.New("workflow not found")
	}
	return &wf, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/workflow/... -run TestGetWorkflowReturnsStatusesOrderedByPosition -v`
Expected: PASS

- [ ] **Step 5: Run the full workflow package test suite**

Run: `go test ./internal/domain/workflow/... -v`
Expected: all tests PASS (existing `TestGetWorkflow` etc. still pass — they don't assert on order).

- [ ] **Step 6: Commit**

```bash
git add internal/domain/workflow/service.go internal/domain/workflow/service_test.go
git commit -m "fix(workflow): return statuses ordered by position from GetWorkflow"
```

---

### Task 2: Add `@dnd-kit/utilities` as an explicit frontend dependency

`@dnd-kit/utilities` (the `CSS.Transform.toString` helper used by every `@dnd-kit/sortable` consumer) is currently only present as a transitive dependency of `@dnd-kit/sortable` — it must be declared directly since Task 4 imports from it.

**Files:**
- Modify: `frontend-next/package.json:11-18`

- [ ] **Step 1: Add the dependency**

In `frontend-next/package.json`, change:

```json
  "dependencies": {
    "@dnd-kit/core": "^6.3.1",
    "@dnd-kit/sortable": "^8.0.0",
    "@tanstack/react-query": "^5.101.2",
```

to:

```json
  "dependencies": {
    "@dnd-kit/core": "^6.3.1",
    "@dnd-kit/sortable": "^8.0.0",
    "@dnd-kit/utilities": "^3.2.2",
    "@tanstack/react-query": "^5.101.2",
```

- [ ] **Step 2: Install and verify the lockfile updates cleanly**

Run: `cd frontend-next && npm install`
Expected: exits 0; `package-lock.json` shows `@dnd-kit/utilities` promoted into the top-level `dependencies` of the root package (it was already resolved as a transitive dep, so no new package is downloaded).

- [ ] **Step 3: Commit**

```bash
git add frontend-next/package.json frontend-next/package-lock.json
git commit -m "chore(frontend): declare @dnd-kit/utilities as a direct dependency"
```

---

### Task 3: Drag & drop reordering in `WorkflowEditor.tsx`

**Files:**
- Modify: `frontend-next/components/workflow/WorkflowEditor.tsx` (full rewrite of the statuses section, lines 1-77)

- [ ] **Step 1: Replace the component**

Replace the entire content of `frontend-next/components/workflow/WorkflowEditor.tsx` with:

```tsx
"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { DndContext, DragEndEvent, closestCenter } from "@dnd-kit/core";
import { SortableContext, arrayMove, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { workflow, type Workflow, type WorkflowStatus } from "@/lib/api";

const CATEGORIES = [
  { value: "todo", label: "To Do" },
  { value: "inprogress", label: "In Progress" },
  { value: "done", label: "Done" },
] as const;

function StatusRow({ status, onDelete }: { status: WorkflowStatus; onDelete: (id: string) => void }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: status.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  return (
    <li
      ref={setNodeRef}
      style={style}
      className="flex items-center gap-2 text-sm"
      data-testid={`status-${status.name}`}
    >
      <button
        type="button"
        {...attributes}
        {...listeners}
        className="cursor-grab text-slate-400 hover:text-slate-600"
        aria-label={`Drag to reorder ${status.name}`}
        data-testid={`drag-handle-${status.name}`}
      >
        ⠿
      </button>
      <span className="inline-block h-3 w-3 rounded" style={{ backgroundColor: status.color }} />
      <span className="text-[#1a1f36]">{status.name}</span>
      <span className="text-xs text-slate-400">({status.category})</span>
      <button
        onClick={() => onDelete(status.id)}
        className="ml-auto text-xs text-red-600 hover:underline"
        aria-label={`Delete status ${status.name}`}
      >
        Remove
      </button>
    </li>
  );
}

export function WorkflowEditor({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const [newStatus, setNewStatus] = useState("");
  const [newCat, setNewCat] = useState("todo");

  const wf = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => workflow.get(projectKey) });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["workflow", projectKey] });

  const addStatus = useMutation({
    mutationFn: () => workflow.addStatus(projectKey, newStatus, newCat, "#6B7280"),
    onSuccess: () => {
      setNewStatus("");
      invalidate();
    },
  });
  const delStatus = useMutation({
    mutationFn: (id: string) => workflow.deleteStatus(projectKey, id),
    onSuccess: invalidate,
  });
  const reorderStatuses = useMutation({
    mutationFn: (orderedIds: string[]) => workflow.reorderStatuses(projectKey, orderedIds),
    onMutate: async (orderedIds: string[]) => {
      await qc.cancelQueries({ queryKey: ["workflow", projectKey] });
      const previous = qc.getQueryData<Workflow>(["workflow", projectKey]);
      if (previous) {
        const byId = new Map(previous.statuses.map((s) => [s.id, s]));
        const reordered = orderedIds.map((id) => byId.get(id)).filter((s): s is WorkflowStatus => Boolean(s));
        qc.setQueryData<Workflow>(["workflow", projectKey], { ...previous, statuses: reordered });
      }
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) qc.setQueryData(["workflow", projectKey], context.previous);
    },
    onSettled: invalidate,
  });

  const statuses = wf.data?.statuses ?? [];
  const nameByID = (id: string) => statuses.find((s) => s.id === id)?.name ?? id;

  const handleDragEnd = (e: DragEndEvent) => {
    const { active, over } = e;
    if (!over || active.id === over.id) return;
    const ids = statuses.map((s) => s.id);
    const oldIndex = ids.indexOf(String(active.id));
    const newIndex = ids.indexOf(String(over.id));
    if (oldIndex === -1 || newIndex === -1) return;
    reorderStatuses.mutate(arrayMove(ids, oldIndex, newIndex));
  };

  return (
    <div className="space-y-6">
      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Statuses</h3>
        <DndContext collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={statuses.map((s) => s.id)} strategy={verticalListSortingStrategy}>
            <ul className="space-y-1" data-testid="workflow-statuses">
              {statuses.map((s) => (
                <StatusRow key={s.id} status={s} onDelete={(id) => delStatus.mutate(id)} />
              ))}
            </ul>
          </SortableContext>
        </DndContext>
        <div className="mt-2 flex gap-2">
          <input
            aria-label="New status name"
            value={newStatus}
            onChange={(e) => setNewStatus(e.target.value)}
            placeholder="Status name"
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          />
          <select
            aria-label="Category (reporting only)"
            value={newCat}
            onChange={(e) => setNewCat(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            {CATEGORIES.map((c) => (
              <option key={c.value} value={c.value}>{c.label}</option>
            ))}
          </select>
          <button
            onClick={() => newStatus && addStatus.mutate()}
            className="rounded bg-[#0052cc] px-3 py-1 text-sm text-white disabled:opacity-60"
            disabled={addStatus.isPending}
          >
            Add status
          </button>
        </div>
        <p className="mt-1 text-xs text-slate-400">
          Category is used for reports and to auto-set the resolution — it does not control column order.
          Drag statuses in the list above to reorder board columns.
        </p>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Transitions</h3>
        <ul className="space-y-1 text-sm" data-testid="workflow-transitions">
          {(wf.data?.transitions ?? []).map((t) => (
            <li key={t.id} className="flex items-center gap-2">
              <span className="text-[#1a1f36]">{t.name || `${nameByID(t.from_status_id)} → ${nameByID(t.to_status_id)}`}</span>
              <span className="text-xs text-slate-400">
                {nameByID(t.from_status_id)} → {nameByID(t.to_status_id)}
                {t.require_assignee ? " · requires assignee" : ""}
                {t.set_resolution ? " · sets resolution" : ""}
              </span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
```

Note: the category `<select>`'s `aria-label` changed from `"New status category"` to `"Category (reporting only)"` — Task 5 updates the existing e2e test that references the old label.

- [ ] **Step 2: Type-check and build**

Run: `cd frontend-next && npm run build`
Expected: exits 0, no TypeScript errors (in particular, confirm `Workflow` is exported from `@/lib/api` — it is, at `lib/api.ts:491-496`).

- [ ] **Step 3: Commit**

```bash
git add frontend-next/components/workflow/WorkflowEditor.tsx
git commit -m "feat(workflow): drag-and-drop status reordering, clarify category field"
```

---

### Task 4: Update the existing e2e test for the renamed category label

**Files:**
- Modify: `frontend-next/e2e/workflow.spec.ts:22`

- [ ] **Step 1: Update the selector**

In `frontend-next/e2e/workflow.spec.ts`, change:

```ts
  await page.getByLabel("New status category").selectOption("inprogress");
```

to:

```ts
  await page.getByLabel("Category (reporting only)").selectOption("inprogress");
```

- [ ] **Step 2: Run this spec to verify it still passes**

Run: `cd frontend-next && npx playwright test e2e/workflow.spec.ts`
Expected: PASS (1 test)

- [ ] **Step 3: Commit**

```bash
git add frontend-next/e2e/workflow.spec.ts
git commit -m "test(e2e): update workflow spec for renamed category label"
```

---

### Task 5: E2E test for drag-and-drop reordering + persistence

**Files:**
- Modify: `frontend-next/e2e/workflow.spec.ts`

- [ ] **Step 1: Write the failing test**

Append to `frontend-next/e2e/workflow.spec.ts`:

```ts
async function dragStatus(page: Page, fromTestId: string, toTestId: string) {
  const source = page.getByTestId(fromTestId);
  const target = page.getByTestId(toTestId);
  const sourceBox = await source.boundingBox();
  const targetBox = await target.boundingBox();
  if (!sourceBox || !targetBox) throw new Error("drag handle bounding box not found");
  await page.mouse.move(sourceBox.x + sourceBox.width / 2, sourceBox.y + sourceBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(targetBox.x + targetBox.width / 2, targetBox.y + targetBox.height / 2, { steps: 10 });
  await page.mouse.up();
}

test("workflow editor persists status order after drag-and-drop reorder", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();

  await expect(page.getByTestId("status-TO DO")).toBeVisible();
  await expect(page.getByTestId("status-DONE")).toBeVisible();

  const rowsBefore = await page.getByTestId("workflow-statuses").locator("li").allTextContents();
  expect(rowsBefore.findIndex((r) => r.includes("DONE"))).toBeGreaterThan(
    rowsBefore.findIndex((r) => r.includes("TO DO"))
  );

  // Drag "DONE" above "TO DO".
  await dragStatus(page, "drag-handle-DONE", "drag-handle-TO DO");

  await expect(async () => {
    const rows = await page.getByTestId("workflow-statuses").locator("li").allTextContents();
    expect(rows.findIndex((r) => r.includes("DONE"))).toBeLessThan(rows.findIndex((r) => r.includes("TO DO")));
  }).toPass();

  // Reload and verify the new order persisted server-side.
  await page.reload();
  await page.getByRole("button", { name: "Workflow" }).click();
  const rowsAfter = await page.getByTestId("workflow-statuses").locator("li").allTextContents();
  expect(rowsAfter.findIndex((r) => r.includes("DONE"))).toBeLessThan(
    rowsAfter.findIndex((r) => r.includes("TO DO"))
  );
});
```

- [ ] **Step 2: Run it to verify it fails against the pre-Task-3 behavior**

This step is diagnostic only if you're running tasks out of order; if Task 3 is already committed, skip straight to Step 3. Otherwise:
Run: `cd frontend-next && npx playwright test e2e/workflow.spec.ts -g "persists status order"`
Expected (before Task 3's fix): FAIL — no drag handles exist yet (`drag-handle-DONE` not found).

- [ ] **Step 3: Run it against the current (Task 3 + Task 1 applied) code**

Run: `cd frontend-next && npx playwright test e2e/workflow.spec.ts`
Expected: PASS (2 tests: the existing add-status test and the new reorder test).

- [ ] **Step 4: Commit**

```bash
git add frontend-next/e2e/workflow.spec.ts
git commit -m "test(e2e): cover drag-and-drop status reordering and persistence"
```

---

### Task 6: Full quality gate

**Files:** none (verification only)

- [ ] **Step 1: Backend build, vet, full test suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all pass, no vet warnings.

- [ ] **Step 2: Frontend build + full e2e suite**

Run: `cd frontend-next && npm run build && npx playwright test`
Expected: build succeeds; all Playwright specs pass (including `workflow.spec.ts`'s two tests).

- [ ] **Step 3: Contract freshness check**

Run: `go run ./cmd/gapreport`
Expected: no diff against the committed `docs/contracts/gap-report.md` (this change touches no routes, so none is expected — run `git diff --stat docs/contracts/gap-report.md` to confirm it's empty).

- [ ] **Step 4: Manual smoke check**

Run: `APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/seed && APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/server` in one terminal, `cd frontend-next && npm run dev` in another. Log in at `http://localhost:3000/app` as `admin@example.com` / `admin-demo-123`, open a project's Settings → Workflow tab, drag "DONE" above "TO DO" using the ⠿ handle, confirm the board's column order updates on the board page after a refresh, then drag it back.

- [ ] **Step 5: Final commit if any gate step required fixes**

Only if Steps 1-3 required code changes:

```bash
git add -A
git commit -m "fix: address quality gate findings for workflow reorder feature"
```
