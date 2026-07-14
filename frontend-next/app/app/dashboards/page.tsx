"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { dashboards } from "@/lib/api";

export default function DashboardsPage() {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const list = useQuery({ queryKey: ["dashboards"], queryFn: dashboards.list });
  const create = useMutation({
    mutationFn: (n: string) => dashboards.create(n),
    onSuccess: () => {
      setName("");
      qc.invalidateQueries({ queryKey: ["dashboards"] });
    },
  });

  const items = list.data ?? [];

  return (
    <div className="mx-auto max-w-3xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">Dashboards</h1>
      <div className="mb-4 flex gap-2">
        <input
          aria-label="New dashboard name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Dashboard name"
          className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
        />
        <button
          onClick={() => name && create.mutate(name)}
          disabled={!name.trim() || create.isPending}
          className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
        >
          Create
        </button>
      </div>
      <ul className="space-y-1" data-testid="dashboards-list">
        {items.map((d) => (
          <li key={d.id}>
            <a href={`/app/dashboards/${d.id}`} className="text-[#0052cc] hover:underline">
              {d.name}
            </a>
          </li>
        ))}
        {items.length === 0 && <li className="text-sm text-slate-400">No dashboards yet</li>}
      </ul>
    </div>
  );
}
