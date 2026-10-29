"use client";

// Level-aware "children" section of the issue detail (Round 28):
//   - on an EPIC (level 1): "Child issues" — progress bar + "X of Y done",
//     inline create of a Story (default) / Task / Bug chosen among the
//     project's level-0 types;
//   - on a Story/Task/Bug (level 0): "Subtasks" — inline create of a Subtask;
//   - on a Subtask (level -1): hidden (nothing can live below it).
// Children come from GET /issue/{key}/subtasks, which returns the DIRECT
// children of any level. The pre-R28 testids (subtasks-section,
// subtasks-progress, subtask-row-*) are kept for the story case.

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import Link from "next/link";
import { fieldError, hierarchyLevelOf, issues, projects, type Issue, type IssueTypeRef } from "@/lib/api";

interface Props {
  issueKey: string;
  projectKey: string;
  // Hierarchy level of the issue whose children are listed (1/0/-1).
  parentLevel: number;
}

export function ChildIssues(props: Props) {
  // A subtask has no children; the inner component keeps its hooks
  // unconditional by never being mounted for it.
  if (props.parentLevel <= -1) return null;
  return <ChildIssuesInner {...props} />;
}

// Backwards-compatible name (pre-R28 callers).
export const Subtasks = ChildIssues;

function ChildIssuesInner({ issueKey, projectKey, parentLevel }: Props) {
  const qc = useQueryClient();
  const epicMode = parentLevel === 1;
  const childLevel = parentLevel - 1;
  const childrenKey = ["issue", issueKey, "subtasks"];
  const { data, isLoading } = useQuery({
    queryKey: childrenKey,
    queryFn: () => issues.children(issueKey),
  });
  const children = data?.values ?? [];
  const total = children.length;
  // "done" è la statusCategory.key reale usata dal backend v3 (v3.CategoryFor,
  // internal/api/v3/reference.go) — le altre due sono "new" e "indeterminate".
  const done = children.filter((c) => c.fields.status?.statusCategory?.key === "done").length;
  const pct = total === 0 ? 0 : Math.round((done / total) * 100);

  // The project's own types pick the child type: level-0 types under an epic
  // (Story by default), the level -1 type under a story.
  const { data: project } = useQuery({
    queryKey: ["project", projectKey],
    queryFn: () => projects.get(projectKey),
  });
  const childTypes: IssueTypeRef[] = (project?.issueTypes ?? []).filter(
    (t) => hierarchyLevelOf(t) === childLevel,
  );
  const defaultChildType = epicMode
    ? (childTypes.find((t) => t.name === "Story") ?? childTypes[0])?.name ?? "Story"
    : childTypes[0]?.name ?? "Subtask";

  const [typeName, setTypeName] = useState("");
  useEffect(() => {
    if (!typeName || (childTypes.length > 0 && !childTypes.some((t) => t.name === typeName))) {
      setTypeName(defaultChildType);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [defaultChildType, childTypes.length]);
  const effectiveTypeName = typeName || defaultChildType;

  const [summary, setSummary] = useState("");
  const create = useMutation({
    mutationFn: () =>
      issues.create({ projectKey, summary, issueTypeName: effectiveTypeName, parentKey: issueKey }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: childrenKey });
      qc.invalidateQueries({ queryKey: ["projectIssues", projectKey] });
      setSummary("");
    },
  });

  function submit() {
    if (summary.trim() && !create.isPending) create.mutate();
  }

  const heading = epicMode ? "Child issues" : "Subtasks";
  const noun = epicMode ? "child issue" : "subtask";

  return (
    <section className="mt-8" data-testid={epicMode ? "child-issues" : "subtasks-section"}>
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-slate-500">{heading}</h2>
        <span
          className="text-xs text-slate-500"
          data-testid={epicMode ? "child-issues-progress" : "subtasks-progress"}
        >
          {done} of {total} done
        </span>
      </div>

      {epicMode && (
        <div
          role="progressbar"
          aria-label="Child issues progress"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={pct}
          data-testid="child-issues-progress-bar"
          data-done={done}
          data-total={total}
          className="mb-3 h-2 w-full overflow-hidden rounded-full bg-slate-100"
        >
          <div className="h-full rounded-full bg-[#36b37e] transition-all" style={{ width: `${pct}%` }} />
        </div>
      )}

      {!isLoading && total === 0 && <p className="mb-3 text-sm text-slate-400">No {noun}s yet.</p>}

      {total > 0 && (
        <ul className="mb-3 space-y-2">
          {children.map((child) => (
            <ChildRow key={child.id} child={child} parentChildrenKey={childrenKey} />
          ))}
        </ul>
      )}

      <div className="flex items-center gap-2">
        {epicMode && childTypes.length > 1 && (
          <select
            aria-label="Child issue type"
            data-testid="create-child-type"
            value={effectiveTypeName}
            onChange={(e) => setTypeName(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
          >
            {childTypes.map((t) => (
              <option key={t.id} value={t.name}>
                {t.name}
              </option>
            ))}
          </select>
        )}
        <input
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit();
          }}
          placeholder={`Add a ${noun}…`}
          aria-label={`Add a ${noun}`}
          data-testid="create-child"
          className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
        />
        <button
          onClick={submit}
          disabled={!summary.trim() || create.isPending}
          data-testid="create-child-submit"
          className="rounded bg-[#0052cc] px-3 py-1.5 text-sm font-semibold text-white hover:bg-[#0065ff] disabled:opacity-60"
        >
          {create.isPending ? "Adding…" : "Add"}
        </button>
      </div>
      {create.isError && (
        <p className="mt-2 text-xs text-red-600">
          {fieldError(create.error, "parent") ??
            (create.error instanceof Error ? create.error.message : `Failed to create ${noun}.`)}
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

function ChildRow({ child, parentChildrenKey }: { child: Issue; parentChildrenKey: string[] }) {
  const qc = useQueryClient();
  const f = child.fields;
  const typeName = f.issuetype?.name ?? "Subtask";

  const { data: transitionsData } = useQuery({
    queryKey: ["transitions", child.key],
    queryFn: () => issues.availableTransitions(child.key),
  });

  const transition = useMutation({
    mutationFn: (statusId: string) => issues.transition(child.key, statusId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: parentChildrenKey });
      qc.invalidateQueries({ queryKey: ["transitions", child.key] });
    },
  });

  return (
    <li
      className="flex items-center gap-3 rounded border border-slate-200 px-3 py-2"
      data-testid={`subtask-row-${child.key}`}
    >
      <span
        className={`flex h-5 w-5 shrink-0 items-center justify-center rounded text-[10px] font-bold ${typeBadgeClasses(typeName)}`}
        title={typeName}
      >
        {typeName.charAt(0).toUpperCase()}
      </span>
      <Link href={`/app/browse/${child.key}`} className="shrink-0 font-mono text-xs text-[#0052cc] hover:underline">
        {child.key}
      </Link>
      <span className="flex-1 truncate text-sm text-[#1a1f36]">{f.summary}</span>
      <select
        aria-label={`Status of ${child.key}`}
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
