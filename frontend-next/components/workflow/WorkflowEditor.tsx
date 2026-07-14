"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { workflow, type WorkflowStatus } from "@/lib/api";

const CATEGORIES = [
  { value: "todo", label: "To Do" },
  { value: "inprogress", label: "In Progress" },
  { value: "done", label: "Done" },
] as const;

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

  const statuses = wf.data?.statuses ?? [];
  const nameByID = (id: string) => statuses.find((s) => s.id === id)?.name ?? id;

  return (
    <div className="space-y-6">
      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Statuses</h3>
        <ul className="space-y-1" data-testid="workflow-statuses">
          {statuses.map((s: WorkflowStatus) => (
            <li key={s.id} className="flex items-center gap-2 text-sm" data-testid={`status-${s.name}`}>
              <span className="inline-block h-3 w-3 rounded" style={{ backgroundColor: s.color }} />
              <span className="text-[#1a1f36]">{s.name}</span>
              <span className="text-xs text-slate-400">({s.category})</span>
              <button
                onClick={() => delStatus.mutate(s.id)}
                className="ml-auto text-xs text-red-600 hover:underline"
                aria-label={`Delete status ${s.name}`}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
        <div className="mt-2 flex gap-2">
          <input
            aria-label="New status name"
            value={newStatus}
            onChange={(e) => setNewStatus(e.target.value)}
            placeholder="Status name"
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          />
          <select aria-label="New status category" value={newCat} onChange={(e) => setNewCat(e.target.value)} className="rounded border border-slate-300 px-2 py-1 text-sm">
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
