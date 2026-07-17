# Round 22 — Hardening post-1.0 (webhook retry, worker cleanup, git dedup, auth rate-limit, SMTP docs) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Reliability + security hardening: make webhook delivery a real persistent retry queue (survives crashes), stop the worker double-processing automation, dedupe git auto-comments on repeated commit SHAs, rate-limit auth endpoints, and fix the stale SMTP docs.

**Architecture:** Today webhook delivery is fire-and-forget in the request goroutine and the worker's `processWebhookDeliveries` is a stub; `webhook_deliveries` is a post-hoc log with no retry state. This round adds retry columns (migration `000021`), changes the dispatcher to ENQUEUE a pending delivery, and rewrites the worker loop to deliver retryable rows with exponential backoff and a max-attempts→dead cutoff. The redundant `processAutomationRules` worker loop is removed (the dispatcher already runs rules event-driven). Git commit linking gets a unique `(issue_id, commit_sha)` index (migration `000022`) + a dedup lookup so a repeated SHA doesn't re-comment. A dependency-light in-memory per-IP rate limiter middleware guards `/auth/register` + `/auth/login`. `.env.example`/README SMTP notes are corrected (SMTP is wired since R20).

**Tech Stack:** Go 1.25 (`net/http`, `net/smtp`, GORM, golang-migrate, in-memory SQLite tests), no new deps (custom in-memory limiter). No frontend changes this round.

## Global Constraints

- **Module path `github.com/it4nodummies/heureum`** — verbatim in imports.
- **No new dependency** — the rate limiter is a custom in-memory token/window (Redis is configured but not wired; do NOT introduce a Redis client this round).
- **Webhook retry semantics:** the dispatcher enqueues a `webhook_deliveries` row with `status='pending'`, `attempts=0`, `next_attempt_at=now`, and the serialized `payload`; the worker delivers rows where `status IN ('pending','failed') AND next_attempt_at <= now`, oldest first, capped per tick. On 2xx → `status='success'`; else `attempts++`, and if `attempts >= maxAttempts` (5) → `status='dead'`, otherwise `status='failed'` + `next_attempt_at = now + base<<（attempts-1)` (exponential backoff, base ~30s, capped e.g. 1h). Persist `status_code`/`error` as the last attempt result. Delivery must survive a crash (state is in the DB, not a goroutine).
- **Keep `processNotificationQueue`** (the only email sender, R20). **Remove `processAutomationRules`** (redundant double-processing). Replace the stub `processWebhookDeliveries` with the real retry processor.
- **Git dedup:** a repeated `(issue_id, commit_sha)` must NOT create a second `issue_commits` row NOR a second comment. Enforce with a unique index + an existence check in `LinkCommit` that signals "already linked" so the handler skips commenting.
- **Rate limit:** per-client (IP from `RemoteAddr`/`X-Forwarded-For` first hop) sliding window on the two auth routes; exceed → `429` with a JSON error and a `Retry-After` header. Sane defaults (e.g. 10 attempts / 5 min per IP), overridable via config env if trivial (optional). Must not break the existing auth E2E (login within the limit).
- **Testability:** extract the worker's webhook-processing into a pure function that takes the DB + an `*http.Client` (inject a stub `RoundTripper`) so retry/backoff transitions are unit-tested without real HTTP; the rate limiter is unit-tested (N allowed → 429 → window reset); git dedup tested at the service level.
- Three-level gate before done: (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test --workers=1`; (3) `go run ./cmd/gapreport` — no route changes expected (auth/webhook routes unchanged); commit if it moves.
- Conventional Commits; branch `feat/frontend-next`. Full E2E `--workers=1`. **Next migration numbers: `000021` (webhook retry), `000022` (git commit unique index).**

---

### Task 1: Webhook delivery retry/backoff + persistent queue + worker cleanup

**Files:**
- Create: `migrations/000021_webhook_delivery_retry.up.sql` / `.down.sql`
- Modify: `internal/domain/webhook/model.go` (Delivery fields), `service.go` (enqueue/list-retryable/mark-result), `internal/integration/dispatcher.go` (enqueue instead of fire-and-forget)
- Modify: `cmd/worker/main.go` (real `processWebhookDeliveries` with backoff; remove `processAutomationRules`)
- Test: `internal/domain/webhook/service_test.go` (retry state), a worker-processor test (stub transport), and update `internal/integration/dispatcher_test.go`

**Interfaces:**
- Migration: `ALTER TABLE webhook_deliveries ADD COLUMN status TEXT NOT NULL DEFAULT 'pending'; ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0; ADD COLUMN next_attempt_at TIMESTAMP; ADD COLUMN payload TEXT NOT NULL DEFAULT '';` (keep existing status_code/success/error/created_at). Down drops the four columns.
- `Delivery` gains `Status string`, `Attempts int`, `NextAttemptAt *time.Time`, `Payload string`.
- `webhook.Service`: `EnqueueDelivery(webhookID, eventType, url, payload string) error` (inserts status='pending', attempts=0, next_attempt_at=now, payload); `ListRetryable(now time.Time, limit int) ([]Delivery, error)` (status in pending/failed AND next_attempt_at<=now, oldest first); `MarkDeliveryResult(id string, statusCode int, success bool, errMsg string, status string, attempts int, nextAttemptAt *time.Time) error`. Keep `RecordDelivery` if still referenced, or drop if unused.
- Dispatcher: replace the fire-and-forget goroutine with `d.webhookSvc.EnqueueDelivery(hook.ID, eventType, hook.URL, string(payload))` for each matching hook (synchronous enqueue; no HTTP in the request path).
- Worker: `processWebhookDeliveries(logger, db, client *http.Client)` selects `ListRetryable`, for each looks up the webhook (URL+secret) via a service method, calls `webhook.Deliver(client, hook, eventType, []byte(payload))`, computes the new status/attempts/next_attempt_at (backoff, maxAttempts=5, base 30s), and `MarkDeliveryResult`. Extract the per-row decision (`func nextDelivery(attempts int, success bool, now time.Time) (status string, attempts int, next *time.Time)`) as a pure, unit-tested helper.

- [ ] **Step 1: Write the failing tests** — service test: `EnqueueDelivery` → `ListRetryable(now)` returns it (pending); after a simulated failure `MarkDeliveryResult(..., status="failed", attempts=1, next=now+30s)`, `ListRetryable(now)` excludes it but `ListRetryable(now+31s)` includes it; after `attempts>=5` mark `dead` → never returned. A pure-helper test for `nextDelivery` (success→success; fail<5→failed+backoff; fail@5→dead). Update the dispatcher test to assert an enqueued pending row (not an immediate delivery).

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/domain/webhook/ ./internal/integration/ -v` → FAIL.

- [ ] **Step 3: Implement** — migration, model fields, service methods, dispatcher enqueue, worker retry processor + backoff helper; delete `processAutomationRules` and its call; keep `processNotificationQueue`; pass an `*http.Client` (with a sane timeout) into `processWebhookDeliveries`.

- [ ] **Step 4: Verify** — `go test ./internal/domain/webhook/ ./internal/integration/ ./... -v 2>&1 | tail` green; `go build ./... && go vet ./...`; `go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md` (no route change).

- [ ] **Step 5: Commit**
```bash
git add migrations/000021_webhook_delivery_retry.up.sql migrations/000021_webhook_delivery_retry.down.sql internal/domain/webhook/ internal/integration/dispatcher.go internal/integration/dispatcher_test.go cmd/worker/main.go
git commit -m "feat(webhook): persistent retry queue with exponential backoff; drop redundant worker automation polling"
```

---

### Task 2: Git auto-comment dedup on repeated commit SHA

**Files:**
- Create: `migrations/000022_issue_commit_unique.up.sql` / `.down.sql` (`CREATE UNIQUE INDEX ... ON issue_commits(issue_id, commit_sha);`)
- Modify: `internal/domain/git/config.go` (`LinkCommit` dedups; add `CommitExists`/return a signal)
- Modify: `internal/api/handlers/git_handler.go` (skip the comment when already linked)
- Test: `internal/domain/git/*_test.go` (dedup) + a handler-level assertion if feasible

**Interfaces:**
- Migration up: `CREATE UNIQUE INDEX IF NOT EXISTS idx_issue_commits_issue_sha ON issue_commits(issue_id, commit_sha);` (down: `DROP INDEX`). If existing seed/data could contain dupes, the index creation may fail — check; the seed doesn't link commits, so it's safe.
- `LinkCommit(issueID, configID, sha, message, author string) (created bool, err error)` — returns `created=false` (no error) when the `(issue_id, sha)` row already exists (look up first; or attempt insert and treat a unique-violation as `created=false`). The handler only comments when `created==true`.

- [ ] **Step 1: Write the failing test** — service test: `LinkCommit` twice with the same `(issueID, sha)` → first returns `created=true`, second returns `created=false` and no second row exists (`count==1`). (Adapt to the real current `LinkCommit` signature — it currently returns only `error`; changing it to `(bool, error)` requires updating the handler caller.)

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/domain/git/ -v` → FAIL (signature / dup row).

- [ ] **Step 3: Implement** — the unique index migration; `LinkCommit` returns `(created bool, err error)` — look up an existing `(issue_id, sha)` and return `(false, nil)` if present, else insert and return `(true, nil)`; update `git_handler.go` `processPushEvent` to only call `commentCommitReference` when `created`.

- [ ] **Step 4: Verify** — `go test ./internal/domain/git/ ./internal/api/handlers/ -run 'Commit|Git' -v && go build ./... && go vet ./... && go test ./...`.

- [ ] **Step 5: Commit**
```bash
git add migrations/000022_issue_commit_unique.up.sql migrations/000022_issue_commit_unique.down.sql internal/domain/git/config.go internal/api/handlers/git_handler.go internal/domain/git/*_test.go
git commit -m "fix(git): dedupe commit links + auto-comments on repeated SHA (unique index + LinkCommit guard)"
```

---

### Task 3: Auth rate-limiting (register/login)

**Files:**
- Create: `internal/api/middleware/ratelimit.go` (+ test)
- Modify: `internal/api/router.go` (wrap the two auth routes)
- Optional: `internal/config/config.go` (env for limit/window — only if trivial)

**Interfaces:**
- `middleware.RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler` — an in-memory per-client sliding-window (or token-bucket) limiter keyed on client IP (`X-Forwarded-For` first hop else `RemoteAddr` host). On exceed: `429` with a small JSON body and `Retry-After` header. A background/lazy cleanup of stale buckets to avoid unbounded memory. Thread-safe (mutex).
- Applied ONLY to `POST /rest/api/3/auth/register` and `POST /rest/api/3/auth/login`, e.g. `mux.Handle("POST .../auth/login", loginLimiter(http.HandlerFunc(authH.Login)))`. Defaults: 10 requests / 5 min per IP (generous enough that the E2E login flow is unaffected).

- [ ] **Step 1: Write the failing test** — `ratelimit_test.go`: a handler wrapped with `RateLimit(3, time.Minute)`; 3 requests from the same `RemoteAddr` → 200, the 4th → 429 with `Retry-After`; a different IP → allowed; after advancing past the window (inject a clock or use a tiny window + sleep-free logic — prefer a settable `now` func) → allowed again.

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/api/middleware/ -run RateLimit -v` → FAIL.

- [ ] **Step 3: Implement** — the limiter (map of IP→timestamps or a token bucket; mutex; prune old entries), the middleware, and wire it on the two auth routes in `router.go`. Use a generous default so real logins pass. Make the clock injectable for the test (package-level `var now = time.Now` or a struct field) to avoid sleeps.

- [ ] **Step 4: Verify** — `go test ./internal/api/middleware/ -v && go build ./... && go vet ./... && go test ./...`; and confirm the auth E2E still logs in: `cd frontend-next && npx playwright test e2e/login.spec.ts --workers=1` (login must succeed — the limit is well above one login).

- [ ] **Step 5: Commit**
```bash
git add internal/api/middleware/ratelimit.go internal/api/middleware/ratelimit_test.go internal/api/router.go internal/config/config.go
git commit -m "feat(security): in-memory per-IP rate limiting on auth register/login"
```

---

### Task 4: SMTP docs correction (.env.example + README)

**Files:** `.env.example`, `README.md`.

**Interfaces:** none — docs only. `config.Load` already reads `SMTP_HOST/PORT/USER/PASS/FROM` (R20).

- [ ] **Step 1: Implement** — in `.env.example`, move the `SMTP_*` block out of "Planned (NOT yet read)" into an active (commented-with-example-values) section describing that setting them enables notification email from the worker (empty host = disabled). In `README.md`, fix the stale "SMTP … planned and not yet wired" line to state SMTP email is wired (worker delivery, `via_email` prefs, once per notification), noting OAuth/object-storage remain planned.

- [ ] **Step 2: Verify** — `grep -n SMTP .env.example README.md` shows the corrected text; no code changed (`go build ./...` still clean as a sanity check).

- [ ] **Step 3: Commit**
```bash
git add .env.example README.md
git commit -m "docs: SMTP_* is wired (worker email) — correct .env.example + README"
```

---

### Task 5: Round close — gate, docs

- [ ] **Step 1: Full three-level gate**
```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test --workers=1 && cd ..
go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md
```
- [ ] **Step 2: CHANGELOG.md** — Added: persistent webhook retry queue (exponential backoff, migration 000021); per-IP auth rate limiting. Fixed: worker no longer double-processes automation rules; git auto-comments no longer duplicate on repeated commit SHAs (migration 000022); SMTP docs corrected. Note: rate limiter is in-memory (per-instance).
- [ ] **Step 3: STATE.md** — Round 22 entry (hardening: reliability + security + ops). Close the corresponding follow-ups: R9 "webhook fire-and-forget / disable redundant worker polling / git dedup" and the R20 "SMTP env commented". New follow-ups: distributed rate-limit via Redis (config present, unwired); webhook dead-letter surfacing in UI; S3 object storage still open. Keep "Prossimo" pointing at the remaining hardening backlog / release.
- [ ] **Step 4: Commit** docs + plan.
- [ ] **Step 5: Update auto-memory** (controller action).

---

## Self-Review

**Spec coverage:** Affidabilità → T1 (webhook retry queue + backoff) + T1 (drop redundant automation polling) + T2 (git dedup). Sicurezza → T3 (auth rate limit). Ops/config → T4 (SMTP docs). ✅

**Placeholder scan:** migration DDL, model/service/middleware signatures, backoff + limiter semantics given exactly; tests specified with concrete assertions. No TBD.

**Type consistency:** `LinkCommit` new `(bool, error)` signature updated at its handler caller; `webhook.Service` enqueue/list/mark signatures consumed by the worker; `RateLimit(limit, window)` middleware matches the router wiring; `Delivery` new fields used by the worker processor.

**Cross-cutting risks:** (1) changing the dispatcher to enqueue (no in-request HTTP) means webhooks are delivered by the worker within a tick — update the dispatcher test to assert enqueue, and note the latency tradeoff (reliability over immediacy). (2) The webhook processor must be a testable pure-ish function with an injected `*http.Client` (stub transport) — no real network in tests. (3) rate-limit defaults must be generous enough not to break the login E2E; make the clock injectable to test window reset without sleeps. (4) git unique index: verify no existing dupes (seed doesn't link commits). (5) two migrations this round (000021, 000022) — keep them ordered and reversible. (6) full suite `--workers=1`; worker isn't run in CI so its logic must be covered by Go unit tests, not E2E.
