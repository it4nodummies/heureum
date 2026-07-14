"use client";

import { useQuery } from "@tanstack/react-query";
import { reports } from "@/lib/api";

export function ProjectSummary({ projectKey }: { projectKey: string }) {
  const summary = useQuery({ queryKey: ["summary", projectKey], queryFn: () => reports.summary(projectKey) });
  const s = summary.data;
  return (
    <div className="space-y-4" data-testid="project-summary">
      <div className="grid grid-cols-3 gap-3">
        <Stat label="Created (7d)" value={s?.created_last_7_days ?? 0} />
        <Stat label="Updated (7d)" value={s?.updated_last_7_days ?? 0} />
        <Stat label="Completed (7d)" value={s?.completed_last_7_days ?? 0} />
      </div>
      <div>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Issues by status</h3>
        <ul className="text-sm" data-testid="summary-status-counts">
          {Object.entries(s?.issue_count_by_status ?? {}).map(([name, n]) => (
            <li key={name} className="flex justify-between border-b border-slate-100 py-1">
              <span className="text-[#1a1f36]">{name}</span>
              <span className="text-slate-500">{n}</span>
            </li>
          ))}
        </ul>
      </div>
      {s?.active_sprint && (
        <p className="text-sm text-slate-600">
          Active sprint: <strong>{s.active_sprint.name}</strong>
        </p>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded border border-slate-200 bg-white p-3 text-center">
      <div className="text-2xl font-semibold text-[#1a1f36]">{value}</div>
      <div className="text-xs text-slate-500">{label}</div>
    </div>
  );
}
