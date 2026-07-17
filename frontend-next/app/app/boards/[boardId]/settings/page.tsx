"use client";

import { use, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { boards, workflow, type WorkflowStatus } from "@/lib/api";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

interface EditColumn {
  name: string;
  statusIds: string[];
}

interface EditQuickFilter {
  name: string;
  jql: string;
}

type Swimlane = "none" | "assignee" | "epic";

export default function BoardSettingsPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);
  const qc = useQueryClient();

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });
  const projectKey = board.data?.location?.projectKey;

  const config = useQuery({ queryKey: ["board", id, "config"], queryFn: () => boards.config(id) });
  const wf = useQuery({
    queryKey: ["workflow", projectKey],
    queryFn: () => workflow.get(projectKey!),
    enabled: !!projectKey,
  });

  const statuses: WorkflowStatus[] = useMemo(
    () => [...(wf.data?.statuses ?? [])].sort((a, b) => a.position - b.position),
    [wf.data],
  );

  const [columns, setColumns] = useState<EditColumn[]>([]);
  const [swimlane, setSwimlane] = useState<Swimlane>("none");
  const [quickFilters, setQuickFilters] = useState<EditQuickFilter[]>([]);
  const [newFilterName, setNewFilterName] = useState("");
  const [newFilterJql, setNewFilterJql] = useState("");
  const [saved, setSaved] = useState(false);

  // Hydrate the editable form from the persisted config ONCE (guarded by a ref),
  // not on every `config.data` change: TanStack refetches (refetchOnWindowFocus,
  // staleTime expiry) would otherwise clobber in-progress edits with server data
  // on a window blur/refocus. The guard is re-armed after a successful save so the
  // post-save invalidate/refetch re-seeds local state from the freshly-saved config.
  const hydrated = useRef(false);
  useEffect(() => {
    if (hydrated.current || !config.data) return;
    hydrated.current = true;
    setColumns(config.data.columns.map((c) => ({ name: c.name, statusIds: [...c.statusIds] })));
    setSwimlane(config.data.swimlane);
    setQuickFilters(config.data.quickFilters.map((q) => ({ name: q.name, jql: q.jql })));
  }, [config.data]);

  const save = useMutation({
    mutationFn: () =>
      boards.saveConfig(id, {
        columns: columns.map((c) => ({ name: c.name.trim(), statusIds: c.statusIds })),
        swimlane,
        quickFilters: quickFilters.map((q) => ({ name: q.name, jql: q.jql })),
      }),
    onSuccess: () => {
      setSaved(true);
      // Re-arm hydration so the post-save refetch re-seeds local state from the
      // saved config instead of being ignored.
      hydrated.current = false;
      qc.invalidateQueries({ queryKey: ["board", id, "config"] });
      qc.invalidateQueries({ queryKey: ["board", id, "configuration"] });
    },
  });

  // Client-side validation: block save on empty/duplicate column names or no
  // columns (the server would otherwise persist a broken board layout).
  const validationError = useMemo(() => {
    if (columns.length === 0) return "Add at least one column.";
    if (columns.some((c) => c.name.trim() === "")) return "Column names cannot be empty.";
    const seen = new Set<string>();
    for (const c of columns) {
      const key = c.name.trim().toLowerCase();
      if (seen.has(key)) return "Column names must be unique.";
      seen.add(key);
    }
    return null;
  }, [columns]);

  function renameColumn(idx: number, name: string) {
    setSaved(false);
    setColumns((cols) => cols.map((c, i) => (i === idx ? { ...c, name } : c)));
  }

  function toggleStatus(colIdx: number, statusId: string) {
    setSaved(false);
    setColumns((cols) =>
      cols.map((c, i) => {
        if (i !== colIdx) return c;
        const has = c.statusIds.includes(statusId);
        return {
          ...c,
          statusIds: has ? c.statusIds.filter((s) => s !== statusId) : [...c.statusIds, statusId],
        };
      }),
    );
  }

  function addColumn() {
    setSaved(false);
    setColumns((cols) => [...cols, { name: `Column ${cols.length + 1}`, statusIds: [] }]);
  }

  function removeColumn(idx: number) {
    setSaved(false);
    setColumns((cols) => cols.filter((_, i) => i !== idx));
  }

  function addQuickFilter() {
    if (!newFilterName.trim() || !newFilterJql.trim()) return;
    setSaved(false);
    setQuickFilters((qf) => [...qf, { name: newFilterName.trim(), jql: newFilterJql.trim() }]);
    setNewFilterName("");
    setNewFilterJql("");
  }

  function removeQuickFilter(idx: number) {
    setSaved(false);
    setQuickFilters((qf) => qf.filter((_, i) => i !== idx));
  }

  return (
    <div>
      {projectKey && <ProjectHeader projectKey={projectKey} active="board" />}
      <div className="p-8" data-testid="board-settings">
        <div className="mb-6 flex items-center gap-3">
          <h2 className="text-xl font-bold text-[#1a1f36]">Board settings</h2>
          <Link
            href={`/app/boards/${id}`}
            className="ml-auto text-sm font-medium text-[#0052cc] hover:underline"
          >
            Back to board
          </Link>
        </div>

        {(config.isLoading || wf.isLoading) && (
          <p className="text-sm text-slate-400">Loading board configuration…</p>
        )}

        {config.isError && (
          <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {config.error instanceof Error ? config.error.message : "Failed to load config."}
          </div>
        )}

        {config.data && (
          <div className="space-y-8">
            {/* Columns + status mapping */}
            <section>
              <div className="mb-3 flex items-center gap-3">
                <h3 className="text-base font-semibold text-[#1a1f36]">Columns</h3>
                <button
                  type="button"
                  onClick={addColumn}
                  className="ml-auto rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                >
                  Add column
                </button>
              </div>
              <p className="mb-4 text-sm text-slate-500">
                Each column maps a set of workflow statuses. An issue appears in the column whose set
                contains its status.
              </p>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {columns.map((col, colIdx) => (
                  <div
                    key={colIdx}
                    data-testid={`settings-column-${colIdx}`}
                    className="rounded-xl border border-slate-200 bg-white p-4"
                  >
                    <div className="mb-3 flex items-center gap-2">
                      <label className="sr-only" htmlFor={`col-name-${colIdx}`}>
                        Column name
                      </label>
                      <input
                        id={`col-name-${colIdx}`}
                        aria-label="Column name"
                        value={col.name}
                        onChange={(e) => renameColumn(colIdx, e.target.value)}
                        className="w-full rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm font-semibold"
                      />
                      <button
                        type="button"
                        onClick={() => removeColumn(colIdx)}
                        aria-label={`Remove column ${col.name}`}
                        className="shrink-0 rounded-lg px-2 py-1 text-sm text-red-600 hover:bg-red-50"
                      >
                        Remove
                      </button>
                    </div>
                    <div className="space-y-1.5">
                      {statuses.map((st) => (
                        <label
                          key={st.id}
                          className="flex items-center gap-2 text-sm text-slate-700"
                        >
                          <input
                            type="checkbox"
                            checked={col.statusIds.includes(st.id)}
                            onChange={() => toggleStatus(colIdx, st.id)}
                          />
                          {st.name}
                        </label>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </section>

            {/* Swimlanes */}
            <section>
              <h3 className="mb-3 text-base font-semibold text-[#1a1f36]">Swimlanes</h3>
              <label className="mr-2 text-sm text-slate-700" htmlFor="swimlane-select">
                Group rows by
              </label>
              <select
                id="swimlane-select"
                aria-label="Swimlane"
                value={swimlane}
                onChange={(e) => {
                  setSaved(false);
                  setSwimlane(e.target.value as Swimlane);
                }}
                className="rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm"
              >
                <option value="none">None</option>
                <option value="assignee">Assignee</option>
                <option value="epic">Epic</option>
              </select>
            </section>

            {/* Quick filters */}
            <section>
              <h3 className="mb-3 text-base font-semibold text-[#1a1f36]">Quick filters</h3>
              <ul className="mb-4 space-y-2" data-testid="quick-filter-list">
                {quickFilters.length === 0 && (
                  <li className="text-sm text-slate-400">No quick filters yet.</li>
                )}
                {quickFilters.map((qf, idx) => (
                  <li
                    key={idx}
                    data-testid={`quick-filter-${idx}`}
                    className="flex items-center gap-3 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm"
                  >
                    <span className="font-medium text-[#1a1f36]">{qf.name}</span>
                    <code className="text-slate-500">{qf.jql}</code>
                    <button
                      type="button"
                      onClick={() => removeQuickFilter(idx)}
                      aria-label={`Remove quick filter ${qf.name}`}
                      className="ml-auto rounded px-2 py-1 text-red-600 hover:bg-red-50"
                    >
                      Remove
                    </button>
                  </li>
                ))}
              </ul>
              <div className="flex flex-wrap items-end gap-3">
                <div>
                  <label className="mb-1 block text-xs font-medium text-slate-500" htmlFor="qf-name">
                    Name
                  </label>
                  <input
                    id="qf-name"
                    aria-label="Quick filter name"
                    value={newFilterName}
                    onChange={(e) => setNewFilterName(e.target.value)}
                    className="rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm"
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-slate-500" htmlFor="qf-jql">
                    JQL
                  </label>
                  <input
                    id="qf-jql"
                    aria-label="Quick filter JQL"
                    value={newFilterJql}
                    onChange={(e) => setNewFilterJql(e.target.value)}
                    placeholder="assignee = currentUser()"
                    className="w-72 rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm"
                  />
                </div>
                <button
                  type="button"
                  onClick={addQuickFilter}
                  className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                >
                  Add filter
                </button>
              </div>
            </section>

            <div className="flex items-center gap-3 border-t border-slate-200 pt-5">
              <button
                type="button"
                onClick={() => {
                  if (validationError) return;
                  save.mutate();
                }}
                disabled={save.isPending || validationError !== null}
                className="rounded-lg bg-[#0052cc] px-4 py-2 text-sm font-semibold text-white hover:bg-[#0065ff] disabled:cursor-not-allowed disabled:opacity-60"
              >
                {save.isPending ? "Saving…" : "Save board settings"}
              </button>
              {validationError && (
                <span data-testid="board-settings-validation" className="text-sm text-amber-600">
                  {validationError}
                </span>
              )}
              {saved && !validationError && (
                <span data-testid="board-settings-saved" className="text-sm font-medium text-green-600">
                  Saved
                </span>
              )}
              {save.isError && (
                <span className="text-sm text-red-600">
                  {save.error instanceof Error ? save.error.message : "Save failed"}
                </span>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
