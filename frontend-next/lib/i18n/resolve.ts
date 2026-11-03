// Pure locale resolution (no React, safe on the server and the client).

import type { Locale } from "./types";

/** Cookie mirroring the last resolved locale (read by the root layout on the server). */
export const LOCALE_COOKIE = "heureum_locale";

/**
 * Maps a stored / browser language tag to a supported locale:
 * `it`, `it-IT`, `it_IT` → "it"; `en`, `en-US` → "en"; anything else
 * (empty, `es`, `fr`, garbage) → null (= unset).
 */
export function normalizeLocale(value?: string | null): Locale | null {
  const primary = (value ?? "").trim().toLowerCase().split(/[-_]/)[0];
  if (primary === "it") return "it";
  if (primary === "en") return "en";
  return null;
}

/**
 * First match wins: the user's saved locale → the `heureum_locale` cookie →
 * the browser language → "en".
 */
export function resolveLocale(input: {
  user?: string | null;
  cookie?: string | null;
  browser?: string | null;
}): Locale {
  return (
    normalizeLocale(input.user) ??
    normalizeLocale(input.cookie) ??
    normalizeLocale(input.browser) ??
    "en"
  );
}

/**
 * The browser's preferred language from an `Accept-Language` header — its first
 * tag, i.e. the server-side equivalent of `navigator.language`.
 */
export function firstAcceptLanguage(header?: string | null): string | null {
  const first = (header ?? "").split(",")[0]?.split(";")[0]?.trim();
  return first ? first : null;
}
