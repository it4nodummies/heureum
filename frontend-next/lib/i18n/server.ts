// Server-only locale resolution for a request (server components, layouts,
// generateMetadata). Kept out of `@/lib/i18n` (index) so client bundles never
// pull in `next/headers`.
import { cookies, headers } from "next/headers";
import { LOCALE_COOKIE, firstAcceptLanguage, resolveLocale } from "./resolve";
import type { Locale } from "./types";

/**
 * Locale of the current request: the `heureum_locale` cookie (last resolved
 * locale) → the browser's Accept-Language → en. The client re-resolves against
 * the user's saved locale once /myself loads (components/layout/LocaleSync).
 */
export async function requestLocale(): Promise<Locale> {
  const [cookieStore, headerList] = await Promise.all([cookies(), headers()]);
  return resolveLocale({
    cookie: cookieStore.get(LOCALE_COOKIE)?.value,
    browser: firstAcceptLanguage(headerList.get("accept-language")),
  });
}
