"use client";

import { use, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { versions as versionsApi, type Version } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

type StatusFilter = "all" | "released" | "unreleased";

export default function ReleasesPage({ params }: { params: Promise<{ key: string }> }) {
  const { key } = use(params);
  const qc = useQueryClient();

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
          <h2 className="text-lg font-semibold text-[#1a1f36]">Releases</h2>
          <label className="flex items-center gap-2 text-sm text-slate-500">
            <span>Show</span>
            <select
              aria-label="Status filter"
              value={filter}
              onChange={(e) => setFilter(e.target.value as StatusFilter)}
              className="rounded border border-slate-300 px-2 py-1 text-sm"
            >
              <option value="all">All</option>
              <option value="released">Released</option>
              <option value="unreleased">Unreleased</option>
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
            Name
            <input
              aria-label="Release name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. v1.0"
              className="rounded border border-slate-300 px-2 py-1.5 text-sm text-[#1a1f36]"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium text-slate-500">
            Description
            <input
              aria-label="Release description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1.5 text-sm text-[#1a1f36]"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium text-slate-500">
            Start date
            <input
              type="date"
              aria-label="Start date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1.5 text-sm text-[#1a1f36]"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium text-slate-500">
            Release date
            <input
              type="date"
              aria-label="Release date"
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
              Create release
            </button>
            {createMut.isError && (
              <span className="ml-3 text-sm text-red-600">
                {createMut.error instanceof Error ? createMut.error.message : "Failed to create"}
              </span>
            )}
          </div>
        </form>

        {q.isLoading ? (
          <p className="text-sm text-slate-400">Loading releases…</p>
        ) : rows.length === 0 ? (
          <p className="text-sm text-slate-400">No releases yet.</p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-slate-200">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50 text-left text-xs uppercase text-slate-400">
                  <th className="px-3 py-2 font-medium">Name</th>
                  <th className="px-3 py-2 font-medium">Status</th>
                  <th className="px-3 py-2 font-medium">Start date</th>
                  <th className="px-3 py-2 font-medium">Release date</th>
                  <th className="px-3 py-2 font-medium">Description</th>
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
                        title={v.released ? "Mark as unreleased" : "Mark as released"}
                        className={
                          v.released
                            ? "rounded-full bg-green-100 px-2 py-0.5 text-xs font-semibold text-green-700"
                            : "rounded-full bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-500"
                        }
                      >
                        {v.released ? "Released" : "Unreleased"}
                      </button>
                    </td>
                    <td className="px-3 py-2 text-slate-500">{v.startDate ?? "—"}</td>
                    <td className="px-3 py-2 text-slate-500">{v.releaseDate ?? "—"}</td>
                    <td className="px-3 py-2 text-slate-500">{v.description || "—"}</td>
                    <td className="px-3 py-2 text-right">
                      <button
                        onClick={() => removeMut.mutate(v.id)}
                        disabled={removeMut.isPending}
                        className="text-xs text-slate-400 hover:text-red-600"
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <p className="mt-4 text-xs text-slate-400">
          Per-version issue progress is not shown: the JQL grammar does not yet support the{" "}
          <code>fixVersion</code> field, and there is no version issue-count endpoint.
        </p>
      </div>
    </div>
  );
}
