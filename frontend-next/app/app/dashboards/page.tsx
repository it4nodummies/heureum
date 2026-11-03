"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { dashboards } from "@/lib/api";
import { useT } from "@/lib/i18n";

export default function DashboardsPage() {
  const t = useT();
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
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">{t("dashboards.title")}</h1>
      <div className="mb-4 flex gap-2">
        <input
          aria-label={t("dashboards.newName")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("dashboards.namePlaceholder")}
          className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm"
        />
        <button
          onClick={() => name && create.mutate(name)}
          disabled={!name.trim() || create.isPending}
          className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
        >
          {t("common.actions.create")}
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
        {items.length === 0 && <li className="text-sm text-slate-400">{t("dashboards.empty")}</li>}
      </ul>
    </div>
  );
}
