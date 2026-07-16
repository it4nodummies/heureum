# Creazione manuale transizioni + feedback drag fallito — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users create/remove workflow transitions between statuses from the Project Settings → Workflow panel (so custom statuses/columns become usable via drag-and-drop on the board), and surface a visible error when a board drag fails because no transition exists.

**Architecture:** The backend already has working `AddTransition`/`DeleteTransition` service methods, `POST`/`DELETE /rest/api/3/project/{key}/workflow/transitions[/​{id}]` endpoints, and ready frontend API client functions (`workflow.addTransition`, `workflow.deleteTransition`) — none of it wired to any UI. This plan (1) adds a small form + remove buttons to `WorkflowEditor.tsx`'s existing read-only Transitions list, and (2) adds an `onError` handler + visible banner to the board's drag-move mutation, which currently swallows failed transitions silently. No backend changes.

**Tech Stack:** Next.js + React + TanStack Query + Tailwind (frontend only). Playwright for e2e.

Reference spec: `docs/superpowers/specs/2026-07-15-workflow-transitions-ui-design.md`

---

### Task 1: "Add transition" form + Remove button in `WorkflowEditor.tsx`

**Files:**
- Modify: `frontend-next/components/workflow/WorkflowEditor.tsx`

- [x] **Step 1: Replace the component**

Replace the entire content of `frontend-next/components/workflow/WorkflowEditor.tsx` with:

```tsx
"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { DndContext, DragEndEvent, closestCenter, KeyboardSensor, PointerSensor, useSensor, useSensors } from "@dnd-kit/core";
import { SortableContext, arrayMove, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
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
  const [fromStatusId, setFromStatusId] = useState("");
  const [toStatusId, setToStatusId] = useState("");
  const [transitionName, setTransitionName] = useState("");
  const [requireAssignee, setRequireAssignee] = useState(false);
  const [setResolutionFlag, setSetResolutionFlag] = useState(false);

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
  const addTransition = useMutation({
    mutationFn: () =>
      workflow.addTransition(projectKey, {
        from_status_id: fromStatusId,
        to_status_id: toStatusId,
        name: transitionName,
        require_assignee: requireAssignee,
        set_resolution: setResolutionFlag,
      }),
    onSuccess: () => {
      setTransitionName("");
      setRequireAssignee(false);
      setSetResolutionFlag(false);
      invalidate();
    },
  });
  const delTransition = useMutation({
    mutationFn: (id: string) => workflow.deleteTransition(projectKey, id),
    onSuccess: invalidate,
  });

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );

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
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
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
          {(wf.data?.transitions ?? []).map((t) => {
            const label = t.name || `${nameByID(t.from_status_id)} → ${nameByID(t.to_status_id)}`;
            return (
              <li key={t.id} className="flex items-center gap-2" data-testid={`transition-${label}`}>
                <span className="text-[#1a1f36]">{label}</span>
                <span className="text-xs text-slate-400">
                  {nameByID(t.from_status_id)} → {nameByID(t.to_status_id)}
                  {t.require_assignee ? " · requires assignee" : ""}
                  {t.set_resolution ? " · sets resolution" : ""}
                </span>
                <button
                  onClick={() => delTransition.mutate(t.id)}
                  className="ml-auto text-xs text-red-600 hover:underline"
                  aria-label={`Delete transition ${label}`}
                >
                  Remove
                </button>
              </li>
            );
          })}
        </ul>
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <select
            aria-label="From status"
            value={fromStatusId}
            onChange={(e) => setFromStatusId(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            <option value="">From status</option>
            {statuses.map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
          <select
            aria-label="To status"
            value={toStatusId}
            onChange={(e) => setToStatusId(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            <option value="">To status</option>
            {statuses.map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
          <input
            aria-label="Transition name"
            value={transitionName}
            onChange={(e) => setTransitionName(e.target.value)}
            placeholder="Transition name (optional)"
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          />
          <label className="flex items-center gap-1 text-xs text-slate-600">
            <input
              type="checkbox"
              aria-label="Require assignee"
              checked={requireAssignee}
              onChange={(e) => setRequireAssignee(e.target.checked)}
            />
            Require assignee
          </label>
          <label className="flex items-center gap-1 text-xs text-slate-600">
            <input
              type="checkbox"
              aria-label="Set resolution"
              checked={setResolutionFlag}
              onChange={(e) => setSetResolutionFlag(e.target.checked)}
            />
            Set resolution
          </label>
          <button
            onClick={() => fromStatusId && toStatusId && addTransition.mutate()}
            disabled={!fromStatusId || !toStatusId || addTransition.isPending}
            className="rounded bg-[#0052cc] px-3 py-1 text-sm text-white disabled:opacity-60"
          >
            Add transition
          </button>
        </div>
      </section>
    </div>
  );
}
```

This is additive relative to the current file (post-drag-and-drop-reorder state): only the `Transitions` section and the new mutations/state are new; the `Statuses` section is unchanged.

- [x] **Step 2: Type-check and build**

Run: `cd frontend-next && npm run build`
Expected: exits 0, no TypeScript errors.

- [x] **Step 3: Commit**

```bash
git add frontend-next/components/workflow/WorkflowEditor.tsx
git commit -m "feat(workflow): add form to create/remove transitions between statuses"
```

---

### Task 2: Visible error banner when a board drag fails

**Files:**
- Modify: `frontend-next/app/app/boards/[boardId]/page.tsx`

- [x] **Step 1: Add error state and banner**

Replace the full content of `frontend-next/app/app/boards/[boardId]/page.tsx` with:

```tsx
"use client";

import { use, useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { boards, issues as issuesApi, type SearchIssue } from "@/lib/api";
import { BoardColumns } from "@/components/board/BoardColumns";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

export default function BoardPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);
  const qc = useQueryClient();
  const [moveError, setMoveError] = useState<string | null>(null);

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });
  const config = useQuery({ queryKey: ["board", id, "config"], queryFn: () => boards.configuration(id) });
  const issueList = useQuery({ queryKey: ["board", id, "issues"], queryFn: () => boards.issues(id) });
  const projectKey = board.data?.location?.projectKey;

  const columns = useMemo(
    () => (config.data?.columnConfig.columns ?? []).map((c) => ({ id: c.statuses[0]?.id ?? c.name, name: c.name })),
    [config.data],
  );

  const issuesByStatus = useMemo(() => {
    const map: Record<string, SearchIssue[]> = {};
    for (const iss of issueList.data?.issues ?? []) {
      const sid = iss.fields.status?.name ?? "";
      // la colonna è per status id; mappiamo per id via config
      const col = (config.data?.columnConfig.columns ?? []).find((c) => c.name === iss.fields.status?.name);
      const key = col?.statuses[0]?.id ?? sid;
      (map[key] ??= []).push(iss);
    }
    return map;
  }, [issueList.data, config.data]);

  // Lo status non è un campo "fields" libero su PUT /rest/api/3/issue/{key}
  // (vedi lib/api.ts issues.update): il backend richiede una transizione
  // validata dal workflow, esposta come POST /rest/api/3/issue/{key}/transitions.
  const move = useMutation({
    mutationFn: ({ issueKey, toStatusId }: { issueKey: string; toStatusId: string }) =>
      issuesApi.transition(issueKey, toStatusId),
    onSuccess: () => {
      setMoveError(null);
      qc.invalidateQueries({ queryKey: ["board", id, "issues"] });
    },
    onError: (err: unknown) => {
      setMoveError(err instanceof Error ? err.message : "Move failed");
    },
  });

  return (
    <div>
      {projectKey && <ProjectHeader projectKey={projectKey} active="board" />}
      <div className="p-4">
        {moveError && (
          <div
            role="alert"
            data-testid="move-error"
            className="mb-2 flex items-center gap-2 rounded border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700"
          >
            <span>Move failed: {moveError}</span>
            <button
              onClick={() => setMoveError(null)}
              aria-label="Dismiss error"
              className="ml-auto text-red-700 hover:underline"
            >
              ×
            </button>
          </div>
        )}
        {columns.length > 0 && (
          <BoardColumns
            columns={columns}
            issuesByStatus={issuesByStatus}
            onMove={(issueKey, toStatusId) => move.mutate({ issueKey, toStatusId })}
          />
        )}
      </div>
    </div>
  );
}
```

The only changes from the current file: `useState` import, `moveError` state, `onError` on the `move` mutation, `onSuccess` also clears `moveError`, and the new banner block rendered above `BoardColumns`.

- [x] **Step 2: Type-check and build**

Run: `cd frontend-next && npm run build`
Expected: exits 0, no TypeScript errors.

- [x] **Step 3: Commit**

```bash
git add frontend-next/app/app/boards/\[boardId\]/page.tsx
git commit -m "feat(board): show visible error when a drag-and-drop move is rejected"
```

---

### Task 3: E2E test for creating/removing a transition

**Files:**
- Modify: `frontend-next/e2e/workflow.spec.ts`

- [x] **Step 1: Write the test**

Append to `frontend-next/e2e/workflow.spec.ts`:

```ts
test("workflow editor creates and removes a transition between two statuses", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();

  // Two fresh statuses with no transitions between them yet.
  await page.getByLabel("New status name").fill("Test A");
  await page.getByLabel("Category (reporting only)").selectOption("inprogress");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByTestId("status-Test A")).toBeVisible();

  await page.getByLabel("New status name").fill("Test B");
  await page.getByLabel("Category (reporting only)").selectOption("inprogress");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByTestId("status-Test B")).toBeVisible();

  // Create a transition Test A -> Test B.
  await page.getByLabel("From status").selectOption({ label: "Test A" });
  await page.getByLabel("To status").selectOption({ label: "Test B" });
  await page.getByLabel("Transition name").fill("Test Transition");
  await page.getByRole("button", { name: "Add transition" }).click();
  await expect(page.getByTestId("transition-Test Transition")).toBeVisible();

  // Remove it again.
  await page.getByLabel("Delete transition Test Transition").click();
  await expect(page.getByTestId("transition-Test Transition")).not.toBeVisible();
});
```

- [x] **Step 2: Run it**

Run: `cd frontend-next && npx playwright test e2e/workflow.spec.ts`
Expected: PASS (3 tests total: the two pre-existing ones plus this new one).

- [x] **Step 3: Commit**

```bash
git add frontend-next/e2e/workflow.spec.ts
git commit -m "test(e2e): cover creating and removing a workflow transition"
```

---

### Task 4: E2E test for the drag-error banner

**Files:**
- Modify: `frontend-next/e2e/board.spec.ts`

- [x] **Step 1: Write the test**

Append to `frontend-next/e2e/board.spec.ts`:

```ts
async function dragElement(page: Page, fromTestId: string, toTestId: string) {
  const source = page.getByTestId(fromTestId);
  const target = page.getByTestId(toTestId);
  const sourceBox = await source.boundingBox();
  const targetBox = await target.boundingBox();
  if (!sourceBox || !targetBox) throw new Error("drag element bounding box not found");
  await page.mouse.move(sourceBox.x + sourceBox.width / 2, sourceBox.y + sourceBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(targetBox.x + targetBox.width / 2, targetBox.y + targetBox.height / 2, { steps: 10 });
  await page.mouse.up();
}

test("dragging a card into a column with no valid transition shows an error", async ({ page }) => {
  await login(page);

  // Add a status with no transitions to/from it via the Workflow settings panel.
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();
  await page.getByLabel("New status name").fill("Blocked");
  await page.getByLabel("Category (reporting only)").selectOption("todo");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByTestId("status-Blocked")).toBeVisible();

  // Go to the board: the new status appears as a column, but no transition
  // reaches it, so dropping a card there must fail visibly instead of silently.
  await page.goto("/app/boards/1");
  await expect(page.locator('[data-testid="column-Blocked"]')).toBeVisible();
  const card = page.locator('[data-testid^="card-DEMO-"]').first();
  const cardTestId = await card.getAttribute("data-testid");
  if (!cardTestId) throw new Error("no seeded card found on board 1");

  await dragElement(page, cardTestId, "column-Blocked");

  await expect(page.getByTestId("move-error")).toBeVisible();
  await expect(page.getByTestId("move-error")).toContainText("invalid transition");

  // Dismissing clears the banner.
  await page.getByRole("button", { name: "Dismiss error" }).click();
  await expect(page.getByTestId("move-error")).not.toBeVisible();
});
```

- [x] **Step 2: Run it**

Run: `cd frontend-next && npx playwright test e2e/board.spec.ts`
Expected: PASS (all tests in the file, including this new one).

- [x] **Step 3: Commit**

```bash
git add frontend-next/e2e/board.spec.ts
git commit -m "test(e2e): cover error banner on invalid board drag transition"
```

---

### Task 5: Full quality gate

**Files:** none (verification only)

- [x] **Step 1: Backend build, vet, full test suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all pass (this feature made no backend changes, so this just confirms no regression).

- [x] **Step 2: Frontend build + full e2e suite**

Run: `cd frontend-next && npm run build && npx playwright test`
Expected: build succeeds; all Playwright specs pass, including the two new tests from Tasks 3 and 4.

- [x] **Step 3: Contract freshness check**

Run: `go run ./cmd/gapreport`
Expected: no diff against the committed `docs/contracts/gap-report.md` (no routes changed).

- [x] **Step 4: Manual smoke check**

If a local Docker stack is running and holding ports 8080/3000, stop it first (`docker compose -f deploy/docker/docker-compose.yml stop`) so Playwright's own webServer can bind those ports, then restart it afterward (`docker compose -f deploy/docker/docker-compose.yml start`) — do not leave it stopped.

- [x] **Step 5: Final commit if any gate step required fixes**

Only if Steps 1-3 required code changes:

```bash
git add -A
git commit -m "fix: address quality gate findings for workflow transitions UI"
```

---

### Task 6: Visible error feedback on Workflow panel mutations

A final holistic review of Tasks 1-5 found that the new `addTransition`/`delTransition` mutations
(and the pre-existing `addStatus`/`delStatus`) in `WorkflowEditor.tsx` have no `onError`/`isError`
rendering — the exact "silent failure" defect class this round exists to fix, just relocated to a
sibling mutation. E.g. a non-admin project member who opens Settings → Workflow and clicks "Add
transition" gets a 403 with zero visible feedback. Confirmed with the user this should be fixed now
rather than filed as a separate follow-up.

**Files:**
- Modify: `frontend-next/components/workflow/WorkflowEditor.tsx`

- [ ] **Step 1: Add per-mutation error paragraphs**

Follow the existing convention already used in `frontend-next/components/projects/ProjectSettings.tsx`
(e.g. lines 149-152, 181-185): a conditional `<p>` right after the control that triggers the
mutation, rendered only when `mutation.isError`, showing `mutation.error instanceof Error ?
mutation.error.message : "<fallback text>"`.

Add four such paragraphs in `WorkflowEditor.tsx`:

1. After the "Add status" button row (inside the `<div className="mt-2 flex gap-2">` block, or
   directly after it), when `addStatus.isError`:
   ```tsx
   {addStatus.isError && (
     <p className="mt-1 text-sm text-red-600">
       {addStatus.error instanceof Error ? addStatus.error.message : "Failed to add status"}
     </p>
   )}
   ```
2. Somewhere visible in the Statuses section (e.g. right after the statuses `<ul>`/`DndContext`
   block, before the add-status form), when `delStatus.isError`:
   ```tsx
   {delStatus.isError && (
     <p className="mt-1 text-sm text-red-600">
       {delStatus.error instanceof Error ? delStatus.error.message : "Failed to delete status"}
     </p>
   )}
   ```
3. After the "Add transition" button row, when `addTransition.isError`:
   ```tsx
   {addTransition.isError && (
     <p className="mt-1 text-sm text-red-600">
       {addTransition.error instanceof Error ? addTransition.error.message : "Failed to add transition"}
     </p>
   )}
   ```
4. Right after the transitions `<ul>`, when `delTransition.isError`:
   ```tsx
   {delTransition.isError && (
     <p className="mt-1 text-sm text-red-600">
       {delTransition.error instanceof Error ? delTransition.error.message : "Failed to delete transition"}
     </p>
   )}
   ```

Exact placement within each section is at the implementer's discretion as long as each paragraph is
visually associated with the section/control whose mutation it reports on, and only one of the four
can be relevant at a time in practice (a user triggers one mutation at a time) so no layout
conflicts are expected.

- [ ] **Step 2: Type-check and build**

Run: `cd frontend-next && npm run build`
Expected: exits 0, no TypeScript errors.

- [ ] **Step 3: E2E test for one error path**

`cmd/seed/main.go` already seeds a non-admin demo user with a real, deterministic 403 path:
`dev@example.com` / `dev-demo-123` (username `dev`), added to the DEMO project with `MemberRole`
`"member"` (`cmd/seed/main.go:45-47,97`). `AddTransition`/`AddStatus` require `AdministerProjects`
(`router.go:246,249`, `internal/domain/permission`), which a `member` role does not have — so this
user's login is a real, reliable way to trigger a genuine 403 without any new seed data.

Add a test to `frontend-next/e2e/workflow.spec.ts`:

```ts
test("workflow editor shows an error when a non-admin tries to add a transition", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("dev@example.com");
  await page.getByLabel(/password/i).fill("dev-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app/);

  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();

  await page.getByLabel("From status").selectOption({ label: "TO DO" });
  await page.getByLabel("To status").selectOption({ label: "DONE" });
  await page.getByRole("button", { name: "Add transition" }).click();

  await expect(page.getByText("you do not have permission to perform this action")).toBeVisible();
});
```

The exact message is not a guess: `authz.WriteForbidden` (`internal/api/authz/enforce.go:13-15`)
always responds with `v3.WriteError(w, 403, []string{"you do not have permission to perform this
action"}, nil)`, and `apiFetch` (`lib/api.ts`) joins `errorMessages` into `err.message` verbatim —
so this exact string is what `addTransition.error.message` will contain, taking precedence over the
`"Failed to add transition"` fallback from Step 1 (which only applies when `error` isn't an
`Error` instance, which won't happen here).

- [ ] **Step 4: Manual smoke check**

If a local Docker stack is running and holding ports 8080/3000, stop it first (`docker compose -f
deploy/docker/docker-compose.yml stop` from the repo root), then restart it afterward (`docker
compose -f deploy/docker/docker-compose.yml start`) — do not leave it stopped. With the app running,
log in as `dev@example.com` / `dev-demo-123`, open the DEMO project's Settings → Workflow, and
confirm the exact rendered error text after clicking "Add transition" matches what Step 3's test
asserts, before finalizing that test's assertion string.

- [ ] **Step 5: Run the full quality gate once more**

Run: `go build ./... && go vet ./... && go test ./...`, then `cd frontend-next && npm run build &&
npx playwright test` (full suite, not a single file — this round's history shows full-suite runs can
surface issues isolation-only runs miss), then `go run ./cmd/gapreport` and confirm no diff.

- [ ] **Step 6: Commit**

```bash
git add frontend-next/components/workflow/WorkflowEditor.tsx frontend-next/e2e/workflow.spec.ts
git commit -m "feat(workflow): show error feedback on failed status/transition mutations"
```
