"use client";

// "Epic" (on a Story/Task/Bug) or "Parent" (on a Subtask) field of the issue
// detail. View: the epic lozenge (colour + name, link) or the parent key +
// summary; edit: the ParentPicker (same project, one level up), saved
// immediately through PUT /issue/{key} { fields: { parent } }; "×" clears it
// (parent: null). The backend's errors.parent reason is shown on 400.

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import { epics, fieldError, issues, type ParentRef } from "@/lib/api";
import { EpicLozenge } from "./EpicLozenge";
import { ParentPicker, type ParentOption } from "./ParentPicker";

interface Props {
  issueKey: string;
  projectKey: string;
  // Hierarchy level of the issue being shown (0 → its parent is an epic,
  // -1 → its parent is a story/task/bug).
  childLevel: number;
  parent: ParentRef | null;
  // Whether the viewer may change the parent (mirrors the Edit button).
  canEdit?: boolean;
}

export function ParentField({ issueKey, projectKey, childLevel, parent, canEdit = true }: Props) {
  const qc = useQueryClient();
  const label = childLevel === 0 ? "Epic" : "Parent";
  const [editing, setEditing] = useState(false);

  // The epic's colour is not part of fields.parent; one small read of the
  // agile epic shape paints the lozenge (and is shared with the colour picker
  // via the ["epic", key] query key).
  const { data: epic } = useQuery({
    queryKey: ["epic", parent?.key],
    queryFn: () => epics.get(parent!.key),
    enabled: childLevel === 0 && !!parent,
  });

  const setParent = useMutation({
    mutationFn: (next: ParentOption | null) => issues.setParent(issueKey, next?.key ?? null),
    onSuccess: (_void, next) => {
      qc.invalidateQueries({ queryKey: ["issue", issueKey] });
      if (parent) qc.invalidateQueries({ queryKey: ["issue", parent.key, "subtasks"] });
      if (next) qc.invalidateQueries({ queryKey: ["issue", next.key, "subtasks"] });
      qc.invalidateQueries({ queryKey: ["projectIssues", projectKey] });
      setEditing(false);
    },
  });

  const value: ParentOption | null = parent ? { key: parent.key, summary: parent.fields?.summary ?? parent.key } : null;
  const errorText = setParent.isError
    ? fieldError(setParent.error, "parent") ??
      (setParent.error instanceof Error ? setParent.error.message : "Could not change the parent.")
    : null;

  return (
    <div data-testid="parent-field" data-parent-key={parent?.key ?? ""}>
      <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{label}</div>
      {editing ? (
        <ParentPicker
          projectKey={projectKey}
          childLevel={childLevel}
          value={value}
          excludeKey={issueKey}
          disabled={setParent.isPending}
          autoOpen
          clearable={false}
          label={`Select ${label.toLowerCase()}`}
          onChange={(next) => {
            if ((next?.key ?? null) === (value?.key ?? null)) {
              setEditing(false);
              return;
            }
            setParent.mutate(next);
          }}
          onCancel={() => setEditing(false)}
        />
      ) : (
        <div className="mt-0.5 flex items-center gap-2">
          <div className="min-w-0 flex-1 text-sm text-[#1a1f36]" data-testid="parent-value">
            {parent ? (
              childLevel === 0 ? (
                <EpicLozenge
                  name={parent.fields?.summary ?? parent.key}
                  colorKey={epic?.color.key}
                  issueKey={parent.key}
                  href={`/app/browse/${parent.key}`}
                />
              ) : (
                <Link href={`/app/browse/${parent.key}`} className="flex min-w-0 items-center gap-1.5 hover:underline">
                  <span className="shrink-0 font-mono text-xs text-[#0052cc]">{parent.key}</span>
                  <span className="truncate">{parent.fields?.summary}</span>
                </Link>
              )
            ) : (
              <span className="text-slate-400">None</span>
            )}
          </div>
          {canEdit && (
            <button
              type="button"
              onClick={() => setEditing(true)}
              disabled={setParent.isPending}
              data-testid="parent-edit"
              className="shrink-0 text-xs text-[#0052cc] hover:underline disabled:opacity-60"
            >
              {parent ? "Change" : "Set"}
            </button>
          )}
          {canEdit && parent && (
            <button
              type="button"
              aria-label={`Remove ${label.toLowerCase()}`}
              title={`Remove ${label.toLowerCase()}`}
              onClick={() => setParent.mutate(null)}
              disabled={setParent.isPending}
              data-testid="parent-clear"
              className="shrink-0 rounded px-1 text-sm leading-none text-slate-400 hover:text-red-600 disabled:opacity-60"
            >
              ×
            </button>
          )}
        </div>
      )}
      {errorText && (
        <p className="mt-1 text-xs text-red-600" data-testid="parent-error">
          {errorText}
        </p>
      )}
    </div>
  );
}
