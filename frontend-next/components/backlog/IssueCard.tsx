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
