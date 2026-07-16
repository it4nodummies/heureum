"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import { issues, meta, watchers, votes, parseJiraDuration, formatSeconds } from "@/lib/api";
import { AdfRenderer, adfToText, textToAdf } from "./adf";
import { Activity } from "./Activity";
import { Attachments } from "./Attachments";
import { DevelopmentPanel } from "./DevelopmentPanel";
import { LinkedWorkItems } from "./LinkedWorkItems";
import { Subtasks } from "./Subtasks";
import { TimeTracking } from "./TimeTracking";

interface Props {
  issueKey: string;
}

export function IssueView({ issueKey }: Props) {
  const { data: issue, isLoading, isError, error } = useQuery({
    queryKey: ["issue", issueKey],
    queryFn: () => issues.get(issueKey),
  });

  const qc = useQueryClient();

  // Single "Edit" mode covers summary, description, priority and labels —
  // they're saved together via one PUT /rest/api/3/issue/{key}. Assignee
  // stays read-only here (needs a user picker — out of scope for now).
  const [editMode, setEditMode] = useState(false);
  const [draftSummary, setDraftSummary] = useState("");
  const [draftDescription, setDraftDescription] = useState("");
  const [draftPriorityId, setDraftPriorityId] = useState("");
  const [draftLabels, setDraftLabels] = useState("");
  const [draftStoryPoints, setDraftStoryPoints] = useState("");
  const [draftOriginalEstimate, setDraftOriginalEstimate] = useState("");
  const [draftRemainingEstimate, setDraftRemainingEstimate] = useState("");

  const { data: priorities } = useQuery({
    queryKey: ["priorities"],
    queryFn: () => meta.priorities(),
    enabled: editMode,
  });

  const save = useMutation({
    mutationFn: () =>
      issues.update(issueKey, {
        summary: draftSummary,
        description: textToAdf(draftDescription),
        ...(draftPriorityId ? { priority: { id: draftPriorityId } } : {}),
        labels: draftLabels
          .split(",")
          .map((l) => l.trim())
          .filter(Boolean),
        customfield_10016: draftStoryPoints.trim() === "" ? 0 : Number(draftStoryPoints),
        timetracking: {
          originalEstimateSeconds: parseJiraDuration(draftOriginalEstimate),
          remainingEstimateSeconds: parseJiraDuration(draftRemainingEstimate),
        },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["issue", issueKey] });
      setEditMode(false);
    },
  });

  const { data: w } = useQuery({ queryKey: ["watchers", issueKey], queryFn: () => watchers.get(issueKey) });
  const { data: v } = useQuery({ queryKey: ["votes", issueKey], queryFn: () => votes.get(issueKey) });
  const toggleWatch = useMutation({
    mutationFn: () => (w?.isWatching ? watchers.unwatch(issueKey) : watchers.watch(issueKey)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["watchers", issueKey] }),
  });
  const toggleVote = useMutation({
    mutationFn: () => (v?.hasVoted ? votes.unvote(issueKey) : votes.vote(issueKey)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["votes", issueKey] }),
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

  function startEdit() {
    setDraftSummary(f.summary);
    setDraftDescription(adfToText(f.description));
    setDraftPriorityId(f.priority?.id ?? "");
    setDraftLabels(f.labels.join(", "));
    setDraftStoryPoints(f.customfield_10016 != null ? String(f.customfield_10016) : "");
    setDraftOriginalEstimate(
      f.timetracking?.originalEstimateSeconds ? formatSeconds(f.timetracking.originalEstimateSeconds) : ""
    );
    setDraftRemainingEstimate(
      f.timetracking?.remainingEstimateSeconds ? formatSeconds(f.timetracking.remainingEstimateSeconds) : ""
    );
    setEditMode(true);
  }

  const projectKey = f.project?.key ?? issue.key.split("-")[0];
  const projectName = f.project?.name ?? projectKey;

  return (
    <div className="max-w-5xl px-8 py-8">
      <nav data-testid="issue-breadcrumb" className="mb-3 flex items-center gap-1.5 text-xs text-slate-400">
        <Link href="/app/projects" className="text-[#0052cc] hover:underline">
          Projects
        </Link>
        <span>›</span>
        <Link href={`/app/projects/${projectKey}`} className="text-[#0052cc] hover:underline">
          {projectName}
        </Link>
        <span>›</span>
        <span className="text-slate-400">{issue.key}</span>
      </nav>

      <div className="mb-2 flex items-center justify-between">
        <div className="text-xs font-mono text-slate-400">{issue.key}</div>
        {!editMode && (
          <button
            onClick={startEdit}
            className="rounded border border-[#0052cc] px-3 py-1.5 text-xs font-semibold text-[#0052cc] hover:bg-[#0052cc]/5"
          >
            Edit
          </button>
        )}
      </div>

      {editMode ? (
        <input
          autoFocus
          value={draftSummary}
          onChange={(e) => setDraftSummary(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") setEditMode(false);
          }}
          className="mb-6 w-full rounded border border-[#0052cc] px-2 py-1 text-2xl font-bold text-[#1a1f36] tracking-tight"
        />
      ) : (
        <h1 className="mb-6 text-2xl font-bold text-[#1a1f36] tracking-tight">{f.summary}</h1>
      )}

      <div className="grid grid-cols-1 md:grid-cols-[1fr_260px] gap-8">
        <div className="bg-white border border-slate-100 rounded-2xl shadow-sm shadow-slate-100/80 p-6">
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
            Description
          </h2>
          {editMode ? (
            <textarea
              rows={8}
              value={draftDescription}
              onChange={(e) => setDraftDescription(e.target.value)}
              placeholder="Add a description…"
              className="w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
            />
          ) : (
            <AdfRenderer doc={f.description} />
          )}
        </div>

        <aside className="space-y-4 bg-white border border-slate-100 rounded-2xl shadow-sm shadow-slate-100/80 p-5 h-fit">
          <button onClick={() => toggleWatch.mutate()} disabled={toggleWatch.isPending}
            className="w-full rounded border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50 disabled:opacity-60">
            {w?.isWatching ? "Stop watching" : "Watch"} ({w?.watchCount ?? 0})
          </button>
          <button onClick={() => toggleVote.mutate()} disabled={toggleVote.isPending}
            className="mt-2 w-full rounded border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50 disabled:opacity-60">
            {v?.hasVoted ? "Unvote" : "Vote"} ({v?.votes ?? 0})
          </button>
          <Field label="Status" value={f.status?.name} />
          <Field label="Type" value={f.issuetype?.name} />

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Priority</div>
              <select
                value={draftPriorityId}
                onChange={(e) => setDraftPriorityId(e.target.value)}
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              >
                {!draftPriorityId && <option value="">Select…</option>}
                {(priorities ?? []).map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>
          ) : (
            <Field label="Priority" value={f.priority?.name} />
          )}

          <Field label="Assignee" value={f.assignee?.displayName ?? "Unassigned"} />
          <Field label="Reporter" value={f.reporter?.displayName ?? "—"} />

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Labels</div>
              <input
                value={draftLabels}
                onChange={(e) => setDraftLabels(e.target.value)}
                placeholder="comma, separated, labels"
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          ) : (
            <Field label="Labels" value={f.labels.length ? f.labels.join(", ") : "None"} />
          )}

          {editMode ? (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                Story points
              </div>
              <input
                type="number"
                min={0}
                value={draftStoryPoints}
                onChange={(e) => setDraftStoryPoints(e.target.value)}
                placeholder="0"
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          ) : (
            <Field label="Story points" value={f.customfield_10016 != null ? String(f.customfield_10016) : undefined} />
          )}

          {editMode && (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                Original estimate
              </div>
              <input
                value={draftOriginalEstimate}
                onChange={(e) => setDraftOriginalEstimate(e.target.value)}
                placeholder="e.g. 1d 4h"
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          )}

          {editMode && (
            <div>
              <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">
                Remaining estimate
              </div>
              <input
                value={draftRemainingEstimate}
                onChange={(e) => setDraftRemainingEstimate(e.target.value)}
                placeholder="e.g. 4h"
                className="mt-1 w-full rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
              />
            </div>
          )}

          {editMode && (
            <div className="flex gap-2 pt-2">
              <button
                onClick={() => save.mutate()}
                disabled={save.isPending}
                className="flex-1 rounded bg-[#0052cc] px-3 py-2 text-sm font-semibold text-white hover:bg-[#0065ff] disabled:opacity-60"
              >
                {save.isPending ? "Saving…" : "Save"}
              </button>
              <button
                onClick={() => setEditMode(false)}
                className="flex-1 rounded border border-slate-300 px-3 py-2 text-sm hover:bg-slate-50"
              >
                Cancel
              </button>
            </div>
          )}

          {save.isError && (
            <p className="text-xs text-red-600">
              {save.error instanceof Error ? save.error.message : "Failed to save changes."}
            </p>
          )}
        </aside>
      </div>

      <DevelopmentPanel issueKey={issue.key} />

      <Subtasks issueKey={issue.key} projectKey={projectKey} />

      <LinkedWorkItems issueKey={issue.key} />

      <Attachments issueKey={issue.key} />

      <TimeTracking issueKey={issue.key} />

      <Activity issueKey={issue.key} />
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
