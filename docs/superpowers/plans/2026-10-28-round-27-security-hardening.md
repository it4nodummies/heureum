# Round 27 — Security hardening — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to
> implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the authorization/perimeter holes found by the audit so the instance is safe to onboard
a real team: closed-by-default signup with first-user bootstrap, git credentials locked down and
encrypted, every nested child bound to its parent, body-sourced issues authorized, SSRF-safe
webhooks, per-user WebSocket routing, revocable/expiring API tokens, proxy-safe rate limiting, body
limits.

**Architecture:** Go backend (`internal/`), two small migrations, no change to Jira-conformant DTOs;
one profile-page addition in the frontend (API tokens). Every task is TDD: the first step is a test
that demonstrates the hole and FAILS on current `main`.

**Tech Stack:** Go (GORM, net/http ServeMux, `internal/api/authz`), golang-migrate, Next.js + lib/api.ts.

**Spec:** `docs/superpowers/specs/2026-10-28-round-27-security-hardening-design.md`

## Global Constraints

- Branch `feat/round-27-security` off `origin/main`. Conventional Commits; **no `Co-Authored-By`**.
- Three-level gate must pass at the end; **`gap-report.md` WILL change** (two new extension routes
  in Task 7) → regenerate with `go run ./cmd/gapreport` and commit it.
- Nullable FK/string columns are `*string`, never `""` (anti-pattern #2). Migrations: no
  `IF NOT EXISTS`. New config vars only if the server reads them (anti-pattern #13).
- Mismatch on a nested child → **404** (don't leak existence). Never return a secret in a response.
- If a "hole-demonstrating" test PASSES before the fix, STOP and report (the finding was wrong) —
  do not change code to make a passing test fail.

## Review Focus

- Zero-user production instance: first register → admin; second → refused (Task 1).
- Viewer → git provider GET 403; admin → no token/secret in body; inbound hook works only with the
  URL token (Task 2).
- Cross-project child mutation → 404 and row unchanged, for each route class (Task 3).
- Private/loopback webhook URL rejected at create AND delivery; allowlisted host accepted (Task 5).
- Forged XFF from an untrusted peer doesn't change the bucket (Task 8). Expired/revoked token → 401 (Task 7).

---

### Task 1: Perimeter — closed signup in production, first-user bootstrap, secret policy

**Files:**
- Modify: `internal/config/config.go:55` (`SignupOpen` default), add `Env`-aware default + secret check
- Modify: `internal/api/handlers/auth_handler.go:24` (`Register`)
- Modify: `internal/domain/user` (or `auth`) — a `CountUsers()`/first-user path; set `IsAdmin=true`
- Modify: `deploy/helm/heureum/values.yaml` + `templates/configmap.yaml` (expose `config.signup` → `APP_SIGNUP`); `deploy/docker/docker-compose.prod.yml` (pass `APP_SIGNUP`, require `APP_SECRET` via `${APP_SECRET:?}`)
- Test: `internal/config/config_test.go`, `internal/api/handlers/auth_handler_test.go`

**Interfaces:**
- Produces: `config.Load()` → `SignupOpen=false` when `APP_ENV=production` and `APP_SIGNUP` unset;
  explicit `APP_SIGNUP=open|closed` always wins; returns an error when `APP_ENV=production` and
  `len(APP_SECRET) < 32`. `Register`: if the users table is empty, allow registration regardless of
  policy and create the user with `IsAdmin=true`; otherwise honor `SignupOpen` (403 when closed).

- [ ] **Step 1: Failing tests** — `TestLoad_ProductionDefaultsSignupClosed`, `TestLoad_ProductionRejectsShortSecret`, `TestRegister_FirstUserBecomesAdminEvenWhenClosed`, `TestRegister_SecondUserRefusedWhenClosed`.
- [ ] **Step 2:** run, confirm FAIL. **Step 3:** implement. **Step 4:** chart/compose values. **Step 5:** `go test ./internal/config/... ./internal/api/handlers/... ./internal/domain/...` PASS.
- [ ] **Step 6: Commit** `feat(auth): closed-by-default signup in production with first-user admin bootstrap; enforce APP_SECRET length`

### Task 2: Git provider lockdown — admin-only, no secrets in responses, encrypted token, separate URL token

**Files:**
- Modify: `internal/api/router.go:210` (GET → `permission.AdministerProjects`)
- Modify: `internal/api/handlers/git_handler.go` (`GetProvider` → DTO without token/secret, `has_token`; `Webhook` → match `webhook_url_token`)
- Modify: `internal/domain/git/config.go` (`TokenEncrypted` encrypt/decrypt via new `internal/domain/git/crypto.go`: AES-256-GCM, key = SHA-256(APP_SECRET), ciphertext `v1:`+base64; `WebhookURLToken *string`; `FindByWebhookURLToken`; `EnsureBackfill()` startup: generate URL tokens for rows without one, encrypt rows without `v1:` prefix)
- Create: `migrations/000024_git_webhook_url_token.up.sql` / `.down.sql` (nullable `webhook_url_token TEXT` + unique index)
- Modify: `cmd/server/main.go` (call `EnsureBackfill` after migrations); wire the key from config
- Test: `internal/domain/git/crypto_test.go`, `internal/api/handlers/git_handler_test.go`

**Interfaces:**
- Produces: `git.Service.Configure(...)` stores an encrypted token and a random URL token; `GET` returns
  `{provider, repo_url, has_token, webhook_url}` (URL built from `webhook_url_token`), never the
  token/secret. Inbound route `POST /rest/api/3/webhooks/git/{token}` resolves by `webhook_url_token`.

- [ ] **Step 1: Failing tests** — `TestGetProvider_ViewerForbidden` (viewer → 403), `TestGetProvider_AdminBodyHasNoSecrets` (admin → no `token_encrypted`/`webhook_secret` keys), `TestCrypto_RoundTripAndPrefix`, `TestWebhook_RejectsHMACSecretAsURLToken_AcceptsURLToken`, `TestBackfill_EncryptsLegacyAndAddsURLToken`.
- [ ] **Step 2:** FAIL. **Step 3:** migration + crypto + service + handler + router + startup backfill. **Step 4:** `go test ./...` PASS; contract tests green (custom route, no v3 DTO).
- [ ] **Step 5: Commit** `fix(git): admin-only provider read, encrypt token at rest, separate webhook URL token`

### Task 3: Bind nested children to their parent (IDOR)

**Files:**
- Modify: `internal/domain/issue/comment_service.go:94-118` (lookups take `issueID`), `internal/api/handlers/comment_handler.go:59,141,166` (+ author-or-project-admin check on update/delete)
- Modify: `internal/api/handlers/worklog_handler.go:165` + worklog service (scope by issue)
- Modify: `internal/api/handlers/webhook_handler.go:81` + service (scope by project)
- Modify: `internal/domain/workflow/service.go:60-97` (`UpdateStatus`/`RemoveStatus`/transition ops take `workflowID`) + `internal/api/handlers/workflow_handler.go:69,89,150,169`
- Modify: `internal/api/handlers/sprint_handler.go:59-106` (scope sprint by project)
- Test: `internal/api/handlers/authz_nested_test.go` (new): for EACH class, create two projects A/B, a child in B, authenticate as admin of A, mutate B's child via A's path → expect 404 and the row unchanged.

**Interfaces:**
- Produces: all child lookups are `WHERE id = ? AND <parent_fk> = ?`; services return a not-found
  sentinel on mismatch; handlers answer 404.

- [ ] **Step 1: Failing tests** (one per class: comment update/delete, worklog delete, project webhook update/delete, workflow status update/remove + transition update/remove, sprint update/delete) — all must FAIL on main (currently 200/204).
- [ ] **Step 2:** implement scoping + comment authorship. **Step 3:** `go test ./...` PASS.
- [ ] **Step 4: Commit** `fix(authz): bind nested child ids to their authorized parent (comments, worklogs, webhooks, workflow, sprints)`

### Task 4: Authorize body-sourced issues (MoveToSprint, issueLink)

**Files:**
- Modify: `internal/api/handlers/agile_sprint_handler.go:222-246` (each issue must belong to the sprint's project; else 400 with no partial move)
- Modify: `internal/api/handlers/issuelink_handler.go:179-199` (require *BrowseProjects* on BOTH issues' projects)
- Test: `internal/api/handlers/agile_sprint_handler_test.go` (extend), `issuelink_handler_test.go`

- [ ] **Step 1: Failing tests** — `TestMoveToSprint_RejectsIssueFromOtherProject` (no partial move), `TestIssueLink_RequiresBrowseOnBothSides`.
- [ ] **Step 2:** implement. **Step 3:** `go test ./internal/api/handlers/... ./internal/contract/...` PASS.
- [ ] **Step 4: Commit** `fix(authz): authorize issues taken from request bodies (sprint move, issue links)`

### Task 5: SSRF-safe outgoing webhooks

**Files:**
- Create: `internal/domain/webhook/urlcheck.go` (`ValidateTargetURL(raw string, allow []string) error`: http/https only; resolve; reject loopback/private/link-local/multicast/unspecified v4+v6 unless host/IP matches `allow`)
- Modify: `internal/api/handlers/webhook_handler.go:61-72` (validate on create/update), `internal/domain/webhook/delivery.go:22` (validate again before sending; `http.Client` with `CheckRedirect` → error, `Timeout: 10s`), `internal/config/config.go` (`WebhookAllowPrivateHosts []string` from `APP_WEBHOOK_ALLOW_PRIVATE_HOSTS`), `.env.example`
- Test: `internal/domain/webhook/urlcheck_test.go`, handler test

- [ ] **Step 1: Failing tests** — `TestValidateTargetURL` table: `http://127.0.0.1:8080`, `http://169.254.169.254/latest`, `http://10.0.0.1`, `http://[::1]`, `ftp://x` → error; `https://example.com` → ok; `http://192.168.1.58:3000` with allow `192.168.1.0/24` → ok. `TestCreateWebhook_RejectsPrivateURL`. `TestDeliver_NoRedirectFollow`.
- [ ] **Step 2:** implement. **Step 3:** `go test ./...` PASS.
- [ ] **Step 4: Commit** `fix(webhook): validate target URLs against SSRF, no redirects, explicit private-host allowlist`

### Task 6: WebSocket hardening

**Files:**
- Modify: `internal/api/ws/hub.go` (clients keyed by userID; `SendToUser(userID, msg)`; read pump; unregister on write error), `internal/domain/notification/service.go:48-54` (route to the notification's `user_id` only), `internal/api/router.go:400` (board ws → `chk.Enforce(permission.BrowseProjects, chk.ByKey, …)`), `CheckOrigin` → allow `APP_BASE_URL` origin / same host
- Test: `internal/api/ws/hub_test.go`

- [ ] **Step 1: Failing tests** — `TestHub_NotificationOnlyReachesOwner`, `TestBoardWS_RequiresProjectBrowse`, `TestCheckOrigin_RejectsForeignOrigin`, `TestHub_UnregistersOnClose`.
- [ ] **Step 2:** implement. **Step 3:** `go test ./internal/api/ws/... ./internal/domain/notification/...` PASS.
- [ ] **Step 4: Commit** `fix(ws): per-user notification routing, project authz on board socket, origin check, connection cleanup`

### Task 7: API token lifecycle — list, revoke, expiry (+ profile UI)

**Files:**
- Create: `migrations/000025_api_token_expiry.up.sql`/`.down.sql` (nullable `expires_at TIMESTAMP`)
- Modify: `internal/domain/auth/apitoken.go` (`ExpiresAt *time.Time`, `List(userID)`, `Revoke(userID, id)`, create accepts optional expiry; auth lookup rejects expired), `internal/api/handlers/auth_handler.go` (`ListAPITokens`, `RevokeAPIToken`), `internal/api/router.go:159` (+ `GET /rest/api/3/auth/api-tokens`, `DELETE /rest/api/3/auth/api-tokens/{id}`), `internal/api/middleware` (expired → 401)
- Frontend: `frontend-next/lib/api.ts` (`auth.listApiTokens`, `auth.revokeApiToken`, create with `expiresInDays?`), `frontend-next/app/app/profile/page.tsx` (list + revoke + optional expiry on create)
- Regenerate `docs/contracts/gap-report.md`
- Test: `internal/domain/auth/apitoken_test.go` (extend), handler test; e2e extend `users.spec.ts` (create → list → revoke)

- [ ] **Step 1: Failing tests** — `TestAPIToken_ExpiredRejected`, `TestAPIToken_RevokedRejected`, `TestListAPITokens_OwnOnlyNoHash`, `TestRevoke_OtherUsersTokenIs404`.
- [ ] **Step 2:** migration + domain + routes + middleware + UI. **Step 3:** `go test ./...`, `npm run build`, e2e `users.spec.ts` PASS; `go run ./cmd/gapreport` → commit the regenerated report.
- [ ] **Step 4: Commit** `feat(auth): list/revoke API tokens with optional expiry`

### Task 8: Misc hardening — trusted-proxy rate limiting, body limits, MarkRead ownership, no raw error leaks

**Files:**
- Modify: `internal/api/middleware/ratelimit.go:121` (`clientIP`: use rightmost XFF hop only if `RemoteAddr` ∈ `APP_TRUSTED_PROXIES` CIDRs, else `RemoteAddr`), `internal/config/config.go` (`TrustedProxies []string`), `.env.example`
- Modify: `internal/api/handlers/attachment_handler.go:31` (`http.MaxBytesReader` 10 MB), a global body-limit middleware (32 MB) applied in `router.go` incl. `POST /webhooks/git/{token}` (`git_handler.go:113`)
- Modify: `internal/api/handlers/notification_handler.go:31` + `notification/service.go:89` (`MarkRead(userID, id)` ownership)
- Modify: `internal/api/handlers/auth_handler.go:44`, `project_handler.go:126` (generic client messages; log detail). From the Task 1 review: `Register`'s non-`ErrSignupClosed` branch currently writes `409 {"error": err.Error()}` by string concatenation and can now surface raw GORM errors from the bootstrap path → map unexpected errors to a generic **500**, keep 409 only for the sanitized "already taken" case.
- Modify: `internal/domain/auth/service.go` (Task 1 follow-ups): hash the password **before** taking `bootstrapMu` (today every registration is serialized behind bcrypt); make the multi-replica bootstrap race symmetric — under **open** policy the loser is deleted and a retryable error returned (today both replicas can demote themselves → zero admins, and bootstrap can't re-trigger). Add `TestRegisterWithPolicy_RacedBootstrap_OpenDeletesLoser`.
- Test: `ratelimit_test.go`, handler tests

- [ ] **Step 1: Failing tests** — `TestClientIP_IgnoresXFFFromUntrustedPeer`, `TestClientIP_UsesRightmostHopFromTrustedProxy`, `TestAttachmentUpload_RejectsOver10MB`, `TestMarkRead_OtherUsersNotificationIs404`, `TestRegister_DoesNotLeakInternalError`.
- [ ] **Step 2:** implement. **Step 3:** `go test ./...` PASS.
- [ ] **Step 4: Commit** `fix(api): trusted-proxy client IP, request body limits, notification ownership, generic auth errors`

### Task 9: Inbound git webhook signature hardening (found by the Task 2 review)

**Files:**
- Modify: `internal/domain/git/provider.go:78` (`noopProvider.ValidateWebhook` returns `true` → any provider type without a real validator must **reject**; `bitbucket` passes the DB CHECK but has no signature check → either implement its signature scheme or refuse to create a bitbucket provider until it does)
- Modify: `internal/domain/git/gitlab.go:16` (`signature == secret` → `crypto/subtle.ConstantTimeCompare`)
- Modify: `internal/domain/git/config.go` `CreateProvider` + `internal/api/handlers/git_handler.go` (reject an **empty** `webhook_secret` at create with 400; an empty HMAC key makes GitHub/Forgejo signatures forgeable)
- Test: `internal/domain/git/provider_test.go`, `gitlab_test.go`, handler test

- [ ] **Step 1: Failing tests** — `TestNoopProvider_RejectsWebhook`, `TestGitLab_ValidateWebhook_ConstantTime` (behavioral: wrong token rejected, right token accepted; plus a static check that `subtle.ConstantTimeCompare` is used), `TestCreateProvider_RejectsEmptyWebhookSecret`, `TestBitbucket_CreateRefusedOrValidated`.
- [ ] **Step 2:** implement. **Step 3:** `go test ./internal/domain/git/... ./internal/api/handlers/... ./internal/contract/...` PASS.
- [ ] **Step 4: Commit** `fix(git): inbound webhooks require a real signature check, constant-time compare, non-empty secret`

---

## Final

- [ ] Full three-level gate (backend; frontend build + full Playwright; `go run ./cmd/gapreport` → commit the regenerated report from Task 7).
- [ ] Whole-branch security review (fresh reviewer, most capable model): re-attempt each original hole end-to-end against the branch; confirm 404/403/401 semantics and no secret in any response body.
- [ ] Update `docs/superpowers/STATE.md` (R27 entry; remaining items: CSP/JWT storage, chart reconciliation, R24 merge + data-integrity bugs) and `.env.example`/README for `APP_SIGNUP`, `APP_TRUSTED_PROXIES`, `APP_WEBHOOK_ALLOW_PRIVATE_HOSTS`, first-user bootstrap.
- [ ] Doc drift found by the Task 1 review (must land before merge): `README.md:87` still says `APP_SIGNUP` default is `open`; `README.md:127,159` and `docs/RELEASE.md:57` still advise "set `APP_SIGNUP=closed` once your users exist" → now the production default with first-user bootstrap; document that an **unknown `APP_SIGNUP` value is a startup error** and the **32-char `APP_SECRET` rule in production**; add `CHANGELOG.md [Unreleased]` "Changed"/"Security" entries for every task (signup default, admin bootstrap, git provider response shape — `token_encrypted`/`webhook_secret`/`project_id` no longer returned, encrypted-at-rest + rotating `APP_SECRET` re-keys stored git tokens, new webhook URL token, API token list/revoke/expiry, trusted proxies, webhook allowlist).
