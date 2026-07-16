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
    <div className="mb-3 rounded border border-slate-200 p-2">
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
