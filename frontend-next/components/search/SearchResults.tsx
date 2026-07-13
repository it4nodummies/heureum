"use client";

import { useState } from "react";
import type { SearchIssue } from "@/lib/api";

const ALL_COLUMNS = [
  { key: "key", label: "Key" },
  { key: "summary", label: "Summary" },
  { key: "status", label: "Status" },
  { key: "priority", label: "Priority" },
  { key: "assignee", label: "Assignee" },
  { key: "updated", label: "Updated" },
] as const;

type ColKey = (typeof ALL_COLUMNS)[number]["key"];

export function SearchResults({ issues }: { issues: SearchIssue[] }) {
  const [cols, setCols] = useState<Set<ColKey>>(
    new Set(ALL_COLUMNS.map((c) => c.key)),
  );

  const toggle = (k: ColKey) =>
    setCols((prev) => {
      const next = new Set(prev);
      if (next.has(k)) next.delete(k);
      else next.add(k);
      return next;
    });

  const cell = (iss: SearchIssue, k: ColKey) => {
    switch (k) {
      case "key":
        return (
          <a href={`/jira/browse/${iss.key}`} className="text-[#0052cc] hover:underline">
            {iss.key}
          </a>
        );
      case "summary":
        return iss.fields.summary ?? "";
      case "status":
        return iss.fields.status?.name ?? "";
      case "priority":
        return iss.fields.priority?.name ?? "";
      case "assignee":
        return iss.fields.assignee?.displayName ?? "Unassigned";
      case "updated":
        return iss.fields.updated?.slice(0, 10) ?? "";
    }
  };

  const visible = ALL_COLUMNS.filter((c) => cols.has(c.key));

  return (
    <div>
      <div className="mb-3 flex flex-wrap gap-3 text-sm text-slate-500" aria-label="Columns">
        {ALL_COLUMNS.map((c) => (
          <label key={c.key} className="flex items-center gap-1">
            <input type="checkbox" checked={cols.has(c.key)} onChange={() => toggle(c.key)} />
            {c.label}
          </label>
        ))}
      </div>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b border-slate-200 text-left text-slate-500">
            {visible.map((c) => (
              <th key={c.key} className="py-2 pr-4 font-semibold">
                {c.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {issues.map((iss) => (
            <tr key={iss.id} className="border-b border-slate-100 hover:bg-slate-50">
              {visible.map((c) => (
                <td key={c.key} className="py-2 pr-4 text-[#1a1f36]">
                  {cell(iss, c.key)}
                </td>
              ))}
            </tr>
          ))}
          {issues.length === 0 && (
            <tr>
              <td className="py-4 text-slate-400" colSpan={Math.max(visible.length, 1)}>
                No results
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
