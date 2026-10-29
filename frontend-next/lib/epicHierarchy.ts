// Client-side epic membership (Round 28). The backend only emits the DIRECT
// parent ref on an issue (fields.parent); the epic of a subtask is its story's
// parent, which we resolve from the issues the page has already loaded — no
// extra request. Shared by the backlog epic panel, the backlog/board cards,
// the board epic quick filter and the swimlane-by-epic grouping, so every
// surface agrees on which epic an issue belongs to.

import { hierarchyLevelOf, isEpic, type AgileEpic, type SearchIssue } from "./api";

export interface EpicRef {
  key: string;
  name: string;
  // Palette key (lib/epicColors.ts) when the board's epics list carries it.
  colorKey?: string;
}

// Active epic filter of a backlog / board: every issue, issues without an
// epic, or the members of one epic (by key).
export type EpicFilter = "all" | "none" | string;

// Lookup table of the board's epics by key, built from GET /board/{id}/epic.
export type EpicsByKey = Record<string, AgileEpic>;

export function indexEpics(epics: AgileEpic[] | undefined): EpicsByKey {
  const out: EpicsByKey = {};
  for (const e of epics ?? []) out[e.key] = e;
  return out;
}

export function indexIssues(issues: SearchIssue[]): Map<string, SearchIssue> {
  const out = new Map<string, SearchIssue>();
  for (const iss of issues) out.set(iss.key, iss);
  return out;
}

// The epic an issue belongs to, or null when it has none (an epic itself, an
// orphan story, a subtask of an orphan story). A story's epic is its parent;
// a subtask's epic is its parent story's parent, read from the loaded issues
// (`byKey`). When the parent ref carries no issue type (pre-R28 payloads) and
// the parent isn't loaded, the direct parent key is used as a best effort.
// The name comes from the board's epics list when available, else from the
// parent ref's own summary.
export function epicOf(
  issue: SearchIssue,
  byKey: Map<string, SearchIssue>,
  epics: EpicsByKey = {},
): EpicRef | null {
  if (isEpic(issue)) return null;
  const p = issue.fields.parent;
  if (!p) return null;

  let key: string | undefined;
  let name: string | undefined;
  if (p.fields?.issuetype) {
    const level = hierarchyLevelOf(p.fields.issuetype);
    if (level === 1) {
      key = p.key;
      name = p.fields.summary;
    } else if (level === 0) {
      // Subtask → story → epic: the story must be loaded to know its epic
      // (a story's parent can only be an epic, so no level check is needed).
      const gp = byKey.get(p.key)?.fields.parent;
      if (gp) {
        key = gp.key;
        name = gp.fields?.summary;
      }
    }
  } else {
    const loaded = byKey.get(p.key);
    if (loaded) {
      if (isEpic(loaded)) {
        key = p.key;
        name = loaded.fields.summary;
      } else if (loaded.fields.parent) {
        key = loaded.fields.parent.key;
        name = loaded.fields.parent.fields?.summary;
      }
    } else {
      key = p.key;
      name = p.fields?.summary;
    }
  }
  if (!key) return null;
  const epic = epics[key];
  return { key, name: epic?.name ?? name ?? key, colorKey: epic?.color?.key };
}

// Whether an issue passes the epic filter. Epics themselves are hidden by any
// filter other than "all" (they are the grouping, not members).
export function matchesEpicFilter(
  issue: SearchIssue,
  filter: EpicFilter,
  byKey: Map<string, SearchIssue>,
): boolean {
  if (filter === "all") return true;
  if (isEpic(issue)) return false;
  const epic = epicOf(issue, byKey);
  return filter === "none" ? epic === null : epic?.key === filter;
}

// done/total of an epic's DIRECT children (stories, tasks, bugs) among the
// loaded issues — the same count the epic detail's progress bar shows.
export function epicProgress(epicKey: string, issues: SearchIssue[]): { done: number; total: number } {
  let done = 0;
  let total = 0;
  for (const iss of issues) {
    if (iss.fields.parent?.key !== epicKey) continue;
    total += 1;
    if (iss.fields.status?.statusCategory?.key === "done") done += 1;
  }
  return { done, total };
}
