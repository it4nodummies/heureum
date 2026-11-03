// Core i18n types. `en` (lib/i18n/messages/en) is the source of truth; every
// other locale is typed as `Catalog<typeof en…>`, so a missing or extra key in
// a translation is a TypeScript error (and `npm run build` fails).

import type { Messages } from "./messages/en";

export type Locale = "en" | "it";

export const LOCALES: readonly Locale[] = ["en", "it"];

/** Plural message: the form is chosen with `Intl.PluralRules(locale)` from `params.count`. */
export type Plural = { one: string; other: string };

/** True when V is exactly a `{ one, other }` plural object (no other keys). */
type IsPlural<V> = V extends Plural
  ? Exclude<keyof V, "one" | "other"> extends never
    ? true
    : false
  : false;

/**
 * Same keys as T (no more, no fewer), `string` at every string leaf and `Plural`
 * wherever T has a plural object. Declare translations as
 * `const x: Catalog<typeof enX> = { … }` so the object literal is checked
 * deeply (missing keys → error, extra keys → excess-property error).
 */
export type Catalog<T> = {
  [K in keyof T]: T[K] extends string
    ? string
    : IsPlural<T[K]> extends true
      ? Plural
      : Catalog<T[K]>;
};

/** Union of the dotted paths of every leaf (string or plural) of T. */
export type DottedKeys<T, Prefix extends string = ""> = {
  [K in keyof T & string]: T[K] extends string
    ? `${Prefix}${K}`
    : IsPlural<T[K]> extends true
      ? `${Prefix}${K}`
      : DottedKeys<T[K], `${Prefix}${K}.`>;
}[keyof T & string];

/** Every valid message key, e.g. `"layout.nav.projects"`. A typo is a type error. */
export type MessageKey = DottedKeys<Messages>;

export type MessageParams = Record<string, string | number>;
