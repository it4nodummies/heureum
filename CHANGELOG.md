# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/it4nodummies/heureum/compare/v1.0.1...HEAD
[1.0.1]: https://github.com/it4nodummies/heureum/releases/tag/v1.0.1
[1.0.0]: https://github.com/it4nodummies/heureum/releases/tag/v1.0.0
