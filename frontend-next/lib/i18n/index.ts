// Public entry point of the hand-rolled i18n layer (no dependencies).
// Server code that must not pull in React (e.g. the root layout) can import
// `@/lib/i18n/resolve` / `@/lib/i18n/translate` directly.
export type { Locale, Plural, Catalog, MessageKey, MessageParams, DottedKeys } from "./types";
export { LOCALES } from "./types";
export { LOCALE_COOKIE, resolveLocale, normalizeLocale, firstAcceptLanguage } from "./resolve";
export { translate } from "./translate";
export { I18nProvider, useT, useLocale, useFormat, readLocaleCookie, type Formatters } from "./I18nProvider";
