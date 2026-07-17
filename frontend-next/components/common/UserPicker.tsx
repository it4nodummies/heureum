"use client";

// Reusable "assign a person" combobox: a trigger button showing the current
// selection, opening a dropdown with a debounced search box over
// GET /rest/api/3/user/assignable/search (project-membership scoped, see
// internal/api/handlers/user_handler.go AssignableSearch), plus pinned
// "Unassigned" and "Assign to me" shortcuts.
//
// `value`/`onChange` deal only in accountId (Jira's convention) — callers
// that already know the current assignee's displayName (e.g. IssueView reads
// it off `issue.fields.assignee`) can pass `valueLabel` so the trigger button
// doesn't need an extra round trip just to render the current selection.

import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { users, profile, type JiraUserRef } from "@/lib/api";

interface Props {
  projectKey: string;
  value: string | null;
  valueLabel?: string | null;
  onChange: (accountId: string | null, user?: JiraUserRef) => void;
  disabled?: boolean;
  label?: string;
}

export function UserPicker({ projectKey, value, valueLabel, onChange, disabled, label = "Assignee" }: Props) {
  const [open, setOpen] = useState(false);
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
      }
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [open]);

  const { data: results, isFetching } = useQuery({
    queryKey: ["assignableSearch", projectKey, debouncedQuery],
    queryFn: () => users.assignableSearch(projectKey, debouncedQuery),
    enabled: open && !!projectKey,
  });

  const { data: me } = useQuery({
    queryKey: ["myself"],
    queryFn: () => profile.me(),
    enabled: open,
    staleTime: 5 * 60 * 1000,
  });

  function select(accountId: string | null, u?: JiraUserRef) {
    onChange(accountId, u);
    setOpen(false);
    setQuery("");
  }

  const currentLabel = value ? valueLabel ?? "Assigned" : "Unassigned";

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className="mt-1 flex w-full items-center justify-between rounded border border-slate-300 px-2 py-1.5 text-left text-sm hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
      >
        <span className={value ? "text-[#1a1f36]" : "text-slate-400"}>{currentLabel}</span>
        <span aria-hidden className="text-slate-400">
          ▾
        </span>
      </button>

      {open && (
        <div className="absolute z-20 mt-1 w-64 rounded border border-slate-200 bg-white shadow-lg">
          <input
            autoFocus
            aria-label="Search people"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search people…"
            className="w-full border-b border-slate-100 px-2 py-1.5 text-sm focus:outline-none"
          />
          <div className="max-h-56 overflow-y-auto py-1" role="listbox">
            <button
              type="button"
              onClick={() => select(null)}
              className="flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm hover:bg-slate-50"
            >
              <span
                aria-hidden
                className="flex h-5 w-5 items-center justify-center rounded-full border border-dashed border-slate-300 text-[10px] text-slate-400"
              >
                —
              </span>
              Unassigned
            </button>
            {me && (
              <button
                type="button"
                onClick={() =>
                  select(me.accountId, {
                    accountId: me.accountId,
                    displayName: me.displayName,
                    emailAddress: me.emailAddress,
                    avatarUrls: me.avatarUrls,
                  })
                }
                className="flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm hover:bg-slate-50"
              >
                <UserAvatar displayName={me.displayName} avatarUrls={me.avatarUrls} />
                Assign to me
              </button>
            )}
            <div className="my-1 border-t border-slate-100" />
            {isFetching && <div className="px-2 py-1.5 text-xs text-slate-400">Searching…</div>}
            {!isFetching &&
              (results ?? [])
                .filter((u) => u.accountId !== me?.accountId)
                .map((u) => (
                  <button
                    key={u.accountId}
                    type="button"
                    onClick={() => select(u.accountId, u)}
                    className="flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm hover:bg-slate-50"
                  >
                    <UserAvatar displayName={u.displayName} avatarUrls={u.avatarUrls} />
                    {u.displayName}
                  </button>
                ))}
            {!isFetching && (results ?? []).length === 0 && (
              <div className="px-2 py-1.5 text-xs text-slate-400">No matching people</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function UserAvatar({ displayName, avatarUrls }: { displayName: string; avatarUrls?: Record<string, string> }) {
  const src = avatarUrls?.["24x24"] ?? avatarUrls?.["32x32"] ?? avatarUrls?.["48x48"];
  if (src) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt="" className="h-5 w-5 rounded-full" />;
  }
  return (
    <span
      aria-hidden
      className="flex h-5 w-5 items-center justify-center rounded-full bg-slate-200 text-[10px] font-semibold text-slate-600"
    >
      {displayName?.[0]?.toUpperCase() ?? "?"}
    </span>
  );
}
