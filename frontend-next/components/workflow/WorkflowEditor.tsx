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
          {(wf.data?.transitions ?? []).map((t) => (
            <li key={t.id} className="flex items-center gap-2">
              <span className="text-[#1a1f36]">{t.name || `${nameByID(t.from_status_id)} → ${nameByID(t.to_status_id)}`}</span>
              <span className="text-xs text-slate-400">
                {nameByID(t.from_status_id)} → {nameByID(t.to_status_id)}
                {t.require_assignee ? " · requires assignee" : ""}
                {t.set_resolution ? " · sets resolution" : ""}
              </span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
