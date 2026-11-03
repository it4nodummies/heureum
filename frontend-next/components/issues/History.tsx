"use client";

import { Fragment, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { issues, type ChangeItem, type Changelog } from "@/lib/api";
import { useFormat, useT, type MessageKey } from "@/lib/i18n";
import { issues as enIssues } from "@/lib/i18n/messages/en/issues";

// Changelog field ids (`status`, `assignee`, `story_points`…) are mapped to a
// translated label; an id the catalogue doesn't know is shown raw.
function fieldLabel(t: (key: MessageKey) => string, field: string): string {
  return Object.prototype.hasOwnProperty.call(enIssues.history.fields, field)
    ? t(`issues.history.fields.${field}` as MessageKey)
    : field;
}

// Placeholder marker: the sentence comes from the catalogue as one string, the
// author and field names are spliced back in as styled nodes.
const MARK = "\u0001";
const stripMark = (s: string) => s.split(MARK).join("");
function richSentence(text: string, nodes: Record<string, ReactNode>): ReactNode[] {
  return text
    .split(MARK)
    .map((part, i) => (i % 2 === 1 ? <Fragment key={i}>{nodes[part] ?? part}</Fragment> : part));
}

// Flattens a changelog entry (author/created + one or more field changes)
// into one row per changed field, keeping the entry's author/created/id so
// each row still has a stable key and its own relative timestamp.
interface HistoryRow {
  key: string;
  authorName: string;
  created: string;
  item: ChangeItem;
}

function toRows(entries: Changelog[], systemName: string): HistoryRow[] {
  const rows: HistoryRow[] = [];
  for (const entry of entries) {
    const authorName = entry.author?.displayName ?? systemName;
    entry.items.forEach((item, idx) => {
      rows.push({ key: `${entry.id}-${idx}`, authorName, created: entry.created, item });
    });
  }
  return rows;
}

export function History({ issueKey }: { issueKey: string }) {
  const t = useT();
  const fmt = useFormat();
  const { data, isLoading } = useQuery({
    queryKey: ["changelog", issueKey],
    queryFn: () => issues.changelog(issueKey),
  });

  // Backend already returns entries newest-first (ORDER BY created_at DESC in
  // issue.Service.GetHistory), but sort defensively here too so the UI
  // doesn't silently regress if that ever changes.
  const entries = [...(data?.values ?? [])].sort(
    (a, b) => new Date(b.created).getTime() - new Date(a.created).getTime()
  );
  const rows = toRows(entries, t("issues.history.system"));

  if (isLoading) {
    return <p className="text-sm text-slate-400">{t("issues.history.loading")}</p>;
  }

  if (rows.length === 0) {
    return <p className="text-sm text-slate-400">{t("issues.history.empty")}</p>;
  }

  return (
    <ul data-testid="history-list" className="space-y-2">
      {rows.map((row) => {
        // `toString` (and, defensively, `fromString`) collides with the name
        // of a property every JS object inherits from Object.prototype. When
        // the backend's JSON omits the field entirely, plain property access
        // (`row.item.toString`) falls through the prototype chain to
        // Function.prototype.toString instead of yielding undefined — always
        // truthy, so a naive `|| "—"` fallback never triggers and React is
        // asked to render a function as a child (crash/warning). Guard with
        // an explicit typeof check so only an actual string value from the
        // payload is ever rendered.
        const fromStr = typeof row.item.fromString === "string" ? row.item.fromString : undefined;
        const toStr = typeof row.item.toString === "string" ? row.item.toString : undefined;
        return (
          <li key={row.key} data-testid="history-row" className="text-sm text-[#1a1f36]">
            {richSentence(
              t(row.item.field === "created" ? "issues.history.createdIssue" : "issues.history.changed", {
                author: `${MARK}author${MARK}`,
                field: `${MARK}field${MARK}`,
                // Values are data: drop the marker so they can't inject a node.
                from: fromStr && fromStr.length ? stripMark(fromStr) : "—",
                to: toStr && toStr.length ? stripMark(toStr) : "—",
              }),
              {
                author: <span className="font-semibold">{row.authorName}</span>,
                field: <span className="font-medium">{fieldLabel(t, row.item.field)}</span>,
              },
            )}{" "}
            <span className="text-xs text-slate-400" title={fmt.dateTime(row.created)}>
              · {fmt.relative(row.created)}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
