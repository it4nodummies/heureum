"use client";

import { use } from "react";
import { useQuery } from "@tanstack/react-query";
import { dashboards, type AssignedIssue, type ActivityItem, type DashboardWidget } from "@/lib/api";

function Gadget({ widget }: { widget: DashboardWidget }) {
  if (widget.widget_type === "assigned_to_me") {
    const issues = (widget.data as AssignedIssue[] | undefined) ?? [];
    return (
      <ul className="space-y-1 text-sm">
        {issues.map((i) => (
          <li key={i.id} className="flex justify-between gap-2">
            <a href={`/app/browse/${i.key}`} className="truncate text-[#0052cc] hover:underline">
              {i.key} {i.title}
            </a>
            <span className="shrink-0 text-xs text-slate-400">{i.status_name}</span>
          </li>
        ))}
        {issues.length === 0 && <li className="text-slate-400">No assigned issues</li>}
      </ul>
    );
  }

  if (widget.widget_type === "activity_stream") {
    const activity = (widget.data as ActivityItem[] | undefined) ?? [];
    return (
      <ul className="space-y-1 text-sm">
        {activity.map((a) => (
          <li key={a.id}>
            <span className="font-medium">{a.actor_name}</span> changed {a.field_name} on{" "}
            <a href={`/app/browse/${a.issue_key}`} className="text-[#0052cc] hover:underline">
              {a.issue_key}
            </a>
            : {a.old_value} → {a.new_value}
          </li>
        ))}
        {activity.length === 0 && <li className="text-slate-400">No recent activity</li>}
      </ul>
    );
  }

  return <pre className="max-h-48 overflow-auto text-xs text-slate-600">{JSON.stringify(widget.data ?? {}, null, 2)}</pre>;
}

export default function DashboardView({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const dash = useQuery({ queryKey: ["dashboard", id], queryFn: () => dashboards.get(id) });
  const d = dash.data;
  const widgets = d?.widgets ?? [];

  return (
    <div className="mx-auto max-w-4xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">{d?.name ?? "Dashboard"}</h1>
      <div className="grid grid-cols-2 gap-4" data-testid="dashboard-gadgets">
        {widgets.map((wgt) => (
          <section key={wgt.id} className="rounded border border-slate-200 bg-white p-4">
            <h2 className="mb-2 text-sm font-semibold text-slate-700">{wgt.widget_type}</h2>
            <Gadget widget={wgt} />
          </section>
        ))}
        {widgets.length === 0 && <p className="text-sm text-slate-400">No gadgets</p>}
      </div>
    </div>
  );
}
