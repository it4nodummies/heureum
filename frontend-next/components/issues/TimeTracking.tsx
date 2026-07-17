"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { issues, worklogs as worklogsApi, parseJiraDuration, formatSeconds, textToADF } from "@/lib/api";
import { AdfRenderer } from "./adf";

interface Props {
  issueKey: string;
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

// Time tracking block: progress bar + summary text (derived from the issue's
// fields.timetracking, shared react-query cache with IssueView so this
// doesn't trigger a second GET /issue/{key}), the "Log work" dialog, and the
// worklog list. Kept as its own component (rather than folded into
// IssueView.tsx) because the worklog list/dialog state is self-contained and
// would otherwise bloat an already-large component.
export function TimeTracking({ issueKey }: Props) {
  const qc = useQueryClient();
  const issueQueryKey = ["issue", issueKey];
  const worklogQueryKey = ["issue", issueKey, "worklogs"];

  const { data: issue } = useQuery({ queryKey: issueQueryKey, queryFn: () => issues.get(issueKey) });
  const { data } = useQuery({ queryKey: worklogQueryKey, queryFn: () => worklogsApi.list(issueKey) });

  const [dialogOpen, setDialogOpen] = useState(false);
  const [timeSpent, setTimeSpent] = useState("");
  const [description, setDescription] = useState("");

  const logWork = useMutation({
    mutationFn: () =>
      worklogsApi.add(issueKey, {
        timeSpentSeconds: parseJiraDuration(timeSpent),
        ...(description.trim() ? { comment: textToADF(description) } : {}),
      }),
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: issueQueryKey }),
        qc.invalidateQueries({ queryKey: worklogQueryKey }),
      ]);
      setDialogOpen(false);
      setTimeSpent("");
      setDescription("");
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => worklogsApi.delete(issueKey, id),
    onSuccess: () =>
      Promise.all([
        qc.invalidateQueries({ queryKey: issueQueryKey }),
        qc.invalidateQueries({ queryKey: worklogQueryKey }),
      ]),
  });

  const tt = issue?.fields.timetracking;
  const timeSpentSeconds = tt?.timeSpentSeconds ?? 0;
  const originalEstimateSeconds = tt?.originalEstimateSeconds ?? 0;
  const remainingEstimateSeconds = tt?.remainingEstimateSeconds;
  const hasEstimate = originalEstimateSeconds > 0;
  const progressPct = hasEstimate
    ? Math.min(100, Math.round((timeSpentSeconds / originalEstimateSeconds) * 100))
    : 0;
  const overEstimate = hasEstimate && timeSpentSeconds > originalEstimateSeconds;

  const list = data?.worklogs ?? [];
  const canSubmit = parseJiraDuration(timeSpent) > 0 && !logWork.isPending;

  return (
    <section className="mt-8" data-testid="time-tracking-section">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-slate-500">Time tracking</h2>
        <button
          onClick={() => setDialogOpen(true)}
          className="rounded border border-[#0052cc] px-3 py-1.5 text-xs font-semibold text-[#0052cc] hover:bg-[#0052cc]/5"
        >
          Log work
        </button>
      </div>

      {hasEstimate ? (
        <div className="mb-4">
          <div className="h-2 w-full overflow-hidden rounded-full bg-slate-100" data-testid="time-tracking-bar">
            <div
              className={`h-full rounded-full ${overEstimate ? "bg-red-500" : "bg-[#0052cc]"}`}
              style={{ width: `${progressPct}%` }}
            />
          </div>
          <p data-testid="time-tracking-text" className="mt-1 text-xs text-slate-500">
            {formatSeconds(timeSpentSeconds)} logged / {formatSeconds(originalEstimateSeconds)} estimated
            {remainingEstimateSeconds != null ? ` · ${formatSeconds(remainingEstimateSeconds)} remaining` : ""}
          </p>
        </div>
      ) : (
        <p data-testid="time-tracking-text" className="mb-4 text-sm text-slate-400">
          {timeSpentSeconds > 0 ? `${formatSeconds(timeSpentSeconds)} logged · no estimate set` : "No time logged"}
        </p>
      )}

      {list.length === 0 && <p className="text-sm text-slate-400">No work logged yet.</p>}

      {list.length > 0 && (
        <ul className="space-y-2">
          {list.map((wl) => (
          <li
            key={wl.id}
            data-testid={`worklog-row-${wl.id}`}
            className="flex items-start justify-between gap-3 rounded-lg border border-slate-200 p-3 text-sm"
          >
            <div className="min-w-0">
              <div className="font-semibold text-[#1a1f36]">
                {wl.author?.displayName ?? "Unknown"}
                <span className="ml-2 font-normal text-slate-400">{formatDate(wl.started || wl.created)}</span>
              </div>
              <div className="mt-0.5 text-slate-600">{wl.timeSpent}</div>
              {wl.comment && (
                <div className="mt-1 text-slate-500">
                  <AdfRenderer doc={wl.comment} />
                </div>
              )}
            </div>
            <button
              onClick={() => remove.mutate(wl.id)}
              disabled={remove.isPending && remove.variables === wl.id}
              aria-label={`Delete worklog ${wl.id}`}
              className="shrink-0 text-xs text-slate-400 hover:text-red-600 disabled:opacity-60"
            >
              {remove.isPending && remove.variables === wl.id ? "Deleting…" : "Delete"}
            </button>
          </li>
          ))}
        </ul>
      )}

      {dialogOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
          <div className="w-[420px] rounded-lg bg-white p-6 shadow-xl">
            <h2 className="mb-4 text-lg font-semibold text-[#1a1f36]">Log work</h2>

            <label
              htmlFor="worklog-time-spent"
              className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
            >
              Time spent
            </label>
            <input
              id="worklog-time-spent"
              autoFocus
              value={timeSpent}
              onChange={(e) => setTimeSpent(e.target.value)}
              placeholder="e.g. 2h 30m"
              className="mb-3 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
            />

            <label
              htmlFor="worklog-description"
              className="mb-1 block text-xs font-semibold uppercase tracking-wider text-slate-500"
            >
              Description (optional)
            </label>
            <textarea
              id="worklog-description"
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What did you work on?"
              className="mb-4 w-full rounded border border-slate-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
            />

            {logWork.isError && (
              <p className="mb-3 text-sm text-red-600">
                {logWork.error instanceof Error ? logWork.error.message : "Failed to log work."}
              </p>
            )}

            <div className="flex justify-end gap-3">
              <button
                onClick={() => setDialogOpen(false)}
                className="rounded px-4 py-2 text-sm text-slate-600 hover:text-slate-800"
              >
                Cancel
              </button>
              <button
                onClick={() => logWork.mutate()}
                disabled={!canSubmit}
                className="rounded bg-[#0052cc] px-4 py-2 text-sm font-semibold text-white hover:bg-[#0065ff] disabled:opacity-60"
              >
                {logWork.isPending ? "Logging…" : "Log work"}
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
