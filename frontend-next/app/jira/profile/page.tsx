"use client";

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { profile, notifications, type NotificationSetting } from "@/lib/api";

export default function ProfilePage() {
  const qc = useQueryClient();
  const me = useQuery({ queryKey: ["profile", "me"], queryFn: profile.me });
  const [displayName, setDisplayName] = useState("");
  const [timeZone, setTimeZone] = useState("");

  useEffect(() => {
    if (me.data) {
      setDisplayName(me.data.displayName ?? "");
      setTimeZone(me.data.timeZone ?? "");
    }
  }, [me.data]);

  const save = useMutation({
    mutationFn: () => profile.update({ displayName, timeZone }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["profile", "me"] }),
  });

  const settings = useQuery({ queryKey: ["notif", "settings"], queryFn: notifications.settings });
  const updateSetting = useMutation({
    mutationFn: (s: { project_id?: string; event_type: string; via_email: boolean; via_app: boolean }) =>
      notifications.updateSettings(s),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notif", "settings"] }),
  });

  const toggleApp = (s: NotificationSetting, via_app: boolean) =>
    updateSetting.mutate({ project_id: s.project_id, event_type: s.event_type, via_app, via_email: s.via_email });
  const toggleEmail = (s: NotificationSetting, via_email: boolean) =>
    updateSetting.mutate({ project_id: s.project_id, event_type: s.event_type, via_app: s.via_app, via_email });

  return (
    <div className="mx-auto max-w-2xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">Profile</h1>

      <section className="mb-6 rounded border border-slate-200 bg-white p-4">
        <label className="mb-1 block text-xs font-semibold text-slate-500">Display name</label>
        <input
          aria-label="Display name"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm"
        />
        <label className="mb-1 block text-xs font-semibold text-slate-500">Time zone</label>
        <input
          aria-label="Time zone"
          value={timeZone}
          onChange={(e) => setTimeZone(e.target.value)}
          placeholder="Europe/Rome"
          className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm"
        />
        <button
          onClick={() => save.mutate()}
          disabled={save.isPending}
          className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
        >
          Save profile
        </button>
        <p className="mt-2 text-xs text-slate-500">{me.data?.emailAddress}</p>
      </section>

      <section className="rounded border border-slate-200 bg-white p-4" data-testid="notif-prefs">
        <h2 className="mb-2 text-sm font-semibold text-[#1a1f36]">Notification preferences</h2>
        <ul className="space-y-1 text-sm">
          {(settings.data ?? []).map((s) => (
            <li key={`${s.project_id}:${s.event_type}`} className="flex items-center justify-between border-b border-slate-100 py-1">
              <span className="text-[#1a1f36]">
                {s.event_type}
                {s.project_id ? ` · ${s.project_id}` : ""}
              </span>
              <span className="flex gap-3">
                <label className="flex items-center gap-1 text-xs">
                  <input type="checkbox" checked={s.via_app} onChange={(e) => toggleApp(s, e.target.checked)} /> app
                </label>
                <label className="flex items-center gap-1 text-xs">
                  <input type="checkbox" checked={s.via_email} onChange={(e) => toggleEmail(s, e.target.checked)} /> email
                </label>
              </span>
            </li>
          ))}
          {settings.data && settings.data.length === 0 && (
            <li className="py-2 text-slate-400">Default preferences (all channels on)</li>
          )}
        </ul>
      </section>
    </div>
  );
}
