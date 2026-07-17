"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { timeline, type TimelineBar } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

type Zoom = "weeks" | "months" | "quarters";

function pct(value: number, min: number, max: number): number {
  if (max <= min) return 0;
  return ((value - min) / (max - min)) * 100;
}

export default function TimelinePage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);
  const [zoom, setZoom] = useState<Zoom>("weeks");
  const q = useQuery({
    queryKey: ["timeline", key, zoom],
    queryFn: () => timeline.get(key, zoom),
  });

  return (
    <div>
      <ProjectHeader projectKey={key} active="timeline" />
      <div className="mx-auto max-w-5xl p-6">
        <div className="mb-4 flex items-center gap-2">
          <label className="text-sm text-slate-500">Zoom</label>
          <select
            aria-label="Timeline zoom"
            value={zoom}
            onChange={(e) => setZoom(e.target.value as Zoom)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            <option value="weeks">Weeks</option>
            <option value="months">Months</option>
            <option value="quarters">Quarters</option>
          </select>
        </div>

        {q.isLoading && <p className="text-sm text-slate-400">Loading timeline…</p>}
        {q.data && q.data.bars.length === 0 && (
          <p className="text-sm text-slate-400">No epics or sprints with dates yet.</p>
        )}
        {q.data && q.data.bars.length > 0 && (
          <TimelineChart data={q.data} />
        )}
      </div>
    </div>
  );
}

function TimelineChart({ data }: { data: { start_date: string; end_date: string; bars: TimelineBar[]; headers: string[] } }) {
  const min = new Date(data.start_date).getTime();
  const max = new Date(data.end_date).getTime();

  return (
    <div data-testid="timeline-chart" className="rounded border border-slate-200 bg-white">
      {/* Header ruler */}
      <div className="flex border-b border-slate-100 px-3 py-2 text-[10px] text-slate-400">
        {data.headers.map((h, i) => (
          <div key={i} className="flex-1 text-center">{h}</div>
        ))}
      </div>
      {/* Bars */}
      <div className="space-y-2 p-3">
        {data.bars.map((bar) => {
          const s = bar.start_date ? new Date(bar.start_date).getTime() : min;
          const e = bar.end_date ? new Date(bar.end_date).getTime() : max;
          const left = pct(s, min, max);
          const width = Math.max(pct(e, min, max) - left, 1.5);
          return (
            <div key={bar.id} className="flex items-center gap-2">
              <div className="w-32 shrink-0 truncate text-xs text-[#1a1f36]" title={bar.name}>
                {bar.name}
              </div>
              <div className="relative h-5 flex-1 rounded bg-slate-50">
                <div
                  data-testid="timeline-bar"
                  className="absolute top-0 flex h-5 items-center rounded px-1.5 text-[10px] font-medium text-white"
                  style={{ left: `${left}%`, width: `${width}%`, backgroundColor: bar.color }}
                  title={`${bar.name} · ${bar.type}${bar.type === "sprint" ? ` · ${Math.round(bar.progress)}%` : ""}`}
                >
                  {bar.type === "sprint" ? `${Math.round(bar.progress)}%` : ""}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
