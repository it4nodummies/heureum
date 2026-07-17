# Round 20 — Editor & Notifiche (rich ADF editor + @mentions, notification hub + SMTP email) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace the plain textareas for description/comments with a lightweight rich editor (minimal toolbar + @mention autocomplete producing valid ADF), make @mentions actually notify the mentioned user, add Direct/Watching tabs + issue grouping to the notification bell, let users add a new notification preference, and deliver notification emails over SMTP from the worker.

**Architecture:** No editor library (dependency-light convention) — a small contentEditable `RichTextEditor` serializes to/from a constrained ADF vocabulary that `AdfRenderer` fully supports (round-trip safe). @mentions insert an ADF `mention` node (`attrs.id` = accountId = internal user id); the server-side path that already extracts ADF mention nodes (`v3.ExtractMentions`, currently discarded) is wired to create notifications. Email: add SMTP config + a `Mailer` interface (real `net/smtp` impl, no-op when unconfigured) used by the worker's notification loop, guarded by a new `email_sent` flag (migration `000020`) so each notification emails at most once.

**Tech Stack:** Go 1.25 (`net/http`, `net/smtp`, GORM, golang-migrate, in-memory SQLite tests), Next.js 16 App Router + React 19 + TanStack Query + Tailwind (contentEditable, no editor lib), Playwright.

## Global Constraints

- **Module path `github.com/it4nodummies/heureum`** — verbatim in imports.
- **Constrained ADF vocabulary (round-trip safe):** the editor may ONLY produce node/mark types that `AdfRenderer` renders. Extend `AdfRenderer` and the editor together so every node the editor emits is rendered, and hydrating the editor from stored ADF then re-serializing is stable. Target vocabulary: doc/paragraph/text; marks `strong`,`em`,`code`; `bulletList`/`orderedList`/`listItem`; `heading` (level 1-3); `mention` (`attrs:{id,text}`). Do not emit node types the renderer can't show.
- **@mention id semantics:** an ADF mention node is `{"type":"mention","attrs":{"id":"<accountId>","text":"@Display"}}` where `attrs.id` = the user's accountId = internal user id (`v3.JiraUser.AccountID = u.ID`). Server-side mention notifications resolve by that id directly (NOT by username).
- **Email must not spam:** the worker emails a notification at most once — gate on a persisted `email_sent` flag (set true after a successful send), plus the existing `via_email` preference and `is_read=false`. When SMTP is unconfigured the mailer is a no-op (log once, mark nothing) so behavior is unchanged in dev/CI.
- **Config additions are real:** add `SMTP_HOST/SMTP_PORT/SMTP_USER/SMTP_PASS/SMTP_FROM` to `config.Config` + `config.Load()` (they're only commented in `.env.example` today). Keep them optional (empty = email disabled).
- **Permission/keying:** notification and settings routes stay as-is (auth'd, user-scoped from context). Settings `project_id` handling: when showing prefs, resolve the project name for display; adding a pref may send `project_id: ""` (global) or a real project. Do not require an internal UUID typed by the user — pick projects from a list (send the project's id the frontend already has, i.e. the seq_id/uuid the settings API expects — confirm which the backend stores; it stores projects.id UUID, so map via the project list the frontend loads).
- **Contract:** comments/notifications aren't tightening the Jira contract here; adding an ADF `mention` node to a comment body is still a valid ADF doc. Run `go test ./internal/contract/...` after backend changes; expect no drift (no route changes except possibly none). Run `go run ./cmd/gapreport` and commit if it changes.
- UI accent `#0052cc`; UI under `/app`; single typed client `frontend-next/lib/api.ts`.
- Three-level gate before done: (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test --workers=1`; (3) `go run ./cmd/gapreport` clean.
- Conventional Commits; branch `feat/frontend-next`. E2E inline login from `e2e/export.spec.ts`; full suite `--workers=1`. **Next migration: `000020`.**

---

### Task 1: SMTP mailer + config + worker delivery (backend)

**Files:**
- Create: `migrations/000020_notification_email_sent.up.sql` / `.down.sql` (`ALTER TABLE notifications ADD COLUMN email_sent BOOLEAN NOT NULL DEFAULT FALSE;`)
- Modify: `internal/config/config.go` (SMTP fields + Load)
- Create: `internal/mailer/mailer.go` (`Mailer` interface + `SMTPMailer` + `NoopMailer`)
- Modify: `cmd/worker/main.go` (use the mailer; set email_sent; gate)
- Test: `internal/mailer/mailer_test.go`, and a worker-loop test if feasible (or a notification-email selection test)

**Interfaces:**
- `config.Config` gains `SMTPHost, SMTPUser, SMTPPass, SMTPFrom string; SMTPPort int`; `Load()` reads `SMTP_HOST/SMTP_PORT/SMTP_USER/SMTP_PASS/SMTP_FROM`.
- `mailer.Mailer` interface: `Send(to, subject, body string) error`. `NewSMTPMailer(host string, port int, user, pass, from string) *SMTPMailer` (uses `net/smtp`, PLAIN auth when user set). `NewFromConfig(cfg) Mailer` returns a `NoopMailer` (logs, returns nil) when `SMTPHost==""`.
- Worker: the notification loop selects `notifications` where `via_email` pref true AND `is_read=false` AND `email_sent=false`, resolves the recipient email, `mailer.Send(...)`, and on success sets `email_sent=true` (so it never re-sends).

- [ ] **Step 1: Write the failing tests** — `mailer_test.go`: a fake `Mailer` capturing calls; assert `NewFromConfig` with empty host returns a no-op (Send returns nil, no panic) and with a host returns an SMTPMailer (don't dial a real server — just assert type/params via a constructor test, and test the message formatting helper if you extract one). Add a config test asserting `Load()` populates the SMTP fields from env. If the worker loop is testable, add a test that a via_email+unread+not-sent notification is picked and marks email_sent after a (mock) send.

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/mailer/ ./internal/config/ -v` → FAIL.

- [ ] **Step 3: Implement** — migration; config fields+Load; the mailer package; wire the worker to construct `mailer.NewFromConfig(cfg)` and replace the `logger.Info("email notification"...)` with a real send + `email_sent=true` update, gated as above.

- [ ] **Step 4: Verify** — `go test ./internal/mailer/ ./internal/config/ ./... -v 2>&1 | tail` green; `go build ./... && go vet ./...`; `go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md` (no route change).

- [ ] **Step 5: Commit**
```bash
git add migrations/000020_notification_email_sent.up.sql migrations/000020_notification_email_sent.down.sql internal/config/config.go internal/mailer/ cmd/worker/main.go internal/mailer/mailer_test.go
git commit -m "feat(notifications): SMTP mailer + worker email delivery (once per notification, migration 000020)"
```

---

### Task 2: ADF @mention → notification (backend)

**Problem:** `comment_handler.go` extracts ADF mention nodes via `v3.ExtractMentions` but discards the result (`_ = ...`); only the textual `@username` regex path notifies. Wire the ADF path: mention `attrs.id` = user id → notify those users.

**Files:**
- Modify: `internal/api/handlers/comment_handler.go` (use the extracted ids)
- Modify: `internal/domain/issue/comment_service.go` and/or `internal/domain/notification` (a notify-by-user-ids entry point)
- Test: extend a comment handler/service test

**Interfaces:**
- On comment create, collect accountIds from `v3.ExtractMentions(bodyJSON)`, and notify those users (skip the author) via a `NotifyUsersMentionedByIDs(userIDs []string, authorID, issueKey, issueTitle string)` (add if the existing `NotifyUsersMentioned` takes usernames). Keep it idempotent-ish (dedupe ids).

- [ ] **Step 1: Write the failing test** — create a comment whose ADF body contains `{"type":"mention","attrs":{"id":"<userB-id>","text":"@B"}}`; assert a notification of type `"mention"` is created for user B (and none for the author). (Mirror how existing comment/notification tests assert via the notification service.)

- [ ] **Step 2: Run to verify it fails** — no notification from the ADF node (result discarded today).

- [ ] **Step 3: Implement** — in `comment_handler.go`, replace `_ = v3.ExtractMentions(...)` with capturing the ids and passing them into the comment service (or call the notifier). Add `NotifyUsersMentionedByIDs` to the notification service if needed. Keep the existing textual path OR consolidate — but do not double-notify the same user (dedupe). Guard against notifying the comment author.

- [ ] **Step 4: Verify** — `go test ./internal/... -run 'Comment|Mention|Notif' -v && go build ./... && go test ./...`.

- [ ] **Step 5: Commit**
```bash
git add internal/api/handlers/comment_handler.go internal/domain/issue/comment_service.go internal/domain/notification/
git commit -m "feat(notifications): ADF @mention nodes notify the mentioned user"
```

---

### Task 3: RichTextEditor component + AdfRenderer parity (frontend)

**Files:**
- Create: `frontend-next/components/common/RichTextEditor.tsx`
- Modify: `frontend-next/components/issues/adf.tsx` (render the full editor vocabulary incl. mention)
- Modify: `frontend-next/components/issues/IssueView.tsx` (description) + `frontend-next/components/issues/Comments.tsx` (comment body) to use the editor
- Test: `frontend-next/e2e/editor.spec.ts` (create)

**Interfaces:**
- `RichTextEditor({ valueAdf, onChangeAdf, placeholder })` — a contentEditable with a minimal toolbar (Bold, Italic, Code, Bullet list, H3). It hydrates from an ADF doc and serializes DOM → ADF using ONLY the constrained vocabulary (paragraph/text+marks/bulletList/listItem/heading/mention). Exposes the current ADF via `onChangeAdf`.
- `adf.tsx`: `AdfRenderer` renders `bulletList`/`orderedList`/`listItem`/`heading`(1-3)/`mention` (as a styled `@name` chip) in addition to today's paragraph + strong/em/code. Keep `adfToText` tolerant (mentions → `@text`).

- [ ] **Step 1: Write the failing test** — `editor.spec.ts` (inline login): open DEMO-1, edit the description with the rich editor, apply Bold to some text, save, reload → the bold text renders (`<strong>`); add a comment with a bullet list → renders `<ul><li>`.

- [ ] **Step 2: Run to verify it fails** — no editor / bold not rendered.

- [ ] **Step 3: Implement** — build `RichTextEditor` (contentEditable + toolbar; serialize on input). Replace the description `<textarea>` in IssueView (hydrate from `f.description`, save `onChangeAdf` value directly instead of `textToAdf(text)`) and the comment `<textarea>` in Comments (submit the editor's ADF). Extend `AdfRenderer` for the new node types. Keep it dependency-free.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/editor.spec.ts e2e/issues.spec.ts --workers=1` (issues.spec description-edit test must still pass — adapt its selectors if the textarea became a contentEditable, keeping its assertions).

- [ ] **Step 5: Commit**
```bash
git add frontend-next/components/common/RichTextEditor.tsx frontend-next/components/issues/adf.tsx frontend-next/components/issues/IssueView.tsx frontend-next/components/issues/Comments.tsx frontend-next/e2e/editor.spec.ts
git commit -m "feat(frontend): lightweight rich ADF editor (toolbar) for description + comments"
```

---

### Task 4: @mention autocomplete (frontend)

**Files:**
- Modify: `frontend-next/components/common/RichTextEditor.tsx` (mention autocomplete)
- Test: extend `frontend-next/e2e/editor.spec.ts` or a new `e2e/mention.spec.ts`

**Interfaces:** typing `@` opens an autocomplete over `profile.searchUsers(query)` (or `users.assignableSearch(projectKey, query)` when a projectKey is provided); selecting a user inserts an ADF `mention` node `{type:"mention",attrs:{id:accountId,text:"@"+displayName}}`.

- [ ] **Step 1: Write the failing test** — in a comment on DEMO-1, type `@`, pick a user from the autocomplete, submit; assert the rendered comment shows the mention chip (`@Name`); (optionally) assert the mentioned user gets a notification (check via the bell as that user, or assert the backend created it — keep deterministic).

- [ ] **Step 2: Run to verify it fails** — no autocomplete.

- [ ] **Step 3: Implement** — detect `@` + following token in the contentEditable, show a positioned dropdown of user results (debounced search), on select replace the token with a non-editable mention chip that serializes to a `mention` ADF node. Pass a `projectKey` prop from Comments/IssueView so `assignableSearch` can be used (fallback to global search).

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/editor.spec.ts e2e/mention.spec.ts --workers=1`.

- [ ] **Step 5: Commit**
```bash
git add frontend-next/components/common/RichTextEditor.tsx frontend-next/components/issues/Comments.tsx frontend-next/components/issues/IssueView.tsx frontend-next/e2e/
git commit -m "feat(frontend): @mention autocomplete producing ADF mention nodes"
```

---

### Task 5: Bell Direct/Watching tabs + grouping (frontend)

**Files:**
- Modify: `frontend-next/components/notifications/NotificationBell.tsx`
- Test: extend `frontend-next/e2e/users.spec.ts` (the bell test) or a new spec

**Interfaces:** classify each `AppNotification` by `type`: Direct = `assignment`/`mention`; Watching = `comment`/`status_change`/`sprint_started`/`sprint_completed`. Tabs `data-testid="notif-tab-direct"`/`notif-tab-watching`; within a tab, group items by their issue (derive from `link` or `title`).

- [ ] **Step 1: Write the failing test** — open the bell, assert `data-testid="notif-tab-direct"` and `notif-tab-watching` exist and switch the list; the existing "bell opens dropdown" assertion still holds.

- [ ] **Step 2: Run to verify it fails** — no tabs.

- [ ] **Step 3: Implement** — add the two tabs with counts; filter the list by the tab's type set; group entries under an issue header when several share an issue (parse the issue key from `link`/`title`). Keep mark-read / mark-all-read.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/users.spec.ts --workers=1`.

- [ ] **Step 5: Commit**
```bash
git add frontend-next/components/notifications/NotificationBell.tsx frontend-next/e2e/users.spec.ts
git commit -m "feat(frontend): notification bell Direct/Watching tabs + issue grouping"
```

---

### Task 6: Notification preferences — add a preference (frontend)

**Files:**
- Modify: `frontend-next/app/app/profile/page.tsx` (add-a-pref UI + project name resolution)
- Test: extend `frontend-next/e2e/users.spec.ts`

**Interfaces:** consumes `notifications.settings` + `notifications.updateSettings({project_id, event_type, via_email, via_app})` (upsert). Adds a form to create a NEW pref row: pick an event type (assignment/comment/mention/status_change/sprint), pick a project (from the user's projects list — send the project's id the settings API expects; `""` = global/all), set channels. Resolve project ids to names for display instead of raw UUIDs.

- [ ] **Step 1: Write the failing test** — on the profile page, add a preference (event type + channels), save, assert a new pref row appears with the chosen event type and toggles.

- [ ] **Step 2: Run to verify it fails** — no add-a-pref form.

- [ ] **Step 3: Implement** — an "Add preference" form (event-type select, project select from `projects.list`/the user's memberships with a "All projects" option, app/email checkboxes) → `notifications.updateSettings(...)` (upsert) → invalidate the settings query. Map each existing pref's `project_id` to a project name (from the loaded projects) instead of the raw UUID; `""` → "All projects".

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/users.spec.ts --workers=1`.

- [ ] **Step 5: Commit**
```bash
git add "frontend-next/app/app/profile/page.tsx" frontend-next/e2e/users.spec.ts
git commit -m "feat(frontend): add-a-notification-preference form + project name display"
```

---

### Task 7: Round close — gate, docs

- [ ] **Step 1: Full three-level gate**
```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test --workers=1 && cd ..
go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md
```
- [ ] **Step 2: CHANGELOG.md** — Added: rich ADF editor (toolbar) + @mention autocomplete for description/comments; @mentions now notify; bell Direct/Watching tabs + grouping; add-a-notification-preference; SMTP email delivery (worker, once per notification, migration 000020). Note deferred: slash-commands, WYSIWYG tables/images, email templating.
- [ ] **Step 3: STATE.md** — Round 20 entry (rich editor as dependency-free contentEditable with a constrained ADF vocabulary; ADF mention path revived; SMTP mailer no-op when unconfigured; email_sent guard) + follow-ups (slash-commands, richer ADF nodes, email HTML templates, Direct/Watching would be cleaner with a stored reason field, `.env.example` SMTP uncommented). Set "Prossimo: Round 21".
- [ ] **Step 4: Commit** docs + plan.
- [ ] **Step 5: Update auto-memory** (controller action).

---

## Self-Review

**Spec coverage:** B.5 rich editor toolbar → T3; @mention autocomplete + ADF node → T4; @mention notification (server) → T2. B.6 Direct/Watching tabs + grouping → T5; add-a-preference → T6; SMTP email delivery → T1. Deferred (noted): slash-commands, tables/images, email templating. ✅

**Placeholder scan:** backend interfaces (mailer, config, mention wiring, migration) exact; editor/AdfRenderer vocabulary explicitly bounded; UI specified by behavior + testids. No TBD.

**Type consistency:** `mailer.Mailer.Send(to,subject,body)` used by the worker; ADF mention node shape (`attrs.id`=accountId) consistent between the editor (T4), the renderer (T3), and the server parser (T2); `notifications.updateSettings` body matches the existing handler.

**Cross-cutting risks:** (1) contentEditable↔ADF round-trip is the riskiest part — keep the vocabulary strictly to what `AdfRenderer` renders and round-trip test it (T3). (2) @mention id is accountId=user id — the server resolves by id, not username (T2). (3) email must send once — the `email_sent` flag gates it; no-op mailer when SMTP unset so CI is unaffected (T1). (4) don't double-notify a mentioned user (dedupe; skip author) (T2). (5) description save now sends the editor's ADF directly (not `textToAdf(text)`) — adapt `issues.spec` description test selectors while keeping assertions. (6) full suite `--workers=1`.
