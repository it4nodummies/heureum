# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Zero-config project creation.** `POST /rest/api/3/project` now makes a project immediately
  usable: after the project row is created, the handler scaffolds a default workflow
  (`TO DO`/`IN PROGRESS`/`DONE`), a set of standard issue types, a global `Done` resolution, and a
  default board — in that order — so a user can create a project and drag an issue on its board
  without ever opening Settings. The scaffold fails loudly (no silently-swallowed errors) and, on
  any step failing, the half-created project is deleted (full-graph cleanup) and the request
  returns 500 — no half-created projects. The scrum/kanban/business templates now produce genuinely
  different configurations: the board type is `scrum` for scrum and `kanban` for kanban/business,
  and the issue-type set is `Epic, Story, Task, Bug, Subtask` for software templates (scrum/kanban)
  vs `Task, Bug, Subtask` for business. The `POST /project` request/response shape is unchanged
  (still `{self,id,key}`); the workflow is uniform across templates for now. The project-overview
  "Create board" prompt is kept only as a fallback for legacy board-less projects.
- **API token lifecycle.** Personal API tokens can now be listed and revoked, and may carry an
  expiry: `POST /rest/api/3/auth/api-tokens` accepts an optional `expiresInDays` (> 0, else 400),
  new Heureum-extension routes `GET /rest/api/3/auth/api-tokens` (own tokens, never the hash or
  plaintext) and `DELETE /rest/api/3/auth/api-tokens/{id}` (204; another user's id is 404). An
  expired or revoked token is rejected with 401 on the next request. Migration `000025` adds
  `api_tokens.expires_at`; the profile page gains an "API tokens" section (create with optional
  expiry, plaintext shown once, list, confirm-guarded revoke).

### Changed

- **Signup is closed by default in production.** With `APP_ENV=production` and `APP_SIGNUP` unset,
  public registration is disabled; outside production it stays open. An explicit
  `APP_SIGNUP=open|closed` always wins, and **any other value is now a startup error**.
- **First-admin bootstrap.** Whatever the signup policy, the first registration on an instance
  whose users table is **empty** is accepted and that user becomes global admin, so a fresh
  production instance gets its administrator without toggling `APP_SIGNUP`. From the second user
  on the policy applies. The prod compose file and the Helm chart expose `APP_SIGNUP`.
  **Upgrade note:** an instance that already has users but no global admin (or whose only admin
  is deactivated) does **not** bootstrap on the next registration — promote one before or after
  the rollout with `UPDATE users SET is_admin = true WHERE email = '…';` (a promote route is a
  planned follow-up).
- **`APP_SECRET` must be at least 32 characters when `APP_ENV=production`** (startup error
  otherwise). It now also derives the key that encrypts git provider tokens at rest, so **rotating
  `APP_SECRET` makes stored git tokens undecryptable** — delete and recreate the provider after a
  rotation.
- **Git provider response shape.** `GET /rest/api/3/project/{key}/git/providers` no longer returns
  `token_encrypted`, `webhook_secret` or `project_id`; it returns `has_token` and `webhook_url`
  instead (plus `id`, `provider_type`, `base_url`, `created_at`).
- **Git inbound webhook URL.** The inbound hook is now `POST /rest/api/3/webhooks/git/{token}`
  with a dedicated random URL token, not the HMAC secret. Existing providers get a URL token at
  startup (idempotent backfill) — **update the webhook URL configured in GitHub / Forgejo / GitLab**
  to the `webhook_url` shown by the provider GET; the old secret-in-URL form stops matching.
- **Git provider creation rules.** `webhook_secret` is required and must be at least 16 characters
  (400 otherwise); `bitbucket` is refused as a provider type (no signature validator exists for it).
- **Rate limiting behind a proxy.** New `APP_TRUSTED_PROXIES` (IPs/CIDRs; a malformed entry is a
  startup error). Only when the connecting peer is a trusted proxy is `X-Forwarded-For` used —
  every header line is read and the client is the rightmost non-trusted hop; from any other peer
  the header is ignored and `RemoteAddr` is the client. Deployments that relied on the old
  "trust any XFF" behaviour must set this variable or every client behind the proxy shares one
  bucket.
- **Request body limits.** Attachment uploads are capped at the advertised 10 MB (413 above it),
  and a global 32 MB body limit applies to every route, including the unauthenticated git webhook.
- **Generic error messages.** Register, login and project create no longer surface raw driver /
  GORM error text: unexpected errors are a generic 500 with the detail logged; "already taken" (409),
  invalid credentials and duplicate project key (400) are sanitized sentinels.
- `gofmt` applied to nine pre-existing unformatted files (no behaviour change).
- **Creating an issue now emits only `issue_created`** (after commit, with initial status/assignee/
  labels already applied); the spurious `issue_updated`/`issue_transitioned` events and history
  rows that used to fire on create are gone — webhooks and automation rules keyed on those no
  longer trigger at creation, and `issue_created` rules that set assignee/transition are no longer
  overwritten by the handler. The assignment notification is still sent (after commit).

### Fixed

- **Project invites failed on Postgres.** `project_invites.accepted_by` was written as `""` on
  creation; it is a nullable FK to `users(id)`, so the insert violated the constraint on Postgres
  (SQLite never enforced it). It is now `NULL` until the invite is accepted.
- **Issue descriptions typed at creation were stored as broken JSON.** The create path wrapped the
  already-serialized ADF document in `{"content":"…"}` without escaping, so the detail view fell
  back to rendering the raw string. The ADF document is now stored as-is (valid JSON); plain-text
  descriptions are still wrapped into a paragraph.
- **Issue keys are allocated atomically.** The next key number used to be "latest row's key + 1"
  outside any transaction (with the lookup error ignored), so concurrent creates, `created_at`
  ties or a DB error could produce duplicate keys surfacing as 400 with raw driver text.
  Migration `000026` adds `projects.issue_counter`; the key, the `seq_id` and the row insert now
  happen in one transaction, errors are propagated, and an unknown project is a clean error.
  **Upgrade note:** the counters are backfilled once at first start from the existing keys
  (idempotent, in Go, logged); until then — and for rows inserted behind the server's back — a
  create whose allocated key already exists re-syncs the counter instead of failing.
- **Issue creation follow-ups are transactional.** The initial status, assignee, labels and
  fix versions are written in the same transaction as the issue row: a failure in any of them
  leaves no half-initialized issue (previously each step's error was discarded and a failed
  status assignment could dereference a nil issue). An unknown assignee is now a 400 with a
  field error instead of a later FK violation.
- **Sprint completion is NULL-safe and transactional.** Open issues whose `status_id` is NULL were
  skipped by `status_id NOT IN (…)` and left behind in the completed sprint; they are now moved
  (or kept) with the others, and the whole completion is one transaction with errors propagated.
- **A workflow status in use can no longer be removed.** `DELETE` of a status still referenced by
  issues is refused with **409** and a message naming the count, instead of orphaning them.
- **`CompleteSprint` returns 500 for database errors** (it mapped every error to 400).
- **Automation "add comment" actions write valid ADF.** The comment body was built by formatting
  the text into a JSON literal by hand, which broke on the first quote or newline.
- The project-create scaffold logs (instead of discarding) a failed rollback delete.
- **Unknown issue type on create is a 400** with `errors.issuetype` (it surfaced as a 500 after the follow-ups were made transactional); the type must exist and belong to the project.
- **Removing a workflow status checks the parent first**: a status id from another workflow/project is a 404 before any in-use count, so the 409 guard cannot leak another project's status or issue count.

### Security

- **Git provider credentials locked down.** The provider GET was readable by any project viewer and
  serialized the plaintext access token and the webhook secret. It is now gated by
  `ADMINISTER_PROJECTS`, never returns the token or secret, and the token is **encrypted at rest**
  (AES-256-GCM, key derived from `APP_SECRET`, `v1:`-prefixed ciphertext; legacy plaintext rows are
  encrypted by the startup backfill). Migration `000024` adds `git_providers.webhook_url_token`.
- **Inbound git webhooks require a real signature check.** Provider types without a validator are
  rejected instead of accepted by a no-op; GitLab's token comparison is constant-time
  (`crypto/subtle`); an empty HMAC secret is a hard reject because it makes GitHub/Forgejo
  signatures forgeable. **Legacy providers stored with an empty `webhook_secret` stop receiving
  events until deleted and recreated with a secret** — the server logs a startup warning with the
  count.
- **Nested routes are bound to their authorized parent.** Comment, worklog, project webhook,
  workflow status/transition and sprint lookups now include the parent id (`issue_id`,
  `project_id`, `workflow_id`); a child reached through another project's path is a **404** and the
  row is untouched. Comment update/delete and worklog delete additionally require being the author
  or a project admin (403 otherwise). Generic DB errors are no longer collapsed into not-found.
- **Issues taken from request bodies are authorized.** `POST /rest/agile/1.0/sprint/{id}/issue`
  (move to sprint), backlog and rank moves refuse an issue that does not belong to the sprint's /
  board's project with no partial write; `POST /rest/api/3/issueLink` requires `BROWSE_PROJECTS` on
  **both** issues; `CompleteSprint`'s `moveToSprintId` target must be in the same project (400).
- **Outgoing webhooks are SSRF-safe.** Target URLs are validated on create and again at delivery:
  http/https only, no userinfo, and the host (literal or DNS-resolved) must not be loopback,
  private, link-local, multicast, unspecified, CGNAT/reserved/NAT64, 6to4/Teredo or
  IPv4-mapped/translated. Redirects are never followed (a 3xx is a failed attempt) and deliveries
  time out after 10 s. `APP_WEBHOOK_ALLOW_PRIVATE_HOSTS` (hosts/IPs/CIDRs) is the explicit escape
  hatch — **existing webhooks pointing at private hosts fail at delivery until it is set**.
- **WebSocket hardening.** Notifications are routed to the owning user only (the hub broadcast
  every user's notifications to every socket); the board socket requires `BROWSE_PROJECTS` on the
  project; `CheckOrigin` accepts only the `APP_BASE_URL` origin / same host; dead connections are
  unregistered (read pump, write errors, full send buffer).
- **Notification mark-read ownership.** `MarkRead` binds the notification to the caller; another
  user's notification id is 404 and the row is untouched.
- **Rate limiter no longer trusts a client-forged `X-Forwarded-For`** (see `APP_TRUSTED_PROXIES`
  above): a direct peer rotating the header no longer gets a fresh bucket per request.
- **API tokens can be revoked and can expire** (see Added above): a leaked token no longer lives
  forever.
- **Multi-replica first-user bootstrap is race-safe**: the active-admin count is re-checked after
  the bootstrap insert; the losing replica's row is deleted and the client gets a retryable 503
  (open policy) or 403 (closed), so two concurrent first registrations can never yield two admins,
  and a regular registration landing on another replica in the same instant never strands the
  bootstrapper. The bootstrap itself only ever happens on an empty users table (see Changed).
- **SSRF ranges extended**: outgoing webhook targets in the IPv4-compatible `::/96` (e.g.
  `[::127.0.0.1]`) and site-local `fec0::/10` IPv6 ranges are refused too.
- **`expiresInDays` is capped at 3650** on `POST /rest/api/3/auth/api-tokens` (400 above it): an
  unbounded value overflowed the duration arithmetic and produced a bogus expiry.
- **Database logging no longer prints bound SQL values** outside development (`ParameterizedQueries`,
  and `ErrRecordNotFound` is not logged): a failed or slow statement used to log webhook secrets,
  password hashes and URL tokens in clear.
- **Inbound git webhooks only touch issues of the provider's own project.** Issue keys extracted
  from commit messages, branch names and PR titles are resolved within the project the provider is
  configured on; a key from another project is skipped like an unknown one. Previously any user
  able to create a project could configure a provider with their own secret and, with correctly
  signed events, add commit comments/links to any issue on the instance and — on "PR merged" —
  move it to a status of the attacker's workflow.

### Internal

- **Real-schema test helper and CI matrix (Round 24).** `internal/storetest` gives a test a
  database built from the real migrations (per-test isolated DB, `TEST_DB_DRIVER=sqlite|postgres`,
  SQLite with `PRAGMA foreign_keys` on), replacing `AutoMigrate` + hand-written stubs that hid
  foreign keys and column types; the `backend` CI job runs the suite on both SQLite and Postgres
  (`TZ=Europe/Rome` on the Postgres leg so timestamp zone skews are visible). Every fix above has
  a storetest-based test that was red before it. The API-token expiry round-trip is pinned on the
  real schema too (`TestAPIToken_ExpiryRoundTripsOnRealSchema`). The MySQL leg (migration type
  alignment) is deferred and parked unverified on `wip/r24-mysql-migrations`.

## [1.2.0] - 2026-09-23

Round 23 ("riapertura cantiere"): teams on projects (migration `000023`), a set of infrastructure
fixes uncovered while raising the `go` directive to 1.26.0, an operations runbook, and a public
statement of which parts of the Jira-compatible surface are stable.

### Added

- **Teams associated to projects.** A team (a user group) can now be associated to a project with a
  role (`admin`/`member`/`viewer`); every member of the team inherits that role on the project. A
  user's effective role is the most permissive of their individual project role and any role
  inherited through a team, and `authz.RequireProject` (plus read-scoping) resolves permissions from
  that effective role — so team members can view and work the project's issues without an individual
  membership row. Managed from the renamed **Teams** page (`/app/groups`, sidebar entry activated)
  and a new **Teams** section in a project's Settings → Access tab. Backed by the new `project_teams`
  table (migration `000023`) and the Heureum-extension endpoints `GET/POST /project/{key}/teams` and
  `PUT/DELETE /project/{key}/teams/{groupId}`, gated by `ADMINISTER_PROJECTS`.
- **`docs/OPERATIONS.md`**: a backup and restore procedure for the Docker stack, covering the
  Postgres database, the `uploads` volume, and `deploy/docker/.env`.

### Changed

- Issue detail now uses a pencil icon next to the title to enter edit mode, replacing the top-right
  "Edit" button, and shows Original estimate and Remaining estimate in view mode for field parity
  with edit mode.
- The README now states which parts of the Jira-compatible surface are considered **stable**
  (issues, projects, search/JQL, agile) versus best-effort — consult the gap report for what
  exists today, and expect the rest to keep changing.
- The project's architecture decisions are now recorded as ADRs under `docs/adr/`, starting with
  the Jira-compat surface being frozen as a migration bridge (not grown further as a product
  surface in its own right), no application-level multi-tenancy, and a selective stability promise
  for the areas above.

### Fixed

- **The e2e suite ran in an unsupported configuration.** `playwright.config.ts` set no `workers`,
  so CI used Playwright's default (half the CPU count) — two workers against the single shared
  SQLite backend that `scripts/e2e-backend.sh` starts. This is the write-contention flakiness the
  project had documented since Round 13 and worked around manually with `--workers=1`. Adding one
  spec file was enough to turn it into a hard failure; `workers` is now pinned to `1` in the config
  itself.
- **Migrations ran against a second, unrelated database connection.** `store.RunMigrations` built a
  URL by concatenating `"sqlite3://"` with the DSN, which opened its own connection instead of
  reusing the one the application had already opened; with a plain `:memory:` DSN the migrations
  applied to a database the application never saw. The malformed URL
  (`sqlite3://file::memory:?cache=shared`, where `:memory:` parses as a port) was also rejected
  outright by Go 1.26.0, which blocked every dependency update that raised the `go` directive.
  Migrations now run against the already-open `*sql.DB` through each driver's `WithInstance`.
- **MySQL migrations would have failed on a fresh database.** golang-migrate's URL-based MySQL
  driver forced `multiStatements=true` on the connection it opened; `WithInstance` uses the
  application's own pool, where it defaults to `false`. Since each migration file is executed as a
  single statement and `000001_init_schema.up.sql` contains 36, a fresh MySQL or MariaDB install
  would have failed at startup with Error 1064. The DSN is now normalized to require
  `multiStatements=true` when the connection is opened. No automated test exercises MySQL yet, so
  treat this as reviewed rather than proven.
- **Local attachments could be committed by accident.** `data/uploads/` (the default
  `APP_UPLOADS_DIR`) was untracked but not ignored; `/data/` is now in `.gitignore`.
- **Saving an unassigned issue returned HTTP 500 on Postgres.** The issue update path wrote an empty
  string (`""`) into the nullable `issues.assignee_id` foreign key instead of SQL `NULL`, violating
  `issues_assignee_id_fkey` (SQLSTATE 23503) whenever an issue with no assignee was edited. The same
  empty-string-into-nullable-FK bug affected `issue_history.actor_id` and `comments.author_id`: every
  history/changelog insert (and system-generated git commit comments) silently failed on Postgres, so
  the History tab stayed permanently empty there. All three now coerce an empty id to `NULL`. SQLite
  (used by all unit tests and e2e) does not enforce foreign keys, which is why this only surfaced on
  the production-default Postgres driver. (Actor on history rows is `NULL` for now; threading the
  authenticated user id through `issue.Service.Update` is a separate follow-up.)
- **Notifications failed to insert on Postgres.** The `Notification.CreatedAt` field was declared as
  `int64` with `gorm:"autoCreateTime"`, which made GORM write a Unix epoch integer into the
  `created_at TIMESTAMP` column. The pgx driver rejects this (`unable to encode ... into binary
  format for timestamp`), so every notification insert — welcome (seed), @mention, comment/watch,
  sprint and status-change notifications — failed on the production-default Postgres driver (SQLite
  masked the bug because it is loosely typed). `CreatedAt` is now `time.Time`, matching every other
  domain model; the JSON `created_at` field is consequently now an RFC3339 string (frontend
  `AppNotification.created_at` type updated accordingly).
- Added a Postgres-backed `postgres-smoke` CI job that runs `cmd/seed` against a real Postgres
  service container, exercising the notification insert path so this class of Go-model/DB-column
  type mismatch can no longer ship undetected.

### Dependencies

- **The `go` directive is now `1.26.0`** (was `1.25.0`) — building from source requires that
  toolchain or newer.
- **`@dnd-kit/sortable` 8.0.0 → 10.0.0**, crossing two major versions. It powers board and backlog
  drag & drop; the full e2e suite passes on it, but review its changelog before relying on any
  removed API.
- CI: `actions/setup-go` 5 → 7, `docker/setup-buildx-action` 3 → 4.
- Go: `golang.org/x/crypto` 0.54.0 → 0.57.0, `golang.org/x/oauth2` 0.36.0 → 0.37.0,
  `github.com/getkin/kin-openapi` 0.142.0 → 0.149.0, `gorm.io/driver/postgres` 1.6.0 → 1.6.3.
- Frontend: `react`/`@types/react` and `react-dom`/`@types/react-dom` → 19.3.0, `tailwindcss`
  4.3.2 → 4.3.3, `@tailwindcss/postcss` 4.3.0 → 4.3.3.

## [1.1.0] - 2026-07-17

Accumulates Rounds 13–22 of the Jira-parity effort: complete issue view, project views
(Timeline/Calendar/Burnup/CSV), configurability (automation, custom fields, workflow rules),
administration & sharing (members/roles, groups, shared filters, dashboard gadgets), a productive
issue list (bulk + inline edit, cursor pagination, hierarchy), board & sprint pro (configurable
columns/swimlanes/quick filters, sprint goal/dates), releases/versions, a rich editor with
@mentions and notification email over SMTP, profile locale/theme/avatar, and post-1.0 hardening
(persistent webhook retry queue, git auto-comment dedup, auth rate limiting). Adds migrations
`000016`–`000022`.

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
- **Automation rules UI**: an Automation tab in Project Settings — list rules with an active toggle
  and delete, a create form (trigger + condition/action builder limited to the engine's supported
  keys), and a per-rule run log (`/project/{key}/automation`, `/automation/{ruleID}/runs`).
- **Custom fields UI**: a Fields tab in Project Settings (create/list/delete fields across the six
  types, with option management for select/multiselect) plus dynamic rendering of a project's custom
  fields in the create-issue modal (with required-field validation) and the issue detail panel
  (view + edit). Native story points (`customfield_10016`) remain separate from this system.
- **Workflow transition rules**: existing transitions can now be edited in place (name +
  require-assignee / set-resolution) in the workflow editor, not just added and deleted.
- **Access / People**: a project-settings tab to list members with their roles, change a member's
  role, remove members, and add a member by global user search (`/project/{key}/members`).
- **Groups admin**: a global `/app/groups` page to create/list/delete groups and manage their
  members (reachable from the sidebar).
- **Shared filters**: a "Share with team" toggle when saving a filter, a shared/private badge in the
  filter list, and inline rename / re-share / delete — sharing is expressed through the Jira-native
  `sharePermissions` field.
- **Dashboard gadgets**: add a gadget from the supported catalog (Assigned to me, Activity stream)
  and remove gadgets on the dashboard detail page.
- **Productive issue List**: multi-select rows with a bulk action bar (set priority / assignee /
  labels / delete) backed by a new `POST /issues/bulk` endpoint; **inline edit** of priority,
  assignee, and status directly in list cells; real **cursor pagination** with a result count; and
  epic/parent **children indented** under their parent when both are on the current page.
- **Sprint goal & dates**: create/edit sprints with a goal and start/end dates; the goal and date
  range show on the sprint header; **Complete sprint** now opens a dialog to move incomplete issues
  to the backlog OR another sprint (new `POST /rest/agile/1.0/sprint/{id}/complete`).
- **Configurable board**: a Board settings editor to define columns that map a **set** of statuses,
  choose a **swimlane** mode (none / assignee / epic), and save **quick-filter** JQL chips
  (migration `000018` + `GET`/`PUT /rest/agile/1.0/board/{id}/config`). The board renders the
  configured columns (bucketing by status id), swimlane bands, and quick-filter chips; unconfigured
  boards keep the previous 1:1-status layout.
- **Releases / Versions**: project versions with a **Releases** tab/page (create, edit, mark
  released, released/unreleased filter, start/release dates), multi **Fix versions** on issues (view
  + edit + create), and a **Releases lane** in the Timeline (version bars with done/total progress).
  Backed by Jira-conformant endpoints — `GET /rest/api/3/project/{key}/versions`, `POST /version`,
  `GET`/`PUT`/`DELETE /rest/api/3/version/{id}` — over the pre-existing `versions` table plus an
  `issue_versions` pivot for multi fix-versions (migration `000019`).
- **Rich text editor**: a lightweight (dependency-free) WYSIWYG editor with a minimal toolbar
  (bold / italic / code / bullet list / heading) for issue descriptions and comments, producing valid
  ADF — replacing the plain textareas.
- **@mentions**: an autocomplete in the editor inserts a mention of a user; the mentioned user now
  receives a notification (the ADF mention node's account id is wired to notifications).
- **Notification bell** now has **Direct / Watching** tabs and groups entries by issue.
- **Notification preferences**: you can now **add** a preference (event type + channels), not only
  toggle existing ones.
- **Email delivery**: the worker sends notification emails over **SMTP** (config `SMTP_*`),
  respecting the `via_email` preference and emailing each notification at most once (migration
  `000020` adds an `email_sent` flag); email is a no-op when SMTP is unconfigured.
- **Profile — language, theme, avatar**: a language/locale selector on the profile; a persisted
  **light/dark theme** toggle (in the profile and the top-bar menu) applied to the app chrome; and
  **avatar upload** (`POST /rest/api/3/myself/avatar`, image-validated, served publicly at
  `GET /rest/api/3/user/avatar/{userId}`), surfaced in the top bar and user pickers.
- **Webhook delivery is now a persistent retry queue** (migration `000021`): the dispatcher enqueues
  a delivery and the worker delivers it with exponential backoff (up to 5 attempts, then `dead`),
  so a crash no longer loses a webhook (previously fire-and-forget in the request goroutine).
- **Auth rate limiting**: per-IP limiting on `/auth/register` and `/auth/login` (configurable via
  `APP_AUTH_RATELIMIT`, default 10 per 5 min; `0` disables).

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
- **Authorization bypass on automation & custom-field routes:** these were keyed on the internal
  project/issue UUID (never exposed by the API), so a request carrying the public seq_id made the
  authorization resolver miss and the permission check was skipped entirely (a `POST` could create
  orphaned rows without `AdministerProjects`). The routes now key on the project/issue **key** and
  resolve to the UUID server-side, restoring the permission gate.
- Custom-field `SetValue` now rejects an unknown field id (previously it silently wrote an orphaned
  value row) and returns `400`/`404` for invalid-value / unknown-field instead of a blanket `500`;
  it also gained `date` (RFC3339) and `user` write paths and a `required` flag on fields.
- Filter sharing is now settable through the API (create/update accept `is_shared`, and the filter
  response carries `sharePermissions`) — it was previously hardcoded to private at the HTTP layer.
- `GET /project/{key}/members` now returns each member's display name / avatar (and email, subject
  to the usual self-or-global-admin visibility rule) instead of only the user id and role.

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
- Automation: the condition/action builder is limited to the engine's hardcoded keys (`priority`,
  `title_contains`; `set_assignee`/`add_label`/`transition_issue`/`add_comment`); existing rules'
  conditions/actions aren't shown/editable in the list (create + toggle + delete only). The HTTP
  `execute` endpoint runs the rule in test mode.
- Custom fields: the value model stores a single option per field, so a `multiselect` field's UI
  persists only the first selected option; `required` is enforced in the UI (create + edit) but not
  server-side.
- Access/People: a project admin who is not a global admin can demote or remove their own membership
  (no self-guard yet). Project invites can be created but there is no accept-invite route.
- Groups: there is no "list all groups" endpoint, so the admin page lists via the name picker; dashboard
  gadgets are limited to the two hydrated types and gadget layout/position isn't persisted.
- List bulk edit: `POST /issues/bulk` supports assignee/priority/labels/delete but NOT bulk status
  change (cross-project workflow validation) or bulk sprint move; the "Set label(s)" action replaces
  the label set (it does not append); `POST /issues/bulk` has no request-size cap (systemic across
  POST routes). List pagination is cursor-based (`/search/jql` has no total), so there's no numbered
  "N of M" — Prev/Next + a per-page count only. Hierarchy indentation is per-page and derived from
  `parent_id` (there is no distinct epic-link field).
- Board config: card layout and per-column min/max constraints aren't implemented; a status can be
  mapped to more than one column in the editor (real Jira disallows it — first-matching column wins
  at render); quick-filter chips evaluate their JQL server-side capped at 200 results and are not yet
  covered by an end-to-end test; the `000018` migration adds no indexes on the new tables' `board_id`.
- Releases: JQL has no `fixVersion` field yet, so the Releases page shows version status/dates but
  not a live per-version progress bar (the Timeline lane does show version progress via the pivot).
  The `/version/{id}/relatedIssueCounts`, `/unresolvedIssueCount`, and `/move` endpoints and version
  archiving UI aren't implemented; the dead single `issue.version_id` FK remains unused.
- Editor/notifications: the rich editor supports a bounded ADF vocabulary (no tables/images/quote,
  no slash-commands); Direct/Watching is inferred from the notification type (no stored reason
  field); notification preferences can only be added at "All projects" scope from the UI (the
  settings API keys on the internal project UUID, which the frontend doesn't hold); notification
  emails are plain text (no HTML templating) and the `SMTP_*` vars remain commented in
  `.env.example`.
- Profile: the dark theme covers the app chrome (top bar, sidebar, page backgrounds, profile) —
  deep per-view content (project tables, board columns, issue detail, charts, modals) still uses the
  light palette and is a follow-up; avatars have no crop/resize and are served publicly by design.
- The worker no longer double-processes automation rules (the dispatcher already runs them
  event-driven); git commit links + auto-comments are de-duplicated on a repeated commit SHA
  (unique index, migration `000022`); the stale "SMTP planned/not wired" docs are corrected (SMTP
  email is wired since the previous round).
- Known limits: the rate limiter is in-memory (per-instance, not shared across replicas) and trusts
  the `X-Forwarded-For` first hop (assumes a trusted proxy sets it); the webhook queue assumes a
  single worker instance (no row-level lease for horizontal scale).

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

[Unreleased]: https://github.com/it4nodummies/heureum/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/it4nodummies/heureum/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/it4nodummies/heureum/compare/v1.0.2...v1.1.0
[1.0.2]: https://github.com/it4nodummies/heureum/releases/tag/v1.0.2
[1.0.1]: https://github.com/it4nodummies/heureum/releases/tag/v1.0.1
[1.0.0]: https://github.com/it4nodummies/heureum/releases/tag/v1.0.0
