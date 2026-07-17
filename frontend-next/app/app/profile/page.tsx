"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { profile, notifications, type NotificationSetting, type JiraUser } from "@/lib/api";
import { ThemeToggle } from "@/components/layout/ThemeProvider";

// Editable identity fields. Rendered only once `me` has loaded, and seeds its
// local state LAZILY from the loaded user (no post-mount useEffect reset). This
// keeps the inputs stable: an unrelated re-render (a background query resolving,
// theme context, etc.) can never re-assert a stale controlled value mid-edit,
// which otherwise races fast programmatic input.
function ProfileDetails({ me }: { me: JiraUser }) {
  const qc = useQueryClient();
  const [displayName, setDisplayName] = useState(me.displayName ?? "");
  const [timeZone, setTimeZone] = useState(me.timeZone ?? "");
  const [locale, setLocale] = useState(me.locale ?? "");

  const save = useMutation({
    mutationFn: () => profile.update({ displayName, timeZone, locale }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["profile", "me"] }),
  });

  const uploadAvatar = useMutation({
    mutationFn: (file: File) => profile.uploadAvatar(file),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["profile", "me"] }),
  });

  const initials = (me.displayName ?? me.emailAddress ?? "?")
    .trim()
    .split(/\s+/)
    .map((p) => p[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();

  return (
    <section className="mb-6 rounded border border-slate-200 bg-white p-4 dark:border-[#2a3142] dark:bg-[#1a1f2e]">
      <label className="mb-1 block text-xs font-semibold text-slate-500">Avatar</label>
      <div className="mb-3 flex items-center gap-3">
        {me.avatarUrls?.["48x48"] ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={me.avatarUrls["48x48"]}
            alt="Avatar"
            className="h-12 w-12 rounded-full border border-slate-200 object-cover"
          />
        ) : (
          <span className="flex h-12 w-12 items-center justify-center rounded-full bg-slate-200 text-sm font-semibold text-slate-600">
            {initials}
          </span>
        )}
        <div>
          <label className="inline-block cursor-pointer rounded border border-slate-300 px-3 py-1.5 text-sm text-[#1a1f36] hover:bg-slate-50 dark:border-[#2a3142] dark:text-[#e6e8eb] dark:hover:bg-[#232a3a]">
            {uploadAvatar.isPending ? "Uploading…" : "Upload avatar"}
            <input
              data-testid="avatar-upload"
              type="file"
              accept="image/*"
              className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) uploadAvatar.mutate(f);
                e.target.value = "";
              }}
            />
          </label>
          {uploadAvatar.isError && (
            <p className="mt-1 text-xs text-red-600">
              {(uploadAvatar.error as Error)?.message ?? "Upload failed"}
            </p>
          )}
        </div>
      </div>

      <label className="mb-1 block text-xs font-semibold text-slate-500">Display name</label>
      <input
        aria-label="Display name"
        value={displayName}
        onChange={(e) => setDisplayName(e.target.value)}
        className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm dark:border-[#2a3142] dark:bg-[#0e1116] dark:text-[#e6e8eb]"
      />
      <label className="mb-1 block text-xs font-semibold text-slate-500">Time zone</label>
      <input
        aria-label="Time zone"
        value={timeZone}
        onChange={(e) => setTimeZone(e.target.value)}
        placeholder="Europe/Rome"
        className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm dark:border-[#2a3142] dark:bg-[#0e1116] dark:text-[#e6e8eb]"
      />
      <label className="mb-1 block text-xs font-semibold text-slate-500">Language</label>
      <select
        data-testid="profile-locale"
        aria-label="Language"
        value={locale}
        onChange={(e) => setLocale(e.target.value)}
        className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm dark:border-[#2a3142] dark:bg-[#0e1116] dark:text-[#e6e8eb]"
      >
        <option value="">Default</option>
        <option value="en">English</option>
        <option value="it">Italiano</option>
        <option value="es">Español</option>
        <option value="fr">Français</option>
        <option value="de">Deutsch</option>
        <option value="pt">Português</option>
      </select>
      <button
        onClick={() => save.mutate()}
        disabled={save.isPending}
        className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
      >
        Save profile
      </button>
      <p className="mt-2 text-xs text-slate-500">{me.emailAddress}</p>
    </section>
  );
}

export default function ProfilePage() {
  const qc = useQueryClient();
  const me = useQuery({ queryKey: ["profile", "me"], queryFn: profile.me });

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

  // Add-a-preference form state. Scope is fixed to "All projects" (project_id: "")
  // because the settings API keys prefs on the internal project UUID (projects.id),
  // which no frontend API exposes — the projects list only returns seq_id/key/name.
  const [newEvent, setNewEvent] = useState("assignment");
  const [newApp, setNewApp] = useState(true);
  const [newEmail, setNewEmail] = useState(false);
  const addPref = () =>
    updateSetting.mutate({ project_id: "", event_type: newEvent, via_app: newApp, via_email: newEmail });

  return (
    <div className="mx-auto max-w-2xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36] dark:text-[#e6e8eb]">Profile</h1>

      <section className="mb-6 rounded border border-slate-200 bg-white p-4 dark:border-[#2a3142] dark:bg-[#1a1f2e]">
        <label className="mb-1 block text-xs font-semibold text-slate-500">Appearance</label>
        <div className="flex items-center gap-3">
          <ThemeToggle />
          <span className="text-sm text-slate-500 dark:text-[#8b95a5]">Toggle light / dark theme</span>
        </div>
      </section>

      {me.data ? (
        <ProfileDetails me={me.data} />
      ) : (
        <section className="mb-6 rounded border border-slate-200 bg-white p-4 dark:border-[#2a3142] dark:bg-[#1a1f2e]">
          <p className="text-sm text-slate-400">Loading profile…</p>
        </section>
      )}

      <section className="rounded border border-slate-200 bg-white p-4 dark:border-[#2a3142] dark:bg-[#1a1f2e]" data-testid="notif-prefs">
        <h2 className="mb-2 text-sm font-semibold text-[#1a1f36] dark:text-[#e6e8eb]">Notification preferences</h2>
        <ul className="space-y-1 text-sm">
          {(settings.data ?? []).map((s) => (
            <li
              key={`${s.project_id}:${s.event_type}`}
              data-testid="notif-pref-row"
              data-event={s.event_type}
              className="flex items-center justify-between border-b border-slate-100 py-1 dark:border-[#2a3142]"
            >
              <span className="text-[#1a1f36] dark:text-[#e6e8eb]">
                <span className="font-medium">{s.event_type}</span>
                <span className="text-slate-400"> · {s.project_id ? "Project-scoped" : "All projects"}</span>
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

        <form
          data-testid="add-pref-form"
          className="mt-4 flex flex-wrap items-end gap-3 border-t border-slate-100 pt-4 dark:border-[#2a3142]"
          onSubmit={(e) => {
            e.preventDefault();
            addPref();
          }}
        >
          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-500">Event type</label>
            <select
              aria-label="Event type"
              value={newEvent}
              onChange={(e) => setNewEvent(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1 text-sm dark:border-[#2a3142] dark:bg-[#0e1116] dark:text-[#e6e8eb]"
            >
              <option value="assignment">assignment</option>
              <option value="comment">comment</option>
              <option value="mention">mention</option>
              <option value="status_change">status_change</option>
              <option value="sprint_started">sprint_started</option>
              <option value="sprint_completed">sprint_completed</option>
            </select>
          </div>
          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-500">Scope</label>
            <span className="block rounded border border-slate-200 bg-slate-50 px-2 py-1 text-sm text-slate-500 dark:border-[#2a3142] dark:bg-[#0e1116]">
              All projects
            </span>
          </div>
          <label className="flex items-center gap-1 pb-1.5 text-xs">
            <input type="checkbox" aria-label="App" checked={newApp} onChange={(e) => setNewApp(e.target.checked)} /> App
          </label>
          <label className="flex items-center gap-1 pb-1.5 text-xs">
            <input type="checkbox" aria-label="Email" checked={newEmail} onChange={(e) => setNewEmail(e.target.checked)} /> Email
          </label>
          <button
            type="submit"
            disabled={updateSetting.isPending}
            className="rounded bg-[#0052cc] px-3 py-1.5 text-sm text-white disabled:opacity-60"
          >
            Add preference
          </button>
        </form>
      </section>
    </div>
  );
}
