"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projects as projectsApi, Project } from "@/lib/api";
import CreateProjectModal from "./CreateProjectModal";

// ── Helpers ───────────────────────────────────────────────────────────────────

function getProjectIcon(p: Project) {
  const src = p.avatarUrls?.["24x24"];
  if (src) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt="" className="w-7 h-7 rounded-md object-cover" />;
  }
  const color = p.projectTypeKey === "software" ? "#0052cc" : "#f97316";
  const letter = (p.key || p.name).charAt(0).toUpperCase();
  return (
    <div
      className="w-7 h-7 rounded-md flex items-center justify-center text-white text-xs font-bold shrink-0"
      style={{ background: color }}
    >
      {letter}
    </div>
  );
}

function typeLabel(projectTypeKey: Project["projectTypeKey"]): string {
  return projectTypeKey === "software" ? "Software" : "Business";
}

// ── Sub-components ────────────────────────────────────────────────────────────

function ProjectActionsMenu({ project, onArchive }: { project: Project; onArchive: () => void }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handle(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", handle);
    return () => document.removeEventListener("mousedown", handle);
  }, []);

  return (
    <div className="relative" ref={ref} onClick={(e) => e.stopPropagation()}>
      <button
        onClick={() => setOpen((v) => !v)}
        className="p-1.5 rounded-lg text-slate-300 hover:text-slate-500 hover:bg-slate-100 transition-colors opacity-0 group-hover:opacity-100"
        title="Actions"
        aria-label="Project actions"
      >
        <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
          <path d="M6 10a2 2 0 11-4 0 2 2 0 014 0zM12 10a2 2 0 11-4 0 2 2 0 014 0zM16 12a2 2 0 100-4 2 2 0 000 4z" />
        </svg>
      </button>
      {open && (
        <div className="absolute right-0 top-8 w-44 bg-white rounded-xl shadow-lg shadow-slate-200/80 border border-slate-100 py-1.5 z-30">
          <button
            onClick={() => { setOpen(false); router.push(`/app/projects/${project.key}/settings`); }}
            className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-[#42526e] hover:bg-slate-50 transition-colors"
          >
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4 text-slate-400">
              <path fillRule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clipRule="evenodd" />
            </svg>
            Project settings
          </button>
          <div className="border-t border-slate-100 mt-1 pt-1">
            <button
              onClick={() => { setOpen(false); onArchive(); }}
              className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-red-500 hover:bg-red-50 transition-colors"
            >
              <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
                <path d="M4 3a2 2 0 100 4h12a2 2 0 100-4H4z" />
                <path fillRule="evenodd" d="M3 8h14v7a2 2 0 01-2 2H5a2 2 0 01-2-2V8zm5 3a1 1 0 011-1h2a1 1 0 110 2H9a1 1 0 01-1-1z" clipRule="evenodd" />
              </svg>
              Archive
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Main component ─────────────────────────────────────────────────────────────

export default function ProjectsPage() {
  const queryClient = useQueryClient();

  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);

  // Debounce search
  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(t);
  }, [search]);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["projects", debouncedSearch],
    queryFn: () => projectsApi.search({ query: debouncedSearch || undefined, maxResults: 50 }),
  });

  const items = data?.values ?? [];
  const total = data?.total ?? 0;

  async function handleArchive(project: Project) {
    if (!confirm(`Archive "${project.name}"? It will no longer appear in the list.`)) return;
    try {
      await projectsApi.archive(project.key);
      queryClient.invalidateQueries({ queryKey: ["projects"] });
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : "Failed to archive project");
    }
  }

  return (
    <div className="h-full flex flex-col">
      {/* Page header */}
      <div className="px-8 pt-8 pb-0 flex items-start justify-between">
        <h1 className="text-2xl font-bold text-[#1a1f36] tracking-tight">Projects</h1>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setCreateOpen(true)}
            className="flex items-center gap-1.5 px-4 py-2 bg-[#0052cc] hover:bg-[#0065ff] text-white text-sm font-semibold rounded-lg transition-colors shadow-sm shadow-blue-200"
          >
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
              <path fillRule="evenodd" d="M10 5a1 1 0 011 1v3h3a1 1 0 110 2h-3v3a1 1 0 11-2 0v-3H6a1 1 0 110-2h3V6a1 1 0 011-1z" clipRule="evenodd" />
            </svg>
            Create project
          </button>
          <button className="px-4 py-2 border border-slate-200 text-sm font-medium text-slate-600 hover:bg-slate-50 rounded-lg transition-colors">
            Templates
          </button>
        </div>
      </div>

      {/* Filters row */}
      <div className="px-8 py-5 flex items-center gap-4 flex-wrap">
        {/* Search */}
        <div className="relative">
          <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400">
            <path fillRule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clipRule="evenodd" />
          </svg>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search projects…"
            className="pl-9 pr-4 py-2 text-sm border border-slate-200 rounded-lg w-56 focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] focus:w-72 transition-all placeholder:text-slate-400 bg-white"
          />
          {search && (
            <button
              onClick={() => setSearch("")}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600"
            >
              <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
                <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
              </svg>
            </button>
          )}
        </div>

        {/* Result count */}
        {!isLoading && (
          <span className="ml-auto text-xs text-slate-400">
            {total} {total === 1 ? "project" : "projects"}
          </span>
        )}
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto px-8 pb-6">
        {isError && (
          <div className="mb-4 p-4 bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl">
            {error instanceof Error ? error.message : "Failed to load projects"}
          </div>
        )}

        <div className="bg-white border border-slate-100 rounded-2xl overflow-hidden shadow-sm shadow-slate-100/80">
          {/* Table header */}
          <div className="grid grid-cols-[1fr_140px_160px_1fr_48px] items-center px-4 py-3 bg-slate-50/80 border-b border-slate-100">
            <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Name</div>
            <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Key</div>
            <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Type</div>
            <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Lead</div>
            <div /> {/* actions col */}
          </div>

          {/* Rows */}
          {isLoading ? (
            <div className="flex flex-col items-center justify-center py-20 gap-3">
              <div className="w-7 h-7 rounded-full border-2 border-[#0052cc] border-t-transparent animate-spin" />
              <span className="text-sm text-slate-400">Loading projects…</span>
            </div>
          ) : items.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 gap-3">
              <div className="w-14 h-14 rounded-2xl bg-slate-100 flex items-center justify-center">
                <svg viewBox="0 0 24 24" fill="none" className="w-7 h-7 text-slate-400">
                  <path d="M3 7a2 2 0 012-2h14a2 2 0 012 2v10a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" stroke="currentColor" strokeWidth="1.5" />
                  <path d="M8 12h8M8 9h4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                </svg>
              </div>
              <div className="text-center">
                <p className="text-sm font-semibold text-slate-600">No projects found</p>
                <p className="text-xs text-slate-400 mt-0.5">
                  {search ? "Try adjusting your search" : "Create your first project to get started"}
                </p>
              </div>
              {!search && (
                <button
                  onClick={() => setCreateOpen(true)}
                  className="mt-1 px-4 py-2 bg-[#0052cc] hover:bg-[#0065ff] text-white text-sm font-semibold rounded-lg transition-colors"
                >
                  Create project
                </button>
              )}
            </div>
          ) : (
            items.map((project, idx) => (
              <div
                key={project.id}
                className={`grid grid-cols-[1fr_140px_160px_1fr_48px] items-center px-4 py-3 group cursor-pointer hover:bg-[#f8faff] transition-colors ${
                  idx !== 0 ? "border-t border-slate-50" : ""
                }`}
              >
                {/* Name */}
                <div className="flex items-center gap-3 min-w-0 pr-4">
                  {getProjectIcon(project)}
                  <Link
                    href={`/app/projects/${project.key}`}
                    className="text-sm font-semibold text-[#0052cc] hover:underline truncate"
                  >
                    {project.name}
                  </Link>
                </div>

                {/* Key */}
                <div className="text-sm text-slate-500 font-mono">{project.key}</div>

                {/* Type */}
                <div>
                  <span className="text-sm text-slate-600">{typeLabel(project.projectTypeKey)}</span>
                </div>

                {/* Lead */}
                <div className="flex items-center gap-2 min-w-0">
                  {project.lead ? (
                    <>
                      {project.lead.avatarUrls?.["24x24"] ? (
                        // eslint-disable-next-line @next/next/no-img-element
                        <img
                          src={project.lead.avatarUrls["24x24"]}
                          alt=""
                          className="w-6 h-6 rounded-full object-cover shrink-0"
                        />
                      ) : (
                        <div className="w-6 h-6 rounded-full bg-gradient-to-br from-[#0052cc] to-[#7c3aed] flex items-center justify-center text-white text-[10px] font-bold shrink-0">
                          {project.lead.displayName?.charAt(0).toUpperCase() ||
                            project.lead.emailAddress?.charAt(0).toUpperCase()}
                        </div>
                      )}
                      <span className="text-sm text-slate-600 truncate">
                        {project.lead.displayName || project.lead.emailAddress}
                      </span>
                    </>
                  ) : (
                    <span className="text-sm text-slate-400">—</span>
                  )}
                </div>

                {/* Actions */}
                <div className="flex justify-end">
                  <ProjectActionsMenu
                    project={project}
                    onArchive={() => handleArchive(project)}
                  />
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {createOpen && (
        <CreateProjectModal
          onClose={() => setCreateOpen(false)}
          onCreated={() => setCreateOpen(false)}
        />
      )}
    </div>
  );
}
