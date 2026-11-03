"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { auth, profile, notifications, type NotificationSetting, type JiraUser } from "@/lib/api";
import { ThemeToggle } from "@/components/layout/ThemeProvider";
import { normalizeLocale, resolveLocale, useFormat, useLocale, useT, type MessageKey } from "@/lib/i18n";

const inputCls =
  "rounded border border-slate-300 px-2 py-1 text-sm dark:border-[#2a3142] dark:bg-[#0e1116] dark:text-[#e6e8eb]";

// Notification event codes offered in the preferences form (values sent to the API).
const EVENT_TYPES = ["assignment", "comment", "mention", "status_change", "sprint_started", "sprint_completed"] as const;

function eventLabelKey(code: string): MessageKey | null {
  return (EVENT_TYPES as readonly string[]).includes(code) ? (`profile.prefs.events.${code}` as MessageKey) : null;
}

// Personal API tokens: list / create (plaintext shown once) / revoke. The list
// never carries the hash or the plaintext — only metadata from the server.
function ApiTokens() {
  const qc = useQueryClient();
  const t = useT();
  const fmt = useFormat();
  const formatDate = (iso?: string) => (iso ? fmt.date(iso) : t("common.never"));
  const tokens = useQuery({ queryKey: ["profile", "api-tokens"], queryFn: auth.listApiTokens });
  const [label, setLabel] = useState("");
  const [expiresInDays, setExpiresInDays] = useState("");
  const [justCreated, setJustCreated] = useState<{ label: string; token: string } | null>(null);

  const create = useMutation({
    mutationFn: () => {
      const days = expiresInDays.trim() === "" ? undefined : Number(expiresInDays);
      return auth.createApiToken(label.trim(), days);
    },
    onSuccess: (created) => {
      setJustCreated({ label: created.label, token: created.token });
      setLabel("");
      setExpiresInDays("");
      qc.invalidateQueries({ queryKey: ["profile", "api-tokens"] });
    },
  });

  const revoke = useMutation({
    mutationFn: (id: string) => auth.revokeApiToken(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["profile", "api-tokens"] }),
  });

  const daysInvalid = expiresInDays.trim() !== "" && !(Number(expiresInDays) > 0);

  return (
    <section
      className="mb-6 rounded border border-slate-200 bg-white p-4 dark:border-[#2a3142] dark:bg-[#1a1f2e]"
      data-testid="api-tokens"
    >
      <h2 className="mb-1 text-sm font-semibold text-[#1a1f36] dark:text-[#e6e8eb]">{t("profile.tokens.title")}</h2>
      <p className="mb-3 text-xs text-slate-500 dark:text-[#8b95a5]">
        {t("profile.tokens.help")}
      </p>

      <ul className="space-y-1 text-sm">
        {(tokens.data ?? []).map((tk) => (
          <li
            key={tk.id}
            data-testid={`api-token-row-${tk.id}`}
            className="flex items-center justify-between gap-3 border-b border-slate-100 py-1 dark:border-[#2a3142]"
          >
            <span className="min-w-0 text-[#1a1f36] dark:text-[#e6e8eb]">
              <span className="font-medium">{tk.label || t("profile.tokens.noLabel")}</span>
              <span className="text-slate-400">
                {" "}
                · {t("profile.tokens.created", { date: formatDate(tk.created_at) })} · {t("profile.tokens.expires")}{" "}
                <span data-testid={`api-token-expires-${tk.id}`}>{formatDate(tk.expires_at)}</span>
              </span>
            </span>
            <button
              type="button"
              data-testid={`api-token-revoke-${tk.id}`}
              disabled={revoke.isPending}
              onClick={() => {
                if (window.confirm(t("profile.tokens.revokeConfirm", { name: tk.label || tk.id }))) {
                  revoke.mutate(tk.id);
                }
              }}
              className="shrink-0 rounded border border-red-300 px-2 py-0.5 text-xs text-red-700 hover:bg-red-50 disabled:opacity-60 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-950"
            >
              {t("profile.tokens.revoke")}
            </button>
          </li>
        ))}
        {tokens.data && tokens.data.length === 0 && <li className="py-2 text-slate-400">{t("profile.tokens.empty")}</li>}
      </ul>
      {revoke.isError && (
        <p className="mt-1 text-xs text-red-600">{(revoke.error as Error)?.message ?? t("profile.tokens.revokeFailed")}</p>
      )}

      {justCreated && (
        <div
          data-testid="api-token-created"
          className="mt-3 rounded border border-amber-300 bg-amber-50 p-2 text-xs text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200"
        >
          <p className="mb-1 font-semibold">{t("profile.tokens.createdNotice", { label: justCreated.label })}</p>
          <code data-testid="api-token-plaintext" className="block select-all break-all font-mono">
            {justCreated.token}
          </code>
        </div>
      )}

      <form
        data-testid="api-token-create-form"
        className="mt-4 flex flex-wrap items-end gap-3 border-t border-slate-100 pt-4 dark:border-[#2a3142]"
        onSubmit={(e) => {
          e.preventDefault();
          if (!daysInvalid) create.mutate();
        }}
      >
        <div>
          <label className="mb-1 block text-xs font-semibold text-slate-500">{t("profile.tokens.label")}</label>
          <input
            aria-label={t("profile.tokens.labelAria")}
            data-testid="api-token-label-input"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder={t("profile.tokens.labelPlaceholder")}
            maxLength={255}
            className={inputCls}
          />
        </div>
        <div>
          <label className="mb-1 block text-xs font-semibold text-slate-500">{t("profile.tokens.expiresIn")}</label>
          <input
            aria-label={t("profile.tokens.expiresInAria")}
            data-testid="api-token-expires-input"
            type="number"
            min={1}
            step={1}
            value={expiresInDays}
            onChange={(e) => setExpiresInDays(e.target.value)}
            placeholder={t("common.never")}
            className={`${inputCls} w-28`}
          />
        </div>
        <button
          type="submit"
          data-testid="api-token-create"
          disabled={create.isPending || daysInvalid}
          className="rounded bg-[#0052cc] px-3 py-1.5 text-sm text-white disabled:opacity-60"
        >
          {t("profile.tokens.create")}
        </button>
        {daysInvalid && <span className="pb-1.5 text-xs text-red-600">{t("profile.tokens.daysInvalid")}</span>}
        {create.isError && (
          <span className="pb-1.5 text-xs text-red-600">{(create.error as Error)?.message ?? t("profile.tokens.createFailed")}</span>
        )}
      </form>
    </section>
  );
}

// Editable identity fields. Rendered only once `me` has loaded, and seeds its
// local state LAZILY from the loaded user (no post-mount useEffect reset). This
// keeps the inputs stable: an unrelated re-render (a background query resolving,
// theme context, etc.) can never re-assert a stale controlled value mid-edit,
// which otherwise races fast programmatic input.
function ProfileDetails({ me }: { me: JiraUser }) {
  const qc = useQueryClient();
  const t = useT();
  const { setLocale: applyLocale } = useLocale();
  const [displayName, setDisplayName] = useState(me.displayName ?? "");
  const [timeZone, setTimeZone] = useState(me.timeZone ?? "");
  // Only Default ("") / English / Italiano are offered; a legacy stored value
  // (es, fr, de, pt, …) is shown — and saved back — as Default.
  const [locale, setLocale] = useState<string>(normalizeLocale(me.locale) ?? "");

  const save = useMutation({
    mutationFn: () => profile.update({ displayName, timeZone, locale }),
    onSuccess: (updated) => {
      // Keep the ["myself"] cache (read by LocaleSync) in step BEFORE switching,
      // so LocaleSync never re-applies the previous saved locale.
      qc.setQueryData(["myself"], updated);
      qc.invalidateQueries({ queryKey: ["myself"] });
      qc.invalidateQueries({ queryKey: ["profile", "me"] });
      // Apply immediately, resolving exactly like LocaleSync (saved locale →
      // browser; never the cookie): "Default" follows the browser.
      applyLocale(resolveLocale({ user: locale, browser: navigator.language }));
    },
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
      <label className="mb-1 block text-xs font-semibold text-slate-500">{t("profile.details.avatar")}</label>
      <div className="mb-3 flex items-center gap-3">
        {me.avatarUrls?.["48x48"] ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={me.avatarUrls["48x48"]}
            alt={t("profile.details.avatar")}
            className="h-12 w-12 rounded-full border border-slate-200 object-cover"
          />
        ) : (
          <span className="flex h-12 w-12 items-center justify-center rounded-full bg-slate-200 text-sm font-semibold text-slate-600">
            {initials}
          </span>
        )}
        <div>
          <label className="inline-block cursor-pointer rounded border border-slate-300 px-3 py-1.5 text-sm text-[#1a1f36] hover:bg-slate-50 dark:border-[#2a3142] dark:text-[#e6e8eb] dark:hover:bg-[#232a3a]">
            {uploadAvatar.isPending ? t("profile.details.uploading") : t("profile.details.uploadAvatar")}
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
              {(uploadAvatar.error as Error)?.message ?? t("profile.details.uploadFailed")}
            </p>
          )}
        </div>
      </div>

      <label className="mb-1 block text-xs font-semibold text-slate-500">{t("profile.details.displayName")}</label>
      <input
        aria-label={t("profile.details.displayName")}
        value={displayName}
        onChange={(e) => setDisplayName(e.target.value)}
        className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm dark:border-[#2a3142] dark:bg-[#0e1116] dark:text-[#e6e8eb]"
      />
      <label className="mb-1 block text-xs font-semibold text-slate-500">{t("profile.details.timeZone")}</label>
      <input
        aria-label={t("profile.details.timeZone")}
        value={timeZone}
        onChange={(e) => setTimeZone(e.target.value)}
        placeholder="Europe/Rome"
        className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm dark:border-[#2a3142] dark:bg-[#0e1116] dark:text-[#e6e8eb]"
      />
      <label className="mb-1 block text-xs font-semibold text-slate-500">{t("profile.details.language")}</label>
      <select
        data-testid="profile-locale"
        aria-label={t("profile.details.language")}
        value={locale}
        onChange={(e) => setLocale(e.target.value)}
        className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm dark:border-[#2a3142] dark:bg-[#0e1116] dark:text-[#e6e8eb]"
      >
        <option value="">{t("profile.details.languageDefault")}</option>
        {/* Language names are shown in their own language, whatever the UI locale. */}
        <option value="en">English</option>
        <option value="it">Italiano</option>
      </select>
      <button
        onClick={() => save.mutate()}
        disabled={save.isPending}
        className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
      >
        {t("profile.details.save")}
      </button>
      <p className="mt-2 text-xs text-slate-500">{me.emailAddress}</p>
    </section>
  );
}

export default function ProfilePage() {
  const qc = useQueryClient();
  const t = useT();
  const eventLabel = (code: string) => {
    const key = eventLabelKey(code);
    return key ? t(key) : code;
  };
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
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36] dark:text-[#e6e8eb]">{t("profile.title")}</h1>

      <section className="mb-6 rounded border border-slate-200 bg-white p-4 dark:border-[#2a3142] dark:bg-[#1a1f2e]">
        <label className="mb-1 block text-xs font-semibold text-slate-500">{t("profile.appearance")}</label>
        <div className="flex items-center gap-3">
          <ThemeToggle />
          <span className="text-sm text-slate-500 dark:text-[#8b95a5]">{t("profile.themeHint")}</span>
        </div>
      </section>

      {me.data ? (
        <ProfileDetails me={me.data} />
      ) : (
        <section className="mb-6 rounded border border-slate-200 bg-white p-4 dark:border-[#2a3142] dark:bg-[#1a1f2e]">
          <p className="text-sm text-slate-400">{t("profile.loading")}</p>
        </section>
      )}

      <ApiTokens />

      <section className="rounded border border-slate-200 bg-white p-4 dark:border-[#2a3142] dark:bg-[#1a1f2e]" data-testid="notif-prefs">
        <h2 className="mb-2 text-sm font-semibold text-[#1a1f36] dark:text-[#e6e8eb]">{t("profile.prefs.title")}</h2>
        <ul className="space-y-1 text-sm">
          {(settings.data ?? []).map((s) => (
            <li
              key={`${s.project_id}:${s.event_type}`}
              data-testid="notif-pref-row"
              data-event={s.event_type}
              className="flex items-center justify-between border-b border-slate-100 py-1 dark:border-[#2a3142]"
            >
              <span className="text-[#1a1f36] dark:text-[#e6e8eb]">
                <span className="font-medium">{eventLabel(s.event_type)}</span>
                <span className="text-slate-400"> · {s.project_id ? t("profile.prefs.projectScoped") : t("profile.prefs.allProjects")}</span>
              </span>
              <span className="flex gap-3">
                <label className="flex items-center gap-1 text-xs">
                  <input type="checkbox" checked={s.via_app} onChange={(e) => toggleApp(s, e.target.checked)} /> {t("profile.prefs.app")}
                </label>
                <label className="flex items-center gap-1 text-xs">
                  <input type="checkbox" checked={s.via_email} onChange={(e) => toggleEmail(s, e.target.checked)} /> {t("profile.prefs.email")}
                </label>
              </span>
            </li>
          ))}
          {settings.data && settings.data.length === 0 && (
            <li className="py-2 text-slate-400">{t("profile.prefs.defaults")}</li>
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
            <label className="mb-1 block text-xs font-semibold text-slate-500">{t("profile.prefs.eventType")}</label>
            <select
              aria-label={t("profile.prefs.eventType")}
              value={newEvent}
              onChange={(e) => setNewEvent(e.target.value)}
              className="rounded border border-slate-300 px-2 py-1 text-sm dark:border-[#2a3142] dark:bg-[#0e1116] dark:text-[#e6e8eb]"
            >
              {EVENT_TYPES.map((code) => (
                <option key={code} value={code}>
                  {eventLabel(code)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="mb-1 block text-xs font-semibold text-slate-500">{t("profile.prefs.scope")}</label>
            <span className="block rounded border border-slate-200 bg-slate-50 px-2 py-1 text-sm text-slate-500 dark:border-[#2a3142] dark:bg-[#0e1116]">
              {t("profile.prefs.allProjects")}
            </span>
          </div>
          <label className="flex items-center gap-1 pb-1.5 text-xs">
            <input type="checkbox" aria-label={t("profile.prefs.appOption")} checked={newApp} onChange={(e) => setNewApp(e.target.checked)} /> {t("profile.prefs.appOption")}
          </label>
          <label className="flex items-center gap-1 pb-1.5 text-xs">
            <input type="checkbox" aria-label={t("profile.prefs.emailOption")} checked={newEmail} onChange={(e) => setNewEmail(e.target.checked)} /> {t("profile.prefs.emailOption")}
          </label>
          <button
            type="submit"
            disabled={updateSetting.isPending}
            className="rounded bg-[#0052cc] px-3 py-1.5 text-sm text-white disabled:opacity-60"
          >
            {t("profile.prefs.add")}
          </button>
        </form>
      </section>
    </div>
  );
}
