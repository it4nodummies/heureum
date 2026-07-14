"use client";

import { useQuery } from "@tanstack/react-query";
import { issueGit } from "@/lib/api";

export function DevelopmentPanel({ issueKey }: { issueKey: string }) {
  const info = useQuery({ queryKey: ["issue-git", issueKey], queryFn: () => issueGit.info(issueKey) });
  const d = info.data;
  const empty = d && d.commits.length === 0 && d.branches.length === 0 && d.pull_requests.length === 0;
  if (info.isLoading) return null;
  return (
    <section className="mt-4 rounded border border-slate-200 bg-white p-3" data-testid="development-panel">
      <h3 className="mb-2 text-sm font-semibold text-slate-700">Development</h3>
      {empty && <p className="text-sm text-slate-400">No linked commits, branches or pull requests</p>}
      {d && d.commits.length > 0 && (
        <div className="mb-2">
          <div className="text-xs font-semibold text-slate-500">Commits</div>
          <ul className="text-sm">
            {d.commits.map((c) => (
              <li key={c.id} className="text-[#1a1f36]">
                <span className="font-mono text-xs text-slate-500">{c.commit_sha.slice(0, 8)}</span> {c.message}
              </li>
            ))}
          </ul>
        </div>
      )}
      {d && d.pull_requests.length > 0 && (
        <div className="mb-2">
          <div className="text-xs font-semibold text-slate-500">Pull requests</div>
          <ul className="text-sm">
            {d.pull_requests.map((p) => (
              <li key={p.id}>
                <a href={p.url} className="text-[#0052cc] hover:underline">
                  #{p.pr_number} {p.title}
                </a>{" "}
                <span className="text-xs text-slate-400">({p.state})</span>
              </li>
            ))}
          </ul>
        </div>
      )}
      {d && d.branches.length > 0 && (
        <div>
          <div className="text-xs font-semibold text-slate-500">Branches</div>
          <ul className="text-sm text-[#1a1f36]">
            {d.branches.map((b) => (
              <li key={b.id}>{b.branch_name}</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
