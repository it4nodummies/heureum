"use client";

import { use, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { calendar } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";
import { useLocale, useT } from "@/lib/i18n";

// The week starts on Monday in Italian, on Sunday in English (unchanged).
function weekStart(locale: string): 0 | 1 {
  return locale === "it" ? 1 : 0;
}

export default function CalendarPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);
  const t = useT();
  const { locale } = useLocale();
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

  // Month/weekday names from Intl in the active locale.
  const start = weekStart(locale);
  const weekdays = useMemo(() => {
    const fmt = new Intl.DateTimeFormat(locale, { weekday: "short" });
    // 2023-01-01 is a Sunday: day i of the week (0 = Sunday) is Jan 1 + i.
    return Array.from({ length: 7 }, (_, i) => fmt.format(new Date(2023, 0, 1 + ((i + start) % 7))));
  }, [locale, start]);
  const monthFmt = useMemo(() => new Intl.DateTimeFormat(locale, { month: "long", year: "numeric" }), [locale]);
  const monthTitle = monthFmt.format(new Date(ym.year, ym.month - 1, 1));
  const title = monthTitle.charAt(0).toUpperCase() + monthTitle.slice(1);

  // Leading blank cells so day 1 aligns to its weekday column.
  const firstWeekday = (new Date(ym.year, ym.month - 1, 1).getDay() - start + 7) % 7;
  const blanks = Array.from({ length: firstWeekday });

  return (
    <div>
      <ProjectHeader projectKey={key} active="calendar" />
      <div className="mx-auto max-w-5xl p-6">
        <div className="mb-4 flex items-center gap-3">
          <button
            aria-label={t("projects.calendar.previous")}
            onClick={() => shift(-1)}
            className="rounded border border-slate-300 px-2 py-1 text-sm hover:bg-slate-50"
          >
            ‹
          </button>
          <h2 data-testid="calendar-title" className="text-sm font-semibold text-[#1a1f36]">
            {title}
          </h2>
          <button
            aria-label={t("projects.calendar.next")}
            onClick={() => shift(1)}
            className="rounded border border-slate-300 px-2 py-1 text-sm hover:bg-slate-50"
          >
            ›
          </button>
        </div>

        <div data-testid="calendar-grid" className="grid grid-cols-7 gap-1">
          {weekdays.map((d) => (
            <div key={d} data-testid="calendar-weekday" className="pb-1 text-center text-[10px] font-medium uppercase text-slate-400">
              {d}
            </div>
          ))}
          {blanks.map((_, i) => (
            <div key={`b${i}`} data-testid="calendar-blank" className="min-h-24 rounded bg-slate-50/50" />
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
