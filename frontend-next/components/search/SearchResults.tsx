"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { issues, profile, type SearchIssue, type JiraUser } from "@/lib/api";

const ALL_COLUMNS = [
  { key: "key", label: "Key" },
  { key: "summary", label: "Summary" },
  { key: "status", label: "Status" },
  { key: "priority", label: "Priority" },
  { key: "assignee", label: "Assignee" },
  { key: "updated", label: "Updated" },
] as const;

type ColKey = (typeof ALL_COLUMNS)[number]["key"];

// The 5 fixed Jira priorities (id "1".."5" = highest..lowest), matching
// internal/api/v3/reference.go priorityOrder.
const PRIORITIES = [
  { id: "1", name: "Highest" },
  { id: "2", name: "High" },
  { id: "3", name: "Medium" },
  { id: "4", name: "Low" },
  { id: "5", name: "Lowest" },
] as const;

export function SearchResults({
  issues: rows,
  onChanged,
}: {
  issues: SearchIssue[];
  onChanged?: () => void;
}) {
  const [cols, setCols] = useState<Set<ColKey>>(
    new Set(ALL_COLUMNS.map((c) => c.key)),
  );
  const [selected, setSelected] = useState<Set<string>>(new Set());

  // bulk-bar form state
  const [priorityId, setPriorityId] = useState("");
  const [assignee, setAssignee] = useState<{ accountId: string; displayName: string } | null>(null);
  const [label, setLabel] = useState("");
  const [failed, setFailed] = useState<string[]>([]);

  // Drop selections that are no longer present after a refetch.
  useEffect(() => {
    const present = new Set(rows.map((r) => r.key));
    setSelected((prev) => {
      const next = new Set([...prev].filter((k) => present.has(k)));
      return next.size === prev.size ? prev : next;
    });
  }, [rows]);

  const toggle = (k: ColKey) =>
    setCols((prev) => {
      const next = new Set(prev);
      if (next.has(k)) next.delete(k);
      else next.add(k);
      return next;
    });

  const toggleRow = (key: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const allShownKeys = useMemo(() => rows.map((r) => r.key), [rows]);
  const allSelected = allShownKeys.length > 0 && allShownKeys.every((k) => selected.has(k));

  const toggleAll = () =>
    setSelected((prev) => {
      if (allSelected) return new Set();
      return new Set(allShownKeys);
    });

  const clearBulk = () => {
    setSelected(new Set());
    setPriorityId("");
    setAssignee(null);
    setLabel("");
  };

  const bulk = useMutation({
    mutationFn: (body: Parameters<typeof issues.bulk>[0]) => issues.bulk(body),
    onSuccess: (data) => {
      setFailed(data.results.filter((r) => !r.ok).map((r) => r.key));
      clearBulk();
      onChanged?.();
    },
  });

  const applyChanges = () => {
    const fields: NonNullable<Parameters<typeof issues.bulk>[0]["fields"]> = {};
    if (priorityId) fields.priority = { id: priorityId };
    if (assignee) fields.assignee = { accountId: assignee.accountId };
    const labels = label
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    // NB: the bulk endpoint REPLACES the whole label set (SetLabels), so this
    // sets the labels rather than appending — hence the "Set label(s)" wording.
    if (labels.length > 0) fields.labels = labels;
    if (Object.keys(fields).length === 0) return;
    setFailed([]);
    bulk.mutate({ keys: [...selected], fields });
  };

  const applyDelete = () => {
    setFailed([]);
    bulk.mutate({ keys: [...selected], delete: true });
  };

  const cell = (iss: SearchIssue, k: ColKey) => {
    switch (k) {
      case "key":
        return (
          <a href={`/app/browse/${iss.key}`} className="text-[#0052cc] hover:underline">
            {iss.key}
          </a>
        );
      case "summary":
        return iss.fields.summary ?? "";
      case "status":
        return iss.fields.status?.name ?? "";
      case "priority":
        return iss.fields.priority?.name ?? "";
      case "assignee":
        return iss.fields.assignee?.displayName ?? "Unassigned";
      case "updated":
        return iss.fields.updated?.slice(0, 10) ?? "";
    }
  };

  const visible = ALL_COLUMNS.filter((c) => cols.has(c.key));
  const canApply =
    selected.size > 0 && (!!priorityId || !!assignee || !!label.trim()) && !bulk.isPending;

  return (
    <div>
      <div className="mb-3 flex flex-wrap gap-3 text-sm text-slate-500" aria-label="Columns">
        {ALL_COLUMNS.map((c) => (
          <label key={c.key} className="flex items-center gap-1">
            <input type="checkbox" checked={cols.has(c.key)} onChange={() => toggle(c.key)} />
            {c.label}
          </label>
        ))}
      </div>

      {selected.size > 0 && (
        <div
          data-testid="bulk-bar"
          className="mb-3 flex flex-wrap items-center gap-3 rounded border border-slate-200 bg-slate-50 px-3 py-2 text-sm"
        >
          <span className="font-semibold text-[#1a1f36]">{selected.size} selected</span>

          <select
            aria-label="Bulk priority"
            value={priorityId}
            onChange={(e) => setPriorityId(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            <option value="">Priority…</option>
            {PRIORITIES.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>

          <BulkAssigneePicker value={assignee} onChange={setAssignee} />

          <input
            aria-label="Set label(s)"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="Set label(s), comma-separated"
            title="Replaces the existing labels on the selected issues"
            className="w-56 rounded border border-slate-300 px-2 py-1 text-sm"
          />

          <button
            type="button"
            onClick={applyChanges}
            disabled={!canApply}
            className="rounded bg-[#0052cc] px-3 py-1 text-sm font-semibold text-white disabled:opacity-60"
          >
            Apply
          </button>

          <button
            type="button"
            onClick={applyDelete}
            disabled={bulk.isPending}
            className="rounded border border-red-300 px-3 py-1 text-sm font-semibold text-red-600 hover:bg-red-50 disabled:opacity-60"
          >
            Delete
          </button>

          <button
            type="button"
            onClick={clearBulk}
            className="text-xs text-slate-400 hover:text-slate-700"
          >
            Clear
          </button>
        </div>
      )}

      {failed.length > 0 && (
        <p data-testid="bulk-warning" className="mb-3 text-sm text-amber-700">
          Some issues could not be updated: {failed.join(", ")}
        </p>
      )}

      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b border-slate-200 text-left text-slate-500">
            <th className="w-8 py-2 pr-2 font-semibold">
              <input
                type="checkbox"
                data-testid="select-all"
                aria-label="Select all"
                checked={allSelected}
                onChange={toggleAll}
              />
            </th>
            {visible.map((c) => (
              <th key={c.key} className="py-2 pr-4 font-semibold">
                {c.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((iss) => (
            <tr key={iss.id} className="border-b border-slate-100 hover:bg-slate-50">
              <td className="w-8 py-2 pr-2">
                <input
                  type="checkbox"
                  data-testid="row-select"
                  aria-label={`Select ${iss.key}`}
                  checked={selected.has(iss.key)}
                  onChange={() => toggleRow(iss.key)}
                />
              </td>
              {visible.map((c) => (
                <td key={c.key} className="py-2 pr-4 text-[#1a1f36]">
                  {cell(iss, c.key)}
                </td>
              ))}
            </tr>
          ))}
          {rows.length === 0 && (
            <tr>
              <td className="py-4 text-slate-400" colSpan={visible.length + 1}>
                No results
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

// Lightweight person picker for bulk assignment: results may span multiple
// projects, so it searches globally via profile.searchUsers (not the
// project-scoped assignable search that UserPicker uses).
function BulkAssigneePicker({
  value,
  onChange,
}: {
  value: { accountId: string; displayName: string } | null;
  onChange: (v: { accountId: string; displayName: string } | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");
  const [results, setResults] = useState<JiraUser[]>([]);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  useEffect(() => {
    let cancelled = false;
    if (!open) return;
    profile
      .searchUsers(debounced)
      .then((r) => {
        if (!cancelled) setResults(r);
      })
      .catch(() => {
        if (!cancelled) setResults([]);
      });
    return () => {
      cancelled = true;
    };
  }, [debounced, open]);

  useEffect(() => {
    if (!open) return;
    function onDocClick(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
        setQuery("");
      }
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [open]);

  const select = (v: { accountId: string; displayName: string } | null) => {
    onChange(v);
    setOpen(false);
    setQuery("");
  };

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        aria-label="Bulk assignee"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="flex w-40 items-center justify-between rounded border border-slate-300 px-2 py-1 text-left text-sm hover:bg-slate-50"
      >
        <span className={value ? "text-[#1a1f36]" : "text-slate-400"}>
          {value ? value.displayName : "Assignee…"}
        </span>
        <span aria-hidden className="text-slate-400">
          ▾
        </span>
      </button>

      {open && (
        <div className="absolute z-20 mt-1 w-56 rounded border border-slate-200 bg-white shadow-lg">
          <input
            autoFocus
            aria-label="Search people"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search people…"
            className="w-full border-b border-slate-100 px-2 py-1.5 text-sm focus:outline-none"
          />
          <div className="max-h-56 overflow-y-auto py-1" role="listbox">
            {results.length === 0 && (
              <div className="px-2 py-1.5 text-xs text-slate-400">No matching people</div>
            )}
            {results.map((u) => (
              <button
                key={u.accountId}
                type="button"
                onClick={() => select({ accountId: u.accountId, displayName: u.displayName })}
                className="flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm hover:bg-slate-50"
              >
                {u.displayName}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
