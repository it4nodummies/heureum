"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { projects as projectsApi, Project, ProjectType } from "@/lib/api";
import CreateProjectModal from "./CreateProjectModal";

// ── Types ─────────────────────────────────────────────────────────────────────

type SortKey = "name" | "key" | "type" | "created_at";
type SortDir = "asc" | "desc";

type FilterType = "scrum" | "kanban" | "business";

const TYPE_LABELS: Record<FilterType, string> = {
  scrum: "Scrum software",
  kanban: "Kanban software",
  business: "Business",
};

const TYPE_COLORS: Record<string, string> = {
  scrum: "#0052cc",
  kanban: "#22c55e",
  business: "#f97316",
};

const PAGE_SIZE = 20;

// ── Helpers ───────────────────────────────────────────────────────────────────

function getProjectIcon(p: Project) {
  if (p.icon_url) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={p.icon_url} alt="" className="w-7 h-7 rounded-md object-cover" />;
  }
  const color = TYPE_COLORS[p.type] ?? "#94a3b8";
  const letter = p.name.charAt(0).toUpperCase();
  return (
    <div
      className="w-7 h-7 rounded-md flex items-center justify-center text-white text-xs font-bold shrink-0"
      style={{ background: color }}
    >
      {letter}
    </div>
  );
}

function typeLabel(type: string): string {
  return TYPE_LABELS[type as FilterType] ?? type;
}

// ── Sub-components ────────────────────────────────────────────────────────────

function StarButton({ starred, onToggle }: { starred: boolean; onToggle: () => void }) {
  return (
    <button
      onClick={(e) => { e.stopPropagation(); onToggle(); }}
      title={starred ? "Remove from starred" : "Add to starred"}
      className={`p-1 rounded transition-all ${
        starred
          ? "text-amber-400 hover:text-amber-300"
          : "text-transparent hover:text-slate-300 group-hover:text-slate-300"
      }`}
    >
      <svg viewBox="0 0 20 20" fill={starred ? "currentColor" : "none"} stroke="currentColor" strokeWidth="1.5" className="w-4 h-4">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"
        />
      </svg>
    </button>
  );
}

function ProjectActionsMenu({ project, onArchive }: { project: Project; onArchive: () => void }) {
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
      >
        <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
          <path d="M6 10a2 2 0 11-4 0 2 2 0 014 0zM12 10a2 2 0 11-4 0 2 2 0 014 0zM16 12a2 2 0 100-4 2 2 0 000 4z" />
        </svg>
      </button>
      {open && (
        <div className="absolute right-0 top-8 w-44 bg-white rounded-xl shadow-lg shadow-slate-200/80 border border-slate-100 py-1.5 z-30">
          <button className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-[#42526e] hover:bg-slate-50 transition-colors">
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

function TypeFilterDropdown({
  selected,
  onChange,
}: {
  selected: FilterType[];
  onChange: (v: FilterType[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const ALL: FilterType[] = ["scrum", "kanban", "business"];

  useEffect(() => {
    function handle(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", handle);
    return () => document.removeEventListener("mousedown", handle);
  }, []);

  function toggle(t: FilterType) {
    onChange(selected.includes(t) ? selected.filter((x) => x !== t) : [...selected, t]);
  }

  return (
    <div className="relative" ref={ref}>
      {/* Active tags + dropdown trigger */}
      <div className="flex items-center gap-1.5 flex-wrap">
        {selected.map((t) => (
          <span
            key={t}
            className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium bg-[#e8f0fe] text-[#0052cc] rounded-full"
          >
            {TYPE_LABELS[t]}
            <button
              onClick={() => toggle(t)}
              className="ml-0.5 hover:text-[#0040a8] transition-colors"
            >
              <svg viewBox="0 0 12 12" fill="currentColor" className="w-3 h-3">
                <path d="M9 3L3 9M3 3l6 6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </button>
          </span>
        ))}
        {selected.length > 0 && (
          <button
            onClick={() => onChange([])}
            className="p-1 rounded text-slate-400 hover:text-slate-600 transition-colors"
            title="Clear filters"
          >
            <svg viewBox="0 0 16 16" fill="currentColor" className="w-3.5 h-3.5">
              <path d="M2 2l12 12M2 14L14 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        )}
        <button
          onClick={() => setOpen((v) => !v)}
          className="flex items-center gap-1 px-2.5 py-1 text-xs font-medium text-slate-500 bg-slate-100 hover:bg-slate-200 rounded-full transition-colors"
        >
          {selected.length === 0 ? "Filter by type" : "Edit"}
          <svg viewBox="0 0 12 12" fill="currentColor" className={`w-3 h-3 transition-transform ${open ? "rotate-180" : ""}`}>
            <path d="M2 4l4 4 4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
          </svg>
        </button>
      </div>

      {open && (
        <div className="absolute left-0 top-9 w-52 bg-white rounded-xl shadow-lg shadow-slate-200/80 border border-slate-100 py-1.5 z-30">
          {ALL.map((t) => (
            <button
              key={t}
              onClick={() => toggle(t)}
              className="flex items-center gap-3 w-full px-4 py-2.5 text-sm hover:bg-slate-50 transition-colors"
            >
              <div
                className={`w-4 h-4 rounded border-2 flex items-center justify-center transition-colors ${
                  selected.includes(t)
                    ? "bg-[#0052cc] border-[#0052cc]"
                    : "border-slate-300"
                }`}
              >
                {selected.includes(t) && (
                  <svg viewBox="0 0 10 8" fill="none" className="w-2.5 h-2.5">
                    <path d="M1 4l3 3 5-6" stroke="white" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                )}
              </div>
              <span className="text-[#42526e]">{TYPE_LABELS[t]}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function SortableHeader({
  label,
  sortKey,
  currentKey,
  currentDir,
  onClick,
}: {
  label: string;
  sortKey: SortKey;
  currentKey: SortKey;
  currentDir: SortDir;
  onClick: (k: SortKey) => void;
}) {
  const active = currentKey === sortKey;
  return (
    <button
      onClick={() => onClick(sortKey)}
      className={`flex items-center gap-1 text-xs font-semibold uppercase tracking-wider transition-colors ${
        active ? "text-[#0052cc]" : "text-slate-500 hover:text-slate-700"
      }`}
    >
      {label}
      <svg viewBox="0 0 16 16" fill="currentColor" className={`w-3.5 h-3.5 transition-all ${active ? "opacity-100" : "opacity-30"}`}>
        {active && currentDir === "asc" ? (
          <path d="M8 4l-4 6h8L8 4z" />
        ) : (
          <path d="M8 12l4-6H4l4 6z" />
        )}
      </svg>
    </button>
  );
}

// ── Main component ─────────────────────────────────────────────────────────────

export default function ProjectsPage() {
  const [items, setItems] = useState<Project[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [filterTypes, setFilterTypes] = useState<FilterType[]>([]);
  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortDir, setSortDir] = useState<SortDir>("asc");
  const [page, setPage] = useState(0); // 0-indexed

  const [createOpen, setCreateOpen] = useState(false);

  // Debounce search
  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(t);
  }, [search]);

  // Reset page on filter change
  useEffect(() => { setPage(0); }, [debouncedSearch, filterTypes, sortKey, sortDir]);

  const fetchProjects = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const res = await projectsApi.list({
        query: debouncedSearch || undefined,
        type: filterTypes.length ? filterTypes.join(",") : undefined,
        orderBy: sortKey,
        direction: sortDir,
        startAt: page * PAGE_SIZE,
        maxResults: PAGE_SIZE,
      });
      setItems(res.values ?? []);
      setTotal(res.total ?? 0);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to load projects");
    } finally {
      setLoading(false);
    }
  }, [debouncedSearch, filterTypes, sortKey, sortDir, page]);

  useEffect(() => { fetchProjects(); }, [fetchProjects]);

  function handleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  }

  async function toggleStar(project: Project) {
    const newStarred = !project.is_starred;
    setItems((prev) =>
      prev.map((p) => (p.id === project.id ? { ...p, is_starred: newStarred } : p))
    );
    try {
      if (newStarred) await projectsApi.star(project.key);
      else await projectsApi.unstar(project.key);
    } catch {
      // revert on failure
      setItems((prev) =>
        prev.map((p) => (p.id === project.id ? { ...p, is_starred: !newStarred } : p))
      );
    }
  }

  async function handleArchive(project: Project) {
    if (!confirm(`Archive "${project.name}"? It will no longer appear in the list.`)) return;
    try {
      await projectsApi.archive(project.key);
      fetchProjects();
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : "Failed to archive project");
    }
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);

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

        {/* Type filter */}
        <TypeFilterDropdown selected={filterTypes} onChange={setFilterTypes} />

        {/* Result count */}
        {!loading && (
          <span className="ml-auto text-xs text-slate-400">
            {total} {total === 1 ? "project" : "projects"}
          </span>
        )}
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto px-8 pb-6">
        {error && (
          <div className="mb-4 p-4 bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl">
            {error}
          </div>
        )}

        <div className="bg-white border border-slate-100 rounded-2xl overflow-hidden shadow-sm shadow-slate-100/80">
          {/* Table header */}
          <div className="grid grid-cols-[auto_1fr_140px_160px_1fr_48px] items-center px-4 py-3 bg-slate-50/80 border-b border-slate-100">
            <div className="w-7" /> {/* star col */}
            <SortableHeader label="Name" sortKey="name" currentKey={sortKey} currentDir={sortDir} onClick={handleSort} />
            <SortableHeader label="Key" sortKey="key" currentKey={sortKey} currentDir={sortDir} onClick={handleSort} />
            <SortableHeader label="Type" sortKey="type" currentKey={sortKey} currentDir={sortDir} onClick={handleSort} />
            <div className="text-xs font-semibold uppercase tracking-wider text-slate-500">Lead</div>
            <div /> {/* actions col */}
          </div>

          {/* Rows */}
          {loading ? (
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
                  {search || filterTypes.length
                    ? "Try adjusting your filters"
                    : "Create your first project to get started"}
                </p>
              </div>
              {!search && !filterTypes.length && (
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
                className={`grid grid-cols-[auto_1fr_140px_160px_1fr_48px] items-center px-4 py-3 group cursor-pointer hover:bg-[#f8faff] transition-colors ${
                  idx !== 0 ? "border-t border-slate-50" : ""
                }`}
                onClick={() => window.open(`/jira/project/${project.key}`, "_self")}
              >
                {/* Star */}
                <div className="w-7">
                  <StarButton
                    starred={project.is_starred}
                    onToggle={() => toggleStar(project)}
                  />
                </div>

                {/* Name */}
                <div className="flex items-center gap-3 min-w-0 pr-4">
                  {getProjectIcon(project)}
                  <span className="text-sm font-semibold text-[#0052cc] hover:underline truncate">
                    {project.name}
                  </span>
                </div>

                {/* Key */}
                <div className="text-sm text-slate-500 font-mono">{project.key}</div>

                {/* Type */}
                <div>
                  <span className="text-sm text-slate-600">{typeLabel(project.type)}</span>
                </div>

                {/* Lead */}
                <div className="flex items-center gap-2 min-w-0">
                  {project.lead ? (
                    <>
                      {project.lead.avatar_url ? (
                        // eslint-disable-next-line @next/next/no-img-element
                        <img
                          src={project.lead.avatar_url}
                          alt=""
                          className="w-6 h-6 rounded-full object-cover shrink-0"
                        />
                      ) : (
                        <div className="w-6 h-6 rounded-full bg-gradient-to-br from-[#0052cc] to-[#7c3aed] flex items-center justify-center text-white text-[10px] font-bold shrink-0">
                          {project.lead.display_name?.charAt(0).toUpperCase() ||
                            project.lead.email?.charAt(0).toUpperCase()}
                        </div>
                      )}
                      <span className="text-sm text-slate-600 truncate">
                        {project.lead.display_name || project.lead.email}
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

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-center gap-2 mt-6">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className="p-2 rounded-lg border border-slate-200 text-slate-500 hover:bg-slate-100 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
                <path fillRule="evenodd" d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z" clipRule="evenodd" />
              </svg>
            </button>

            {Array.from({ length: totalPages }, (_, i) => (
              <button
                key={i}
                onClick={() => setPage(i)}
                className={`w-9 h-9 rounded-lg text-sm font-medium transition-colors ${
                  i === page
                    ? "bg-[#0052cc] text-white shadow-sm"
                    : "border border-slate-200 text-slate-600 hover:bg-slate-100"
                }`}
              >
                {i + 1}
              </button>
            ))}

            <button
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
              className="p-2 rounded-lg border border-slate-200 text-slate-500 hover:bg-slate-100 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
                <path fillRule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clipRule="evenodd" />
              </svg>
            </button>
          </div>
        )}
      </div>

      {createOpen && (
        <CreateProjectModal
          onClose={() => setCreateOpen(false)}
          onCreated={() => { setCreateOpen(false); fetchProjects(); }}
        />
      )}
    </div>
  );
}
