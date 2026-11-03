"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { reports, boards } from "@/lib/api";
import { LineChart } from "@/components/charts/LineChart";
import { BarChart } from "@/components/charts/BarChart";
import { PieChart } from "@/components/charts/PieChart";
import { StackedAreaChart } from "@/components/charts/StackedAreaChart";
import { ProjectHeader } from "@/components/projects/ProjectHeader";
import { useT } from "@/lib/i18n";

// Fallback labels the report API emits for missing values (chrome, not data).
const PIE_FALLBACKS = {
  "No status": "projects.reports.pieFallback.noStatus",
  Unassigned: "projects.reports.pieFallback.unassigned",
  "No type": "projects.reports.pieFallback.noType",
} as const;

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-6 rounded border border-slate-200 bg-white p-4">
      <h2 className="mb-3 text-sm font-semibold text-[#1a1f36]">{title}</h2>
      {children}
    </section>
  );
}

export default function ReportsPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);
  const t = useT();
  const [pieField, setPieField] = useState("status");

  // Resolve THIS project's board (not the hardcoded board 1), then its sprints.
  const boardsList = useQuery({ queryKey: ["boards"], queryFn: () => boards.list() });
  const board = boardsList.data?.values.find((b) => b.location?.projectKey === key);
  const sprints = useQuery({
    queryKey: ["reports", key, "sprints", board?.id],
    queryFn: () => boards.sprints(board!.id),
    enabled: !!board,
  });
  const [sprintId, setSprintId] = useState<string>("");
  const defaultSprint =
    sprints.data?.values.find((s) => s.state === "active") ?? sprints.data?.values[0];
  const activeSprintId = sprintId || defaultSprint?.id?.toString() || "";

  const burndown = useQuery({
    queryKey: ["reports", key, "burndown", activeSprintId],
    queryFn: () => reports.burndown(key, activeSprintId),
    enabled: !!activeSprintId,
  });
  const burnup = useQuery({
    queryKey: ["reports", key, "burnup", activeSprintId],
    queryFn: () => reports.burnup(key, activeSprintId),
    enabled: !!activeSprintId,
  });
  const velocity = useQuery({ queryKey: ["reports", key, "velocity"], queryFn: () => reports.velocity(key) });
  const cfd = useQuery({ queryKey: ["reports", key, "cfd"], queryFn: () => reports.cfd(key) });
  const pie = useQuery({ queryKey: ["reports", key, "pie", pieField], queryFn: () => reports.pie(key, pieField) });
  const cvr = useQuery({ queryKey: ["reports", key, "cvr"], queryFn: () => reports.createdVsResolved(key, 14) });

  return (
    <div>
      <ProjectHeader projectKey={key} active="reports" />
      <div className="mx-auto max-w-3xl p-6">
        <div className="mb-4 flex items-center gap-2">
          <label className="text-sm text-slate-500">{t("projects.reports.sprint")}</label>
          <select
            data-testid="reports-sprint-select"
            aria-label={t("projects.reports.sprint")}
            value={activeSprintId}
            onChange={(e) => setSprintId(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            {(sprints.data?.values ?? []).map((s) => (
              <option key={s.id} value={String(s.id)}>
                {s.name}
              </option>
            ))}
          </select>
        </div>

        <Card title={t("projects.reports.burndown")}>
          {burndown.data ? (
            <LineChart
              labels={burndown.data.labels}
              series={[
                { name: t("projects.reports.ideal"), color: "#8993a4", values: burndown.data.ideal },
                { name: t("projects.reports.actual"), color: "#0052cc", values: burndown.data.actual },
              ]}
            />
          ) : (
            <p className="text-sm text-slate-400">{t("projects.reports.noActiveSprint")}</p>
          )}
        </Card>

        <Card title={t("projects.reports.burnup")}>
          {burnup.data ? (
            <LineChart
              labels={burnup.data.labels}
              series={[
                { name: t("projects.reports.scope"), color: "#8993a4", values: burnup.data.ideal },
                { name: t("projects.reports.completed"), color: "#00875a", values: burnup.data.actual },
              ]}
            />
          ) : (
            <p className="text-sm text-slate-400">{t("projects.reports.noActiveSprint")}</p>
          )}
        </Card>

        <Card title={t("projects.reports.velocity")}>
          {velocity.data && velocity.data.sprints.length > 0 ? (
            <BarChart bars={velocity.data.sprints.map((s) => ({ label: s.sprint_name, value: s.completed }))} />
          ) : (
            <p className="text-sm text-slate-400">{t("projects.reports.noCompletedSprints")}</p>
          )}
        </Card>

        <Card title={t("projects.reports.cfd")}>
          {cfd.data ? (
            <StackedAreaChart
              dates={cfd.data.dates}
              categories={cfd.data.categories}
              data={cfd.data.data}
              categoryLabels={{
                todo: t("projects.reports.categories.todo"),
                inprogress: t("projects.reports.categories.inprogress"),
                done: t("projects.reports.categories.done"),
              }}
            />
          ) : null}
        </Card>

        <Card title={t("projects.reports.cvr")}>
          {cvr.data ? (
            <LineChart
              labels={cvr.data.dates}
              series={[
                { name: t("projects.reports.created"), color: "#de350b", values: cvr.data.created },
                { name: t("projects.reports.resolved"), color: "#00875a", values: cvr.data.resolved },
              ]}
            />
          ) : null}
        </Card>

        <Card title={t("projects.reports.breakdown")}>
          <div className="mb-3">
            <select
              aria-label={t("projects.reports.pieField")}
              value={pieField}
              onChange={(e) => setPieField(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1 text-sm"
            >
              <option value="status">{t("projects.reports.pieFields.status")}</option>
              <option value="priority">{t("projects.reports.pieFields.priority")}</option>
              <option value="assignee">{t("projects.reports.pieFields.assignee")}</option>
              <option value="type">{t("projects.reports.pieFields.type")}</option>
            </select>
          </div>
          {pie.data ? (
            <PieChart
              slices={pie.data.map((s) =>
                Object.prototype.hasOwnProperty.call(PIE_FALLBACKS, s.label)
                  ? { ...s, label: t(PIE_FALLBACKS[s.label as keyof typeof PIE_FALLBACKS]) }
                  : s
              )}
            />
          ) : null}
        </Card>
      </div>
    </div>
  );
}
