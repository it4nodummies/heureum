"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { integrations, type Webhook } from "@/lib/api";

export function IntegrationsTab({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const [url, setUrl] = useState("");

  const hooks = useQuery({ queryKey: ["webhooks", projectKey], queryFn: () => integrations.webhooks(projectKey) });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["webhooks", projectKey] });

  const create = useMutation({
    mutationFn: () => integrations.createWebhook(projectKey, url, ["issue_created", "issue_updated", "issue_transitioned"]),
    onSuccess: () => { setUrl(""); invalidate(); },
  });
  const del = useMutation({ mutationFn: (id: string) => integrations.deleteWebhook(projectKey, id), onSuccess: invalidate });

  return (
    <div className="space-y-6" data-testid="integrations-tab">
      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Outgoing webhooks</h3>
        <ul className="mb-2 space-y-1" data-testid="webhooks-list">
          {(hooks.data ?? []).map((h: Webhook) => (
            <li key={h.id} className="flex items-center justify-between border-b border-slate-100 py-1 text-sm">
              <span className="truncate text-[#1a1f36]">{h.url}</span>
              <span className="flex items-center gap-2">
                <span className="text-xs text-slate-400">{h.events.join(", ")}</span>
                <button onClick={() => del.mutate(h.id)} className="text-xs text-red-600 hover:underline" aria-label={`Delete webhook ${h.url}`}>Remove</button>
              </span>
            </li>
          ))}
          {hooks.data && hooks.data.length === 0 && <li className="py-2 text-sm text-slate-400">No webhooks</li>}
        </ul>
        <div className="flex gap-2">
          <input aria-label="Webhook URL" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/hook" className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm" />
          <button onClick={() => url && create.mutate()} disabled={create.isPending} className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60">Add webhook</button>
        </div>
        <p className="mt-1 text-xs text-slate-400">Fires on issue created / updated / transitioned. Payload is signed with HMAC-SHA256 (X-OpenJira-Signature).</p>
      </section>
    </div>
  );
}
