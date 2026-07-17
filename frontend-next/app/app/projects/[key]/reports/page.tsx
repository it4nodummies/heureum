"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { reports, boards } from "@/lib/api";
import { LineChart } from "@/components/charts/LineChart";
import { BarChart } from "@/components/charts/BarChart";
import { PieChart } from "@/components/charts/PieChart";
import { StackedAreaChart } from "@/components/charts/StackedAreaChart";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

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
          <label className="text-sm text-slate-500">Sprint</label>
          <select
            data-testid="reports-sprint-select"
            aria-label="Sprint"
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

        <Card title="Burndown">
          {burndown.data ? (
            <LineChart
              labels={burndown.data.labels}
              series={[
                { name: "Ideal", color: "#8993a4", values: burndown.data.ideal },
                { name: "Actual", color: "#0052cc", values: burndown.data.actual },
              ]}
            />
          ) : (
            <p className="text-sm text-slate-400">No active sprint</p>
          )}
        </Card>

        <Card title="Burnup">
          {burnup.data ? (
            <LineChart
              labels={burnup.data.labels}
              series={[
                { name: "Scope", color: "#8993a4", values: burnup.data.ideal },
                { name: "Completed", color: "#00875a", values: burnup.data.actual },
              ]}
            />
          ) : (
            <p className="text-sm text-slate-400">No active sprint</p>
          )}
        </Card>

        <Card title="Velocity">
          {velocity.data && velocity.data.sprints.length > 0 ? (
            <BarChart bars={velocity.data.sprints.map((s) => ({ label: s.sprint_name, value: s.completed }))} />
          ) : (
            <p className="text-sm text-slate-400">No completed sprints</p>
          )}
        </Card>

        <Card title="Cumulative Flow">
          {cfd.data ? <StackedAreaChart dates={cfd.data.dates} categories={cfd.data.categories} data={cfd.data.data} /> : null}
        </Card>

        <Card title="Created vs Resolved (14d)">
          {cvr.data ? (
            <LineChart
              labels={cvr.data.dates}
              series={[
                { name: "Created", color: "#de350b", values: cvr.data.created },
                { name: "Resolved", color: "#00875a", values: cvr.data.resolved },
              ]}
            />
          ) : null}
        </Card>

        <Card title="Breakdown">
          <div className="mb-3">
            <select
              aria-label="Pie field"
              value={pieField}
              onChange={(e) => setPieField(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1 text-sm"
            >
              <option value="status">Status</option>
              <option value="priority">Priority</option>
              <option value="assignee">Assignee</option>
              <option value="type">Type</option>
            </select>
          </div>
          {pie.data ? <PieChart slices={pie.data} /> : null}
        </Card>
      </div>
    </div>
  );
}
