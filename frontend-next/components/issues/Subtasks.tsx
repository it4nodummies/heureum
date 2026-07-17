"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import { issues, Issue } from "@/lib/api";

interface Props {
  issueKey: string;
  projectKey: string;
}

export function Subtasks({ issueKey, projectKey }: Props) {
  const qc = useQueryClient();
  const subtasksKey = ["issue", issueKey, "subtasks"];
  const { data, isLoading } = useQuery({
    queryKey: subtasksKey,
    queryFn: () => issues.subtasks(issueKey),
  });
  const children = data?.values ?? [];
  const total = children.length;
  // "done" è la statusCategory.key reale usata dal backend v3 (v3.CategoryFor,
  // internal/api/v3/reference.go) — le altre due sono "new" e "indeterminate".
  const done = children.filter((c) => c.fields.status?.statusCategory?.key === "done").length;

  const [summary, setSummary] = useState("");
  const create = useMutation({
    mutationFn: () => issues.create({ projectKey, summary, issueTypeName: "Subtask", parentKey: issueKey }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: subtasksKey });
      setSummary("");
    },
  });

  function submit() {
    if (summary.trim() && !create.isPending) create.mutate();
  }

  return (
    <section className="mt-8" data-testid="subtasks-section">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-slate-500">Subtasks</h2>
        <span className="text-xs text-slate-500" data-testid="subtasks-progress">
          {done} of {total} done
        </span>
      </div>

      {!isLoading && total === 0 && <p className="mb-3 text-sm text-slate-400">No subtasks yet.</p>}

      {total > 0 && (
        <ul className="mb-3 space-y-2">
          {children.map((child) => (
            <SubtaskRow key={child.id} subtask={child} parentSubtasksKey={subtasksKey} />
          ))}
        </ul>
      )}

      <div className="flex items-center gap-2">
        <input
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit();
          }}
          placeholder="Add a subtask…"
          aria-label="Add a subtask"
          className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
        />
        <button
          onClick={submit}
          disabled={!summary.trim() || create.isPending}
          className="rounded bg-[#0052cc] px-3 py-1.5 text-sm font-semibold text-white hover:bg-[#0065ff] disabled:opacity-60"
        >
          {create.isPending ? "Adding…" : "Add"}
        </button>
      </div>
      {create.isError && (
        <p className="mt-2 text-xs text-red-600">
          {create.error instanceof Error ? create.error.message : "Failed to create subtask."}
        </p>
      )}
    </section>
  );
}

// Badge colorato al posto dell'iconUrl v3 (nessun asset servito in dev/e2e):
// una lettera colorata per tipo, coerente con lo stile minimale del resto
// della pagina (Field renderizza solo testo, niente immagini remote).
function typeBadgeClasses(name: string): string {
  switch (name.toLowerCase()) {
    case "bug":
      return "bg-red-100 text-red-700";
    case "story":
      return "bg-green-100 text-green-700";
    case "epic":
      return "bg-purple-100 text-purple-700";
    case "subtask":
      return "bg-[#0052cc]/10 text-[#0052cc]";
    default:
      return "bg-slate-100 text-slate-600";
  }
}

function SubtaskRow({ subtask, parentSubtasksKey }: { subtask: Issue; parentSubtasksKey: string[] }) {
  const qc = useQueryClient();
  const f = subtask.fields;
  const typeName = f.issuetype?.name ?? "Subtask";

  const { data: transitionsData } = useQuery({
    queryKey: ["transitions", subtask.key],
    queryFn: () => issues.availableTransitions(subtask.key),
  });

  const transition = useMutation({
    mutationFn: (statusId: string) => issues.transition(subtask.key, statusId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: parentSubtasksKey });
      qc.invalidateQueries({ queryKey: ["transitions", subtask.key] });
    },
  });

  return (
    <li
      className="flex items-center gap-3 rounded border border-slate-200 px-3 py-2"
      data-testid={`subtask-row-${subtask.key}`}
    >
      <span
        className={`flex h-5 w-5 shrink-0 items-center justify-center rounded text-[10px] font-bold ${typeBadgeClasses(typeName)}`}
        title={typeName}
      >
        {typeName.charAt(0).toUpperCase()}
      </span>
      <Link href={`/app/browse/${subtask.key}`} className="shrink-0 font-mono text-xs text-[#0052cc] hover:underline">
        {subtask.key}
      </Link>
      <span className="flex-1 truncate text-sm text-[#1a1f36]">{f.summary}</span>
      <select
        aria-label={`Status of ${subtask.key}`}
        value={f.status?.id ?? ""}
        onChange={(e) => transition.mutate(e.target.value)}
        disabled={transition.isPending}
        className="rounded border border-slate-300 px-2 py-1 text-xs focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
      >
        {f.status && <option value={f.status.id}>{f.status.name}</option>}
        {(transitionsData?.transitions ?? [])
          .filter((t) => t.to.id !== f.status?.id)
          .map((t) => (
            <option key={t.id} value={t.to.id}>
              {t.to.name}
            </option>
          ))}
      </select>
      <span className="shrink-0 text-xs text-slate-500">{f.assignee?.displayName ?? "Unassigned"}</span>
    </li>
  );
}
