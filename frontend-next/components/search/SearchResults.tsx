"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import {
  issues,
  profile,
  type SearchIssue,
  type JiraUser,
  type IssueTransitionOption,
} from "@/lib/api";

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

  // Lightweight, purely-presentational hierarchy: an issue whose parent key is
  // ALSO present in the current result set renders directly beneath its parent
  // with a visual indent (depth > 0 → data-testid="child-row"). Parents absent
  // (or null) stay at top level. This only re-orders the current page — no
  // cross-page/epic fetch. Cycles/orphans fall back to top level so every row
  // is always rendered exactly once.
  const ordered = useMemo(() => {
    const present = new Set(rows.map((r) => r.key));
    const childrenOf = new Map<string, SearchIssue[]>();
    const roots: SearchIssue[] = [];
    for (const r of rows) {
      const pk = r.fields.parent?.key;
      if (pk && pk !== r.key && present.has(pk)) {
        const arr = childrenOf.get(pk) ?? [];
        arr.push(r);
        childrenOf.set(pk, arr);
      } else {
        roots.push(r);
      }
    }
    const out: { iss: SearchIssue; depth: number }[] = [];
    const emitted = new Set<string>();
    const visit = (iss: SearchIssue, depth: number) => {
      if (emitted.has(iss.key)) return; // guard against parent cycles
      emitted.add(iss.key);
      out.push({ iss, depth });
      for (const c of childrenOf.get(iss.key) ?? []) visit(c, depth + 1);
    };
    for (const r of roots) visit(r, 0);
    // Any row not reached (e.g. mutual parent cycle) still renders top level.
    for (const r of rows) if (!emitted.has(r.key)) visit(r, 0);
    return out;
  }, [rows]);

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
        return <StatusCell iss={iss} onChanged={onChanged} />;
      case "priority":
        return <PriorityCell iss={iss} onChanged={onChanged} />;
      case "assignee":
        return <AssigneeCell iss={iss} onChanged={onChanged} />;
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
          {ordered.map(({ iss, depth }) => (
            <tr
              key={iss.id}
              data-testid={depth > 0 ? "child-row" : undefined}
              className="border-b border-slate-100 hover:bg-slate-50"
            >
              <td className="w-8 py-2 pr-2">
                <input
                  type="checkbox"
                  data-testid="row-select"
                  aria-label={`Select ${iss.key}`}
                  checked={selected.has(iss.key)}
                  onChange={() => toggleRow(iss.key)}
                />
              </td>
              {visible.map((c, ci) => {
                // Indent only the first visible content column so table columns
                // stay aligned; deeper nesting adds more left padding + marker.
                const indent = ci === 0 && depth > 0;
                return (
                  <td
                    key={c.key}
                    className="py-2 pr-4 text-[#1a1f36]"
                    style={indent ? { paddingLeft: depth * 20 } : undefined}
                  >
                    {indent && (
                      <span aria-hidden className="mr-1 text-slate-300">
                        ↳
                      </span>
                    )}
                    {cell(iss, c.key)}
                  </td>
                );
              })}
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

// Debounced global people search (results may span projects) shared by the
// bulk assignee picker and the inline assignee cell.
function usePeopleSearch(active: boolean) {
  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");
  const [results, setResults] = useState<JiraUser[]>([]);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  useEffect(() => {
    let cancelled = false;
    if (!active) return;
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
  }, [debounced, active]);

  return { query, setQuery, results };
}

// Closes the popover when the user clicks outside `ref`.
function useOutsideClose(active: boolean, ref: React.RefObject<HTMLElement | null>, close: () => void) {
  useEffect(() => {
    if (!active) return;
    function onDocClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) close();
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [active, ref, close]);
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
  const rootRef = useRef<HTMLDivElement>(null);
  const { query, setQuery, results } = usePeopleSearch(open);

  useOutsideClose(open, rootRef, () => {
    setOpen(false);
    setQuery("");
  });

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

// ── Inline editable cells ────────────────────────────────────────────────────
// Each cell is read-only text until clicked; clicking swaps in an editor. Edit
// controls stopPropagation so they never trigger the row's key-link navigation.
// A successful mutation resets the cell's own edit state in onSuccess and calls
// onChanged() to refetch; rows are keyed by stable iss.id, so the cell re-renders
// in place (it does NOT remount) — hence state is reset explicitly, not via a key
// change.

function PriorityCell({ iss, onChanged }: { iss: SearchIssue; onChanged?: () => void }) {
  const [editing, setEditing] = useState(false);
  const update = useMutation({
    mutationFn: (id: string) => issues.update(iss.key, { priority: { id } }),
    onSuccess: () => {
      setEditing(false);
      onChanged?.();
    },
  });

  if (!editing) {
    return (
      <button
        type="button"
        data-testid="cell-priority"
        onClick={(e) => {
          e.stopPropagation();
          setEditing(true);
        }}
        className="rounded px-1 text-left text-[#1a1f36] hover:bg-slate-100"
      >
        {iss.fields.priority?.name ?? "—"}
      </button>
    );
  }

  return (
    <select
      autoFocus
      aria-label={`Priority for ${iss.key}`}
      defaultValue={iss.fields.priority?.id ?? ""}
      disabled={update.isPending}
      onClick={(e) => e.stopPropagation()}
      onBlur={() => setEditing(false)}
      onChange={(e) => {
        if (e.target.value) update.mutate(e.target.value);
        else setEditing(false);
      }}
      className="rounded border border-slate-300 px-1 py-0.5 text-sm"
    >
      {PRIORITIES.map((p) => (
        <option key={p.id} value={p.id}>
          {p.name}
        </option>
      ))}
    </select>
  );
}

function StatusCell({ iss, onChanged }: { iss: SearchIssue; onChanged?: () => void }) {
  const [editing, setEditing] = useState(false);
  const [opts, setOpts] = useState<IssueTransitionOption[] | null>(null);
  const [loading, setLoading] = useState(false);
  const transition = useMutation({
    mutationFn: (statusId: string) => issues.transition(iss.key, statusId),
    onSuccess: () => {
      setEditing(false);
      // Rows are keyed by stable iss.id, so onChanged()'s refetch re-renders
      // this same StatusCell instance (no remount). Drop the cached transitions
      // so the next edit re-fetches the ones valid for the NEW status.
      setOpts(null);
      onChanged?.();
    },
  });

  const label = iss.fields.status?.name ?? "—";

  const enterEdit = async () => {
    setEditing(true);
    if (opts === null) {
      setLoading(true);
      try {
        const r = await issues.transitions(iss.key);
        setOpts(r.transitions);
      } catch {
        setOpts([]);
      } finally {
        setLoading(false);
      }
    }
  };

  if (!editing) {
    return (
      <button
        type="button"
        data-testid="cell-status"
        onClick={(e) => {
          e.stopPropagation();
          void enterEdit();
        }}
        className="rounded px-1 text-left text-[#1a1f36] hover:bg-slate-100"
      >
        {label}
      </button>
    );
  }

  // Loading transitions, or none available → stay read-only (revert on blur).
  if (loading || opts === null || opts.length === 0) {
    return (
      <button
        type="button"
        data-testid="cell-status"
        title={opts !== null && opts.length === 0 ? "No transitions available" : undefined}
        onClick={(e) => {
          e.stopPropagation();
          setEditing(false);
        }}
        className="rounded px-1 text-left text-slate-500 hover:bg-slate-100"
      >
        {loading ? "…" : label}
      </button>
    );
  }

  return (
    <select
      autoFocus
      aria-label={`Status for ${iss.key}`}
      defaultValue=""
      disabled={transition.isPending}
      onClick={(e) => e.stopPropagation()}
      onBlur={() => setEditing(false)}
      onChange={(e) => {
        if (e.target.value) transition.mutate(e.target.value);
      }}
      className="rounded border border-slate-300 px-1 py-0.5 text-sm"
    >
      <option value="">{label}…</option>
      {opts.map((t) => (
        <option key={t.id} value={t.to.id}>
          {t.to.name}
        </option>
      ))}
    </select>
  );
}

function AssigneeCell({ iss, onChanged }: { iss: SearchIssue; onChanged?: () => void }) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const { query, setQuery, results } = usePeopleSearch(open);
  const update = useMutation({
    mutationFn: (accountId: string) => issues.update(iss.key, { assignee: { accountId } }),
    onSuccess: () => {
      setOpen(false);
      setQuery("");
      onChanged?.();
    },
  });

  useOutsideClose(open, rootRef, () => {
    setOpen(false);
    setQuery("");
  });

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        data-testid="cell-assignee"
        aria-label={`Assignee for ${iss.key}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={(e) => {
          e.stopPropagation();
          setOpen((o) => !o);
        }}
        className="rounded px-1 text-left text-[#1a1f36] hover:bg-slate-100"
      >
        {iss.fields.assignee?.displayName ?? "Unassigned"}
      </button>

      {open && (
        <div
          onClick={(e) => e.stopPropagation()}
          className="absolute z-20 mt-1 w-56 rounded border border-slate-200 bg-white shadow-lg"
        >
          <input
            autoFocus
            aria-label={`Search people for ${iss.key}`}
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
                disabled={update.isPending}
                onClick={() => update.mutate(u.accountId)}
                className="flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm hover:bg-slate-50 disabled:opacity-60"
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
