"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { DndContext, DragEndEvent, closestCenter, KeyboardSensor, PointerSensor, useSensor, useSensors } from "@dnd-kit/core";
import { SortableContext, arrayMove, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { workflow, type Workflow, type WorkflowStatus, type WorkflowTransition } from "@/lib/api";
import { useT, type MessageKey } from "@/lib/i18n";

// Status categories (chrome, not data): option label and the short tag shown
// next to each status. An unknown category is shown raw.
const CATEGORIES = [
  { value: "todo", label: "boards.workflow.category.todo", tag: "boards.workflow.categoryTag.todo" },
  { value: "inprogress", label: "boards.workflow.category.inprogress", tag: "boards.workflow.categoryTag.inprogress" },
  { value: "done", label: "boards.workflow.category.done", tag: "boards.workflow.categoryTag.done" },
] as const satisfies readonly { value: string; label: MessageKey; tag: MessageKey }[];

function StatusRow({ status, onDelete }: { status: WorkflowStatus; onDelete: (id: string) => void }) {
  const t = useT();
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: status.id });
  const category = CATEGORIES.find((c) => c.value === status.category);
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
        aria-label={t("boards.workflow.dragToReorder", { name: status.name })}
        data-testid={`drag-handle-${status.name}`}
      >
        ⠿
      </button>
      <span className="inline-block h-3 w-3 rounded" style={{ backgroundColor: status.color }} />
      <span className="text-[#1a1f36]">{status.name}</span>
      <span className="text-xs text-slate-400">({category ? t(category.tag) : status.category})</span>
      <button
        onClick={() => onDelete(status.id)}
        className="ml-auto text-xs text-red-600 hover:underline"
        aria-label={t("boards.workflow.deleteStatus", { name: status.name })}
      >
        {t("boards.actions.remove")}
      </button>
    </li>
  );
}

export function WorkflowEditor({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const t = useT();
  const [newStatus, setNewStatus] = useState("");
  const [newCat, setNewCat] = useState("todo");
  const [fromStatusId, setFromStatusId] = useState("");
  const [toStatusId, setToStatusId] = useState("");
  const [transitionName, setTransitionName] = useState("");
  const [requireAssignee, setRequireAssignee] = useState(false);
  const [setResolutionFlag, setSetResolutionFlag] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [editRequireAssignee, setEditRequireAssignee] = useState(false);
  const [editSetResolution, setEditSetResolution] = useState(false);

  const wf = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => workflow.get(projectKey) });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["workflow", projectKey] });

  const addStatus = useMutation({
    mutationFn: () => workflow.addStatus(projectKey, newStatus, newCat, "#6B7280"),
    onSuccess: () => {
      setNewStatus("");
      invalidate();
      delStatus.reset();
    },
  });
  const delStatus = useMutation({
    mutationFn: (id: string) => workflow.deleteStatus(projectKey, id),
    onSuccess: () => {
      invalidate();
      addStatus.reset();
    },
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
  const addTransition = useMutation({
    mutationFn: () =>
      workflow.addTransition(projectKey, {
        from_status_id: fromStatusId,
        to_status_id: toStatusId,
        name: transitionName,
        require_assignee: requireAssignee,
        set_resolution: setResolutionFlag,
      }),
    onSuccess: () => {
      setFromStatusId("");
      setToStatusId("");
      setTransitionName("");
      setRequireAssignee(false);
      setSetResolutionFlag(false);
      invalidate();
      delTransition.reset();
    },
  });
  const delTransition = useMutation({
    mutationFn: (id: string) => workflow.deleteTransition(projectKey, id),
    onSuccess: () => {
      invalidate();
      addTransition.reset();
    },
  });
  const editTransition = useMutation({
    mutationFn: (id: string) =>
      workflow.updateTransition(projectKey, id, {
        name: editName,
        require_assignee: editRequireAssignee,
        set_resolution: editSetResolution,
      }),
    onSuccess: () => {
      setEditingId(null);
      invalidate();
    },
  });

  const startEdit = (tr: WorkflowTransition) => {
    setEditingId(tr.id);
    setEditName(tr.name);
    setEditRequireAssignee(tr.require_assignee);
    setEditSetResolution(tr.set_resolution);
    editTransition.reset();
  };

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
        <h3 className="mb-2 text-sm font-semibold text-slate-700">{t("boards.workflow.statuses")}</h3>
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={statuses.map((s) => s.id)} strategy={verticalListSortingStrategy}>
            <ul className="space-y-1" data-testid="workflow-statuses">
              {statuses.map((s) => (
                <StatusRow key={s.id} status={s} onDelete={(id) => delStatus.mutate(id)} />
              ))}
            </ul>
          </SortableContext>
        </DndContext>
        {delStatus.isError && (
          <p className="mt-1 text-sm text-red-600">
            {delStatus.error instanceof Error ? delStatus.error.message : t("boards.workflow.deleteStatusFailed")}
          </p>
        )}
        <div className="mt-2 flex gap-2">
          <input
            aria-label={t("boards.workflow.newStatusName")}
            value={newStatus}
            onChange={(e) => setNewStatus(e.target.value)}
            placeholder={t("boards.workflow.statusNamePlaceholder")}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          />
          <select
            aria-label={t("boards.workflow.categoryLabel")}
            value={newCat}
            onChange={(e) => setNewCat(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            {CATEGORIES.map((c) => (
              <option key={c.value} value={c.value}>{t(c.label)}</option>
            ))}
          </select>
          <button
            onClick={() => newStatus && addStatus.mutate()}
            className="rounded bg-[#0052cc] px-3 py-1 text-sm text-white disabled:opacity-60"
            disabled={addStatus.isPending}
          >
            {t("boards.workflow.addStatus")}
          </button>
        </div>
        {addStatus.isError && (
          <p className="mt-1 text-sm text-red-600">
            {addStatus.error instanceof Error ? addStatus.error.message : t("boards.workflow.addStatusFailed")}
          </p>
        )}
        <p className="mt-1 text-xs text-slate-400">
          {t("boards.workflow.categoryHelp")}
        </p>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">{t("boards.workflow.transitions")}</h3>
        <ul className="space-y-1 text-sm" data-testid="workflow-transitions">
          {(wf.data?.transitions ?? []).map((tr) => {
            const label = tr.name || `${nameByID(tr.from_status_id)} → ${nameByID(tr.to_status_id)}`;
            return (
              <li key={tr.id} className="space-y-2" data-testid={`transition-${label}`}>
                <div className="flex items-center gap-2">
                  <span className="text-[#1a1f36]">{label}</span>
                  <span className="text-xs text-slate-400">
                    {nameByID(tr.from_status_id)} → {nameByID(tr.to_status_id)}
                    {tr.require_assignee ? t("boards.workflow.requiresAssigneeTag") : ""}
                    {tr.set_resolution ? t("boards.workflow.setsResolutionTag") : ""}
                  </span>
                  <button
                    onClick={() => startEdit(tr)}
                    className="ml-auto text-xs text-[#0052cc] hover:underline"
                    aria-label={t("boards.workflow.editTransition", { label })}
                    data-testid="transition-edit"
                  >
                    {t("boards.actions.edit")}
                  </button>
                  <button
                    onClick={() => delTransition.mutate(tr.id)}
                    className="text-xs text-red-600 hover:underline"
                    aria-label={t("boards.workflow.deleteTransition", { label })}
                  >
                    {t("boards.actions.remove")}
                  </button>
                </div>
                {editingId === tr.id && (
                  <div className="flex flex-wrap items-center gap-2 rounded border border-slate-200 bg-slate-50 p-2">
                    <input
                      aria-label={t("boards.workflow.transitionNameEdit")}
                      value={editName}
                      onChange={(e) => setEditName(e.target.value)}
                      placeholder={t("boards.workflow.transitionNameOptional")}
                      className="rounded border border-slate-300 px-2 py-1 text-sm"
                    />
                    <label className="flex items-center gap-1 text-xs text-slate-600">
                      <input
                        type="checkbox"
                        aria-label={t("boards.workflow.requireAssigneeEdit")}
                        checked={editRequireAssignee}
                        onChange={(e) => setEditRequireAssignee(e.target.checked)}
                      />
                      {t("boards.workflow.requireAssignee")}
                    </label>
                    <label className="flex items-center gap-1 text-xs text-slate-600">
                      <input
                        type="checkbox"
                        aria-label={t("boards.workflow.setResolutionEdit")}
                        checked={editSetResolution}
                        onChange={(e) => setEditSetResolution(e.target.checked)}
                      />
                      {t("boards.workflow.setResolution")}
                    </label>
                    <button
                      onClick={() => editTransition.mutate(tr.id)}
                      disabled={editTransition.isPending}
                      className="rounded bg-[#0052cc] px-3 py-1 text-sm text-white disabled:opacity-60"
                    >
                      {t("boards.workflow.saveTransition")}
                    </button>
                    <button
                      onClick={() => setEditingId(null)}
                      className="rounded border border-slate-300 px-3 py-1 text-sm text-slate-600"
                    >
                      {t("boards.actions.cancel")}
                    </button>
                    {editTransition.isError && (
                      <p className="w-full text-sm text-red-600">
                        {editTransition.error instanceof Error ? editTransition.error.message : t("boards.workflow.updateTransitionFailed")}
                      </p>
                    )}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
        {delTransition.isError && (
          <p className="mt-1 text-sm text-red-600">
            {delTransition.error instanceof Error ? delTransition.error.message : t("boards.workflow.deleteTransitionFailed")}
          </p>
        )}
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <select
            aria-label={t("boards.workflow.fromStatus")}
            value={fromStatusId}
            onChange={(e) => setFromStatusId(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            <option value="">{t("boards.workflow.fromStatus")}</option>
            {statuses.map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
          <select
            aria-label={t("boards.workflow.toStatus")}
            value={toStatusId}
            onChange={(e) => setToStatusId(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            <option value="">{t("boards.workflow.toStatus")}</option>
            {statuses.map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
          <input
            aria-label={t("boards.workflow.transitionName")}
            value={transitionName}
            onChange={(e) => setTransitionName(e.target.value)}
            placeholder={t("boards.workflow.transitionNameOptional")}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          />
          <label className="flex items-center gap-1 text-xs text-slate-600">
            <input
              type="checkbox"
              aria-label={t("boards.workflow.requireAssignee")}
              checked={requireAssignee}
              onChange={(e) => setRequireAssignee(e.target.checked)}
            />
            {t("boards.workflow.requireAssignee")}
          </label>
          <label className="flex items-center gap-1 text-xs text-slate-600">
            <input
              type="checkbox"
              aria-label={t("boards.workflow.setResolution")}
              checked={setResolutionFlag}
              onChange={(e) => setSetResolutionFlag(e.target.checked)}
            />
            {t("boards.workflow.setResolution")}
          </label>
          <button
            onClick={() => fromStatusId && toStatusId && addTransition.mutate()}
            disabled={!fromStatusId || !toStatusId || addTransition.isPending}
            className="rounded bg-[#0052cc] px-3 py-1 text-sm text-white disabled:opacity-60"
          >
            {t("boards.workflow.addTransition")}
          </button>
        </div>
        {addTransition.isError && (
          <p className="mt-1 text-sm text-red-600">
            {addTransition.error instanceof Error ? addTransition.error.message : t("boards.workflow.addTransitionFailed")}
          </p>
        )}
      </section>
    </div>
  );
}
