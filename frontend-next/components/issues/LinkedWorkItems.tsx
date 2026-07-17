"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { issueLinks, search, IssueLinkForIssue, IssueLinkTypeName, SearchIssue } from "@/lib/api";

interface Props {
  issueKey: string;
}

const RELATION_OPTIONS: IssueLinkTypeName[] = ["Blocks", "Duplicate", "Relates"];

// Row derived from an IssueLinkForIssue from THIS issue's point of view: the
// backend only populates the side opposite the requested issue (see
// internal/api/handlers/issuelink_handler.go ListForIssue). So:
//   - inwardIssue populated  => this issue is the outward/source side
//                               => the relation reads as type.outward (e.g. "blocks")
//   - outwardIssue populated => this issue is the inward/target side
//                               => the relation reads as type.inward (e.g. "is blocked by")
interface LinkedRow {
  id: string;
  label: string;
  key: string;
  summary: string;
  statusName?: string;
  statusCategoryKey?: string;
}

function toRow(link: IssueLinkForIssue): LinkedRow | null {
  const other = link.inwardIssue ?? link.outwardIssue;
  if (!other) return null;
  const label = link.inwardIssue ? link.type.outward : link.type.inward;
  return {
    id: link.id,
    label,
    key: other.key,
    summary: other.fields.summary,
    statusName: other.fields.status?.name,
    statusCategoryKey: other.fields.status?.statusCategory?.key,
  };
}

function statusBadgeClasses(categoryKey?: string): string {
  switch (categoryKey) {
    case "done":
      return "bg-green-100 text-green-700";
    case "indeterminate":
      return "bg-blue-100 text-blue-700";
    default:
      return "bg-slate-100 text-slate-600";
  }
}

export function LinkedWorkItems({ issueKey }: Props) {
  const qc = useQueryClient();
  const linksKey = ["issue", issueKey, "links"];
  const { data, isLoading } = useQuery({
    queryKey: linksKey,
    queryFn: () => issueLinks.list(issueKey),
  });

  const rows = useMemo(() => (data?.issuelinks ?? []).map(toRow).filter((r): r is LinkedRow => r !== null), [data]);

  const grouped = useMemo(() => {
    const groups = new Map<string, LinkedRow[]>();
    for (const row of rows) {
      const bucket = groups.get(row.label) ?? [];
      bucket.push(row);
      groups.set(row.label, bucket);
    }
    return Array.from(groups.entries());
  }, [rows]);

  const remove = useMutation({
    mutationFn: (id: string) => issueLinks.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: linksKey }),
  });

  // Add-link form state.
  const [relation, setRelation] = useState<IssueLinkTypeName>("Blocks");
  const [query, setQuery] = useState("");
  const [target, setTarget] = useState<SearchIssue | null>(null);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [suggestions, setSuggestions] = useState<SearchIssue[]>([]);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    const q = query.trim();
    if (!q || target) {
      setSuggestions([]);
      return;
    }
    debounceRef.current = setTimeout(async () => {
      try {
        const escaped = q.replace(/"/g, "");
        const jql = `text ~ "${escaped}"`;
        const res = await search.jql(jql, { fields: ["summary", "status"], maxResults: 8 });
        setSuggestions(res.issues.filter((i) => i.key !== issueKey));
      } catch {
        setSuggestions([]);
      }
    }, 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query, target, issueKey]);

  // Convention: this issue is always the outward/source side of a link
  // created from this form, whichever relation is picked — "Blocks" means
  // "this issue blocks <target>", "Duplicate" means "this issue duplicates
  // <target>", "Relates" is symmetric. See issueLinks.create in lib/api.ts.
  const create = useMutation({
    mutationFn: () =>
      issueLinks.create({ typeName: relation, outwardKey: issueKey, inwardKey: target!.key }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: linksKey });
      setQuery("");
      setTarget(null);
      setSuggestions([]);
    },
  });

  function submit() {
    if (target && !create.isPending) create.mutate();
  }

  return (
    <section className="mt-8" data-testid="linked-work-items-section">
      <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
        Linked work items
      </h2>

      {!isLoading && grouped.length === 0 && (
        <p className="mb-3 text-sm text-slate-400">No linked work items yet.</p>
      )}

      {grouped.length > 0 && (
        <div className="mb-4 space-y-3">
          {grouped.map(([label, items]) => (
            <div key={label}>
              <div className="mb-1 text-xs font-medium text-slate-400">{label}</div>
              <ul className="space-y-2">
                {items.map((row) => (
                  <li
                    key={row.id}
                    data-testid={`issue-link-row-${row.key}`}
                    className="flex items-center gap-3 rounded border border-slate-200 px-3 py-2"
                  >
                    <Link
                      href={`/app/browse/${row.key}`}
                      className="shrink-0 font-mono text-xs text-[#0052cc] hover:underline"
                    >
                      {row.key}
                    </Link>
                    {row.statusName && (
                      <span
                        className={`shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold ${statusBadgeClasses(row.statusCategoryKey)}`}
                      >
                        {row.statusName}
                      </span>
                    )}
                    <span className="flex-1 truncate text-sm text-[#1a1f36]">{row.summary}</span>
                    <button
                      onClick={() => remove.mutate(row.id)}
                      disabled={remove.isPending}
                      aria-label={`Remove link to ${row.key}`}
                      className="shrink-0 text-xs text-slate-400 hover:text-red-600 disabled:opacity-60"
                    >
                      Remove
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      )}

      <div className="flex items-start gap-2">
        <select
          aria-label="Relation type"
          value={relation}
          onChange={(e) => setRelation(e.target.value as IssueLinkTypeName)}
          className="rounded border border-slate-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
        >
          {RELATION_OPTIONS.map((r) => (
            <option key={r} value={r}>
              {r}
            </option>
          ))}
        </select>

        <div className="relative flex-1">
          <input
            value={target ? `${target.key} ${target.fields.summary ?? ""}` : query}
            onChange={(e) => {
              setTarget(null);
              setQuery(e.target.value);
              setShowSuggestions(true);
            }}
            onFocus={() => setShowSuggestions(true)}
            onBlur={() => setTimeout(() => setShowSuggestions(false), 150)}
            placeholder="Search for an issue…"
            aria-label="Search for an issue to link"
            className="w-full rounded border border-slate-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc]"
          />
          {showSuggestions && !target && suggestions.length > 0 && (
            <ul className="absolute z-10 mt-1 w-full max-h-56 overflow-auto rounded border border-slate-200 bg-white shadow-lg">
              {suggestions.map((s) => (
                <li key={s.key}>
                  <button
                    type="button"
                    onMouseDown={(e) => {
                      e.preventDefault();
                      setTarget(s);
                      setQuery("");
                      setShowSuggestions(false);
                    }}
                    className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-slate-50"
                  >
                    <span className="font-mono text-xs text-[#0052cc]">{s.key}</span>
                    <span className="truncate text-[#1a1f36]">{s.fields.summary}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        <button
          onClick={submit}
          disabled={!target || create.isPending}
          className="shrink-0 rounded bg-[#0052cc] px-3 py-1.5 text-sm font-semibold text-white hover:bg-[#0065ff] disabled:opacity-60"
        >
          {create.isPending ? "Adding…" : "Add"}
        </button>
      </div>
      {create.isError && (
        <p className="mt-2 text-xs text-red-600">
          {create.error instanceof Error ? create.error.message : "Failed to create link."}
        </p>
      )}
    </section>
  );
}
