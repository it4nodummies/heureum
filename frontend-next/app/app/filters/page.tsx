"use client";

import { Suspense, useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import { search, filters, type SearchIssue } from "@/lib/api";
import { SearchResults } from "@/components/search/SearchResults";

export default function FiltersPage() {
  return (
    <Suspense fallback={null}>
      <FiltersPageInner />
    </Suspense>
  );
}

function FiltersPageInner() {
  const qc = useQueryClient();
  const params = useSearchParams();
  const [jql, setJql] = useState("");
  const [results, setResults] = useState<SearchIssue[]>([]);
  const [ran, setRan] = useState(false);

  const myFilters = useQuery({ queryKey: ["filters", "my"], queryFn: filters.list });

  const run = useMutation({
    mutationFn: (q: string) => search.jql(q),
    onSuccess: (data) => {
      setResults(data.issues);
      setRan(true);
    },
  });

  const save = useMutation({
    mutationFn: (name: string) => filters.create(name, jql),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["filters", "my"] }),
  });

  useEffect(() => {
    const initial = params.get("jql");
    if (initial) {
      setJql(initial);
      run.mutate(initial);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="mx-auto max-w-5xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">Filters</h1>

      <div className="mb-4 flex gap-2">
        <input
          aria-label="JQL"
          value={jql}
          onChange={(e) => setJql(e.target.value)}
          placeholder="project = DEMO ORDER BY updated DESC"
          className="flex-1 rounded border border-slate-300 px-3 py-2 font-mono text-sm"
        />
        <button
          onClick={() => run.mutate(jql)}
          disabled={!jql.trim() || run.isPending}
          className="rounded bg-[#0052cc] px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
        >
          Search
        </button>
        <button
          onClick={() => {
            const name = prompt("Filter name");
            if (name) save.mutate(name);
          }}
          disabled={!jql.trim() || save.isPending}
          className="rounded border border-slate-300 px-4 py-2 text-sm disabled:opacity-60"
        >
          Save filter
        </button>
      </div>

      {run.isError && <p className="mb-3 text-sm text-red-600">Invalid JQL</p>}

      <div className="grid grid-cols-[220px_1fr] gap-6">
        <aside>
          <h2 className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">
            Saved filters
          </h2>
          <ul className="space-y-1 text-sm">
            {myFilters.data?.map((f) => (
              <li key={f.id}>
                <button
                  className="text-left text-[#0052cc] hover:underline"
                  onClick={() => {
                    setJql(f.jql);
                    run.mutate(f.jql);
                  }}
                >
                  {f.name}
                </button>
              </li>
            ))}
            {myFilters.data?.length === 0 && <li className="text-slate-400">None yet</li>}
          </ul>
        </aside>
        <section>{ran && <SearchResults issues={results} />}</section>
      </div>
    </div>
  );
}
