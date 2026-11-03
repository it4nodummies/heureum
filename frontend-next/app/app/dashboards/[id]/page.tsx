"use client";

import { use, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { dashboards, type AssignedIssue, type ActivityItem, type DashboardWidget } from "@/lib/api";
import { useT, type MessageKey } from "@/lib/i18n";
import { historyFieldLabel, richSentence, slot, stripMark } from "@/components/issues/historyText";

// The two gadget types the backend hydrates + renders; anything else falls back
// to raw JSON, so the "catalog" the UI offers is exactly these two. Titles are
// chrome (translated); an unknown widget type is shown as stored.
const GADGET_CATALOG: { type: string; label: MessageKey }[] = [
  { type: "assigned_to_me", label: "dashboards.gadgets.assignedToMe" },
  { type: "activity_stream", label: "dashboards.gadgets.activityStream" },
];

function gadgetLabel(t: (k: MessageKey) => string, widgetType: string): string {
  const g = GADGET_CATALOG.find((c) => c.type === widgetType);
  return g ? t(g.label) : widgetType;
}

function Gadget({ widget }: { widget: DashboardWidget }) {
  const t = useT();
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
        {issues.length === 0 && <li className="text-slate-400">{t("dashboards.gadgets.noAssigned")}</li>}
      </ul>
    );
  }

  if (widget.widget_type === "activity_stream") {
    const activity = (widget.data as ActivityItem[] | undefined) ?? [];
    return (
      <ul className="space-y-1 text-sm">
        {activity.map((a) => (
          <li key={a.id}>
            {richSentence(
              t(a.field_name === "created" ? "dashboards.gadgets.created" : "dashboards.gadgets.changed", {
                actor: slot("actor"),
                field: slot("field"),
                key: slot("key"),
                from: stripMark(a.old_value ?? ""),
                to: stripMark(a.new_value ?? ""),
              }),
              {
                actor: <span className="font-medium">{a.actor_name || t("issues.history.system")}</span>,
                field: historyFieldLabel(t, a.field_name),
                key: (
                  <a href={`/app/browse/${a.issue_key}`} className="text-[#0052cc] hover:underline">
                    {a.issue_key}
                  </a>
                ),
              },
            )}
          </li>
        ))}
        {activity.length === 0 && <li className="text-slate-400">{t("dashboards.gadgets.noActivity")}</li>}
      </ul>
    );
  }

  return <pre className="max-h-48 overflow-auto text-xs text-slate-600">{JSON.stringify(widget.data ?? {}, null, 2)}</pre>;
}

export default function DashboardView({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const t = useT();
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
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">{d?.name ?? t("dashboards.detail.fallbackTitle")}</h1>

      <div className="mb-4 flex items-center gap-2">
        <select
          aria-label={t("dashboards.detail.gadgetType")}
          value={gadgetType}
          onChange={(e) => setGadgetType(e.target.value)}
          className="rounded border border-slate-300 px-3 py-1.5 text-sm"
        >
          {GADGET_CATALOG.map((g) => (
            <option key={g.type} value={g.type}>
              {t(g.label)}
            </option>
          ))}
        </select>
        <button
          onClick={() => addWidget.mutate(gadgetType)}
          disabled={addWidget.isPending}
          className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
        >
          {t("dashboards.detail.addGadget")}
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
              <h2 className="text-sm font-semibold text-slate-700">{gadgetLabel(t, wgt.widget_type)}</h2>
              <button
                onClick={() => removeWidget.mutate(wgt.id)}
                disabled={removeWidget.isPending}
                className="text-xs text-slate-400 hover:text-red-600 disabled:opacity-60"
              >
                {t("common.actions.remove")}
              </button>
            </div>
            <Gadget widget={wgt} />
          </section>
        ))}
        {widgets.length === 0 && <p className="text-sm text-slate-400">{t("dashboards.detail.noGadgets")}</p>}
      </div>
    </div>
  );
}
