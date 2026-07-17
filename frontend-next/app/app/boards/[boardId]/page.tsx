"use client";

import { use, useEffect, useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { boards, issues as issuesApi, type SearchIssue } from "@/lib/api";
import { BoardColumns } from "@/components/board/BoardColumns";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

export default function BoardPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);
  const qc = useQueryClient();
  const [moveError, setMoveError] = useState<string | null>(null);

  useEffect(() => {
    setMoveError(null);
  }, [id]);

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });
  const config = useQuery({ queryKey: ["board", id, "config"], queryFn: () => boards.configuration(id) });
  const issueList = useQuery({ queryKey: ["board", id, "issues"], queryFn: () => boards.issues(id) });
  const projectKey = board.data?.location?.projectKey;

  const columns = useMemo(
    () => (config.data?.columnConfig.columns ?? []).map((c) => ({ id: c.statuses[0]?.id ?? c.name, name: c.name })),
    [config.data],
  );

  const issuesByStatus = useMemo(() => {
    const map: Record<string, SearchIssue[]> = {};
    for (const iss of issueList.data?.issues ?? []) {
      const sid = iss.fields.status?.name ?? "";
      // la colonna è per status id; mappiamo per id via config
      const col = (config.data?.columnConfig.columns ?? []).find((c) => c.name === iss.fields.status?.name);
      const key = col?.statuses[0]?.id ?? sid;
      (map[key] ??= []).push(iss);
    }
    return map;
  }, [issueList.data, config.data]);

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

  return (
    <div>
      {projectKey && <ProjectHeader projectKey={projectKey} active="board" />}
      <div className="p-4">
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
            issuesByStatus={issuesByStatus}
            onMove={(issueKey, toStatusId) => move.mutate({ issueKey, toStatusId })}
          />
        )}
      </div>
    </div>
  );
}
