"use client";

import { useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { notifications, type AppNotification } from "@/lib/api";

type Tab = "direct" | "watching";

// Nessun campo Direct/Watching persistito: classifichiamo per `type`.
const DIRECT_TYPES = new Set(["assignment", "mention"]);
const WATCHING_TYPES = new Set(["comment", "status_change", "sprint_started", "sprint_completed"]);

function tabOf(n: AppNotification): Tab {
  return DIRECT_TYPES.has(n.type) ? "direct" : "watching";
}

// La `link` contiene un URL con la issue key (es. /app/browse/DEMO-1).
function issueKeyOf(n: AppNotification): string | null {
  const src = n.link || n.title || "";
  const m = src.match(/([A-Z][A-Z0-9]+-\d+)/);
  return m ? m[1] : null;
}

type Group = { key: string; label: string; items: AppNotification[] };

function groupByIssue(items: AppNotification[]): Group[] {
  const order: string[] = [];
  const map = new Map<string, Group>();
  for (const n of items) {
    const key = issueKeyOf(n);
    const gkey = key ?? "__other__";
    if (!map.has(gkey)) {
      map.set(gkey, { key: gkey, label: key ?? "Other", items: [] });
      order.push(gkey);
    }
    map.get(gkey)!.items.push(n);
  }
  return order.map((k) => map.get(k)!);
}

export function NotificationBell() {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState<Tab>("direct");

  const count = useQuery({
    queryKey: ["notif", "count"],
    queryFn: notifications.unreadCount,
    refetchInterval: 30000, // polling ogni 30s (nessun push nativo lato bell)
  });
  const list = useQuery({ queryKey: ["notif", "list"], queryFn: notifications.list, enabled: open });

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["notif", "count"] });
    qc.invalidateQueries({ queryKey: ["notif", "list"] });
  };
  const markRead = useMutation({ mutationFn: (id: string) => notifications.markRead(id), onSuccess: invalidate });
  const markAll = useMutation({ mutationFn: () => notifications.markAllRead(), onSuccess: invalidate });

  const unread = count.data?.count ?? 0;

  const all = list.data ?? [];
  const directItems = useMemo(() => all.filter((n) => tabOf(n) === "direct"), [all]);
  const watchingItems = useMemo(() => all.filter((n) => tabOf(n) === "watching"), [all]);
  const directUnread = directItems.filter((n) => !n.is_read).length;
  const watchingUnread = watchingItems.filter((n) => !n.is_read).length;

  const active = tab === "direct" ? directItems : watchingItems;
  const groups = useMemo(() => groupByIssue(active), [active]);

  return (
    <div className="relative">
      <button
        aria-label="Notifications"
        onClick={() => setOpen((o) => !o)}
        className="relative rounded p-2 text-slate-500 hover:bg-slate-100"
      >
        <span aria-hidden>🔔</span>
        {unread > 0 && (
          <span data-testid="notif-badge" className="absolute -right-0.5 -top-0.5 rounded-full bg-[#de350b] px-1 text-[10px] font-semibold text-white">
            {unread}
          </span>
        )}
      </button>
      {open && (
        <div className="absolute right-0 z-20 mt-2 w-80 rounded border border-slate-200 bg-white shadow-lg" data-testid="notif-dropdown">
          <div className="flex items-center justify-between border-b px-3 py-2">
            <span className="text-sm font-semibold text-[#1a1f36]">Notifications</span>
            <button onClick={() => markAll.mutate()} className="text-xs text-[#0052cc] hover:underline">Mark all read</button>
          </div>
          <div className="flex border-b" role="tablist">
            <button
              data-testid="notif-tab-direct"
              role="tab"
              aria-selected={tab === "direct"}
              onClick={() => setTab("direct")}
              className={`flex-1 px-3 py-2 text-xs font-medium ${
                tab === "direct" ? "border-b-2 border-[#0052cc] text-[#0052cc]" : "text-slate-500 hover:text-[#1a1f36]"
              }`}
            >
              Direct{directUnread > 0 ? ` (${directUnread})` : ""}
            </button>
            <button
              data-testid="notif-tab-watching"
              role="tab"
              aria-selected={tab === "watching"}
              onClick={() => setTab("watching")}
              className={`flex-1 px-3 py-2 text-xs font-medium ${
                tab === "watching" ? "border-b-2 border-[#0052cc] text-[#0052cc]" : "text-slate-500 hover:text-[#1a1f36]"
              }`}
            >
              Watching{watchingUnread > 0 ? ` (${watchingUnread})` : ""}
            </button>
          </div>
          <div className="max-h-96 overflow-auto">
            {groups.map((g) => (
              <div key={g.key}>
                <div className="bg-slate-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-slate-400">
                  {g.label}
                </div>
                <ul>
                  {g.items.map((n) => (
                    <li key={n.id} className={`border-b border-slate-100 px-3 py-2 text-sm ${n.is_read ? "opacity-60" : ""}`}>
                      <div className="flex items-start justify-between gap-2">
                        <div>
                          <div className="font-medium text-[#1a1f36]">{n.title}</div>
                          {n.body && <div className="text-xs text-slate-500">{n.body}</div>}
                        </div>
                        {!n.is_read && (
                          <button onClick={() => markRead.mutate(n.id)} aria-label={`Mark ${n.title} read`} className="text-xs text-[#0052cc] hover:underline">
                            Read
                          </button>
                        )}
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
            {list.data && active.length === 0 && <div className="px-3 py-4 text-sm text-slate-400">No notifications</div>}
          </div>
        </div>
      )}
    </div>
  );
}
