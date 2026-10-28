# Round 27 — Security hardening — Design

**Status:** proposed (2026-10-28)
**Driver:** a code-health audit of `main` (R25+R26) found an authorization/perimeter debt that must be
closed **before the reference team onboards** onto the self-hosted instance. The three most severe
findings were confirmed by hand against `main` (`router.go:210`, `git/config.go:17-75`,
`router.go:175`, `config.go:55`).

## Why

- **Git provider credentials are readable by any project viewer.** `GET /project/{key}/git/providers`
  is gated by *BrowseProjects* and serializes `token_encrypted` and `webhook_secret`; the token is
  stored in **plaintext** (no encryption code exists). The webhook secret doubles as the inbound URL
  token, so it also lands in access logs.
- **Nested routes never bind the child `{id}` to the authorized parent** (comments, worklogs, project
  webhooks, workflow statuses/transitions, sprints): an admin of one project can mutate objects in a
  project where they are only a viewer.
- **Issues taken from a request body are not authorized**: `MoveToSprint` moves any issue in the
  instance; `issueLink` checks only one side.
- **The perimeter is open by default**: `APP_SIGNUP` defaults to `open` (confirmed unset on the live
  instance), and `POST /project` has no permission check — every anonymous visitor can become a
  project admin, which turns the items above into "anyone on the network".
- **Blind SSRF** via outgoing webhooks (URL unvalidated, redirects followed); the **WebSocket hub
  broadcasts every user's notifications to every socket**; **API tokens can never be revoked or
  expire**; the **login rate limit trusts a client-forged `X-Forwarded-For`**; uploads and bodies have
  no size cap; a few handlers leak raw `err.Error()`.

Clean already (keep as is): JQL is parameterized with an ORDER BY allowlist, bcrypt passwords,
SHA-256 API tokens, correct outgoing-webhook HMAC.

## Goals

1. **Close the perimeter with zero config**: in production (`APP_ENV=production`) signup defaults to
   `closed`; **bootstrap**: when the instance has zero users, registration is allowed regardless and
   the first user becomes global admin (solves the "no admin in prod" gap too). `APP_SECRET` shorter
   than 32 bytes is rejected at startup in production. Helm chart and compose expose `APP_SIGNUP`.
2. **Git provider lockdown**: the GET is *AdministerProjects* only and **never returns the token or
   the secret** (a `has_token` boolean instead); the token is **encrypted at rest** (AES-256-GCM, key
   derived from `APP_SECRET`, ciphertext prefixed `v1:`); the inbound webhook URL uses a **separate
   random `webhook_url_token`** (new column), not the HMAC secret.
3. **Bind every nested child to its parent** (404 on mismatch) and require comment authorship (or
   project admin) for comment update/delete.
4. **Authorize every issue that comes from a body**: issues moved into a sprint must belong to the
   sprint's project; both ends of an issue link require *BrowseProjects*.
5. **SSRF-safe outgoing webhooks**: http(s) only; private/loopback/link-local/unspecified ranges are
   rejected after DNS resolution, at create/update **and** at delivery; no redirects; 10s timeout.
   `APP_WEBHOOK_ALLOW_PRIVATE_HOSTS` (comma-separated hosts/CIDRs) is the explicit escape hatch for
   self-hosted setups that target internal services (the homelab Forgejo is on a private IP).
6. **WebSocket hardening**: notifications are routed **per user**; the board socket requires
   *BrowseProjects* on `{key}`; `CheckOrigin` allows only the `APP_BASE_URL` origin (and same host);
   a read pump unregisters dead connections.
7. **API token lifecycle**: list and revoke your own tokens; optional expiry (`expires_at`, reject
   expired at auth); a profile UI for it.
8. **Misc hardening**: rate limiter resolves the client IP from the rightmost `X-Forwarded-For` hop
   only when the request comes from a trusted proxy (`APP_TRUSTED_PROXIES`, CIDRs), otherwise
   `RemoteAddr`; `http.MaxBytesReader` on attachment upload (10 MB, as advertised) and a global body
   limit (32 MB) that also covers the unauthenticated git webhook; `MarkRead` checks ownership;
   register/login/project-create return generic messages and log the detail.

## Non-goals (tracked, separate rounds)

- CSP / moving the JWT out of `localStorage` (frontend auth model — later).
- Reconciling the public Helm chart with the production one (ops round).
- Merging R24 (Postgres CI gate) and the data-integrity bugs it should catch (invite `""`→FK,
  description JSON, atomic key allocation) — data round, right after this one.
- A configurable "who may create projects" policy: with signup closed, authenticated users are the
  team; project creation stays open to them (Jira-like, zero-config). Revisit if needed.
- Rotating `APP_SECRET` re-keys stored git tokens — documented limitation (re-save the provider).

## Design notes

- **Migrations** (no `IF NOT EXISTS`, MySQL-safe): `000024_git_webhook_url_token` adds nullable
  `git_provider_configs.webhook_url_token TEXT` + unique index; `000025_api_token_expiry` adds
  nullable `api_tokens.expires_at TIMESTAMP`. Existing providers get a URL token and legacy plaintext
  tokens get encrypted by a **startup backfill** in `git.Service` (idempotent: only rows without a
  URL token / without the `v1:` prefix). These are nullable string columns → `*string`, never `""`
  (anti-pattern #2).
- **Child binding**: services take the parent id in the lookup (`WHERE id = ? AND issue_id = ?`,
  `… AND project_id = ?`, `… AND workflow_id = ?`) and return not-found on mismatch; handlers map it
  to 404 so existence isn't leaked.
- **Routes**: `GET /rest/api/3/auth/api-tokens`, `DELETE /rest/api/3/auth/api-tokens/{id}` are
  Heureum extensions (like the existing POST) → `gap-report.md` changes (extra routes) and must be
  regenerated and committed. No Jira-conformant DTO is touched (contract tests stay green).
- **Tests are the proof**: every task starts with a failing test that *demonstrates the hole*
  (cross-project child, forged XFF, private webhook URL, viewer reading the token…). If a test passes
  before the fix, the finding was wrong — the implementer reports it instead of "fixing".

## Review focus (edge cases)

- Fresh production instance, zero users, `APP_SIGNUP` unset: the **first** registration succeeds and
  is admin; the **second** is refused (403/closed) until an admin invites.
- `APP_ENV` unset (dev) keeps signup open — the local dev/e2e flows don't break.
- A viewer calling the git provider GET gets 403; an admin gets `has_token:true` and **no** token or
  secret in the body; the inbound webhook works with the new URL token and **not** with the HMAC secret.
- Cross-project child mutation returns **404** (not 403, not 200) and leaves the row unchanged.
- Webhook to `http://127.0.0.1:8080`, `http://169.254.169.254`, `http://10.0.0.1` → rejected;
  `https://example.com` → accepted; a host in `APP_WEBHOOK_ALLOW_PRIVATE_HOSTS` → accepted.
- Forged `X-Forwarded-For` from an untrusted peer does **not** change the rate-limit bucket.
- Expired API token → 401; revoked token → 401 immediately.
