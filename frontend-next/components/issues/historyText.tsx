import { Fragment, type ReactNode } from "react";
import type { MessageKey } from "@/lib/i18n";
import { issues as enIssues } from "@/lib/i18n/messages/en/issues";

// Shared by the issue History tab and the dashboard Activity stream gadget:
// both render backend issue_history rows as one translated sentence.

/**
 * Changelog field ids (`status`, `assignee`, `story_points`…) mapped to a
 * translated label (`issues.history.fields.*`); an id the catalogue doesn't
 * know is shown raw.
 */
export function historyFieldLabel(t: (key: MessageKey) => string, field: string): string {
  return Object.prototype.hasOwnProperty.call(enIssues.history.fields, field)
    ? t(`issues.history.fields.${field}` as MessageKey)
    : field;
}

// Placeholder marker: the sentence comes from the catalogue as one string, the
// author/field/link are spliced back in as styled nodes.
export const MARK = "\u0001";

/** `{name}` param value that `richSentence` replaces with `nodes[name]`. */
export const slot = (name: string) => `${MARK}${name}${MARK}`;

/** Values are data: drop the marker so they can't inject a node. */
export const stripMark = (s: string) => s.split(MARK).join("");

export function richSentence(text: string, nodes: Record<string, ReactNode>): ReactNode[] {
  return text
    .split(MARK)
    .map((part, i) => (i % 2 === 1 ? <Fragment key={i}>{nodes[part] ?? part}</Fragment> : part));
}
