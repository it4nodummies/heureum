# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Subtasks** on the issue detail: list of child issues with a completion counter, inline
  "Add subtask" create, and status/assignee shown per row (`GET /issue/{key}/subtasks`).
- **Linked work items**: add/list/remove issue links (Blocks / Relates / Duplicate) directly from
  the issue view, grouped by relation with the correct inward/outward phrasing
  (`GET /issue/{key}/issuelinks`).
- **Attachments**: drag-and-drop/file-picker upload, thumbnail grid, download and delete, backed by
  a proper v3 attachment shape (`id`, `filename`, `size`, `mimeType`, `created`, `content`) instead
  of the raw domain struct.
- **Time tracking / worklog**: a Time tracking block (time spent vs. original/remaining estimate),
  editable estimates, a "Log work" dialog, and a worklog list with delete.
- **Activity History tab**: a Comments/History switch in the issue Activity area backed by the
  per-issue changelog (`GET /issue/{key}/changelog`); author attribution currently falls back to
  "System" (see Known gaps).
- **Editable assignee** via a new reusable `UserPicker` (debounced search over
  `GET /user/assignable/search`, with pinned "Unassigned" and "Assign to me" shortcuts) — the
  assignee field was previously read-only.
- **Richer create-issue modal**: description, priority, assignee, and an optional parent (for
  Subtasks), plus a "Create another" option that keeps the modal open and resets the form.
- **Project Timeline (Gantt)**: a horizontal-bar timeline of epics and sprints per project, with
  weeks/months/quarters zoom (`GET /project/{key}/timeline`). Sprint bars show completion %.
- **Project Calendar**: a month grid placing issues on their due date (or start date when there's
  no due date), with month navigation (`GET /project/{key}/calendar`).
- **Burnup report** on the project Reports page, alongside a **sprint selector** that resolves the
  project's own board and defaults to the active sprint (previously the page was hardcoded to
  board `1` and its first sprint).
- **Export CSV** button on the project overview (bearer-auth blob download).

### Fixed

- `POST /rest/api/3/issue` now resolves `fields.issuetype.name` (not just `.id`) to a type, and now
  parses `fields.assignee` (previously only `PUT` did).
- Logging work (`POST /issue/{key}/worklog`) now increments `Issue.TimeSpent`; `PUT /issue/{key}`
  can write `fields.timetracking.originalEstimateSeconds`/`remainingEstimateSeconds`.
- `apiFetch` no longer throws on body-less 2xx responses (e.g. `POST /issueLink` returns 201 with
  no body) — it now guards against `res.json()` failing on an empty string instead of assuming
  every non-204 response has a JSON body.
- History rows no longer risk rendering a function as a React child: `ChangeItem.fromString`/
  `toString` collide with the property every JS object inherits from `Object.prototype`, so a
  changelog entry that omits one of those fields made a naive `|| "—"` fallback resolve to
  `Object.prototype.toString` instead of the placeholder. `History.tsx` now uses an explicit
  `typeof === "string"` guard.
- `e2e/search.spec.ts` used an unanchored `/DEMO-1/` regex that also matched `DEMO-10`, `DEMO-11`,
  etc. once the seed grew past nine issues; tightened to `/^DEMO-1$/`.
- CSV export (`GET /project/{key}/issues/export`) now writes resolved **Status/Type/Assignee names**
  instead of raw UUIDs.
- Timeline and Calendar routes now key on the project **key** (resolved to the internal UUID
  server-side); previously they required the internal UUID the frontend never receives, making both
  views uncallable.
- The Calendar service dropped issues that had a start_date in the month but no due_date; they're
  now bucketed onto their start day.
- The Cumulative Flow (CFD) report derived its date axis from SQL `DATE(...)` (UTC) while bucketing
  events with Go's local-time formatting, so across local midnight the two diverged by a day and the
  bands were miscounted; both now derive from the same Go-formatted source.

### Known gaps (tracked for a follow-up round)

- Deleting a worklog doesn't decrement `Issue.TimeSpent`.
- `IssueHandler.Update` logs a history entry for every field on every save, even fields that didn't
  change.
- Changelog entries always have `ActorID = ""` (history shows "System" as the author) — attributing
  the acting user requires threading the uid through `issue.Service.Create`/`Update`.
- The full Playwright suite has SQLite write-contention flakiness when run with more than one
  worker; `--workers=1` is required for a fully green run.
- CSV export does not neutralize spreadsheet formula-injection (leading `=`/`+`/`-`/`@` in a title
  or name); acceptable for an authenticated per-project export, but worth defense-in-depth later.
- The Timeline renders epics with no start/due date as a full-width bar spanning the whole window
  (the backend doesn't filter date-less epics).

## [1.0.2] - 2026-07-14

### Added

- **Create board** from the project overview — unlocks the Board and Backlog once a project has one.
- **Create issue** everywhere it's needed: a "Create" menu in the top bar (Issue, with a project
  picker, or Project) plus contextual "Create issue" buttons on the project overview, board, and backlog.
- **Editable issue detail**: an Edit mode for summary, description, priority, and labels.
- **Story points** on issues (view and edit) and **breadcrumbs** on the issue detail to navigate
  back to the project.
- **Consistent project header** across all project pages (Board / Backlog / Reports / Settings tabs
  with the current one highlighted) and a generated **default project avatar** (no more broken image).

### Fixed

- Issue **description** no longer renders raw JSON for issues created without one (shows a
  placeholder instead).
- `PUT /rest/api/3/issue/{key}` now persists **label** and **story point** changes (previously ignored).

## [1.0.1] - 2026-07-14

### Fixed

- **Project navigation gaps.** Added the project overview page (`/app/projects/[key]`) with
  a section bar (Board/Backlog/Reports/Settings) and a recent-issues list; opening a project
  from the list previously 404'd. Made the project rows in the list clickable, added an
  `/app` → `/app/projects` redirect (the "For you" home no longer 404s), and turned the
  not-yet-implemented sidebar entries (Apps, Plans, Goals, Teams) into disabled "Coming soon"
  items instead of dead links. Added an end-to-end test that clicks through from the project
  list to the overview to guard against this regression.

## [1.0.0] - 2026-07-14

Initial 1.0 release, accumulating the capabilities built across Rounds 0-11 of the Jira
parity effort.

### Security

- **Server-side authorization is enforced on mutating requests.** Every create/update/delete
  is checked against the caller's project role (admin/member/viewer) or their global-admin
  flag and returns **403** when the required permission is missing. Group and
  project-category mutations require a global admin; filters and dashboards are mutable only
  by their owner (or a global admin); creating a project makes the creator its admin.
- **Reads are membership-gated.** Project-scoped GET endpoints return **404** to
  non-members (no existence leak); project lists and JQL search results are filtered to the
  caller's memberships; user email addresses are visible only to the user themselves and to
  global admins; filters/dashboards are readable by owner or when shared/public (this also
  closes a hole where any user could copy another user's private dashboard); the board
  websocket endpoint now requires authentication. Global admins bypass read scoping.

### Added

- **Projects & issue core**: projects, issue types (Epic/Story/Task/Bug/Subtask), issue
  CRUD, priorities, statuses, and labels.
- **Collaboration**: comments, worklogs, watchers, votes, issue links, per-issue changelog
  (history of field changes), and remote links.
- **Search & JQL**: a Jira-compatible JQL-like query language for filtering and searching
  issues.
- **Agile boards**: boards, backlog, and sprints, compatible with a subset of the Jira
  Agile 1.0 API.
- **Workflow**: configurable workflows and status transitions.
- **Reports & dashboards**: project/board reports and dashboards with common Jira-style
  widgets.
- **Users, groups & permissions**: user and group management, and project roles
  (admin/member/viewer) enforced server-side on mutations (see the Security section).
- **Integrations**: signed outbound webhooks, automatic Git commit comments linking commits
  to issues, and an event-driven automation rule engine (via the async worker).
- **API compatibility**: a REST surface compatible with a subset of the Jira Cloud v3
  (`/rest/api/3`) and Agile 1.0 (`/rest/agile/1.0`) APIs, tracked by an automated gap
  report against the official OpenAPI specs.
- **Frontend**: a Next.js (App Router) frontend served under `/app`, covering the above
  functionality end-to-end.
- **Issue attachments**: upload/download/delete for issue attachments, stored on local disk
  under the configurable `APP_UPLOADS_DIR` (default `./data/uploads`).
- **Closable signup**: `APP_SIGNUP` flag (`open`/`closed`) to disable public registration
  on instances that are provisioned out-of-band.
- **Deployment hardening**: release-aware Helm chart resource names (multiple releases can
  coexist in the same or different namespaces without collisions), corrected liveness/
  readiness probe endpoints for the api/worker/frontend components, and a PVC (Helm) /
  named volume (Compose) for the uploads directory so attachments survive restarts.

### Known limitations

- **No object storage for attachments yet.** Attachments are stored on local disk only;
  S3-compatible storage is a planned enhancement.
- **SMTP and OAuth are not wired up.** The corresponding environment variables are
  reserved but not yet read by the server.

[Unreleased]: https://github.com/it4nodummies/heureum/compare/v1.0.2...HEAD
[1.0.2]: https://github.com/it4nodummies/heureum/releases/tag/v1.0.2
[1.0.1]: https://github.com/it4nodummies/heureum/releases/tag/v1.0.1
[1.0.0]: https://github.com/it4nodummies/heureum/releases/tag/v1.0.0
