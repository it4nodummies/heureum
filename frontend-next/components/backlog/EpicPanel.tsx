"use client";

// Backlog epic panel (Round 28): a collapsible column listing the board's
// epics with a client-side progress (done/total of the loaded children),
// an "Issues without epic" / "All issues" filter, drop targets so an issue
// dragged from the backlog/sprint lists lands in an epic (the parent
// DndContext handles the `epic:{KEY}` / `epic:none` over-ids), and an inline
// "Create epic" form. It must be rendered INSIDE the backlog's DndContext.

import { useState, type FormEvent, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { closestCenter, pointerWithin, useDroppable, type CollisionDetection } from "@dnd-kit/core";
import { boards, hierarchyLevelOf, issues as issuesApi, projects, type AgileEpic, type SearchIssue } from "@/lib/api";
import { epicColorHex } from "@/lib/epicColors";
import { epicProgress, type EpicFilter } from "@/lib/epicHierarchy";

export const EPIC_DROP_PREFIX = "epic:";
export const EPIC_DROP_NONE = `${EPIC_DROP_PREFIX}none`;

// Parses a droppable id produced by the panel: the epic key, `null` for the
// "Issues without epic" row, `undefined` when the id isn't a panel target.
export function epicDropTarget(overId: string): string | null | undefined {
  if (!overId.startsWith(EPIC_DROP_PREFIX)) return undefined;
  const key = overId.slice(EPIC_DROP_PREFIX.length);
  return key === "none" ? null : key;
}

// Collision detection for a backlog DndContext that also hosts the panel. The
// lists keep `closestCenter` (measured on the dragged ROW, which is as wide as
// the list — so its centre never reaches the narrow panel on the left); the
// panel rows are hit only when the pointer is actually inside one, and are
// excluded from the closestCenter pass so they can't steal a list drop.
export const epicAwareCollision: CollisionDetection = (args) => {
  const isPanel = (id: string | number) => String(id).startsWith(EPIC_DROP_PREFIX);
  const inPanel = pointerWithin(args).filter((c) => isPanel(c.id));
  if (inPanel.length > 0) return inPanel;
  return closestCenter({
    ...args,
    droppableContainers: args.droppableContainers.filter((c) => !isPanel(c.id)),
  });
};

// A filter row; with `dropId` it is also a drop target for issues dragged from
// the lists ("All issues" has none — it is not an epic to assign).
function DropRow({
  dropId,
  testId,
  active,
  onClick,
  children,
}: {
  dropId?: string;
  testId: string;
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  const { setNodeRef, isOver } = useDroppable({ id: dropId ?? testId, disabled: !dropId });
  return (
    <button
      ref={setNodeRef}
      type="button"
      onClick={onClick}
      aria-pressed={active}
      data-active={active ? "true" : "false"}
      data-testid={testId}
      className={`block w-full rounded px-2 py-1.5 text-left text-sm ${
        active ? "bg-[#e9f2ff] text-[#0052cc]" : "text-[#1a1f36] hover:bg-slate-100"
      } ${isOver ? "ring-2 ring-[#0052cc]" : ""}`}
    >
      {children}
    </button>
  );
}

function EpicRow({
  epic,
  issues,
  active,
  onClick,
}: {
  epic: AgileEpic;
  issues: SearchIssue[];
  active: boolean;
  onClick: () => void;
}) {
  const { done, total } = epicProgress(epic.key, issues);
  const hex = epicColorHex(epic.color?.key);
  const pct = total === 0 ? 0 : Math.round((done / total) * 100);
  return (
    <DropRow dropId={`${EPIC_DROP_PREFIX}${epic.key}`} testId={`epic-panel-item-${epic.key}`} active={active} onClick={onClick}>
      <div className="flex items-center gap-2">
        <span aria-hidden className="h-2.5 w-2.5 shrink-0 rounded-sm" style={{ backgroundColor: hex }} />
        <span className={`min-w-0 flex-1 truncate ${epic.done ? "text-slate-400 line-through" : ""}`} title={`${epic.key} · ${epic.name}`}>
          {epic.name}
        </span>
        <span data-testid={`epic-panel-progress-${epic.key}`} className="shrink-0 font-mono text-xs text-slate-500">
          {done}/{total}
        </span>
      </div>
      <div className="mt-1 h-1 w-full overflow-hidden rounded bg-slate-200" aria-hidden>
        <div className="h-full rounded" style={{ width: `${pct}%`, backgroundColor: hex }} />
      </div>
    </DropRow>
  );
}

export function EpicPanel({
  boardId,
  projectKey,
  issues,
  active,
  onChange,
}: {
  boardId: number;
  projectKey?: string;
  // Every issue the backlog page has loaded (progress + membership).
  issues: SearchIssue[];
  active: EpicFilter;
  onChange: (filter: EpicFilter) => void;
}) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(true);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [createError, setCreateError] = useState<string | null>(null);

  const epics = useQuery({ queryKey: ["board", boardId, "epics"], queryFn: () => boards.epics(boardId) });
  const project = useQuery({
    queryKey: ["project", projectKey],
    enabled: !!projectKey,
    queryFn: () => projects.get(projectKey as string),
  });
  // The project's level-1 type (business projects have none → no create button).
  const epicType = (project.data?.issueTypes ?? []).find((t) => hierarchyLevelOf(t) === 1);

  const createEpic = useMutation({
    mutationFn: (name: string) =>
      issuesApi.create({ projectKey: projectKey as string, summary: name, issueTypeName: epicType?.name }),
    onSuccess: async () => {
      setNewName("");
      setCreating(false);
      setCreateError(null);
      // Epics list + every backlog/sprint list of this board (the new epic is a
      // backlog row too).
      await Promise.all([
        qc.invalidateQueries({ queryKey: ["board", boardId] }),
        qc.invalidateQueries({ queryKey: ["sprint"] }),
      ]);
    },
    onError: (err) => setCreateError(err instanceof Error ? err.message : "Failed to create epic"),
  });

  const submitCreate = (e: FormEvent) => {
    e.preventDefault();
    const name = newName.trim();
    if (!name || !epicType || !projectKey) return;
    createEpic.mutate(name);
  };

  const values = epics.data?.values ?? [];

  return (
    <aside className={`shrink-0 ${open ? "w-64" : "w-auto"}`} aria-label="Epics">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        data-testid="epic-panel-toggle"
        className="mb-2 rounded border border-slate-300 px-2 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
      >
        {open ? "Hide epics" : "Epics"}
      </button>
      {open && (
        <div data-testid="epic-panel" className="rounded border border-slate-200 p-2">
          <div className="mb-1 flex items-center justify-between">
            <span className="text-sm font-semibold text-slate-500">Epics</span>
            {epicType && projectKey && !creating && (
              <button
                type="button"
                onClick={() => setCreating(true)}
                data-testid="epic-panel-create"
                className="rounded border border-slate-300 px-2 py-0.5 text-xs text-slate-700 hover:bg-slate-50"
              >
                Create epic
              </button>
            )}
          </div>
          {creating && (
            <form onSubmit={submitCreate} className="mb-2 space-y-1">
              <input
                autoFocus
                aria-label="New epic name"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="Epic name"
                data-testid="epic-panel-create-input"
                className="w-full rounded border border-slate-300 px-2 py-1 text-sm"
              />
              <div className="flex gap-1">
                <button
                  type="submit"
                  disabled={createEpic.isPending || !newName.trim()}
                  data-testid="epic-panel-create-submit"
                  className="rounded bg-[#0052cc] px-2 py-0.5 text-xs text-white disabled:opacity-60"
                >
                  Create
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setCreating(false);
                    setNewName("");
                    setCreateError(null);
                  }}
                  className="rounded border border-slate-300 px-2 py-0.5 text-xs"
                >
                  Cancel
                </button>
              </div>
              {createError && (
                <p role="alert" data-testid="epic-panel-create-error" className="text-xs text-red-700">
                  {createError}
                </p>
              )}
            </form>
          )}
          <div className="space-y-0.5">
            <DropRow testId="epic-panel-item-all" active={active === "all"} onClick={() => onChange("all")}>
              All issues
            </DropRow>
            <DropRow dropId={EPIC_DROP_NONE} testId="epic-panel-item-none" active={active === "none"} onClick={() => onChange("none")}>
              Issues without epic
            </DropRow>
            {values.map((epic) => (
              <EpicRow
                key={epic.key}
                epic={epic}
                issues={issues}
                active={active === epic.key}
                onClick={() => onChange(active === epic.key ? "all" : epic.key)}
              />
            ))}
            {epics.isSuccess && values.length === 0 && (
              <p className="px-2 py-1 text-xs text-slate-400">No epics yet</p>
            )}
          </div>
        </div>
      )}
    </aside>
  );
}
