"use client";

import type { ReactNode } from "react";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { SearchIssue } from "@/lib/api";
import type { EpicRef } from "@/lib/epicHierarchy";
import { EpicLozenge } from "@/components/issues/EpicLozenge";

export function IssueCard({
  issue,
  selected,
  onToggleSelect,
  // Optional: hide the bulk-select checkbox (the kanban flat backlog has no
  // bulk move-to-sprint action, so the checkbox would be purposeless).
  hideSelect = false,
  // Optional: a trailing per-row action (e.g. the kanban "Send to board"
  // button), rendered right-aligned after the summary.
  action,
  // Optional: the epic the issue belongs to (resolved by the page, see
  // lib/epicHierarchy.ts), shown as a lozenge after the summary.
  epic,
}: {
  issue: SearchIssue;
  selected: boolean;
  onToggleSelect: (key: string) => void;
  hideSelect?: boolean;
  action?: ReactNode;
  epic?: EpicRef | null;
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
      {!hideSelect && (
        <input
          type="checkbox"
          aria-label={`Select ${issue.key}`}
          checked={selected}
          onChange={() => onToggleSelect(issue.key)}
          className="shrink-0"
        />
      )}
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
      {epic && (
        <EpicLozenge
          name={epic.name}
          colorKey={epic.colorKey}
          issueKey={epic.key}
          href={`/app/browse/${epic.key}`}
          testId={`epic-lozenge-${issue.key}`}
          className="shrink-0"
        />
      )}
      {action && <span className="ml-auto shrink-0">{action}</span>}
    </div>
  );
}
