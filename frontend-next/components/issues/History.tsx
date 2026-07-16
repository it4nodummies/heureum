"use client";

import { useQuery } from "@tanstack/react-query";
import { issues, type ChangeItem, type Changelog } from "@/lib/api";

// Renders relative time the same way across History rows ("just now", "5m
// ago", "3d ago", falling back to the date once it's more than a month old).
function relativeTime(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return iso;
  const diffSec = Math.round((Date.now() - then) / 1000);
  if (diffSec < 5) return "just now";
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.round(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHour = Math.round(diffMin / 60);
  if (diffHour < 24) return `${diffHour}h ago`;
  const diffDay = Math.round(diffHour / 24);
  if (diffDay < 30) return `${diffDay}d ago`;
  return new Date(iso).toLocaleDateString();
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

function toRows(entries: Changelog[]): HistoryRow[] {
  const rows: HistoryRow[] = [];
  for (const entry of entries) {
    const authorName = entry.author?.displayName ?? "System";
    entry.items.forEach((item, idx) => {
      rows.push({ key: `${entry.id}-${idx}`, authorName, created: entry.created, item });
    });
  }
  return rows;
}

export function History({ issueKey }: { issueKey: string }) {
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
  const rows = toRows(entries);

  if (isLoading) {
    return <p className="text-sm text-slate-400">Loading history…</p>;
  }

  if (rows.length === 0) {
    return <p className="text-sm text-slate-400">No history yet.</p>;
  }

  return (
    <ul data-testid="history-list" className="space-y-2">
      {rows.map((row) => (
        <li key={row.key} data-testid="history-row" className="text-sm text-[#1a1f36]">
          <span className="font-semibold">{row.authorName}</span> changed{" "}
          <span className="font-medium">{row.item.field}</span> from «
          {row.item.fromString || "—"}» to «{row.item.toString || "—"}»{" "}
          <span className="text-xs text-slate-400">· {relativeTime(row.created)}</span>
        </li>
      ))}
    </ul>
  );
}
