"use client";

import { useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { notifications, type AppNotification } from "@/lib/api";
import { useFormat, useT, type MessageKey, type MessageParams } from "@/lib/i18n";

type Tab = "direct" | "watching";

// Nessun campo Direct/Watching persistito: classifichiamo per `type`.
const DIRECT_TYPES = new Set(["assignment", "mention"]);
const WATCHING_TYPES = new Set(["comment", "status_change", "sprint_started", "sprint_completed"]);

function tabOf(n: AppNotification): Tab {
  return DIRECT_TYPES.has(n.type) ? "direct" : "watching";
}

// La `link` contiene un URL con la issue key (es. /issues/DEMO-1 o /browse/DEMO-1).
function issueKeyOf(n: AppNotification): string | null {
  const fromLink = (n.link || "").match(/\/(?:issues|browse)\/([A-Z][A-Z0-9]+-\d+)/);
  if (fromLink) return fromLink[1];
  const m = (n.link || n.title || "").match(/([A-Z][A-Z0-9]+-\d+)/);
  return m ? m[1] : null;
}

type T = (key: MessageKey, params?: MessageParams) => string;

const KNOWN_TYPES = new Set(["assignment", "comment", "mention", "sprint_started", "sprint_completed", "status_change"]);

// Title rendered client-side from the notification `type` (+ the key from `link`),
// so it follows the UI language. Unknown types fall back to the stored title.
function notificationTitle(n: AppNotification, t: T): string {
  if (!KNOWN_TYPES.has(n.type)) return n.title;
  const key = issueKeyOf(n);
  const needsKey = n.type === "comment" || n.type === "mention" || n.type === "status_change";
  if (needsKey && !key) return n.title;
  return t(`notifications.types.${n.type}` as MessageKey, { key: key ?? "" });
}

// The stored body is data (issue summary, comment text, sprint/status name), but
// for some types the backend wraps it in a fixed English sentence
// (internal/domain/notification/service.go). Recognise those sentences, keep
// only their data parts and re-render them in the UI language; anything else is
// shown as stored.
function notificationBody(n: AppNotification, t: T): string {
  const body = n.body || "";
  let m: RegExpMatchArray | null;
  switch (n.type) {
    case "assignment":
      if ((m = body.match(/^You were assigned to ([A-Z][A-Z0-9]+-\d+): ([\s\S]*)$/)))
        return t("notifications.bodies.assignment", { key: m[1], summary: m[2] });
      break;
    case "mention":
      if ((m = body.match(/^Someone mentioned you in the comments of ([A-Z][A-Z0-9]+-\d+): ([\s\S]*)$/)))
        return t("notifications.bodies.mention", { key: m[1], summary: m[2] });
      break;
    case "sprint_started":
      if ((m = body.match(/^Sprint "([\s\S]*)" has started$/)))
        return t("notifications.bodies.sprint_started", { name: m[1] });
      break;
    case "sprint_completed":
      if ((m = body.match(/^Sprint "([\s\S]*)" has been completed$/)))
        return t("notifications.bodies.sprint_completed", { name: m[1] });
      break;
    case "status_change": {
      const marker = " status changed to ";
      const at = body.lastIndexOf(marker);
      if (at >= 0)
        return t("notifications.bodies.status_change", {
          summary: body.slice(0, at),
          status: body.slice(at + marker.length),
        });
      break;
    }
  }
  return body;
}

type Group = { key: string; label: string | null; items: AppNotification[] };

function groupByIssue(items: AppNotification[]): Group[] {
  const order: string[] = [];
  const map = new Map<string, Group>();
  for (const n of items) {
    const key = issueKeyOf(n);
    const gkey = key ?? "__other__";
    if (!map.has(gkey)) {
      map.set(gkey, { key: gkey, label: key, items: [] });
      order.push(gkey);
    }
    map.get(gkey)!.items.push(n);
  }
  return order.map((k) => map.get(k)!);
}

export function NotificationBell() {
  const qc = useQueryClient();
  const t = useT();
  const fmt = useFormat();
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
        aria-label={t("notifications.title")}
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
            <span className="text-sm font-semibold text-[#1a1f36]">{t("notifications.title")}</span>
            <button onClick={() => markAll.mutate()} className="text-xs text-[#0052cc] hover:underline">{t("notifications.markAllRead")}</button>
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
              {directUnread > 0
                ? t("notifications.withCount", { label: t("notifications.direct"), count: directUnread })
                : t("notifications.direct")}
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
              {watchingUnread > 0
                ? t("notifications.withCount", { label: t("notifications.watching"), count: watchingUnread })
                : t("notifications.watching")}
            </button>
          </div>
          <div className="max-h-96 overflow-auto">
            {groups.map((g) => (
              <div key={g.key}>
                <div className="bg-slate-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-slate-400">
                  {g.label ?? t("notifications.other")}
                </div>
                <ul>
                  {g.items.map((n) => {
                    const title = notificationTitle(n, t);
                    const body = notificationBody(n, t);
                    return (
                    <li key={n.id} className={`border-b border-slate-100 px-3 py-2 text-sm ${n.is_read ? "opacity-60" : ""}`}>
                      <div className="flex items-start justify-between gap-2">
                        <div>
                          <div className="font-medium text-[#1a1f36]">{title}</div>
                          {body && <div className="text-xs text-slate-500">{body}</div>}
                          {n.created_at && (
                            <time dateTime={n.created_at} title={fmt.dateTime(n.created_at)} className="text-[11px] text-slate-400">
                              {fmt.relative(n.created_at)}
                            </time>
                          )}
                        </div>
                        {!n.is_read && (
                          <button onClick={() => markRead.mutate(n.id)} aria-label={t("notifications.markRead", { title })} className="text-xs text-[#0052cc] hover:underline">
                            {t("notifications.read")}
                          </button>
                        )}
                      </div>
                    </li>
                    );
                  })}
                </ul>
              </div>
            ))}
            {list.data && active.length === 0 && <div className="px-3 py-4 text-sm text-slate-400">{t("notifications.empty")}</div>}
          </div>
        </div>
      )}
    </div>
  );
}
