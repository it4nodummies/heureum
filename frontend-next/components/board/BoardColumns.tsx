"use client";

import { DndContext, DragEndEvent, useDraggable, useDroppable } from "@dnd-kit/core";
import type { SearchIssue } from "@/lib/api";

interface Column {
  id: string;
  name: string;
}

// Card issue trascinabile.
function IssueCard({ issue }: { issue: SearchIssue }) {
  const { attributes, listeners, setNodeRef, transform } = useDraggable({ id: issue.key });
  const style = transform ? { transform: `translate(${transform.x}px, ${transform.y}px)` } : undefined;
  return (
    <div
      ref={setNodeRef}
      style={style}
      {...listeners}
      {...attributes}
      className="mb-2 cursor-grab rounded border border-slate-200 bg-white p-2 text-sm shadow-sm"
      data-testid={`card-${issue.key}`}
    >
      <div className="font-mono text-xs text-slate-500">{issue.key}</div>
      <div className="text-[#1a1f36]">{issue.fields.summary}</div>
    </div>
  );
}

// Colonna droppabile.
function ColumnBox({ col, issues }: { col: Column; issues: SearchIssue[] }) {
  const { setNodeRef, isOver } = useDroppable({ id: col.id });
  return (
    <div
      ref={setNodeRef}
      className={`w-64 shrink-0 rounded bg-slate-100 p-2 ${isOver ? "ring-2 ring-[#0052cc]" : ""}`}
      data-testid={`column-${col.name}`}
    >
      <h3 className="mb-2 text-xs font-semibold uppercase text-slate-500">
        {col.name} <span className="text-slate-400">({issues.length})</span>
      </h3>
      {issues.map((iss) => (
        <IssueCard key={iss.key} issue={iss} />
      ))}
    </div>
  );
}

export function BoardColumns({
  columns,
  issuesByStatus,
  onMove,
}: {
  columns: Column[];
  issuesByStatus: Record<string, SearchIssue[]>;
  onMove: (issueKey: string, toStatusId: string) => void;
}) {
  const handleDragEnd = (e: DragEndEvent) => {
    const issueKey = String(e.active.id);
    const toStatusId = e.over ? String(e.over.id) : null;
    if (toStatusId) onMove(issueKey, toStatusId);
  };
  return (
    <DndContext onDragEnd={handleDragEnd}>
      <div className="flex gap-3 overflow-x-auto p-2">
        {columns.map((col) => (
          <ColumnBox key={col.id} col={col} issues={issuesByStatus[col.id] ?? []} />
        ))}
      </div>
    </DndContext>
  );
}
