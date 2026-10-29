# Round 28 — Epic experience — Design

**Status:** proposed (2026-10-29)
**Driver:** the reference team lives by epics (121 epics, 1391 parented Story/Task/Bug; tour of
2026-10-27). Heureum today: an epic is just an issue whose type is named "Epic"; `parent_id` is a
generic pointer set only at create, never validated, never updatable; the agile Epic API is
read-only (`GET /epic/{id}`, `GET /board/{id}/epic`, no colour, `done` always false); JQL has no
`parent`; the UI has no epic panel, no parent field, no epic filter. Direction: zero-config, Jira-compat
surface frozen (ADR-0001) — all API additions below are **existing routes of the Jira agile/platform
specs**, no invented routes.

## Decisions (with the user, 2026-10-29)
- **Strict Jira hierarchy**: Epic (level 1) → Story/Task/Bug (level 0) → Subtask (level −1). A child's
  parent must be exactly one level above, in the **same project**; an epic has no parent; no cycles.
  Violations → 400 (`errors.parent`). Unknown parent key/id → 400 (today silently dropped).
- **Scope**: epic panel in the backlog (filter, progress, drag-to-epic, inline create), parent/epic
  field + child list with progress in the issue detail, epic quick-filter on the board (swimlane kept and
  fixed), **epic colour**.

## Goals
1. **Hierarchy as data**: `issue_types.hierarchy_level` (1/0/−1, Jira's `hierarchyLevel`), backfilled
   by name/`is_subtask`; `SeedStandardTypes` sets it. Validation lives in the domain (`createOnce` and a
   new `SetParent`), so every path (REST create/update, agile epic moves, automation later) obeys it.
2. **Epic data**: `issues.epic_color` (`color_1`…`color_14`, Jira's keys), assigned round-robin at epic
   creation so every epic has a colour with zero config; `done` **computed** from the epic's status
   category (zero-config, no second source of truth). `POST /epic/{id} {done}` moves the epic through
   its workflow to the first done/todo-category status reachable by an available transition (409 if none).
3. **Jira-shaped reads**: `fields.parent` gains `fields{summary, status, issuetype, priority}` (spec-native
   shape; lets lists/cards show the epic name without extra calls); `issuetype.hierarchyLevel` emitted;
   agile epic shapes gain `color{key}`, `self`, computed `done`; `/board/{id}/epic` honours `done`.
4. **Writes that exist in the spec**: `POST /epic/{id}` (name/summary/color/done), `POST /epic/{id}/issue`
   and `POST /epic/none/issue` (move issues into / out of an epic — per-issue authz, same project, no
   partial write), `PUT /epic/{id}/rank` (reuse issue rank), `GET /epic/{id}/issue`, `GET /epic/none/issue`,
   `GET /board/{id}/epic/{epicId}/issue`, `GET /board/{id}/epic/none/issue`; `PUT /issue/{key}` accepts
   `fields.parent` (key/id or `null`) → `SetParent`. gap-report regenerated (missing count drops).
5. **JQL**: `parent = KEY|id`, `!=`, `in`, `is empty`/`is not empty`; `"Epic Link"` accepted as an alias
   of `parent` for Jira users.
6. **Frontend**:
   - **Backlog epic panel** (Scrum + Kanban backlogs): collapsible column listing the project's epics
     (colour, name, progress = done/total children computed from the loaded issues), "Issues without
     epic", click to filter the lists, **drag an issue onto an epic** to set its parent, "Create epic" inline.
   - **Cards** (backlog `IssueCard`, board cards): epic lozenge (colour + epic name) when the issue's
     parent is an epic; for a subtask the epic is resolved through its parent (client-side from loaded
     issues).
   - **Board**: epic quick filter (All / No epic / each epic) in the board header; swimlane-by-epic shows
     the epic **name and colour** and groups subtasks under their story's epic (today: by direct parent key).
   - **Issue detail**: "Epic" (for level-0 types) / "Parent" (for subtasks) field with a picker (search
     within the project, filtered to the right level), clearable; child section renamed by type — "Child
     issues" with a progress bar on an epic, "Subtasks" on a story — creating the right child type.
   - **Create modal**: epic picker for Story/Task/Bug (optional), parent picker (not free text) for
     Subtask; issue types listed **per project** (business projects don't offer Epic).
   - **Epic colour picker** (14 swatches) on the epic detail.

## Non-goals
- `parentEpic`/multi-level JQL, Advanced-Roadmaps-style levels above Epic (out of scope, confirmed).
- Epic burndown/report, epic-level estimation rollups (later).
- Changing an issue's type across levels (e.g. Story→Epic) — stays as today.
- `/rest/software/1.0/...` cursor-paginated variants (frozen compat; not used by the UI).

## Design notes
- **Migration `000027_issue_hierarchy`** (no `IF NOT EXISTS`): `issue_types.hierarchy_level INTEGER NOT
  NULL DEFAULT 0` + backfill `UPDATE … SET hierarchy_level = 1 WHERE lower(name) = 'epic'` and `= -1 WHERE
  is_subtask`; `issues.epic_color TEXT` (nullable, `*string`, never `""`). Existing epics get a colour by a
  Go startup backfill (round-robin per project, idempotent, logged count) — same pattern as the R24 counter
  backfill.
- **Validation** (`issue.validateParent(tx, projectID, childTypeLevel, parentID)`): parent exists and is
  not archived; `parent.ProjectID == projectID`; `parentLevel == childLevel + 1`; walking `parent_id` from
  the parent never reaches the child (cycle); an epic (level 1) must have `parent == nil`. Sentinels:
  `ErrParentNotFound`, `ErrParentInvalid` (message says why) → handler 400 `errors.parent`.
- **Moving issues to an epic** (`POST /epic/{id}/issue`): validate ALL body issues (browsable, same
  project as the epic, level 0), then apply in one transaction (R27 "validate-all-then-apply").
- **Epic progress**: `done/total` over direct children (level 0) by status category; computed in the
  frontend from loaded issues for the panel; a domain `EpicProgress` helper backs the detail view.
- **Epic detection** everywhere via `hierarchy_level == 1` (replaces name checks in `isEpic`, timeline,
  frontend `typeName === …` where reachable; the frontend reads `issuetype.hierarchyLevel`/`subtask`).
- **Per-project types in the create modal**: the v3 project DTO already allows `issueTypes` (spec-native);
  emit it from `GET /rest/api/3/project/{key}` and use it in the modal.

## Review focus
- A Story under a Story, a Subtask under an Epic, an Epic with a parent, a parent from another project,
  a cycle, an unknown key → all **400** with `errors.parent`; valid Story→Epic and Subtask→Story → 200/201.
- `PUT /issue/{key}` with `parent: null` clears; with another epic moves; history logs the change.
- `POST /epic/{id}/issue` with one invalid issue → 400 and **no** issue moved.
- Business project: create modal offers no Epic; `SeedStandardTypes` levels: Task/Bug 0, Subtask −1.
- Backlog: filtering by epic hides other epics' issues and "no epic" shows only orphans; dragging a card onto
  an epic updates the parent and the progress; the swimlane groups a subtask under its story's epic.
- Pre-R28 data: existing epics get colours and `hierarchy_level` by backfill; nothing else changes.
