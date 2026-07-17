"use client";

import { use, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { dashboards, type AssignedIssue, type ActivityItem, type DashboardWidget } from "@/lib/api";

// The two gadget types the backend hydrates + renders; anything else falls back
// to raw JSON, so the "catalog" the UI offers is exactly these two.
const GADGET_CATALOG: { type: string; label: string }[] = [
  { type: "assigned_to_me", label: "Assigned to me" },
  { type: "activity_stream", label: "Activity stream" },
];

function gadgetLabel(widgetType: string): string {
  return GADGET_CATALOG.find((g) => g.type === widgetType)?.label ?? widgetType;
}

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
  const qc = useQueryClient();
  const [gadgetType, setGadgetType] = useState(GADGET_CATALOG[0].type);
  const dash = useQuery({ queryKey: ["dashboard", id], queryFn: () => dashboards.get(id) });
  const d = dash.data;
  const widgets = d?.widgets ?? [];

  const invalidate = () => qc.invalidateQueries({ queryKey: ["dashboard", id] });

  const addWidget = useMutation({
    mutationFn: (widgetType: string) =>
      dashboards.addWidget(id, { widget_type: widgetType, config_json: "{}" }),
    onSuccess: invalidate,
  });

  const removeWidget = useMutation({
    mutationFn: (widgetId: string) => dashboards.removeWidget(id, widgetId),
    onSuccess: invalidate,
  });

  return (
    <div className="mx-auto max-w-4xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">{d?.name ?? "Dashboard"}</h1>

      <div className="mb-4 flex items-center gap-2">
        <select
          aria-label="Gadget type"
          value={gadgetType}
          onChange={(e) => setGadgetType(e.target.value)}
          className="rounded border border-slate-300 px-3 py-1.5 text-sm"
        >
          {GADGET_CATALOG.map((g) => (
            <option key={g.type} value={g.type}>
              {g.label}
            </option>
          ))}
        </select>
        <button
          onClick={() => addWidget.mutate(gadgetType)}
          disabled={addWidget.isPending}
          className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
        >
          Add gadget
        </button>
      </div>

      <div className="grid grid-cols-2 gap-4" data-testid="dashboard-gadgets">
        {widgets.map((wgt) => (
          <section
            key={wgt.id}
            data-testid="gadget"
            className="rounded border border-slate-200 bg-white p-4"
          >
            <div className="mb-2 flex items-center justify-between gap-2">
              <h2 className="text-sm font-semibold text-slate-700">{gadgetLabel(wgt.widget_type)}</h2>
              <button
                onClick={() => removeWidget.mutate(wgt.id)}
                disabled={removeWidget.isPending}
                className="text-xs text-slate-400 hover:text-red-600 disabled:opacity-60"
              >
                Remove
              </button>
            </div>
            <Gadget widget={wgt} />
          </section>
        ))}
        {widgets.length === 0 && <p className="text-sm text-slate-400">No gadgets</p>}
      </div>
    </div>
  );
}
