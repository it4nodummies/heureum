"use client";

// Combobox to pick an issue's parent WITHIN THE SAME PROJECT at the right
// hierarchy level: a Story/Task/Bug (childLevel 0) picks an Epic (level 1), a
// Subtask (childLevel -1) picks a Story/Task/Bug (level 0). Jira's strict
// hierarchy is enforced server-side too (400 errors.parent) — this picker just
// never offers an invalid candidate.
//
// Search goes through POST /rest/api/3/search/jql scoped to the project
// (`project = KEY AND summary ~ "…"`, or `key = KEY` when the text looks like
// an issue key). JQL has no hierarchyLevel field: when the parent is an epic
// the query also narrows on `issuetype = Epic` (otherwise, in a project with
// more than one page of issues, the default list could miss the epics), and
// candidates are ALWAYS filtered client-side on
// `issuetype.hierarchyLevel === childLevel + 1` as a safety net (with the
// name/subtask fallback of hierarchyLevelOf for older payloads).

import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { hierarchyLevelOf, search, type SearchIssue } from "@/lib/api";

export interface ParentOption {
  key: string;
  summary: string;
}

interface Props {
  projectKey: string;
  // Hierarchy level of the CHILD whose parent is being picked (0 or -1).
  childLevel: number;
  value: ParentOption | null;
  onChange: (parent: ParentOption | null) => void;
  // The issue being edited (never offered as its own parent).
  excludeKey?: string;
  disabled?: boolean;
  label?: string;
  placeholder?: string;
  // Whether to render the "clear" action next to the trigger (the issue
  // detail uses it; the create modal clears via the picker's own "None").
  clearable?: boolean;
  inputTestId?: string;
  clearTestId?: string;
  // Open the dropdown on mount (the issue detail mounts the picker only while
  // editing, so the search box should be ready at once).
  autoOpen?: boolean;
  // Called when the dropdown is dismissed without a choice (outside click /
  // Escape) — lets a host leave its editing state.
  onCancel?: () => void;
}

const KEY_RE = /^[A-Z][A-Z0-9_]*-\d+$/i;

// Builds the project-scoped JQL for a free-text query. parentLevel 1 (an
// epic is being picked) adds `issuetype = Epic` so the server-side page holds
// only epics; level 0 has no JQL-expressible type set, so it stays open.
export function parentSearchJql(projectKey: string, query: string, parentLevel: number): string {
  const q = query.trim().replace(/["\\]/g, "");
  const scope = parentLevel === 1 ? `project = ${projectKey} AND issuetype = Epic` : `project = ${projectKey}`;
  if (!q) return `${scope} ORDER BY updated DESC`;
  if (KEY_RE.test(q)) return `${scope} AND key = "${q.toUpperCase()}"`;
  return `${scope} AND summary ~ "${q}" ORDER BY updated DESC`;
}

export function ParentPicker({
  projectKey,
  childLevel,
  value,
  onChange,
  excludeKey,
  disabled,
  label,
  placeholder,
  clearable = true,
  inputTestId = "parent-picker-input",
  clearTestId = "parent-clear",
  autoOpen = false,
  onCancel,
}: Props) {
  const parentLevel = childLevel + 1;
  const noun = parentLevel === 1 ? "epic" : "parent issue";
  const [open, setOpen] = useState(autoOpen);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  useEffect(() => {
    if (!open) return;
    function onDocClick(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
        setQuery("");
        onCancel?.();
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setOpen(false);
        setQuery("");
        onCancel?.();
      }
    }
    document.addEventListener("mousedown", onDocClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open, onCancel]);

  const { data, isFetching } = useQuery({
    queryKey: ["parentSearch", projectKey, parentLevel, debouncedQuery],
    queryFn: () =>
      search.jql(parentSearchJql(projectKey, debouncedQuery, parentLevel), {
        fields: ["summary", "status", "issuetype"],
        maxResults: 100,
      }),
    enabled: open && !!projectKey,
  });

  const candidates: SearchIssue[] = (data?.issues ?? []).filter(
    (i) => i.key !== excludeKey && hierarchyLevelOf(i.fields.issuetype) === parentLevel,
  );

  function select(opt: ParentOption | null) {
    onChange(opt);
    setOpen(false);
    setQuery("");
  }

  return (
    <div ref={rootRef} className="relative" data-testid="parent-picker">
      <div className="mt-1 flex items-stretch gap-1">
        <button
          type="button"
          aria-label={label ?? `Select ${noun}`}
          aria-haspopup="listbox"
          aria-expanded={open}
          disabled={disabled}
          onClick={() => setOpen((o) => !o)}
          className="flex min-w-0 flex-1 items-center justify-between rounded border border-slate-300 px-2 py-1.5 text-left text-sm hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {value ? (
            <span className="flex min-w-0 items-center gap-1.5 text-[#1a1f36]">
              <span className="shrink-0 font-mono text-xs text-[#0052cc]">{value.key}</span>
              <span className="truncate">{value.summary}</span>
            </span>
          ) : (
            <span className="text-slate-400">{placeholder ?? `No ${noun}`}</span>
          )}
          <span aria-hidden className="ml-1 shrink-0 text-slate-400">
            ▾
          </span>
        </button>
        {clearable && value && (
          <button
            type="button"
            aria-label={`Remove ${noun}`}
            title={`Remove ${noun}`}
            data-testid={clearTestId}
            disabled={disabled}
            onClick={() => select(null)}
            className="rounded border border-slate-300 px-2 text-sm text-slate-500 hover:bg-slate-50 hover:text-red-600 disabled:opacity-60"
          >
            ×
          </button>
        )}
      </div>

      {open && (
        <div className="absolute z-20 mt-1 w-full min-w-64 rounded border border-slate-200 bg-white shadow-lg">
          <input
            autoFocus
            aria-label={`Search ${noun}`}
            data-testid={inputTestId}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={`Search by key or summary…`}
            className="w-full border-b border-slate-100 px-2 py-1.5 text-sm focus:outline-none"
          />
          <div className="max-h-56 overflow-y-auto py-1" role="listbox">
            {value && (
              <button
                type="button"
                role="option"
                aria-selected={false}
                onClick={() => select(null)}
                className="flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm text-slate-500 hover:bg-slate-50"
              >
                — No {noun}
              </button>
            )}
            {isFetching && <div className="px-2 py-1.5 text-xs text-slate-400">Searching…</div>}
            {!isFetching &&
              candidates.map((i) => (
                <button
                  key={i.key}
                  type="button"
                  role="option"
                  aria-selected={value?.key === i.key}
                  data-testid={`parent-picker-option-${i.key}`}
                  onClick={() => select({ key: i.key, summary: i.fields.summary ?? i.key })}
                  className={`flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm hover:bg-slate-50 ${
                    value?.key === i.key ? "bg-slate-50" : ""
                  }`}
                >
                  <span className="shrink-0 font-mono text-xs text-[#0052cc]">{i.key}</span>
                  <span className="truncate text-[#1a1f36]">{i.fields.summary}</span>
                </button>
              ))}
            {!isFetching && data && candidates.length === 0 && (
              <div className="px-2 py-1.5 text-xs text-slate-400">
                No matching {parentLevel === 1 ? "epics" : "issues"}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
