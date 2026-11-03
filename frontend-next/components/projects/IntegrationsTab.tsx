"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { integrations, type Webhook } from "@/lib/api";
import { useT } from "@/lib/i18n";

const EVENTS = ["issue_created", "issue_updated", "issue_transitioned"] as const;

function isEvent(v: string): v is (typeof EVENTS)[number] {
  return (EVENTS as readonly string[]).includes(v);
}

export function IntegrationsTab({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const t = useT();
  const [url, setUrl] = useState("");

  const hooks = useQuery({ queryKey: ["webhooks", projectKey], queryFn: () => integrations.webhooks(projectKey) });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["webhooks", projectKey] });

  const create = useMutation({
    mutationFn: () => integrations.createWebhook(projectKey, url, [...EVENTS]),
    onSuccess: () => { setUrl(""); invalidate(); },
  });
  const del = useMutation({ mutationFn: (id: string) => integrations.deleteWebhook(projectKey, id), onSuccess: invalidate });

  return (
    <div className="space-y-6" data-testid="integrations-tab">
      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">{t("projects.integrations.title")}</h3>
        <ul className="mb-2 space-y-1" data-testid="webhooks-list">
          {(hooks.data ?? []).map((h: Webhook) => (
            <li key={h.id} className="flex items-center justify-between border-b border-slate-100 py-1 text-sm">
              <span className="truncate text-[#1a1f36]">{h.url}</span>
              <span className="flex items-center gap-2">
                <span className="text-xs text-slate-400">{h.events.map((e) => (isEvent(e) ? t(`projects.automation.triggers.${e}`) : e)).join(", ")}</span>
                <button onClick={() => del.mutate(h.id)} className="text-xs text-red-600 hover:underline" aria-label={t("projects.integrations.deleteAria", { url: h.url })}>{t("projects.integrations.remove")}</button>
              </span>
            </li>
          ))}
          {hooks.data && hooks.data.length === 0 && <li className="py-2 text-sm text-slate-400">{t("projects.integrations.empty")}</li>}
        </ul>
        <div className="flex gap-2">
          <input aria-label={t("projects.integrations.urlLabel")} value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/hook" className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm" />
          <button onClick={() => url && create.mutate()} disabled={create.isPending} className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60">{t("projects.integrations.add")}</button>
        </div>
        <p className="mt-1 text-xs text-slate-400">{t("projects.integrations.help")}</p>
      </section>
    </div>
  );
}
