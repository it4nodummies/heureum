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
import { boards, sprints, agileIssues, type SearchIssue, type AgileSprint } from "@/lib/api";
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
  const [newGoal, setNewGoal] = useState("");
  const [newStart, setNewStart] = useState("");
  const [newEnd, setNewEnd] = useState("");
  const [editing, setEditing] = useState<number | null>(null);
  const [editName, setEditName] = useState("");
  const [editGoal, setEditGoal] = useState("");
  const [editStart, setEditStart] = useState("");
  const [editEnd, setEditEnd] = useState("");
  const [completing, setCompleting] = useState<number | null>(null);
  const [moveMode, setMoveMode] = useState<"backlog" | "sprint">("backlog");
  const [moveTarget, setMoveTarget] = useState<number | "">("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [activeId, setActiveId] = useState<string | null>(null);
  const [items, setItems] = useState<Record<string, string[]>>({});
  const [dragError, setDragError] = useState<string | null>(null);
  const dragStartContainer = useRef<string | null>(null);
  // See the comment above `handleDragOver`'s use of this ref: guards against a real dnd-kit
  // multi-container "thrash" bug where moving an item across containers shifts their measured
  // rects, which can flip `closestCenter`'s pick back and forth with no actual pointer movement,
  // crashing the page with "Maximum update depth exceeded".
  const recentlyMovedToNewContainer = useRef(false);

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

  const invalidate = async () => {
    await Promise.all([
      qc.invalidateQueries({ queryKey: ["board", id, "backlog"] }),
      qc.invalidateQueries({ queryKey: ["board", id, "sprints"] }),
      ...sprintValues.map((sp) => qc.invalidateQueries({ queryKey: ["sprint", sp.id, "issues"] })),
    ]);
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
    mutationFn: () =>
      sprints.create(newSprint, id, newGoal || undefined, newStart || undefined, newEnd || undefined),
    onSuccess: () => {
      setNewSprint("");
      setNewGoal("");
      setNewStart("");
      setNewEnd("");
      invalidate();
    },
  });
  const updateSprint = useMutation({
    mutationFn: (vars: {
      sprintId: number;
      fields: { name?: string; goal?: string; startDate?: string; endDate?: string };
    }) => sprints.update(vars.sprintId, vars.fields),
    onSuccess: () => {
      setEditing(null);
      invalidate();
    },
  });
  const completeSprint = useMutation({
    mutationFn: (vars: {
      sprintId: number;
      opts: { moveToSprintId?: number | null; moveOpenToBacklog?: boolean };
    }) => sprints.complete(vars.sprintId, vars.opts),
    onSuccess: () => {
      setCompleting(null);
      invalidate();
    },
  });
  const setState = useMutation({
    mutationFn: ({ sprintId, state }: { sprintId: number; state: "active" | "closed" }) =>
      sprints.setState(sprintId, state),
    onSuccess: invalidate,
  });

  const openEdit = (sp: AgileSprint) => {
    setCompleting(null);
    setEditing(sp.id);
    setEditName(sp.name);
    setEditGoal(sp.goal ?? "");
    setEditStart(sp.startDate ? sp.startDate.slice(0, 10) : "");
    setEditEnd(sp.endDate ? sp.endDate.slice(0, 10) : "");
  };
  const openComplete = (sprintId: number) => {
    setEditing(null);
    setCompleting(sprintId);
    setMoveMode("backlog");
    setMoveTarget("");
  };
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
    onSuccess: async (_data, vars) => {
      // Only clear the selection if this drag actually moved it — dragging a lone,
      // unselected issue must not wipe a selection the user is still building for a
      // separate, later bulk action.
      if (vars.wasGroupDrag) setSelected(new Set());
      setDragError(null);
      // Awaited (TanStack Query v5 awaits onSuccess/onError before flipping `isPending` to
      // false) so the sync effect's `moveAndRank.isPending` guard stays true until the
      // refetch actually lands — otherwise isPending flips false as soon as the mutation's
      // own network call resolves, before the invalidated queries' background refetch
      // completes, and the sync effect fires once with stale serverItems: the same class of
      // revert flicker as the activeId-clears-too-early bug, just at a later boundary.
      await invalidate();
    },
    onError: async (err) => {
      setDragError(err instanceof Error ? err.message : "Failed to move issue(s)");
      await invalidate();
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
    // Moving the dragged block out of one container and into another changes both containers'
    // heights, which shifts their measured center points. If the pointer happens to sit near the
    // boundary between two containers, closestCenter can then flip which one is "closest" purely
    // because of the layout shift just caused by our own setItems — with no actual pointer
    // movement in between — producing a self-sustaining flip-flop that blows past React's update
    // depth limit and crashes the page (reproduced: two adjacent sprint containers oscillating
    // forever during a single drag). This is dnd-kit's well-documented multi-container "thrash"
    // gotcha; their official sortable/multiple-containers example guards it the same way: skip
    // processing for one render cycle right after we've just moved something across containers,
    // giving the DOM a frame to settle before re-evaluating collisions.
    if (recentlyMovedToNewContainer.current) return;

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
    recentlyMovedToNewContainer.current = true;
  };

  // Clears the thrash guard one frame after `items` actually re-renders with the new layout, so
  // the very next real collision re-evaluation (whether pointer-driven or triggered by the layout
  // shift itself) is allowed through, but the immediate feedback cycle from this move is not.
  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      recentlyMovedToNewContainer.current = false;
    });
    return () => cancelAnimationFrame(frame);
  }, [items]);

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
          {sprintValues.map((sp, idx) => (
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
                <div className="mb-1">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-semibold text-[#1a1f36]">
                      {sp.name} <span className="text-xs font-normal text-slate-400">({sp.state})</span>
                    </span>
                    <span className="flex gap-2">
                      <button
                        onClick={() => openEdit(sp)}
                        className="rounded border border-slate-300 px-2 py-0.5 text-xs"
                      >
                        Edit sprint
                      </button>
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
                          onClick={() => openComplete(sp.id)}
                          className="rounded border border-slate-300 px-2 py-0.5 text-xs"
                        >
                          Complete sprint
                        </button>
                      )}
                    </span>
                  </div>
                  {(sp.goal || sp.startDate || sp.endDate) && (
                    <div data-testid="sprint-goal" className="mt-0.5 text-xs text-slate-500">
                      {sp.goal && <span>{sp.goal}</span>}
                      {(sp.startDate || sp.endDate) && (
                        <span className="ml-2 text-slate-400">
                          {sp.startDate ? sp.startDate.slice(0, 10) : "…"} → {sp.endDate ? sp.endDate.slice(0, 10) : "…"}
                        </span>
                      )}
                    </div>
                  )}
                  {editing === sp.id && (
                    <div
                      data-testid="edit-sprint-form"
                      className="mt-2 space-y-2 rounded border border-slate-200 bg-slate-50 p-2"
                    >
                      <input
                        aria-label="Edit sprint name"
                        value={editName}
                        onChange={(e) => setEditName(e.target.value)}
                        placeholder="Sprint name"
                        className="w-full rounded border border-slate-300 px-2 py-1 text-sm"
                      />
                      <input
                        aria-label="Edit sprint goal"
                        value={editGoal}
                        onChange={(e) => setEditGoal(e.target.value)}
                        placeholder="Sprint goal"
                        className="w-full rounded border border-slate-300 px-2 py-1 text-sm"
                      />
                      <div className="flex gap-2">
                        <input
                          type="date"
                          aria-label="Edit start date"
                          value={editStart}
                          onChange={(e) => setEditStart(e.target.value)}
                          className="flex-1 rounded border border-slate-300 px-2 py-1 text-sm"
                        />
                        <input
                          type="date"
                          aria-label="Edit end date"
                          value={editEnd}
                          onChange={(e) => setEditEnd(e.target.value)}
                          className="flex-1 rounded border border-slate-300 px-2 py-1 text-sm"
                        />
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={() =>
                            updateSprint.mutate({
                              sprintId: sp.id,
                              fields: {
                                name: editName,
                                goal: editGoal,
                                startDate: editStart || undefined,
                                endDate: editEnd || undefined,
                              },
                            })
                          }
                          disabled={updateSprint.isPending}
                          className="rounded bg-[#0052cc] px-3 py-1 text-xs text-white disabled:opacity-60"
                        >
                          Save
                        </button>
                        <button
                          onClick={() => setEditing(null)}
                          className="rounded border border-slate-300 px-3 py-1 text-xs"
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  )}
                  {completing === sp.id &&
                    (() => {
                      const spIssues = sprintIssuesQueries[idx]?.data?.issues ?? [];
                      const incomplete = spIssues.filter(
                        (i) => i.fields.status?.statusCategory?.key !== "done",
                      );
                      const otherSprints = sprintValues.filter(
                        (o) => o.id !== sp.id && o.state !== "closed",
                      );
                      return (
                        <div
                          role="dialog"
                          aria-label="Complete sprint"
                          data-testid="complete-sprint-dialog"
                          className="mt-2 space-y-2 rounded border border-slate-200 bg-white p-3 shadow"
                        >
                          <div className="text-sm font-semibold text-[#1a1f36]">Complete {sp.name}</div>
                          <div data-testid="incomplete-count" className="text-xs text-slate-500">
                            {incomplete.length} incomplete issue{incomplete.length === 1 ? "" : "s"} will be moved.
                          </div>
                          <label className="flex items-center gap-2 text-sm">
                            <input
                              type="radio"
                              name={`move-${sp.id}`}
                              checked={moveMode === "backlog"}
                              onChange={() => setMoveMode("backlog")}
                            />
                            Move to Backlog
                          </label>
                          <label className="flex items-center gap-2 text-sm">
                            <input
                              type="radio"
                              name={`move-${sp.id}`}
                              checked={moveMode === "sprint"}
                              onChange={() => setMoveMode("sprint")}
                              disabled={otherSprints.length === 0}
                            />
                            Move to sprint
                            <select
                              aria-label="Target sprint"
                              value={moveTarget}
                              onChange={(e) => setMoveTarget(e.target.value ? Number(e.target.value) : "")}
                              disabled={moveMode !== "sprint" || otherSprints.length === 0}
                              className="rounded border border-slate-300 px-2 py-0.5 text-sm"
                            >
                              <option value="">Select sprint…</option>
                              {otherSprints.map((o) => (
                                <option key={o.id} value={o.id}>
                                  {o.name}
                                </option>
                              ))}
                            </select>
                          </label>
                          <div className="flex gap-2">
                            <button
                              onClick={() =>
                                completeSprint.mutate({
                                  sprintId: sp.id,
                                  opts:
                                    moveMode === "sprint" && moveTarget
                                      ? { moveToSprintId: Number(moveTarget) }
                                      : { moveOpenToBacklog: true },
                                })
                              }
                              disabled={completeSprint.isPending || (moveMode === "sprint" && !moveTarget)}
                              className="rounded bg-[#0052cc] px-3 py-1 text-xs text-white disabled:opacity-60"
                            >
                              Complete sprint
                            </button>
                            <button
                              onClick={() => setCompleting(null)}
                              className="rounded border border-slate-300 px-3 py-1 text-xs"
                            >
                              Cancel
                            </button>
                          </div>
                        </div>
                      );
                    })()}
                </div>
              }
            />
          ))}

          <div className="my-3 space-y-2 rounded border border-slate-200 p-2">
            <div className="flex gap-2">
              <input
                aria-label="New sprint name"
                value={newSprint}
                onChange={(e) => setNewSprint(e.target.value)}
                placeholder="Sprint name"
                className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
              />
              <button
                onClick={() => newSprint && createSprint.mutate()}
                className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
                disabled={createSprint.isPending}
              >
                Create sprint
              </button>
            </div>
            <input
              aria-label="Sprint goal"
              value={newGoal}
              onChange={(e) => setNewGoal(e.target.value)}
              placeholder="Sprint goal (optional)"
              className="w-full rounded border border-slate-300 px-3 py-1.5 text-sm"
            />
            <div className="flex gap-2">
              <input
                type="date"
                aria-label="Start date"
                value={newStart}
                onChange={(e) => setNewStart(e.target.value)}
                className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
              />
              <input
                type="date"
                aria-label="End date"
                value={newEnd}
                onChange={(e) => setNewEnd(e.target.value)}
                className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
              />
            </div>
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
