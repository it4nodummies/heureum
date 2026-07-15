"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projects as projectsApi, boards as boardsApi } from "@/lib/api";
import { ProjectAvatar } from "@/components/projects/ProjectAvatar";
import { CreateIssueModal } from "@/components/issues/CreateIssueModal";

type ActiveTab = "overview" | "board" | "backlog" | "reports" | "settings";

interface Props {
  projectKey: string;
  active: ActiveTab;
}

function TabLink({
  href,
  disabled,
  title,
  active,
  children,
}: {
  href?: string;
  disabled?: boolean;
  title?: string;
  active?: boolean;
  children: React.ReactNode;
}) {
  if (disabled || !href) {
    return (
      <span
        className="pb-2.5 text-sm font-medium border-b-2 border-transparent text-slate-300 cursor-not-allowed"
        title={title}
      >
        {children}
      </span>
    );
  }
  const className = active
    ? "pb-2.5 text-sm font-semibold border-b-2 border-[#0052cc] text-[#0052cc] transition-colors"
    : "pb-2.5 text-sm font-medium border-b-2 border-transparent text-slate-500 hover:text-[#1a1f36] hover:border-slate-300 transition-colors";
  return (
    <Link href={href} className={className} aria-current={active ? "page" : undefined}>
      {children}
    </Link>
  );
}

/**
 * Shared header rendered on every project section page (overview, board,
 * backlog, reports, settings): avatar + name + "KEY · Type" + section tabs +
 * Create issue button. Resolves the project's board once here so the
 * Board/Backlog tab hrefs (and their "no board yet" disabled state) don't
 * need to be re-derived on every page.
 */
export function ProjectHeader({ projectKey, active }: Props) {
  const qc = useQueryClient();
  const [createIssueOpen, setCreateIssueOpen] = useState(false);

  const project = useQuery({
    queryKey: ["project", projectKey],
    queryFn: () => projectsApi.get(projectKey),
  });

  const boardsList = useQuery({
    queryKey: ["boards"],
    queryFn: () => boardsApi.list(),
  });
  const board = boardsList.data?.values.find((b) => b.location?.projectKey === projectKey);

  if (project.isLoading) {
    return (
      <div className="px-8 pt-8 pb-5 flex items-center gap-3">
        <div className="w-6 h-6 rounded-full border-2 border-[#0052cc] border-t-transparent animate-spin" />
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
    <div>
      <div className="px-8 pt-8 pb-0 flex items-center gap-4">
        <ProjectAvatar nameOrKey={p.key} size={44} />
        <div>
          <h1 className="text-2xl font-bold text-[#1a1f36] tracking-tight">
            <Link href={`/app/projects/${p.key}`} className="hover:underline">
              {p.name}
            </Link>
          </h1>
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
      </div>

      <div className="px-8 mt-5">
        <nav className="flex gap-6 border-b border-slate-200" data-testid="project-header-tabs">
          <TabLink
            href={board ? `/app/boards/${board.id}` : undefined}
            disabled={!board}
            title={!board ? "No board yet" : undefined}
            active={active === "board"}
          >
            Board
          </TabLink>
          <TabLink
            href={board ? `/app/boards/${board.id}/backlog` : undefined}
            disabled={!board}
            title={!board ? "No board yet" : undefined}
            active={active === "backlog"}
          >
            Backlog
          </TabLink>
          <TabLink href={`/app/projects/${projectKey}/reports`} active={active === "reports"}>
            Reports
          </TabLink>
          <TabLink href={`/app/projects/${projectKey}/settings`} active={active === "settings"}>
            Settings
          </TabLink>
        </nav>
      </div>

      {createIssueOpen && (
        <CreateIssueModal
          projectKey={projectKey}
          onClose={() => setCreateIssueOpen(false)}
          onCreated={() => {
            qc.invalidateQueries({ queryKey: ["project", projectKey, "recent-issues"] });
            if (board) {
              qc.invalidateQueries({ queryKey: ["board", board.id, "issues"] });
              qc.invalidateQueries({ queryKey: ["board", board.id, "backlog"] });
            }
          }}
        />
      )}
    </div>
  );
}
