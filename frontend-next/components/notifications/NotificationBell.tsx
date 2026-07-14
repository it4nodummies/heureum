"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { notifications, type AppNotification } from "@/lib/api";

export function NotificationBell() {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);

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
          <ul className="max-h-96 overflow-auto">
            {(list.data ?? []).map((n: AppNotification) => (
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
            {list.data && list.data.length === 0 && <li className="px-3 py-4 text-sm text-slate-400">No notifications</li>}
          </ul>
        </div>
      )}
    </div>
  );
}
