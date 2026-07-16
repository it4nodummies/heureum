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

  useEffect(() => {
    if (activeId) return;
    setItems(serverItems);
    // Depend on a stable serialized snapshot, not the object identity (which changes every
    // render), so this only re-syncs when the actual server-derived content changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [JSON.stringify(serverItems), activeId]);

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
    onSuccess: () => {
      setSelected(new Set());
      invalidate();
    },
    onError: invalidate,
  });

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

  const handleDragOver = (event: DragOverEvent) => {
    const { active, over } = event;
    if (!over) return;
    const activeKey = String(active.id);
    const overId = String(over.id);
    const draggedKeys = selected.has(activeKey) && selected.size > 1 ? Array.from(selected) : [activeKey];

    setItems((prev) => {
      const overContainer = overId in prev ? overId : findContainer(prev, overId);
      if (!overContainer) return prev;

      const withoutDragged: Record<string, string[]> = {};
      for (const [cid, keys] of Object.entries(prev)) {
        withoutDragged[cid] = keys.filter((k) => !draggedKeys.includes(k));
      }
      const overItemsAfterRemoval = withoutDragged[overContainer];
      const insertIndex =
        overId === overContainer
          ? overItemsAfterRemoval.length
          : (() => {
              const idx = overItemsAfterRemoval.indexOf(overId);
              return idx >= 0 ? idx : overItemsAfterRemoval.length;
            })();
      withoutDragged[overContainer] = [
        ...overItemsAfterRemoval.slice(0, insertIndex),
        ...draggedKeys,
        ...overItemsAfterRemoval.slice(insertIndex),
      ];
      return withoutDragged;
    });
  };

  const handleDragEnd = (event: DragEndEvent) => {
    const activeKey = String(event.active.id);
    const draggedKeys = selected.has(activeKey) && selected.size > 1 ? Array.from(selected) : [activeKey];
    const sourceContainer = dragStartContainer.current;
    setActiveId(null);
    dragStartContainer.current = null;
    if (!sourceContainer) return;

    const targetContainer = findContainer(items, activeKey);
    if (!targetContainer) return;

    const targetList = items[targetContainer];
    const draggedSet = new Set(draggedKeys);
    const firstIndex = targetList.findIndex((k) => draggedSet.has(k));
    const after = firstIndex > 0 ? targetList[firstIndex - 1] : undefined;
    let lastDraggedIndex = -1;
    for (let i = targetList.length - 1; i >= 0; i--) {
      if (draggedSet.has(targetList[i])) {
        lastDraggedIndex = i;
        break;
      }
    }
    const before = lastDraggedIndex >= 0 && lastDraggedIndex + 1 < targetList.length ? targetList[lastDraggedIndex + 1] : undefined;

    if (sourceContainer === targetContainer && !before && !after) return;

    moveAndRank.mutate({ sourceContainer, targetContainer, draggedKeys, before, after });
  };

  const sensors = useSensors(useSensor(PointerSensor), useSensor(KeyboardSensor));

  const activeIssue = activeId ? issuesByKey[activeId] : undefined;
  const activeCount = activeId && selected.has(activeId) && selected.size > 1 ? selected.size : 1;

  return (
    <div>
      {projectKey && <ProjectHeader projectKey={projectKey} active="backlog" />}
      <div className="mx-auto max-w-3xl p-4">
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
