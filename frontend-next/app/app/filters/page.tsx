"use client";

import { Suspense, useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import { search, filters, filterIsShared, LIST_FIELDS, type SearchIssue, type Filter } from "@/lib/api";
import { SearchResults } from "@/components/search/SearchResults";

export default function FiltersPage() {
  return (
    <Suspense fallback={null}>
      <FiltersPageInner />
    </Suspense>
  );
}

const MAX_RESULTS = 25;

function FiltersPageInner() {
  const qc = useQueryClient();
  const params = useSearchParams();
  const [jql, setJql] = useState("");
  const [results, setResults] = useState<SearchIssue[]>([]);
  const [ran, setRan] = useState(false);
  const [lastJql, setLastJql] = useState("");

  // Cursor pagination: /search/jql has no total, only nextPageToken + isLast.
  // pageTokens is the stack of tokens used to reach each loaded page;
  // pageTokens[0] is undefined (page 1). The top of the stack is the token of
  // the page currently shown. nextToken/isLast come from the last response.
  const [pageTokens, setPageTokens] = useState<(string | undefined)[]>([undefined]);
  const [nextToken, setNextToken] = useState<string | undefined>(undefined);
  const [isLast, setIsLast] = useState(true);

  const [name, setName] = useState("");
  const [isShared, setIsShared] = useState(false);

  const myFilters = useQuery({ queryKey: ["filters", "my"], queryFn: filters.list });

  const invalidate = () => qc.invalidateQueries({ queryKey: ["filters", "my"] });

  const run = useMutation({
    mutationFn: ({ q, token }: { q: string; token?: string }) =>
      search.jql(q, { fields: LIST_FIELDS, maxResults: MAX_RESULTS, nextPageToken: token }),
    onSuccess: (data, { q }) => {
      setResults(data.issues);
      setNextToken(data.nextPageToken);
      setIsLast(data.isLast);
      setRan(true);
      setLastJql(q);
    },
  });

  // Fresh search: reset to page 1.
  const runFresh = (q: string) => {
    setPageTokens([undefined]);
    run.mutate({ q, token: undefined });
  };

  const goNext = () => {
    if (isLast || nextToken === undefined) return;
    const token = nextToken;
    setPageTokens((s) => [...s, token]);
    run.mutate({ q: lastJql, token });
  };

  const goPrev = () => {
    if (pageTokens.length <= 1) return;
    const newStack = pageTokens.slice(0, -1);
    const token = newStack[newStack.length - 1];
    setPageTokens(newStack);
    run.mutate({ q: lastJql, token });
  };

  // Re-run the CURRENT page (same token) after bulk/inline edits — never reset
  // to page 1.
  const refetchCurrentPage = () => {
    if (lastJql) run.mutate({ q: lastJql, token: pageTokens[pageTokens.length - 1] });
  };

  const save = useMutation({
    mutationFn: () => filters.create(name.trim(), jql, { is_shared: isShared }),
    onSuccess: () => {
      setName("");
      setIsShared(false);
      invalidate();
    },
  });

  useEffect(() => {
    const initial = params.get("jql");
    if (initial) {
      setJql(initial);
      runFresh(initial);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="mx-auto max-w-5xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">Filters</h1>

      <div className="mb-3 flex gap-2">
        <input
          aria-label="JQL"
          value={jql}
          onChange={(e) => setJql(e.target.value)}
          placeholder="project = DEMO ORDER BY updated DESC"
          className="flex-1 rounded border border-slate-300 px-3 py-2 font-mono text-sm"
        />
        <button
          onClick={() => runFresh(jql)}
          disabled={!jql.trim() || run.isPending}
          className="rounded bg-[#0052cc] px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
        >
          Search
        </button>
      </div>

      <form
        className="mb-4 flex items-center gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim() && jql.trim()) save.mutate();
        }}
      >
        <input
          aria-label="Filter name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Filter name"
          className="rounded border border-slate-300 px-3 py-2 text-sm"
        />
        <label className="flex items-center gap-1.5 text-sm text-slate-600">
          <input
            type="checkbox"
            checked={isShared}
            onChange={(e) => setIsShared(e.target.checked)}
          />
          Share with team
        </label>
        <button
          type="submit"
          disabled={!jql.trim() || !name.trim() || save.isPending}
          className="rounded border border-slate-300 px-4 py-2 text-sm disabled:opacity-60"
        >
          Save
        </button>
      </form>

      {run.isError && <p className="mb-3 text-sm text-red-600">Invalid JQL</p>}

      <div className="grid grid-cols-[260px_1fr] gap-6">
        <aside>
          <h2 className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">
            Saved filters
          </h2>
          <ul className="space-y-1 text-sm">
            {myFilters.data?.map((f) => (
              <FilterRow
                key={f.id}
                filter={f}
                onRun={() => {
                  setJql(f.jql);
                  runFresh(f.jql);
                }}
                onChanged={invalidate}
              />
            ))}
            {myFilters.data?.length === 0 && <li className="text-slate-400">None yet</li>}
          </ul>
        </aside>
        <section>
          {ran && (
            <>
              <div className="mb-2 flex items-center gap-3 text-sm text-slate-600">
                <span data-testid="list-count">
                  {results.length} {results.length === 1 ? "result" : "results"}
                </span>
                <div className="ml-auto flex gap-2">
                  <button
                    data-testid="page-prev"
                    onClick={goPrev}
                    disabled={pageTokens.length <= 1 || run.isPending}
                    className="rounded border border-slate-300 px-3 py-1 text-sm disabled:opacity-50"
                  >
                    Prev
                  </button>
                  <button
                    data-testid="page-next"
                    onClick={goNext}
                    disabled={isLast || run.isPending}
                    className="rounded border border-slate-300 px-3 py-1 text-sm disabled:opacity-50"
                  >
                    Next
                  </button>
                </div>
              </div>
              <SearchResults issues={results} onChanged={refetchCurrentPage} />
            </>
          )}
        </section>
      </div>
    </div>
  );
}

function FilterRow({
  filter,
  onRun,
  onChanged,
}: {
  filter: Filter;
  onRun: () => void;
  onChanged: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(filter.name);
  const [isShared, setIsShared] = useState(filterIsShared(filter));

  const update = useMutation({
    mutationFn: () => filters.update(filter.id, { name: name.trim(), is_shared: isShared }),
    onSuccess: () => {
      setEditing(false);
      onChanged();
    },
  });

  const del = useMutation({
    mutationFn: () => filters.del(filter.id),
    onSuccess: onChanged,
  });

  if (editing) {
    return (
      <li>
        <form
          className="flex flex-col gap-1.5 rounded border border-slate-200 p-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim()) update.mutate();
          }}
        >
          <input
            aria-label="Edit filter name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          />
          <label className="flex items-center gap-1.5 text-xs text-slate-600">
            <input
              type="checkbox"
              checked={isShared}
              onChange={(e) => setIsShared(e.target.checked)}
            />
            Share with team
          </label>
          <div className="flex gap-2">
            <button
              type="submit"
              disabled={!name.trim() || update.isPending}
              className="rounded bg-[#0052cc] px-2 py-0.5 text-xs font-semibold text-white disabled:opacity-60"
            >
              Save
            </button>
            <button
              type="button"
              onClick={() => {
                setName(filter.name);
                setIsShared(filterIsShared(filter));
                setEditing(false);
              }}
              className="rounded border border-slate-300 px-2 py-0.5 text-xs"
            >
              Cancel
            </button>
          </div>
        </form>
      </li>
    );
  }

  return (
    <li className="group flex items-center gap-1">
      <button
        className="flex-1 text-left text-[#0052cc] hover:underline"
        onClick={onRun}
      >
        {filter.name}
      </button>
      {filterIsShared(filter) && (
        <span
          data-testid="filter-shared-badge"
          className="rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-emerald-700"
          title="Shared with team"
        >
          Shared
        </span>
      )}
      <button
        aria-label="Edit filter"
        title={`Edit ${filter.name}`}
        onClick={() => setEditing(true)}
        className="text-xs text-slate-400 hover:text-slate-700"
      >
        Edit
      </button>
      <button
        aria-label="Delete filter"
        title={`Delete ${filter.name}`}
        onClick={() => del.mutate()}
        disabled={del.isPending}
        className="text-xs text-slate-400 hover:text-red-600 disabled:opacity-60"
      >
        Delete
      </button>
    </li>
  );
}
