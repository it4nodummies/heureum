// Pure message lookup + interpolation + plural selection (no React), shared by
// `useT()` and server code (root layout metadata).

import { messages } from "./messages";
import type { Locale, MessageKey, MessageParams, Plural } from "./types";

function lookup(tree: unknown, key: string): unknown {
  let node: unknown = tree;
  for (const part of key.split(".")) {
    if (node === null || typeof node !== "object") return undefined;
    node = (node as Record<string, unknown>)[part];
  }
  return node;
}

function isPlural(v: unknown): v is Plural {
  return (
    v !== null &&
    typeof v === "object" &&
    typeof (v as Plural).one === "string" &&
    typeof (v as Plural).other === "string"
  );
}

function interpolate(template: string, params?: MessageParams): string {
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : match
  );
}

const pluralRulesCache = new Map<Locale, Intl.PluralRules>();
function pluralRules(locale: Locale): Intl.PluralRules {
  let pr = pluralRulesCache.get(locale);
  if (!pr) {
    pr = new Intl.PluralRules(locale);
    pluralRulesCache.set(locale, pr);
  }
  return pr;
}

/**
 * Translates `key` in `locale`: `{name}` placeholders are replaced from
 * `params`; a plural value picks its form from `params.count`. Falls back to
 * English, then to the key itself — never throws.
 */
export function translate(locale: Locale, key: MessageKey | string, params?: MessageParams): string {
  let value = lookup(messages[locale], key);
  if (value === undefined && locale !== "en") value = lookup(messages.en, key);
  if (typeof value === "string") return interpolate(value, params);
  if (isPlural(value)) {
    if (process.env.NODE_ENV !== "production" && typeof params?.count !== "number") {
      // A plural without a numeric `count` silently picks a form and leaves a
      // raw `{count}` in the text — always a call-site bug.
      console.warn(`[i18n] plural key "${key}" used without a numeric count`);
    }
    const count = Number(params?.count ?? 0);
    const form = pluralRules(locale).select(count) === "one" ? value.one : value.other;
    return interpolate(form, params);
  }
  return key;
}
