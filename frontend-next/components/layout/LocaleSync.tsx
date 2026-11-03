"use client";

import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { profile } from "@/lib/api";
import { isAuthenticated } from "@/lib/auth";
import { resolveLocale, useLocale } from "@/lib/i18n";

/**
 * Once the current user is known, re-resolves the locale (saved profile locale
 * → browser → en) and applies it when it differs from the one the server
 * rendered with. The cookie is deliberately NOT consulted here: it only serves
 * the server's first paint (and is rewritten by `setLocale`), so a user with an
 * empty/legacy saved locale follows the browser rather than whatever the
 * previous session on this browser chose. Uses the app-wide ["myself"] query
 * (see UserPicker).
 */
export default function LocaleSync() {
  const { locale, setLocale } = useLocale();
  const me = useQuery({
    queryKey: ["myself"],
    queryFn: () => profile.me(),
    enabled: typeof window !== "undefined" && isAuthenticated(),
  });

  useEffect(() => {
    if (!me.data) return;
    const next = resolveLocale({
      user: me.data.locale,
      browser: navigator.language,
    });
    if (next !== locale) setLocale(next);
  }, [me.data, locale, setLocale]);

  return null;
}
