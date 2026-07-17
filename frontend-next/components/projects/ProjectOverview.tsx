"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { projects as projectsApi, boards as boardsApi, search as searchApi, issues } from "@/lib/api";
import { SearchResults } from "@/components/search/SearchResults";
import { ProjectHeader } from "@/components/projects/ProjectHeader";

interface Props {
  projectKey: string;
}

export function ProjectOverview({ projectKey }: Props) {
  const qc = useQueryClient();

  const project = useQuery({
    queryKey: ["project", projectKey],
    queryFn: () => projectsApi.get(projectKey),
  });

  const boardsList = useQuery({
    queryKey: ["boards"],
    queryFn: () => boardsApi.list(),
  });
  const board = boardsList.data?.values.find((b) => b.location?.projectKey === projectKey);

  const [boardName, setBoardName] = useState<string | null>(null);
  const createBoard = useMutation({
    mutationFn: () => boardsApi.create(boardName || `${project.data?.name ?? projectKey} board`, projectKey, "scrum"),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["boards"] }),
  });

  const recentIssues = useQuery({
    queryKey: ["project", projectKey, "recent-issues"],
    queryFn: () => searchApi.jql(`project = ${projectKey} ORDER BY updated DESC`, { maxResults: 15 }),
  });

  if (project.isLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3">
        <div className="w-7 h-7 rounded-full border-2 border-[#0052cc] border-t-transparent animate-spin" />
        <span className="text-sm text-slate-400">Loading project…</span>
      </div>
    );
  }

  if (project.isError || !project.data) {
    return (
      <div className="px-8 py-8">
        <div className="p-4 bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl">
          {project.error instanceof Error ? project.error.message : "Project not found."}
        </div>
      </div>
    );
  }

  const p = project.data;

  return (
    <div className="h-full flex flex-col">
      <ProjectHeader projectKey={projectKey} active="overview" />

      {/* Body */}
      <div className="flex-1 overflow-auto px-8 py-6">
        {!boardsList.isLoading && !board && (
          <div className="mb-5 p-4 bg-slate-50 border border-slate-100 rounded-xl">
            <p className="text-sm text-slate-500 mb-3">
              This project doesn&apos;t have a board yet — Board and Backlog will unlock once one is created.
            </p>
            <div className="flex items-center gap-2">
              <input
                aria-label="Board name"
                value={boardName ?? `${p.name} board`}
                onChange={(e) => setBoardName(e.target.value)}
                className="flex-1 max-w-xs rounded border border-slate-300 px-3 py-1.5 text-sm"
              />
              <button
                onClick={() => createBoard.mutate()}
                disabled={createBoard.isPending}
                className="rounded bg-[#0052cc] px-4 py-1.5 text-sm font-medium text-white disabled:opacity-60"
              >
                {createBoard.isPending ? "Creating…" : "Create board"}
              </button>
            </div>
            {createBoard.isError && (
              <p className="mt-2 text-xs text-red-600">
                {createBoard.error instanceof Error ? createBoard.error.message : "Failed to create board."}
              </p>
            )}
          </div>
        )}

        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-[#1a1f36]">Recent issues</h2>
          <button
            onClick={() => issues.exportCsv(projectKey)}
            className="rounded border border-slate-300 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50"
          >
            Export CSV
          </button>
        </div>
        <div className="bg-white border border-slate-100 rounded-2xl shadow-sm shadow-slate-100/80 p-4">
          {recentIssues.isLoading ? (
            <div className="flex items-center justify-center py-10">
              <div className="w-6 h-6 rounded-full border-2 border-[#0052cc] border-t-transparent animate-spin" />
            </div>
          ) : (
            <SearchResults issues={recentIssues.data?.issues ?? []} />
          )}
        </div>
      </div>
    </div>
  );
}
