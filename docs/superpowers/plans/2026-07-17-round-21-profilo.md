# Round 21 — Profilo (B.8: locale, tema light/dark, avatar upload) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Complete the user profile: a language/locale selector (backend already persists it — UI only), a light/dark theme toggle (persisted, applied to the app chrome), and avatar upload (new endpoint reusing the uploads dir; the app already renders `avatar_url`).

**Architecture:** Locale is fully wired server-side (`users.locale`, `PUT /myself`, `profile.update({locale})`) — this round only adds the UI control. Avatar: `users.avatar_url` exists and TopBar/UserPicker already render it, but there is no upload route (the attachment service is issue-bound); add a dedicated `POST /rest/api/3/myself/avatar` (multipart) that stores the image under `APP_UPLOADS_DIR/avatars/` and sets `users.avatar_url`, served at a public content route (matching the existing public default-avatar SVG so `<img src>` works without a bearer token). Theme: no dark-mode support exists (Tailwind v4, no config, hardcoded hex everywhere) — add a dependency-free ThemeProvider (localStorage + `prefers-color-scheme` + a `dark` class on `<html>`), enable the Tailwind v4 `dark` variant, and theme the app chrome (layout/body, TopBar, Sidebar, page backgrounds, cards, primary text); deep per-view hex polish is a documented follow-up.

**Tech Stack:** Go 1.25 (`net/http`, `mime/multipart`, GORM, in-memory SQLite tests), Next.js 16 App Router + React 19 + Tailwind v4 (no new lib), Playwright.

## Global Constraints

- **Module path `github.com/it4nodummies/heureum`** — verbatim in imports.
- **No new dependency** for theming — a small custom React context + `localStorage`, not `next-themes`.
- **Avatar content is served publicly** (no `authMw`), matching the existing public `/static/default-avatar.svg` route, so `<img src={avatar_url}>` renders everywhere without a bearer token. Avatar images are low-sensitivity; note this in docs. The UPLOAD route (`POST /myself/avatar`) IS authenticated (writes the caller's own avatar only — keyed on the context user id, never a path UUID).
- **Validate uploads:** accept only image content types (png/jpeg/gif/webp), cap size (e.g. 2 MB); reject otherwise with 400.
- **No migration needed** — `users.avatar_url` and `users.locale` already exist. (If a strong reason arises, the next number is `000021`, but avoid it.)
- **Theme scope is the app chrome**, not every component. Deliver a coherent dark look for: `<body>`/page background, `TopBar`, `Sidebar`, the main content container, common card/panel surfaces, and primary text — via `dark:` utilities and/or the existing `globals.css` CSS variables. Per-view deep-hex polish is an explicit follow-up, not part of the acceptance bar. Do not regress the light theme.
- UI accent `#0052cc`; UI under `/app`; single typed client `frontend-next/lib/api.ts`.
- Three-level gate before done: (1) `go build ./... && go vet ./... && go test ./...`; (2) `cd frontend-next && npm run build && npx playwright test --workers=1`; (3) `go run ./cmd/gapreport` — the new avatar routes are Heureum extensions → commit the regenerated report; clean on a second run.
- Conventional Commits; branch `feat/frontend-next`. E2E inline login from `e2e/export.spec.ts`; full suite `--workers=1`.

---

### Task 1: Avatar upload + serve endpoints (backend)

**Files:**
- Create: `internal/api/handlers/avatar_handler.go` (or add to `user_handler.go`)
- Modify: `internal/api/router.go` (routes), and thread `UploadsDir` where needed
- Modify: `internal/domain/user/service.go` if a helper is cleaner (optional)
- Test: `internal/api/handlers/avatar_handler_test.go`

**Interfaces:**
- `POST /rest/api/3/myself/avatar` — authMw; parses multipart form field `file`; validates image content-type + size (≤2 MB); saves bytes to `filepath.Join(cfg.UploadsDir, "avatars", userID+ext)` (ext from the uploaded filename or content-type; deterministic name per user so re-upload overwrites); sets `users.avatar_url = "/rest/api/3/user/avatar/"+userID` (a served URL); returns 200 `v3.JiraUser` of the updated user.
- `GET /rest/api/3/user/avatar/{userId}` — PUBLIC (no authMw, like `serveDefaultAvatar`); serves the stored file for that user with the right Content-Type; 404 (or redirect to the default avatar) if none.
- Reuse `cfg.UploadsDir`; `os.MkdirAll(avatarsDir)` on write.

- [ ] **Step 1: Write the failing test** — `avatar_handler_test.go` (mirror an existing handler test harness): build a multipart request with a tiny PNG (a few bytes with a `.png` name + `image/png` content type) to `POST /myself/avatar` with an authed user context; assert 200 and the returned user's `avatarUrls` point at `/rest/api/3/user/avatar/<uid>`; then `GET /rest/api/3/user/avatar/<uid>` returns 200 with the bytes. Add a negative: a `text/plain` upload → 400.

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/api/handlers/ -run Avatar -v` → FAIL.

- [ ] **Step 3: Implement** — the handler (multipart parse, content-type/size validation, save to uploads/avatars, update `avatar_url` via the user service), the public serve handler, the routes, and any wiring (the handler needs the DB + UploadsDir + BaseURL). Store a deterministic filename per user (`<uid>.<ext>`); persist the served URL in `avatar_url`.

- [ ] **Step 4: Verify** — `go test ./internal/api/handlers/ -run Avatar -v && go build ./... && go vet ./... && go test ./... && go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md` (2 new extension routes → commit regenerated report; contract stays green).

- [ ] **Step 5: Commit**
```bash
git add internal/api/handlers/avatar_handler.go internal/api/router.go internal/domain/user/ internal/api/handlers/avatar_handler_test.go docs/contracts/gap-report.md
git commit -m "feat(profile): avatar upload + public avatar serve endpoints"
```

---

### Task 2: Profile page — locale selector + avatar upload (frontend)

**Files:**
- Modify: `frontend-next/lib/api.ts` (`profile.uploadAvatar`)
- Modify: `frontend-next/app/app/profile/page.tsx` (locale `<select>` + avatar upload control)
- Test: extend `frontend-next/e2e/users.spec.ts`

**Interfaces:**
- `profile.uploadAvatar(file: File): Promise<JiraUser>` — POST multipart to `/rest/api/3/myself/avatar` with `authHeaders()` (no JSON content-type; let the browser set the multipart boundary). `profile.update` already accepts `locale`.
- Profile page: a locale `<select>` (a small curated list, e.g. `en`, `it`, `es`, `fr`, `de`, `pt`; value from `me.locale`) included in the existing `profile.update({displayName, timeZone, locale})` save; an avatar block showing the current avatar (`me.avatarUrls["48x48"]` or initials) + a file input "Upload avatar" → `profile.uploadAvatar(file)` → invalidate `["profile","me"]` (and the stored user so the TopBar updates if easy).

- [ ] **Step 1: Write the failing test** — extend `users.spec.ts`: on `/app/profile`, the locale `<select>` (`data-testid="profile-locale"` or an aria-label "Language") exists; change it, Save, reload → persists. (Avatar upload via a real file in Playwright is possible with `setInputFiles` on a fixture image — assert the request succeeds / the img src updates; keep it robust or assert at least the control renders.)

- [ ] **Step 2: Run to verify it fails** — no locale select / avatar control.

- [ ] **Step 3: Implement** — add the locale select (seed its value from `me.data?.locale`), include `locale` in the save mutation; add the avatar block (current avatar preview + file input calling `profile.uploadAvatar`, invalidating the profile query on success). Read `profile/page.tsx` first and keep the existing displayName/timeZone/prefs intact.

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/users.spec.ts --workers=1`.

- [ ] **Step 5: Commit**
```bash
git add frontend-next/lib/api.ts "frontend-next/app/app/profile/page.tsx" frontend-next/e2e/users.spec.ts
git commit -m "feat(frontend): profile locale selector + avatar upload"
```

---

### Task 3: Light/dark theme toggle (frontend)

**Files:**
- Create: `frontend-next/components/layout/ThemeProvider.tsx` (context + persistence) and a `ThemeToggle` control
- Modify: `frontend-next/app/globals.css` (Tailwind v4 `dark` variant + dark CSS-var values), `frontend-next/app/layout.tsx` (mount provider, no-flash script), and the chrome components (`TopBar`, `Sidebar`, the app layout container) for dark styling
- Modify: `frontend-next/app/app/profile/page.tsx` (a theme toggle in the profile) — optionally also the TopBar dropdown
- Test: `frontend-next/e2e/theme.spec.ts` (create)

**Interfaces:**
- `ThemeProvider` — React context holding `theme: "light"|"dark"`, initialized from `localStorage["theme"]` else `prefers-color-scheme`; on change writes localStorage and toggles the `dark` class on `document.documentElement`. A small inline `<script>` in `layout.tsx` `<head>` applies the stored/preferred class before paint (no flash).
- `ThemeToggle` — a button (`data-testid="theme-toggle"`) that flips the theme via the context.
- `globals.css` — add the Tailwind v4 dark variant: `@custom-variant dark (&:where(.dark, .dark *));` and dark values (either `.dark { --background: ...; --foreground: ...; --surface: ...; --border: ... }` consumed by chrome, and/or `dark:` utilities on the chrome elements).

- [ ] **Step 1: Write the failing test** — `theme.spec.ts` (inline login): assert `document.documentElement` lacks `dark` by default (or matches system), click `data-testid="theme-toggle"`, assert `<html>` gains the `dark` class AND `localStorage.theme === "dark"`; reload → still dark (persisted). Optionally assert the body background changed.

- [ ] **Step 2: Run to verify it fails** — no toggle / no dark class.

- [ ] **Step 3: Implement** — the provider + toggle + no-flash script; enable the dark variant in `globals.css` and give the CHROME a coherent dark look (body/page background, TopBar, Sidebar, main container, common `bg-white`/card surfaces → `dark:bg-...`, primary text `dark:text-...`). Focus on the navigable shell; do not attempt every deep component. Ensure the light theme is unchanged when `dark` is absent. Place the toggle in the profile page (and optionally the TopBar user dropdown).

- [ ] **Step 4: Verify** — `npm run build && npx playwright test e2e/theme.spec.ts e2e/users.spec.ts --workers=1` (theme toggles + persists; profile still works). Manually sanity-check that the main pages are legible in dark (chrome themed).

- [ ] **Step 5: Commit**
```bash
git add frontend-next/components/layout/ThemeProvider.tsx frontend-next/app/globals.css frontend-next/app/layout.tsx frontend-next/components/layout/TopBar.tsx frontend-next/components/layout/Sidebar.tsx "frontend-next/app/app/profile/page.tsx" frontend-next/e2e/theme.spec.ts
git commit -m "feat(frontend): persisted light/dark theme toggle (app chrome themed)"
```

---

### Task 4: Round close — gate, docs

- [ ] **Step 1: Full three-level gate**
```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test --workers=1 && cd ..
go run ./cmd/gapreport && git status --short docs/contracts/gap-report.md
```
- [ ] **Step 2: CHANGELOG.md** — Added: profile language/locale selector; light/dark theme toggle (persisted; app chrome themed); avatar upload (`POST /myself/avatar` + public serve). Note deferred: full per-view dark polish; avatar cropping.
- [ ] **Step 3: STATE.md** — Round 21 entry (locale was backend-ready — UI only; avatar reuses uploads dir, served publicly; theme = dependency-free provider + chrome, deep polish deferred) + follow-ups (per-component dark polish, avatar crop/resize, `.env.example` SMTP still commented from R20). Note this completes the planned R13→R21 queue; remaining work is FASE D (out of scope) or the post-1.0 hardening list. Set "Prossimo" to the hardening backlog / a possible release.
- [ ] **Step 4: Commit** docs + plan.
- [ ] **Step 5: Update auto-memory** (controller action) — Round 21 done; R13→R21 queue complete.

---

## Self-Review

**Spec coverage (B.8):** locale selector → T2; theme light/dark → T3; avatar upload → T1 (backend) + T2 (UI). C.2 Components explicitly excluded per the user. ✅

**Placeholder scan:** backend avatar endpoints (routes, validation, storage, served URL) exact; theme provider behavior + persistence exact; UI specified by behavior + testids. No TBD.

**Type consistency:** `profile.uploadAvatar(file) → JiraUser` matches the handler's returned updated user; `avatar_url` served at `/rest/api/3/user/avatar/{userId}` and surfaced through the existing `avatarUrls` map; `profile.update` locale field already exists.

**Cross-cutting risks:** (1) avatar content served publicly (matches default-avatar precedent) so `<img>` works without a token — note in docs; the UPLOAD route stays authed and writes only the caller's own avatar. (2) validate content-type + size on upload (reject non-images). (3) theme is scoped to the chrome — do NOT regress light mode; use a no-flash head script to avoid a light flash before hydration. (4) new avatar routes raise gapreport extensions — commit the report. (5) full suite `--workers=1`.
