# Backlog drag & drop + selezione multipla verso gli sprint — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users drag issues between the Backlog and any Sprint (including directly between two sprints), multi-select issues to move as a group, and reorder issues within the same list — with a live visual preview while dragging, backed by already-existing/working backend endpoints.

**Architecture:** `frontend-next/app/app/boards/[boardId]/backlog/page.tsx` currently renders static, non-interactive rows. This plan (1) extracts a sortable, checkbox-enabled `IssueCard` and a droppable `DroppableList` container component, (2) rewrites the backlog page to hold a single `items: Record<containerId, string[]>` piece of local state as the drag source-of-truth (synced from server data when not dragging and no commit is in flight), wired to a single `DndContext` implementing dnd-kit's standard "multiple containers" pattern (`onDragOver` moves a block of one-or-more dragged issue keys between container arrays live, but ONLY for cross-container hovers — same-container reordering is deliberately left to dnd-kit's own `SortableContext` animation and is committed once, in `onDragEnd`, from the real drop position; `onDragEnd` commits via the existing `sprints.moveIssues`/`agileIssues.moveToBacklog`/`agileIssues.rank` API clients, with a dismissible error banner on failure), and (3) adds e2e coverage for single-item cross-container drag, direct sprint-to-sprint drag, multi-select group drag, and same-list reorder with persistence. No backend changes.

**Tech Stack:** Next.js + React + TanStack Query (`useQuery`, `useQueries`, `useMutation`) + `@dnd-kit/core` + `@dnd-kit/sortable` + `@dnd-kit/utilities` + Tailwind. Playwright for e2e.

Reference spec: `docs/superpowers/specs/2026-07-16-backlog-sprint-dnd-design.md`

**Known simplifications carried over from the spec (not defects, just called out so nobody "fixes" them by surprise):**
- No `sortableKeyboardCoordinates`-style cross-container keyboard support (plain `KeyboardSensor` only) — full keyboard-driven cross-container reordering is a larger, separate effort.
- Dropping onto an item always inserts the dragged block immediately *before* that item (no upper-half/lower-half cursor-position detection) — deterministic and simple, not a bug.
- `onDragOver` never mutates `items` for a same-container hover (see Task 2, Step 1's comment above `handleDragOver`) — this is required for correctness, not just style: mutating on every same-container hover (including the trivial `over.id === active.id` case fired the instant a drag starts) caused a real, reproduced infinite-update-loop crash during code review of an earlier draft of this task. Same-container visual feedback during the drag comes from dnd-kit's own `SortableContext`/`useSortable` animation, not from app state.
- (Added during Task 4) A second, related dnd-kit multi-container gotcha surfaced under a full-suite run with 3+ stacked sprint/backlog containers: moving an item across containers changes both containers' measured heights, which can flip `closestCenter`'s "nearest" pick back and forth with zero real pointer movement, crashing the page the same way. Fixed in `handleDragOver`/`handleDragStart` with a `recentlyMovedToNewContainer` ref, set right after a cross-container move and cleared one animation frame later via a `useEffect` keyed on `items` — inspired by (not a literal copy of) dnd-kit's own official "multiple containers" example, which solves the same class of problem inside a custom `collisionDetection` callback rather than as an early-return in application code. See commit `bb26975` for the exact diff; this plan's Task 2 code block above was not retroactively updated with it — read the actual committed `page.tsx` as the source of truth from this point on.

---

### Task 1: Extract `IssueCard` and `DroppableList` components

**Files:**
- Create: `frontend-next/components/backlog/IssueCard.tsx`
- Create: `frontend-next/components/backlog/DroppableList.tsx`

- [ ] **Step 1: Create `IssueCard`**

```tsx
"use client";

import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { SearchIssue } from "@/lib/api";

export function IssueCard({
  issue,
  selected,
  onToggleSelect,
}: {
  issue: SearchIssue;
  selected: boolean;
  onToggleSelect: (key: string) => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: issue.key });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  return (
    <div
      ref={setNodeRef}
      style={style}
      className="flex items-center gap-2 border-b border-slate-100 py-1 text-sm"
      data-testid={`row-${issue.key}`}
    >
      <input
        type="checkbox"
        aria-label={`Select ${issue.key}`}
        checked={selected}
        onChange={() => onToggleSelect(issue.key)}
        className="shrink-0"
      />
      <button
        type="button"
        {...attributes}
        {...listeners}
        className="cursor-grab text-slate-400 hover:text-slate-600"
        aria-label={`Drag ${issue.key}`}
        data-testid={`drag-handle-${issue.key}`}
      >
        ⠿
      </button>
      <span className="font-mono text-xs text-slate-500">{issue.key}</span>
      <span className="text-[#1a1f36]">{issue.fields.summary}</span>
    </div>
  );
}
```

- [ ] **Step 2: Create `DroppableList`**

```tsx
"use client";

import type { ReactNode } from "react";
import { useDroppable } from "@dnd-kit/core";
import { SortableContext, verticalListSortingStrategy } from "@dnd-kit/sortable";
import type { SearchIssue } from "@/lib/api";
import { IssueCard } from "./IssueCard";

export function DroppableList({
  containerId,
  items,
  issuesByKey,
  selected,
  onToggleSelect,
  emptyLabel,
  header,
  testId,
}: {
  containerId: string;
  items: string[];
  issuesByKey: Record<string, SearchIssue>;
  selected: Set<string>;
  onToggleSelect: (key: string) => void;
  emptyLabel: string;
  header?: ReactNode;
  testId: string;
}) {
  const { setNodeRef } = useDroppable({ id: containerId });
  return (
    <div className="mb-3 rounded border border-slate-200 p-2" data-testid={`container-${testId}`}>
      {header}
      <SortableContext items={items} strategy={verticalListSortingStrategy}>
        <div ref={setNodeRef} className="min-h-[2.5rem]" data-testid={testId}>
          {items.map((key) => {
            const issue = issuesByKey[key];
            if (!issue) return null;
            return (
              <IssueCard key={key} issue={issue} selected={selected.has(key)} onToggleSelect={onToggleSelect} />
            );
          })}
          {items.length === 0 && <p className="py-2 text-sm text-slate-400">{emptyLabel}</p>}
        </div>
      </SortableContext>
    </div>
  );
}
```

Note: `data-testid={testId}` is placed on the SAME element as `ref={setNodeRef}` (the actual registered droppable), not on the outer wrapper — this matters for e2e drag simulations that compute drop coordinates from this testid's bounding box; if the testid were on a taller outer wrapper (header + list), the computed center could land outside the real droppable area.

The outer wrapper carries a SEPARATE `data-testid={`container-${testId}`}` for a different purpose: e2e tests that create a sprint via the UI don't know its numeric id upfront, so they need to find "the container whose visible text includes the sprint's name" — but that name lives in `header`, which is a DOM sibling of the inner droppable div, not an ancestor/descendant of it, so a `.filter({ hasText })` search scoped to the inner div alone can never see it. The outer wrapper is the only element that has both the header's text and the droppable div as descendants, so the two testids serve genuinely different consumers (drag coordinates vs. name-based lookup) — this isn't redundant.

- [ ] **Step 3: Type-check**

Run: `cd frontend-next && npm run build`
Expected: exits 0. (These two components aren't wired into any page yet, so this only validates they compile standalone — no runtime behavior to test yet.)

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/backlog/IssueCard.tsx frontend-next/components/backlog/DroppableList.tsx
git commit -m "feat(backlog): add sortable IssueCard and droppable DroppableList components"
```

---

### Task 2: Rewrite the Backlog page with multi-container drag & drop

**Files:**
- Modify: `frontend-next/app/app/boards/[boardId]/backlog/page.tsx` (full rewrite)

- [ ] **Step 1: Replace the page**

Replace the entire content of `frontend-next/app/app/boards/[boardId]/backlog/page.tsx` with:

```tsx
"use client";

import { use, useEffect, useRef, useState } from "react";
import { useQuery, useQueries, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  DndContext,
  DragEndEvent,
  DragOverEvent,
  DragStartEvent,
  DragOverlay,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import { boards, sprints, agileIssues, type SearchIssue } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";
import { DroppableList } from "@/components/backlog/DroppableList";

const BACKLOG_ID = "backlog";
const sprintContainerId = (sprintId: number) => `sprint-${sprintId}`;

function findContainer(itemsMap: Record<string, string[]>, id: string): string | undefined {
  if (id in itemsMap) return id;
  return Object.keys(itemsMap).find((key) => itemsMap[key].includes(id));
}

export default function BacklogPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);
  const qc = useQueryClient();
  const [newSprint, setNewSprint] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [activeId, setActiveId] = useState<string | null>(null);
  const [items, setItems] = useState<Record<string, string[]>>({});
  const [dragError, setDragError] = useState<string | null>(null);
  const dragStartContainer = useRef<string | null>(null);

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });
  const backlog = useQuery({ queryKey: ["board", id, "backlog"], queryFn: () => boards.backlog(id) });
  const sprintList = useQuery({ queryKey: ["board", id, "sprints"], queryFn: () => boards.sprints(id) });
  const projectKey = board.data?.location?.projectKey;

  const sprintValues = sprintList.data?.values ?? [];
  const sprintIssuesQueries = useQueries({
    queries: sprintValues.map((sp) => ({
      queryKey: ["sprint", sp.id, "issues"],
      queryFn: () => sprints.issues(sp.id),
    })),
  });

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["board", id, "backlog"] });
    qc.invalidateQueries({ queryKey: ["board", id, "sprints"] });
    for (const sp of sprintValues) {
      qc.invalidateQueries({ queryKey: ["sprint", sp.id, "issues"] });
    }
  };

  const serverItems: Record<string, string[]> = { [BACKLOG_ID]: (backlog.data?.issues ?? []).map((i) => i.key) };
  const issuesByKey: Record<string, SearchIssue> = {};
  for (const iss of backlog.data?.issues ?? []) issuesByKey[iss.key] = iss;
  sprintValues.forEach((sp, idx) => {
    const list = sprintIssuesQueries[idx]?.data?.issues ?? [];
    serverItems[sprintContainerId(sp.id)] = list.map((i) => i.key);
    for (const iss of list) issuesByKey[iss.key] = iss;
  });

  const createSprint = useMutation({
    mutationFn: (name: string) => sprints.create(name, id),
    onSuccess: () => {
      setNewSprint("");
      invalidate();
    },
  });
  const setState = useMutation({
    mutationFn: ({ sprintId, state }: { sprintId: number; state: "active" | "closed" }) =>
      sprints.setState(sprintId, state),
    onSuccess: invalidate,
  });
  const moveAndRank = useMutation({
    mutationFn: async (vars: {
      sourceContainer: string;
      targetContainer: string;
      draggedKeys: string[];
      before?: string;
      after?: string;
      wasGroupDrag: boolean;
    }) => {
      if (vars.sourceContainer !== vars.targetContainer) {
        if (vars.targetContainer === BACKLOG_ID) {
          await agileIssues.moveToBacklog(vars.draggedKeys);
        } else {
          const sprintId = Number(vars.targetContainer.replace("sprint-", ""));
          await sprints.moveIssues(sprintId, vars.draggedKeys);
        }
      }
      if (vars.before || vars.after) {
        await agileIssues.rank(vars.draggedKeys, vars.before, vars.after);
      }
    },
    onSuccess: (_data, vars) => {
      // Only clear the selection if this drag actually moved it — dragging a lone,
      // unselected issue must not wipe a selection the user is still building for a
      // separate, later bulk action.
      if (vars.wasGroupDrag) setSelected(new Set());
      setDragError(null);
      invalidate();
    },
    onError: (err) => {
      setDragError(err instanceof Error ? err.message : "Failed to move issue(s)");
      invalidate();
    },
  });

  useEffect(() => {
    // Skip while a move/rank commit is in flight, not just while a drag is active: onDragEnd
    // clears activeId synchronously, but the mutation's network round-trip (and the
    // invalidation it triggers) hasn't resolved yet, so serverItems here can still be the
    // pre-drop snapshot — syncing now would flash the UI back to the old arrangement for a
    // moment before the real refetch corrects it.
    if (activeId || moveAndRank.isPending) return;
    setItems(serverItems);
    // Depend on a stable serialized snapshot, not the object identity (which changes every
    // render), so this only re-syncs when the actual server-derived content changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [JSON.stringify(serverItems), activeId, moveAndRank.isPending]);

  const toggleSelect = (key: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const handleDragStart = (event: DragStartEvent) => {
    const key = String(event.active.id);
    setActiveId(key);
    dragStartContainer.current = findContainer(items, key) ?? null;
  };

  // Live cross-container preview ONLY. Same-container hovers are intentionally a no-op here:
  // dnd-kit's own SortableContext/useSortable already animates sibling items shifting within a
  // single list purely from layout, with no app-level state mutation needed — mirroring that is
  // both redundant and actively dangerous: mutating `items` on every same-container onDragOver
  // (including the very first event, where `over.id === active.id`) can flip the dragged item to
  // the end of its own list, which shifts the DOM under the pointer, which fires another
  // onDragOver, which mutates again — a real infinite-update loop ("Maximum update depth
  // exceeded"), not a hypothetical one. The final same-container order is computed once, in
  // `handleDragEnd`, from the actual `over` position at drop time.
  const handleDragOver = (event: DragOverEvent) => {
    const { active, over } = event;
    if (!over) return;
    const activeKey = String(active.id);
    const overId = String(over.id);
    if (activeKey === overId) return;

    const activeContainer = findContainer(items, activeKey);
    const overContainer = overId in items ? overId : findContainer(items, overId);
    if (!activeContainer || !overContainer || activeContainer === overContainer) return;

    const draggedKeys = selected.has(activeKey) && selected.size > 1 ? Array.from(selected) : [activeKey];

    setItems((prev) => {
      const pActiveContainer = findContainer(prev, activeKey);
      const pOverContainer = overId in prev ? overId : findContainer(prev, overId);
      if (!pActiveContainer || !pOverContainer || pActiveContainer === pOverContainer) return prev;

      const withoutDragged: Record<string, string[]> = {};
      for (const [cid, keys] of Object.entries(prev)) {
        withoutDragged[cid] = keys.filter((k) => !draggedKeys.includes(k));
      }
      const overItemsAfterRemoval = withoutDragged[pOverContainer];
      const insertIndex =
        overId === pOverContainer
          ? overItemsAfterRemoval.length
          : (() => {
              const idx = overItemsAfterRemoval.indexOf(overId);
              return idx >= 0 ? idx : overItemsAfterRemoval.length;
            })();
      withoutDragged[pOverContainer] = [
        ...overItemsAfterRemoval.slice(0, insertIndex),
        ...draggedKeys,
        ...overItemsAfterRemoval.slice(insertIndex),
      ];
      return withoutDragged;
    });
  };

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    const activeKey = String(active.id);
    const wasGroupDrag = selected.has(activeKey) && selected.size > 1;
    const draggedKeys = wasGroupDrag ? Array.from(selected) : [activeKey];
    const sourceContainer = dragStartContainer.current;
    setActiveId(null);
    dragStartContainer.current = null;
    if (!sourceContainer || !over) return;

    const overId = String(over.id);
    const draggedSet = new Set(draggedKeys);
    // If onDragOver already moved the block to a different container, `items` reflects that
    // final placement. Otherwise (a same-container drag, which onDragOver deliberately ignores —
    // see the comment above `handleDragOver`) `items[sourceContainer]` is still the pre-drag
    // order, so the reordered block position is computed here, from the actual drop target.
    const liveTargetContainer = findContainer(items, activeKey);
    let targetContainer: string;
    let finalTargetList: string[];
    if (liveTargetContainer && liveTargetContainer !== sourceContainer) {
      targetContainer = liveTargetContainer;
      finalTargetList = items[targetContainer];
    } else {
      targetContainer = sourceContainer;
      const withoutDragged = items[sourceContainer].filter((k) => !draggedSet.has(k));
      const insertIndex =
        overId === sourceContainer
          ? withoutDragged.length
          : (() => {
              const idx = withoutDragged.indexOf(overId);
              return idx >= 0 ? idx : withoutDragged.length;
            })();
      finalTargetList = [...withoutDragged.slice(0, insertIndex), ...draggedKeys, ...withoutDragged.slice(insertIndex)];
      setItems((prev) => ({ ...prev, [sourceContainer]: finalTargetList }));
    }

    const firstIndex = finalTargetList.findIndex((k) => draggedSet.has(k));
    const after = firstIndex > 0 ? finalTargetList[firstIndex - 1] : undefined;
    let lastDraggedIndex = -1;
    for (let i = finalTargetList.length - 1; i >= 0; i--) {
      if (draggedSet.has(finalTargetList[i])) {
        lastDraggedIndex = i;
        break;
      }
    }
    const before =
      lastDraggedIndex >= 0 && lastDraggedIndex + 1 < finalTargetList.length ? finalTargetList[lastDraggedIndex + 1] : undefined;

    if (sourceContainer === targetContainer && !before && !after) return;

    moveAndRank.mutate({ sourceContainer, targetContainer, draggedKeys, before, after, wasGroupDrag });
  };

  const sensors = useSensors(useSensor(PointerSensor), useSensor(KeyboardSensor));

  const activeIssue = activeId ? issuesByKey[activeId] : undefined;
  const activeCount = activeId && selected.has(activeId) && selected.size > 1 ? selected.size : 1;

  return (
    <div>
      {projectKey && <ProjectHeader projectKey={projectKey} active="backlog" />}
      <div className="mx-auto max-w-3xl p-4">
        {dragError && (
          <div
            role="alert"
            data-testid="backlog-drag-error"
            className="mb-2 flex items-center gap-2 rounded border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700"
          >
            <span>Move failed: {dragError}</span>
            <button
              onClick={() => setDragError(null)}
              aria-label="Dismiss error"
              className="ml-auto text-red-700 hover:underline"
            >
              ×
            </button>
          </div>
        )}
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragStart={handleDragStart}
          onDragOver={handleDragOver}
          onDragEnd={handleDragEnd}
        >
          {sprintValues.map((sp) => (
            <DroppableList
              key={sp.id}
              containerId={sprintContainerId(sp.id)}
              items={items[sprintContainerId(sp.id)] ?? []}
              issuesByKey={issuesByKey}
              selected={selected}
              onToggleSelect={toggleSelect}
              emptyLabel="No issues in this sprint"
              testId={sprintContainerId(sp.id)}
              header={
                <div className="mb-1 flex items-center justify-between">
                  <span className="text-sm font-semibold text-[#1a1f36]">
                    {sp.name} <span className="text-xs font-normal text-slate-400">({sp.state})</span>
                  </span>
                  <span className="flex gap-2">
                    {sp.state === "future" && (
                      <button
                        onClick={() => setState.mutate({ sprintId: sp.id, state: "active" })}
                        className="rounded border border-slate-300 px-2 py-0.5 text-xs"
                      >
                        Start sprint
                      </button>
                    )}
                    {sp.state === "active" && (
                      <button
                        onClick={() => setState.mutate({ sprintId: sp.id, state: "closed" })}
                        className="rounded border border-slate-300 px-2 py-0.5 text-xs"
                      >
                        Complete sprint
                      </button>
                    )}
                  </span>
                </div>
              }
            />
          ))}

          <div className="my-3 flex gap-2">
            <input
              aria-label="New sprint name"
              value={newSprint}
              onChange={(e) => setNewSprint(e.target.value)}
              placeholder="Sprint name"
              className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
            />
            <button
              onClick={() => newSprint && createSprint.mutate(newSprint)}
              className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
              disabled={createSprint.isPending}
            >
              Create sprint
            </button>
          </div>

          <DroppableList
            containerId={BACKLOG_ID}
            items={items[BACKLOG_ID] ?? []}
            issuesByKey={issuesByKey}
            selected={selected}
            onToggleSelect={toggleSelect}
            emptyLabel="Backlog is empty"
            testId="backlog-list"
            header={
              <div className="mb-1 text-sm font-semibold text-slate-500">
                Backlog ({(items[BACKLOG_ID] ?? []).length})
              </div>
            }
          />

          <DragOverlay>
            {activeIssue && (
              <div className="rounded border border-slate-300 bg-white px-2 py-1 text-sm shadow-md">
                <span className="font-mono text-xs text-slate-500">{activeIssue.key}</span>{" "}
                <span className="text-[#1a1f36]">{activeIssue.fields.summary}</span>
                {activeCount > 1 && (
                  <span className="ml-2 rounded bg-[#0052cc] px-1.5 py-0.5 text-xs text-white">
                    +{activeCount - 1} more
                  </span>
                )}
              </div>
            )}
          </DragOverlay>
        </DndContext>
      </div>
    </div>
  );
}
```

Notes for the implementer:
- `serverItems`/`issuesByKey` are recomputed as plain values every render (no `useMemo`) — deliberate: memoizing them with `useQueries`' variable-length result array in the dependency list would produce a dependency array whose *length* changes whenever a sprint is added, which violates React's rules of hooks (dependency array size must be constant across renders) and triggers a real runtime warning/bug. Recomputing every render is correct and cheap here (a handful of issues/sprints).
- The `useEffect`'s dependency array (`[JSON.stringify(serverItems), activeId, moveAndRank.isPending]`) is always exactly length-3 — `JSON.stringify(...)` is a plain string, `activeId`/`moveAndRank.isPending` are primitives — so it's safe regardless of how many sprints exist.
- `items` (local state) is the single source of truth passed to every `DroppableList`'s `items` prop and to `DragOverlay`'s count — never read `serverItems` directly for rendering.
- The `moveAndRank` mutation's `useEffect` sync-guard MUST be declared textually AFTER `moveAndRank` itself (not before) — the effect's dependency array references `moveAndRank.isPending`, and since `moveAndRank` is a `const`, referencing it before its own declaration in the same function body is a temporal-dead-zone error, not just a style nit.
- `dragError` surfaces `moveAndRank`'s failures in a dismissible banner (same pattern as the board page's `moveError`, `app/app/boards/[boardId]/page.tsx`) — don't skip it as "just a UI nicety"; a prior review found the earlier draft failed silently on a partial move+rank failure.
- `wasGroupDrag` is threaded through `moveAndRank`'s variables specifically so `onSuccess` only clears the multi-select when the just-completed drag actually moved the selection — dragging one unselected issue while an unrelated selection is still pending must not wipe it.

- [ ] **Step 2: Type-check and build**

Run: `cd frontend-next && npm run build`
Expected: exits 0, no TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/app/app/boards/\[boardId\]/backlog/page.tsx
git commit -m "feat(backlog): multi-container drag-and-drop between backlog and sprints"
```

---

### Task 3: E2E test — single-item drag from Backlog into a Sprint

**Files:**
- Modify: `frontend-next/e2e/board.spec.ts`

- [ ] **Step 1: Add a scroll-safe drag helper and the test**

Append to `frontend-next/e2e/board.spec.ts`:

```ts
async function dragBetween(page: Page, fromTestId: string, toTestId: string) {
  const source = page.getByTestId(fromTestId);
  const target = page.getByTestId(toTestId);
  await source.scrollIntoViewIfNeeded();
  const sourceBox = await source.boundingBox();
  if (!sourceBox) throw new Error(`source ${fromTestId} not found`);
  await page.mouse.move(sourceBox.x + sourceBox.width / 2, sourceBox.y + sourceBox.height / 2);
  await page.mouse.down();
  await target.scrollIntoViewIfNeeded();
  const targetBox = await target.boundingBox();
  if (!targetBox) throw new Error(`target ${toTestId} not found`);
  await page.mouse.move(targetBox.x + targetBox.width / 2, targetBox.y + targetBox.height / 2, { steps: 10 });
  await page.mouse.up();
}

test("backlog: drag a single issue into a sprint", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");

  const sprintName = `Sprint DnD ${Date.now()}`;
  await page.getByLabel("New sprint name").fill(sprintName);
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText(sprintName)).toBeVisible();

  // The sprint's name lives in DroppableList's `header`, a DOM sibling of the inner droppable
  // div — not a descendant of it — so the lookup goes through the outer wrapper's
  // `container-{testId}` testid (see Task 1's note) to find the sprint by name, then strips the
  // prefix to get the real drop-target/containment testid.
  const sprintOuter = page.locator('[data-testid^="container-sprint-"]').filter({ hasText: sprintName });
  const outerTestId = await sprintOuter.getAttribute("data-testid");
  if (!outerTestId) throw new Error("sprint container testid not found");
  const sprintTestId = outerTestId.replace("container-", "");
  const sprintContainer = page.getByTestId(sprintTestId);

  await expect(page.getByTestId("row-DEMO-1")).toBeVisible();
  await dragBetween(page, "drag-handle-DEMO-1", sprintTestId);

  await expect(sprintContainer.getByTestId("row-DEMO-1")).toBeVisible();
  await expect(page.getByTestId("backlog-list").getByTestId("row-DEMO-1")).not.toBeVisible();
});
```

`sprintContainer.getByTestId(...)`/`page.getByTestId("backlog-list").getByTestId(...)` scope the
lookup to descendants of that container, which is how "is this row inside sprint X vs. still in the
backlog" is verified — a plain `page.getByTestId("row-DEMO-1")` would still find it wherever it
moved to, so the scoped variant is required, not stylistic.

- [ ] **Step 2: Run it**

Run: `cd frontend-next && npx playwright test e2e/board.spec.ts`
Expected: PASS (all tests in the file, including this new one). If ports 8080/3000 are occupied by
a local Docker stack, stop it first (`docker compose -f deploy/docker/docker-compose.yml stop`),
run the tests, then restart it (`docker compose -f deploy/docker/docker-compose.yml start`).

If it fails, debug for real (read the actual Playwright trace/output): common causes given this
repo's history — a droppable/sortable target scrolled out of the board's container before its
bounding box is measured (this helper already scrolls both source and target right before
measuring, mirroring the fix from a prior round's board-drag test), or `over` resolving to `null`
because the `DndContext`'s sensors didn't activate (check the built `WorkflowEditor.tsx` sensors
setup for the working reference pattern).

- [ ] **Step 3: Commit**

```bash
git add frontend-next/e2e/board.spec.ts
git commit -m "test(e2e): cover dragging a single backlog issue into a sprint"
```

---

### Task 4: E2E test — direct sprint-to-sprint drag

**Files:**
- Modify: `frontend-next/e2e/board.spec.ts`

- [ ] **Step 1: Add the test**

Append to `frontend-next/e2e/board.spec.ts`:

```ts
test("backlog: drag between two sprints directly, without going through the backlog", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");

  const sprintAName = `Sprint A ${Date.now()}`;
  await page.getByLabel("New sprint name").fill(sprintAName);
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText(sprintAName)).toBeVisible();

  const sprintBName = `Sprint B ${Date.now()}`;
  await page.getByLabel("New sprint name").fill(sprintBName);
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText(sprintBName)).toBeVisible();

  // See Task 3's comment: lookup by name goes through the outer `container-{testId}` wrapper,
  // then strips the prefix to get the real drop-target/containment testid.
  const sprintAOuter = page.locator('[data-testid^="container-sprint-"]').filter({ hasText: sprintAName });
  const sprintBOuter = page.locator('[data-testid^="container-sprint-"]').filter({ hasText: sprintBName });
  const sprintAOuterTestId = await sprintAOuter.getAttribute("data-testid");
  const sprintBOuterTestId = await sprintBOuter.getAttribute("data-testid");
  if (!sprintAOuterTestId || !sprintBOuterTestId) throw new Error("sprint container testid not found");
  const sprintATestId = sprintAOuterTestId.replace("container-", "");
  const sprintBTestId = sprintBOuterTestId.replace("container-", "");
  const sprintA = page.getByTestId(sprintATestId);
  const sprintB = page.getByTestId(sprintBTestId);

  await dragBetween(page, "drag-handle-DEMO-2", sprintATestId);
  await expect(sprintA.getByTestId("row-DEMO-2")).toBeVisible();

  await dragBetween(page, "drag-handle-DEMO-2", sprintBTestId);
  await expect(sprintB.getByTestId("row-DEMO-2")).toBeVisible();
  await expect(sprintA.getByTestId("row-DEMO-2")).not.toBeVisible();
});
```

This uses `DEMO-2` (not `DEMO-1`, already used by Task 3's test) so the two tests don't interfere
with each other regardless of execution order within this file or across a full-suite run.

- [ ] **Step 2: Run it**

Run: `cd frontend-next && npx playwright test e2e/board.spec.ts`
Expected: PASS (all tests, including this one).

- [ ] **Step 3: Commit**

```bash
git add frontend-next/e2e/board.spec.ts
git commit -m "test(e2e): cover dragging an issue directly between two sprints"
```

---

### Task 5: E2E test — multi-select group drag

**Files:**
- Modify: `frontend-next/e2e/board.spec.ts`

- [ ] **Step 1: Add the test**

Append to `frontend-next/e2e/board.spec.ts`:

```ts
test("backlog: multi-select drag moves all selected issues together", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");

  const sprintName = `Sprint Group ${Date.now()}`;
  await page.getByLabel("New sprint name").fill(sprintName);
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText(sprintName)).toBeVisible();
  // See Task 3's comment: lookup by name goes through the outer `container-{testId}` wrapper.
  const sprintOuter = page.locator('[data-testid^="container-sprint-"]').filter({ hasText: sprintName });
  const outerTestId = await sprintOuter.getAttribute("data-testid");
  if (!outerTestId) throw new Error("sprint container testid not found");
  const sprintTestId = outerTestId.replace("container-", "");
  const sprintContainer = page.getByTestId(sprintTestId);

  await page.getByLabel("Select DEMO-4").check();
  await page.getByLabel("Select DEMO-5").check();

  await dragBetween(page, "drag-handle-DEMO-4", sprintTestId);

  await expect(sprintContainer.getByTestId("row-DEMO-4")).toBeVisible();
  await expect(sprintContainer.getByTestId("row-DEMO-5")).toBeVisible();
});
```

Uses `DEMO-4`/`DEMO-5`, disjoint from `DEMO-1`/`DEMO-2` used by the previous two tasks' tests.

- [ ] **Step 2: Run it**

Run: `cd frontend-next && npx playwright test e2e/board.spec.ts`
Expected: PASS (all tests, including this one).

- [ ] **Step 3: Commit**

```bash
git add frontend-next/e2e/board.spec.ts
git commit -m "test(e2e): cover multi-select group drag in the backlog"
```

---

### Task 6: E2E test — same-list reorder with persistence

**Files:**
- Modify: `frontend-next/e2e/board.spec.ts`

- [ ] **Step 1: Add the test**

Append to `frontend-next/e2e/board.spec.ts`:

```ts
test("backlog: reorders issues within the same list and persists after reload", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");

  await expect(page.getByTestId("row-DEMO-6")).toBeVisible();
  await expect(page.getByTestId("row-DEMO-7")).toBeVisible();

  const rowsBefore = await page.getByTestId("backlog-list").locator('[data-testid^="row-"]').allTextContents();
  expect(rowsBefore.findIndex((r) => r.includes("DEMO-6"))).toBeLessThan(
    rowsBefore.findIndex((r) => r.includes("DEMO-7"))
  );

  // Dropping DEMO-7's handle onto DEMO-6 inserts DEMO-7 immediately before DEMO-6.
  await dragBetween(page, "drag-handle-DEMO-7", "drag-handle-DEMO-6");

  await expect(async () => {
    const rows = await page.getByTestId("backlog-list").locator('[data-testid^="row-"]').allTextContents();
    expect(rows.findIndex((r) => r.includes("DEMO-7"))).toBeLessThan(rows.findIndex((r) => r.includes("DEMO-6")));
  }).toPass();

  await page.reload();
  const rowsAfter = await page.getByTestId("backlog-list").locator('[data-testid^="row-"]').allTextContents();
  expect(rowsAfter.findIndex((r) => r.includes("DEMO-7"))).toBeLessThan(
    rowsAfter.findIndex((r) => r.includes("DEMO-6"))
  );
});
```

Uses `DEMO-6`/`DEMO-7`, disjoint from the issue keys used by Tasks 3-5's tests, and never leaves the
backlog (pure reorder), so it's independent of test execution order.

- [ ] **Step 2: Run it**

Run: `cd frontend-next && npx playwright test e2e/board.spec.ts`
Expected: PASS (all tests, including this one).

- [ ] **Step 3: Commit**

```bash
git add frontend-next/e2e/board.spec.ts
git commit -m "test(e2e): cover backlog same-list reorder and persistence"
```

---

### Task 7: Full quality gate

**Files:** none (verification only)

- [ ] **Step 1: Backend build, vet, full test suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all pass (no backend changes in this plan, this just confirms no regression).

- [ ] **Step 2: Frontend build + full e2e suite, run at least twice**

Run: `cd frontend-next && npm run build && npx playwright test`, then run `npx playwright test`
again. This repo's history shows drag-simulation e2e tests that pass in isolation can fail under
full-suite concurrency (shared backend/DB across parallel test files) — two full-suite green runs
are the bar, not one isolated-file run.

If a local Docker stack is running and holding ports 8080/3000, stop it first (`docker compose -f
deploy/docker/docker-compose.yml stop` from the repo root), then restart it afterward (`docker
compose -f deploy/docker/docker-compose.yml start`) — do not leave it stopped.

- [ ] **Step 3: Contract freshness check**

Run: `go run ./cmd/gapreport`
Expected: no diff against the committed `docs/contracts/gap-report.md` (no routes changed).

- [ ] **Step 4: Manual smoke check**

With the app running, open a project's Backlog page, drag an issue from the backlog into a sprint,
drag it into a different sprint, select two issues and drag one into a sprint, and reorder two
issues within the same list — confirm the live preview moves other cards out of the way while
dragging (not just a snap-into-place after drop), and that a page reload preserves the final order.

- [ ] **Step 5: Final commit if any gate step required fixes**

Only if Steps 1-3 required code changes:

```bash
git add -A
git commit -m "fix: address quality gate findings for backlog drag-and-drop"
```
