"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { projects as projectsApi, boards as boardsApi, search as searchApi } from "@/lib/api";
import { SearchResults } from "@/components/search/SearchResults";

interface Props {
  projectKey: string;
}

function TabLink({
  href,
  disabled,
  title,
  children,
}: {
  href?: string;
  disabled?: boolean;
  title?: string;
  children: React.ReactNode;
}) {
  const className =
    "pb-2.5 text-sm font-medium border-b-2 border-transparent text-slate-500 hover:text-[#1a1f36] hover:border-slate-300 transition-colors";
  if (disabled || !href) {
    return (
      <span className={`${className} cursor-not-allowed text-slate-300 hover:text-slate-300 hover:border-transparent`} title={title}>
        {children}
      </span>
    );
  }
  return (
    <Link href={href} className={className}>
      {children}
    </Link>
  );
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
  const color = p.projectTypeKey === "software" ? "#0052cc" : "#f97316";
  const letter = (p.key || p.name).charAt(0).toUpperCase();

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="px-8 pt-8 pb-0 flex items-center gap-4">
        {p.avatarUrls?.["48x48"] ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={p.avatarUrls["48x48"]} alt="" className="w-11 h-11 rounded-lg object-cover shrink-0" />
        ) : (
          <div
            className="w-11 h-11 rounded-lg flex items-center justify-center text-white text-lg font-bold shrink-0"
            style={{ background: color }}
          >
            {letter}
          </div>
        )}
        <div>
          <h1 className="text-2xl font-bold text-[#1a1f36] tracking-tight">{p.name}</h1>
          <p className="text-sm text-slate-400 mt-0.5">
            <span className="font-mono">{p.key}</span>
            {" · "}
            {p.projectTypeKey === "software" ? "Software" : "Business"}
            {p.lead && (
              <>
                {" · Lead: "}
                {p.lead.displayName || p.lead.emailAddress}
              </>
            )}
          </p>
        </div>
      </div>

      {/* Section tabs */}
      <div className="px-8 mt-5">
        <nav className="flex gap-6 border-b border-slate-200" data-testid="project-overview-tabs">
          <TabLink href={board ? `/app/boards/${board.id}` : undefined} disabled={!board} title={!board ? "No board for this project yet" : undefined}>
            Board
          </TabLink>
          <TabLink
            href={board ? `/app/boards/${board.id}/backlog` : undefined}
            disabled={!board}
            title={!board ? "No board for this project yet" : undefined}
          >
            Backlog
          </TabLink>
          <TabLink href={`/app/projects/${projectKey}/reports`}>Reports</TabLink>
          <TabLink href={`/app/projects/${projectKey}/settings`}>Settings</TabLink>
        </nav>
      </div>

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

        <h2 className="mb-3 text-sm font-semibold text-[#1a1f36]">Recent issues</h2>
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
