"use client";

import { use, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { versions as versionsApi, type Version } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";
import { useFormat, useT } from "@/lib/i18n";

type StatusFilter = "all" | "released" | "unreleased";

// Version dates are calendar dates ("YYYY-MM-DD"): build a LOCAL date so the
// formatted day never shifts with the browser's time zone.
function parseCalendarDate(s: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  return m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : null;
}

export default function ReleasesPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);
  const qc = useQueryClient();
  const t = useT();
  const fmt = useFormat();
  const showDate = (s?: string | null) => {
    if (!s) return "—";
    const d = parseCalendarDate(s);
    return d ? fmt.date(d) : s;
  };

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [startDate, setStartDate] = useState("");
  const [releaseDate, setReleaseDate] = useState("");
  const [filter, setFilter] = useState<StatusFilter>("all");

  const q = useQuery({
    queryKey: ["versions", key],
    queryFn: () => versionsApi.list(key),
  });

  const invalidate = () => qc.invalidateQueries({ queryKey: ["versions", key] });

  const createMut = useMutation({
    mutationFn: () =>
      versionsApi.create(key, {
        name: name.trim(),
        description: description.trim() || undefined,
        startDate: startDate || undefined,
        releaseDate: releaseDate || undefined,
      }),
    onSuccess: () => {
      setName("");
      setDescription("");
      setStartDate("");
      setReleaseDate("");
      invalidate();
    },
  });

  const toggleMut = useMutation({
    mutationFn: (v: Version) => versionsApi.update(v.id, { released: !v.released }),
    onSuccess: invalidate,
  });

  const removeMut = useMutation({
    mutationFn: (id: string) => versionsApi.remove(id),
    onSuccess: invalidate,
  });

  const all = q.data ?? [];
  const rows = all.filter((v) =>
    filter === "all" ? true : filter === "released" ? v.released : !v.released
  );

  return (
    <div>
      <ProjectHeader projectKey={key} active="releases" />
      <div data-testid="releases-page" className="mx-auto max-w-5xl p-6">
        <div className="mb-6 flex items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[#1a1f36]">{t("projects.releases.title")}</h2>
          <label className="flex items-center gap-2 text-sm text-slate-500">
            <span>{t("projects.releases.show")}</span>
            <select
              aria-label={t("projects.releases.statusFilter")}
              value={filter}
              onChange={(e) => setFilter(e.target.value as StatusFilter)}
              className="rounded border border-slate-300 px-2 py-1 text-sm"
            >
              <option value="all">{t("projects.releases.filterAll")}</option>
              <option value="released">{t("projects.releases.released")}</option>
              <option value="unreleased">{t("projects.releases.unreleased")}</option>
            </select>
          </label>
        </div>

        {/* Create release form */}
        <form
          className="mb-6 grid grid-cols-1 gap-3 rounded-lg border border-slate-200 bg-slate-50/50 p-4 sm:grid-cols-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim()) createMut.mutate();
          }}
        >
          <label className="flex flex-col gap-1 text-xs font-medium text-slate-500">
            {t("projects.releases.name")}
            <input
              aria-label={t("projects.releases.nameAria")}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("projects.releases.namePlaceholder")}
              className="rounded border border-slate-300 px-2 py-1.5 text-sm text-[#1a1f36]"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium text-slate-500">
            {t("projects.releases.description")}
            <input
              aria-label={t("projects.releases.descriptionAria")}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1.5 text-sm text-[#1a1f36]"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium text-slate-500">
            {t("projects.releases.startDate")}
            <input
              type="date"
              aria-label={t("projects.releases.startDate")}
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1.5 text-sm text-[#1a1f36]"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium text-slate-500">
            {t("projects.releases.releaseDate")}
            <input
              type="date"
              aria-label={t("projects.releases.releaseDate")}
              value={releaseDate}
              onChange={(e) => setReleaseDate(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1.5 text-sm text-[#1a1f36]"
            />
          </label>
          <div className="sm:col-span-2">
            <button
              type="submit"
              disabled={!name.trim() || createMut.isPending}
              className="rounded-lg bg-[#0052cc] px-3.5 py-1.5 text-sm font-semibold text-white transition-colors hover:bg-[#0065ff] disabled:opacity-50"
            >
              {t("projects.releases.create")}
            </button>
            {createMut.isError && (
              <span className="ml-3 text-sm text-red-600">
                {createMut.error instanceof Error ? createMut.error.message : t("projects.releases.createFailed")}
              </span>
            )}
          </div>
        </form>

        {q.isLoading ? (
          <p className="text-sm text-slate-400">{t("projects.releases.loading")}</p>
        ) : rows.length === 0 ? (
          <p className="text-sm text-slate-400">{t("projects.releases.empty")}</p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-slate-200">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50 text-left text-xs uppercase text-slate-400">
                  <th className="px-3 py-2 font-medium">{t("projects.releases.name")}</th>
                  <th className="px-3 py-2 font-medium">{t("projects.releases.colStatus")}</th>
                  <th className="px-3 py-2 font-medium">{t("projects.releases.startDate")}</th>
                  <th className="px-3 py-2 font-medium">{t("projects.releases.releaseDate")}</th>
                  <th className="px-3 py-2 font-medium">{t("projects.releases.description")}</th>
                  <th className="px-3 py-2 font-medium" />
                </tr>
              </thead>
              <tbody>
                {rows.map((v) => (
                  <tr key={v.id} className="border-b border-slate-100 last:border-0">
                    <td className="px-3 py-2 font-medium text-[#1a1f36]">{v.name}</td>
                    <td className="px-3 py-2">
                      <button
                        onClick={() => toggleMut.mutate(v)}
                        disabled={toggleMut.isPending}
                        title={v.released ? t("projects.releases.markUnreleased") : t("projects.releases.markReleased")}
                        className={
                          v.released
                            ? "rounded-full bg-green-100 px-2 py-0.5 text-xs font-semibold text-green-700"
                            : "rounded-full bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-500"
                        }
                      >
                        {v.released ? t("projects.releases.released") : t("projects.releases.unreleased")}
                      </button>
                    </td>
                    <td className="px-3 py-2 text-slate-500">{showDate(v.startDate)}</td>
                    <td className="px-3 py-2 text-slate-500">{showDate(v.releaseDate)}</td>
                    <td className="px-3 py-2 text-slate-500">{v.description || "—"}</td>
                    <td className="px-3 py-2 text-right">
                      <button
                        onClick={() => removeMut.mutate(v.id)}
                        disabled={removeMut.isPending}
                        className="text-xs text-slate-400 hover:text-red-600"
                      >
                        {t("projects.releases.delete")}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <p className="mt-4 text-xs text-slate-400">
          {t("projects.releases.noteBefore")} <code>fixVersion</code> {t("projects.releases.noteAfter")}
        </p>
      </div>
    </div>
  );
}
