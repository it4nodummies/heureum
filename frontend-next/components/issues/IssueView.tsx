"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { issues } from "@/lib/api";
import { AdfRenderer } from "./adf";

interface Props {
  issueKey: string;
}

export function IssueView({ issueKey }: Props) {
  const { data: issue, isLoading, isError, error } = useQuery({
    queryKey: ["issue", issueKey],
    queryFn: () => issues.get(issueKey),
  });

  const qc = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const save = useMutation({
    mutationFn: (summary: string) => issues.update(issueKey, { summary }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["issue", issueKey] });
      setEditing(false);
    },
  });

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3">
        <div className="w-7 h-7 rounded-full border-2 border-[#0052cc] border-t-transparent animate-spin" />
        <span className="text-sm text-slate-400">Loading issue…</span>
      </div>
    );
  }

  if (isError || !issue) {
    return (
      <div className="px-8 py-8">
        <div className="p-4 bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl">
          {error instanceof Error ? error.message : "Issue not found."}
        </div>
      </div>
    );
  }

  const f = issue.fields;

  return (
    <div className="max-w-5xl px-8 py-8">
      <div className="mb-2 text-xs font-mono text-slate-400">{issue.key}</div>
      {editing ? (
        <input
          autoFocus
          defaultValue={f.summary}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={() => save.mutate(draft || f.summary)}
          onKeyDown={(e) => {
            if (e.key === "Enter") save.mutate(draft || f.summary);
            if (e.key === "Escape") setEditing(false);
          }}
          className="mb-6 w-full rounded border border-[#0052cc] px-2 py-1 text-2xl font-bold text-[#1a1f36] tracking-tight"
        />
      ) : (
        <h1
          className="mb-6 cursor-text text-2xl font-bold text-[#1a1f36] tracking-tight"
          onClick={() => {
            setDraft(f.summary);
            setEditing(true);
          }}
        >
          {f.summary}
        </h1>
      )}

      <div className="grid grid-cols-1 md:grid-cols-[1fr_260px] gap-8">
        <div className="bg-white border border-slate-100 rounded-2xl shadow-sm shadow-slate-100/80 p-6">
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
            Description
          </h2>
          <AdfRenderer doc={f.description} />
        </div>

        <aside className="space-y-4 bg-white border border-slate-100 rounded-2xl shadow-sm shadow-slate-100/80 p-5 h-fit">
          <Field label="Status" value={f.status?.name} />
          <Field label="Type" value={f.issuetype?.name} />
          <Field label="Priority" value={f.priority?.name} />
          <Field label="Assignee" value={f.assignee?.displayName ?? "Unassigned"} />
          <Field label="Reporter" value={f.reporter?.displayName ?? "—"} />
          <Field label="Labels" value={f.labels.length ? f.labels.join(", ") : "None"} />
        </aside>
      </div>
    </div>
  );
}

function Field({ label, value }: { label: string; value?: string }) {
  return (
    <div>
      <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">{label}</div>
      <div className="text-sm text-[#1a1f36] mt-0.5">{value ?? "—"}</div>
    </div>
  );
}
