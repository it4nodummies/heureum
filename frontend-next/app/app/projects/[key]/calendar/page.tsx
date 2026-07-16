"use client";

import { use, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { calendar } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

const MONTHS = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];
const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

export default function CalendarPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);
  const now = new Date();
  const [ym, setYm] = useState<{ year: number; month: number }>({
    year: now.getFullYear(),
    month: now.getMonth() + 1, // 1-based
  });

  const q = useQuery({
    queryKey: ["calendar", key, ym.year, ym.month],
    queryFn: () => calendar.get(key, ym.year, ym.month),
  });

  function shift(delta: number) {
    setYm((prev) => {
      const zero = prev.month - 1 + delta;
      const year = prev.year + Math.floor(zero / 12);
      const month = ((zero % 12) + 12) % 12 + 1;
      return { year, month };
    });
  }

  // Leading blank cells so day 1 aligns to its weekday.
  const firstWeekday = new Date(ym.year, ym.month - 1, 1).getDay();
  const blanks = Array.from({ length: firstWeekday });

  return (
    <div>
      <ProjectHeader projectKey={key} active="calendar" />
      <div className="mx-auto max-w-5xl p-6">
        <div className="mb-4 flex items-center gap-3">
          <button
            aria-label="Previous month"
            onClick={() => shift(-1)}
            className="rounded border border-slate-300 px-2 py-1 text-sm hover:bg-slate-50"
          >
            ‹
          </button>
          <h2 data-testid="calendar-title" className="text-sm font-semibold text-[#1a1f36]">
            {MONTHS[ym.month - 1]} {ym.year}
          </h2>
          <button
            aria-label="Next month"
            onClick={() => shift(1)}
            className="rounded border border-slate-300 px-2 py-1 text-sm hover:bg-slate-50"
          >
            ›
          </button>
        </div>

        <div data-testid="calendar-grid" className="grid grid-cols-7 gap-1">
          {WEEKDAYS.map((d) => (
            <div key={d} className="pb-1 text-center text-[10px] font-medium uppercase text-slate-400">
              {d}
            </div>
          ))}
          {blanks.map((_, i) => (
            <div key={`b${i}`} className="min-h-24 rounded bg-slate-50/50" />
          ))}
          {(q.data?.days ?? []).map((day) => (
            <div
              key={day.date}
              data-testid="calendar-day"
              className="min-h-24 rounded border border-slate-100 p-1"
            >
              <div className="mb-1 text-[10px] text-slate-400">{day.day}</div>
              <div className="space-y-1">
                {day.issues.map((iss) => (
                  <div
                    key={iss.id}
                    className="truncate rounded bg-[#0052cc]/10 px-1 py-0.5 text-[10px] text-[#0052cc]"
                    title={`${iss.key} · ${iss.title}`}
                  >
                    {iss.key}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
