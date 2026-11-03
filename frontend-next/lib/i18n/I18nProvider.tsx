"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { LOCALE_COOKIE, resolveLocale } from "./resolve";
import { translate } from "./translate";
import type { Locale, MessageKey, MessageParams } from "./types";

interface I18nContextValue {
  locale: Locale;
  setLocale: (l: Locale) => void;
}

const I18nContext = createContext<I18nContextValue>({ locale: "en", setLocale: () => {} });

/** Reads the `heureum_locale` cookie in the browser (null on the server or when absent). */
export function readLocaleCookie(): string | null {
  if (typeof document === "undefined") return null;
  const prefix = `${LOCALE_COOKIE}=`;
  for (const part of document.cookie.split(";")) {
    const c = part.trim();
    if (c.startsWith(prefix)) {
      try {
        return decodeURIComponent(c.slice(prefix.length));
      } catch {
        return null; // malformed cookie: treat as absent
      }
    }
  }
  return null;
}

function writeLocaleCookie(l: Locale) {
  document.cookie = `${LOCALE_COOKIE}=${l}; path=/; max-age=31536000; samesite=lax`;
}

/**
 * Holds the active locale. `initialLocale` comes from the root layout (server:
 * cookie → Accept-Language → en), so the first client render matches the
 * server HTML — no flash, no hydration mismatch. `LocaleSync` (under /app)
 * re-resolves once the current user is known.
 */
export function I18nProvider({ initialLocale, children }: { initialLocale: Locale; children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(initialLocale);

  const setLocale = useCallback((l: Locale) => {
    setLocaleState(l);
    if (typeof document !== "undefined") {
      document.documentElement.lang = l;
      writeLocaleCookie(l);
    }
  }, []);

  // First visit (no cookie yet): resolve from the browser language and mirror it
  // into the cookie so the next server render (incl. /login) is already right.
  useEffect(() => {
    if (readLocaleCookie() === null) {
      setLocale(resolveLocale({ browser: navigator.language }));
    }
  }, [setLocale]);

  const value = useMemo(() => ({ locale, setLocale }), [locale, setLocale]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

/** Active locale and a setter that also updates `<html lang>` and the cookie. */
export function useLocale(): { locale: Locale; setLocale(l: Locale): void } {
  return useContext(I18nContext);
}

/** `t(key, params?)` for the active locale: `{name}` interpolation, plurals via `params.count`. */
export function useT(): (key: MessageKey, params?: MessageParams) => string {
  const { locale } = useContext(I18nContext);
  return useCallback((key: MessageKey, params?: MessageParams) => translate(locale, key, params), [locale]);
}

export interface Formatters {
  date(d: string | Date, opts?: Intl.DateTimeFormatOptions): string;
  dateTime(d: string | Date): string;
  relative(d: string | Date): string;
  number(n: number): string;
}

function toDate(d: string | Date): Date | null {
  const date = d instanceof Date ? d : new Date(d);
  return Number.isNaN(date.getTime()) ? null : date;
}

const DATE_DEFAULT: Intl.DateTimeFormatOptions = { day: "numeric", month: "short", year: "numeric" };
const DATE_TIME: Intl.DateTimeFormatOptions = { ...DATE_DEFAULT, hour: "2-digit", minute: "2-digit" };

const RELATIVE_STEPS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["second", 60],
  ["minute", 60],
  ["hour", 24],
  ["day", 30],
  ["month", 12],
  ["year", Infinity],
];

/** Locale-aware date / relative time / number formatting (platform `Intl`). */
export function useFormat(): Formatters {
  const { locale } = useContext(I18nContext);
  return useMemo<Formatters>(() => {
    const dateTimeFmt = new Intl.DateTimeFormat(locale, DATE_TIME);
    const defaultDateFmt = new Intl.DateTimeFormat(locale, DATE_DEFAULT);
    const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
    const numberFmt = new Intl.NumberFormat(locale);
    return {
      date(d, opts) {
        const date = toDate(d);
        if (!date) return String(d);
        return (opts ? new Intl.DateTimeFormat(locale, opts) : defaultDateFmt).format(date);
      },
      dateTime(d) {
        const date = toDate(d);
        return date ? dateTimeFmt.format(date) : String(d);
      },
      relative(d) {
        const date = toDate(d);
        if (!date) return String(d);
        let value = (date.getTime() - Date.now()) / 1000;
        for (const [unit, size] of RELATIVE_STEPS) {
          if (Math.abs(Math.round(value)) < size) return rtf.format(Math.round(value), unit);
          value /= size;
        }
        return rtf.format(Math.round(value), "year");
      },
      number(n) {
        return numberFmt.format(n);
      },
    };
  }, [locale]);
}
