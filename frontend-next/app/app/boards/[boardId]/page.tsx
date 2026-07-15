"use client";

import { use, useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { boards, issues as issuesApi, type SearchIssue } from "@/lib/api";
import { BoardColumns } from "@/components/board/BoardColumns";
import { CreateIssueModal } from "@/components/issues/CreateIssueModal";

export default function BoardPage({ params }: { params: Promise<{ boardId: string }> }) {
  const { boardId } = use(params);
  const id = Number(boardId);
  const qc = useQueryClient();

  const board = useQuery({ queryKey: ["board", id], queryFn: () => boards.get(id) });
  const config = useQuery({ queryKey: ["board", id, "config"], queryFn: () => boards.configuration(id) });
  const issueList = useQuery({ queryKey: ["board", id, "issues"], queryFn: () => boards.issues(id) });
  const [createIssueOpen, setCreateIssueOpen] = useState(false);
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
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board", id, "issues"] }),
  });

  return (
    <div className="p-4">
      <div className="mb-3 flex items-center gap-3">
        <h1 className="text-xl font-semibold text-[#1a1f36]">{board.data?.name ?? "Board"}</h1>
        <a href={`/app/boards/${id}/backlog`} className="text-sm text-[#0052cc] hover:underline">
          Backlog
        </a>
        {projectKey && (
          <button
            onClick={() => setCreateIssueOpen(true)}
            className="ml-auto flex items-center gap-1.5 px-3.5 py-1.5 bg-[#0052cc] hover:bg-[#0065ff] text-white text-sm font-semibold rounded-lg transition-colors"
          >
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
              <path
                fillRule="evenodd"
                d="M10 5a1 1 0 011 1v3h3a1 1 0 110 2h-3v3a1 1 0 11-2 0v-3H6a1 1 0 110-2h3V6a1 1 0 011-1z"
                clipRule="evenodd"
              />
            </svg>
            Create issue
          </button>
        )}
      </div>
      {columns.length > 0 && (
        <BoardColumns
          columns={columns}
          issuesByStatus={issuesByStatus}
          onMove={(issueKey, toStatusId) => move.mutate({ issueKey, toStatusId })}
        />
      )}

      {createIssueOpen && projectKey && (
        <CreateIssueModal
          projectKey={projectKey}
          onClose={() => setCreateIssueOpen(false)}
          onCreated={() => {
            qc.invalidateQueries({ queryKey: ["board", id, "issues"] });
          }}
        />
      )}
    </div>
  );
}
