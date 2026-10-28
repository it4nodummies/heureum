"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  DndContext,
  DragEndEvent,
  DragStartEvent,
  DragOverlay,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import { boards, agileIssues, issues as issuesApi, workflow, type SearchIssue } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";
import { DroppableList } from "@/components/backlog/DroppableList";

const BACKLOG_ID = "backlog";
const EMPTY_SELECTION: Set<string> = new Set();

/**
 * Kanban flat backlog: a single rank-ordered list of the board's to-do issues
 * (no sprints). To-do is derived from the Jira status category key "new"
 * (internal/api/v3/reference.go CategoryFor: the "todo" internal category maps
 * to statusCategory.key "new"). Drag-to-reorder persists via PUT /issue/rank
 * (agileIssues.rank); "Send to board" transitions an issue to the first
 * in-progress workflow status (workflow.get → statuses with category
 * "inprogress", lowest position), which DoTransition accepts as {status_id}.
 */
export function KanbanBacklog({ boardId }: { boardId: number }) {
  const id = boardId;
  const qc = useQueryClient();
  const [activeId, setActiveId] = useState<string | null>(null);
  const [items, setItems] = useState<string[]>([]);
  const [actionError, setActionError] = useState<string | null>(null);

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });
  // For a kanban board there are no sprints, so /backlog returns every (not
  // archived) project issue; we keep only the to-do-category ones here.
  const backlog = useQuery({ queryKey: ["board", id, "backlog"], queryFn: () => boards.backlog(id) });
  const projectKey = board.data?.location?.projectKey;

  const wf = useQuery({
    queryKey: ["project", projectKey, "workflow"],
    enabled: !!projectKey,
    queryFn: () => workflow.get(projectKey as string),
  });

  // First in-progress status (lowest position) — the "Send to board" target.
  const firstInProgress = useMemo(() => {
    const inProgress = (wf.data?.statuses ?? [])
      .filter((s) => s.category === "inprogress")
      .sort((a, b) => a.position - b.position);
    return inProgress[0];
  }, [wf.data]);

  // To-do issues only, in the server's rank order.
  const todoIssues = useMemo(
    () =>
      (backlog.data?.issues ?? []).filter(
        (iss) => iss.fields.status?.statusCategory?.key === "new",
      ),
    [backlog.data],
  );

  const issuesByKey: Record<string, SearchIssue> = {};
  for (const iss of todoIssues) issuesByKey[iss.key] = iss;
  const serverKeys = todoIssues.map((i) => i.key);

  const rankMutation = useMutation({
    mutationFn: (vars: { key: string; before?: string; after?: string }) =>
      agileIssues.rank([vars.key], vars.before, vars.after),
    onSuccess: async () => {
      setActionError(null);
      await qc.invalidateQueries({ queryKey: ["board", id, "backlog"] });
    },
    onError: async (err) => {
      setActionError(err instanceof Error ? err.message : "Failed to reorder issue");
      await qc.invalidateQueries({ queryKey: ["board", id, "backlog"] });
    },
  });

  const sendToBoard = useMutation({
    mutationFn: (key: string) => {
      if (!firstInProgress) throw new Error("No in-progress status configured for this board");
      return issuesApi.transition(key, firstInProgress.id);
    },
    onSuccess: async () => {
      setActionError(null);
      await Promise.all([
        qc.invalidateQueries({ queryKey: ["board", id, "backlog"] }),
        qc.invalidateQueries({ queryKey: ["board", id, "issues"] }),
      ]);
    },
    onError: (err) => {
      setActionError(err instanceof Error ? err.message : "Failed to send issue to board");
    },
  });

  // Mirror the server order into local state while nothing is in flight — same
  // guard pattern the scrum backlog uses (don't resync mid-drag or mid-commit).
  useEffect(() => {
    if (activeId || rankMutation.isPending) return;
    setItems(serverKeys);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [JSON.stringify(serverKeys), activeId, rankMutation.isPending]);

  const sensors = useSensors(useSensor(PointerSensor), useSensor(KeyboardSensor));

  const handleDragStart = (event: DragStartEvent) => setActiveId(String(event.active.id));

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    const activeKey = String(active.id);
    setActiveId(null);
    if (!over) return;
    const overId = String(over.id);
    if (overId === activeKey) return;

    const withoutDragged = items.filter((k) => k !== activeKey);
    const insertIndex =
      overId === BACKLOG_ID
        ? withoutDragged.length
        : (() => {
            const idx = withoutDragged.indexOf(overId);
            return idx >= 0 ? idx : withoutDragged.length;
          })();
    const next = [...withoutDragged.slice(0, insertIndex), activeKey, ...withoutDragged.slice(insertIndex)];
    setItems(next);

    const pos = next.indexOf(activeKey);
    const after = pos > 0 ? next[pos - 1] : undefined;
    const before = pos + 1 < next.length ? next[pos + 1] : undefined;
    if (!before && !after) return;
    rankMutation.mutate({ key: activeKey, before, after });
  };

  const activeIssue = activeId ? issuesByKey[activeId] : undefined;

  return (
    <div>
      {projectKey && <ProjectHeader projectKey={projectKey} active="backlog" />}
      <div className="mx-auto max-w-3xl p-4">
        {actionError && (
          <div
            role="alert"
            data-testid="backlog-drag-error"
            className="mb-2 flex items-center gap-2 rounded border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700"
          >
            <span>{actionError}</span>
            <button
              onClick={() => setActionError(null)}
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
          onDragEnd={handleDragEnd}
        >
          <DroppableList
            containerId={BACKLOG_ID}
            items={items}
            issuesByKey={issuesByKey}
            selected={EMPTY_SELECTION}
            onToggleSelect={() => {}}
            hideSelect
            emptyLabel="No to-do issues in the backlog"
            testId="backlog-list"
            header={
              <div className="mb-1 text-sm font-semibold text-slate-500">Backlog ({items.length})</div>
            }
            renderAction={(issue) => (
              <button
                type="button"
                onClick={() => sendToBoard.mutate(issue.key)}
                disabled={sendToBoard.isPending || !firstInProgress}
                title={!firstInProgress ? "No in-progress status configured" : undefined}
                data-testid={`send-to-board-${issue.key}`}
                className="rounded border border-slate-300 px-2 py-0.5 text-xs text-slate-700 hover:bg-slate-50 disabled:opacity-60"
              >
                Send to board
              </button>
            )}
          />

          <DragOverlay>
            {activeIssue && (
              <div className="rounded border border-slate-300 bg-white px-2 py-1 text-sm shadow-md">
                <span className="font-mono text-xs text-slate-500">{activeIssue.key}</span>{" "}
                <span className="text-[#1a1f36]">{activeIssue.fields.summary}</span>
              </div>
            )}
          </DragOverlay>
        </DndContext>
      </div>
    </div>
  );
}
