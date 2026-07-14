# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - YYYY-MM-DD

Initial 1.0 release, accumulating the capabilities built across Rounds 0-9 of the Jira
parity effort.

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
- **Users, groups & permissions**: user and group management, and project permission
  schemes (see Known limitations below — permissions are informational only in this
  release).
- **Integrations**: signed outbound webhooks, automatic Git commit comments linking commits
  to issues, and an event-driven automation rule engine (via the async worker).
- **API compatibility**: a REST surface compatible with a subset of the Jira Cloud v3
  (`/rest/api/3`) and Agile 1.0 (`/rest/agile/1.0`) APIs, tracked by an automated gap
  report against the official OpenAPI specs.
- **Frontend**: a Next.js (App Router) frontend served under `/app`, covering the above
  functionality end-to-end.

### Known limitations

- **Permissions are not yet enforced server-side.** Project-level permissions are
  currently informational (UI-gating only); the API does not independently verify
  project/issue-level authorization on every request. Do not expose an instance to
  untrusted users until server-side enforcement lands (tracked for a future release).
- **Attachments are not implemented.**
- **SMTP and OAuth are not wired up.** The corresponding environment variables are
  reserved but not yet read by the server.

[Unreleased]: https://github.com/it4nodummies/heureum/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/it4nodummies/heureum/releases/tag/v1.0.0
