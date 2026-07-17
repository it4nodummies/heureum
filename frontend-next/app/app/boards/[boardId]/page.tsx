"use client";

import { use, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { boards, issues as issuesApi, search } from "@/lib/api";
import { BoardColumns } from "@/components/board/BoardColumns";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

export default function BoardPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);
  const qc = useQueryClient();
  const [moveError, setMoveError] = useState<string | null>(null);
  // Quick-filter attivi (per id). Più filtri attivi → INTERSEZIONE delle chiavi.
  const [activeFilters, setActiveFilters] = useState<string[]>([]);

  useEffect(() => {
    setMoveError(null);
    setActiveFilters([]);
  }, [id]);

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });
  // `configuration` (contract Agile) è la fonte UNICA delle colonne: riflette sia
  // il fallback 1:1 (board senza config persistita, come la board DEMO) sia le
  // colonne persistite multi-status. `config` (endpoint Heureum-custom) porta solo
  // swimlane + quick filters.
  const configuration = useQuery({
    queryKey: ["board", id, "configuration"],
    queryFn: () => boards.configuration(id),
  });
  const config = useQuery({ queryKey: ["board", id, "config"], queryFn: () => boards.config(id) });
  const issueList = useQuery({ queryKey: ["board", id, "issues"], queryFn: () => boards.issues(id) });
  const projectKey = board.data?.location?.projectKey;

  // Colonne dal contract configuration: ogni colonna è un SET di status. `id` =
  // status primario (statuses[0]) = target di transizione al drop.
  const columns = useMemo(
    () =>
      (configuration.data?.columnConfig.columns ?? []).map((c) => ({
        id: c.statuses[0]?.id ?? c.name,
        name: c.name,
        statusIds: c.statuses.map((s) => s.id),
      })),
    [configuration.data],
  );

  const swimlane = config.data?.swimlane ?? "none";
  const quickFilters = useMemo(() => config.data?.quickFilters ?? [], [config.data]);

  // JQL dei filtri attivi. Nota: intersezione multi-filtro (vedi sotto).
  const activeJqls = useMemo(
    () => quickFilters.filter((q) => activeFilters.includes(q.id)).map((q) => q.jql),
    [quickFilters, activeFilters],
  );

  // Valuta i quick filter attivi via search.jql e restituisce l'INTERSEZIONE
  // delle chiavi issue che li soddisfano tutti. `null` = nessun filtro attivo.
  const filterKeys = useQuery({
    queryKey: ["board", id, "quickfilter-keys", activeJqls],
    enabled: activeJqls.length > 0,
    queryFn: async () => {
      const keySets: Set<string>[] = [];
      for (const jql of activeJqls) {
        const res = await search.jql(jql, { fields: ["summary"], maxResults: 200 });
        keySets.push(new Set<string>(res.issues.map((i) => i.key)));
      }
      if (keySets.length === 0) return new Set<string>();
      // Intersezione: una chiave passa solo se presente in TUTTI i filtri attivi.
      const [first, ...rest] = keySets;
      return new Set<string>(
        Array.from(first).filter((k) => rest.every((s) => s.has(k))),
      );
    },
  });

  // Issue visibili dopo l'applicazione dei quick filter. Mentre la query dei
  // filtri è in volo mostriamo tutto (nessun flash a vuoto).
  const visibleIssues = useMemo(() => {
    const all = issueList.data?.issues ?? [];
    if (activeJqls.length === 0) return all;
    const allowed = filterKeys.data;
    if (!allowed) return all;
    return all.filter((iss) => allowed.has(iss.key));
  }, [issueList.data, activeJqls, filterKeys.data]);

  // Lo status non è un campo "fields" libero su PUT /rest/api/3/issue/{key}
  // (vedi lib/api.ts issues.update): il backend richiede una transizione
  // validata dal workflow, esposta come POST /rest/api/3/issue/{key}/transitions.
  const move = useMutation({
    mutationFn: ({ issueKey, toStatusId }: { issueKey: string; toStatusId: string }) =>
      issuesApi.transition(issueKey, toStatusId),
    onSuccess: () => {
      setMoveError(null);
      qc.invalidateQueries({ queryKey: ["board", id, "issues"] });
    },
    onError: (err: unknown) => {
      setMoveError(err instanceof Error ? err.message : "Move failed");
    },
  });

  function toggleFilter(filterId: string) {
    setActiveFilters((cur) =>
      cur.includes(filterId) ? cur.filter((f) => f !== filterId) : [...cur, filterId],
    );
  }

  return (
    <div>
      {projectKey && <ProjectHeader projectKey={projectKey} active="board" />}
      <div className="p-4">
        <div className="mb-3 flex items-center gap-2">
          {quickFilters.length > 0 && (
            <div className="flex flex-wrap items-center gap-2">
              {quickFilters.map((qf) => {
                const active = activeFilters.includes(qf.id);
                return (
                  <button
                    key={qf.id}
                    type="button"
                    onClick={() => toggleFilter(qf.id)}
                    aria-pressed={active}
                    data-testid={`quickfilter-${qf.name}`}
                    className={`rounded-full border px-3 py-1 text-sm font-medium ${
                      active
                        ? "border-[#0052cc] bg-[#0052cc] text-white"
                        : "border-slate-300 text-slate-700 hover:bg-slate-50"
                    }`}
                  >
                    {qf.name}
                  </button>
                );
              })}
            </div>
          )}
          <Link
            href={`/app/boards/${id}/settings`}
            data-testid="board-settings-link"
            className="ml-auto rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            Board settings
          </Link>
        </div>
        {moveError && (
          <div
            role="alert"
            data-testid="move-error"
            className="mb-2 flex items-center gap-2 rounded border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700"
          >
            <span>Move failed: {moveError}</span>
            <button
              onClick={() => setMoveError(null)}
              aria-label="Dismiss error"
              className="ml-auto text-red-700 hover:underline"
            >
              ×
            </button>
          </div>
        )}
        {columns.length > 0 && (
          <BoardColumns
            columns={columns}
            issues={visibleIssues}
            swimlane={swimlane}
            onMove={(issueKey, toStatusId) => move.mutate({ issueKey, toStatusId })}
          />
        )}
      </div>
    </div>
  );
}
