"use client";

import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { profile } from "@/lib/api";
import { isAuthenticated } from "@/lib/auth";
import { readLocaleCookie, resolveLocale, useLocale } from "@/lib/i18n";

/**
 * Once the current user is known, re-resolves the locale (saved profile locale
 * → cookie → browser → en) and applies it when it differs from the one the
 * server rendered with. Uses the app-wide ["myself"] query (see UserPicker).
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
      cookie: readLocaleCookie(),
      browser: navigator.language,
    });
    if (next !== locale) setLocale(next);
  }, [me.data, locale, setLocale]);

  return null;
}
