# Round 29 — Italian UI (i18n) — Design

**Status:** proposed (2026-11-03)
**Driver:** the reference team is Italian; Heureum's UI is English-only. `users.locale` exists
(migration `000014`, `PUT /rest/api/3/myself {locale}`, a "Language" select in the profile) but
drives nothing. Direction: zero-config — an Italian user should get an Italian UI without anyone
configuring anything.

The user skipped the design questions ("procedi con i default"): every decision below is the
round author's call, stated so it can be challenged.

## Decisions (author's defaults)
- **Two languages: English (source) and Italian.** No other locale ships in this round; the
  machinery must make adding one a matter of adding a messages file.
- **Zero dependencies.** A small hand-rolled i18n layer (typed message catalogues + React context +
  the platform `Intl` APIs), consistent with the repo's "no charting library" stance. No
  `next-intl`/`i18next`.
- **Locale resolution (first match wins):** the user's saved `locale` (`it*` → it, `en*` → en) →
  the `heureum_locale` cookie (last resolved locale, so the login page and the first paint are right)
  → the browser language (`navigator.language`) → `en`. An empty `locale` means "follow the
  browser" — the zero-config path: an Italian browser gets Italian with nobody touching settings.
- **No flash, no hydration mismatch:** the root layout reads the cookie on the server to set
  `<html lang>` and the provider's initial locale; the client re-resolves once `/myself` loads and
  rewrites the cookie when the result differs.
- **Profile "Language" select** offers exactly *Default (browser)*, *English*, *Italiano*. A stored
  value from the old list (`es`, `fr`, `de`, `pt`) is treated as Default and shown as such. Saving
  applies the language immediately (no reload).
- **What is translated: UI chrome only** — labels, buttons, headings, placeholders, `aria-label`s,
  empty states, confirmations, client-side error/validation text, tooltips, status-*category* labels
  (To Do / In Progress / Done), date/time/number formatting and relative times (`Intl.DateTimeFormat`,
  `Intl.RelativeTimeFormat`, `Intl.NumberFormat`), pluralisation (`Intl.PluralRules`).
- **What is NOT translated: user data.** Status names, issue-type names, project/board/sprint/epic
  names, custom-field names, workflow names stay exactly as stored — including the scaffolded
  defaults (`TO DO`, `IN PROGRESS`, `TO BE TESTED`, `DONE`; `Epic`, `Story`, `Task`, `Bug`,
  `Subtask`). They are data that users type in JQL; translating them in display only would make the
  UI and JQL disagree. The reference team already mixes IT/EN status names, so this is familiar.
- **Server-generated text stays English on the wire.** Jira-compat API error messages (`errorMessages`,
  `errors`) are a frozen contract (ADR-0001) and are shown as received. **Notifications** are
  rendered client-side from their `type` (`assignment`, `comment`, `mention`, `sprint_started`,
  `sprint_completed`, `status_change`) with the issue key taken from `link`; the stored `title` is
  the fallback for unknown types. The stored `body` (issue title / comment text) is data. No backend
  or schema change for notifications.
- **Glossary (Italian):** agile terms stay as the team says them — *Sprint, Backlog, Board, Epic,
  Story point, Kanban, Scrum, Swimlane, JQL*. "Issue" → **"ticket"** (the word Italian dev teams and
  Jira's Italian UI use). Workflow → "workflow". Dashboard → "dashboard". Filter → "filtro".
  Assignee → "assegnatario", Reporter → "segnalatore", Watchers → "osservatori", Priority →
  "priorità", Resolution → "risoluzione", Due date → "scadenza", Release/Version → "release".
  Status categories: *Da fare / In corso / Completato*. Tone: informal-professional, imperative for
  actions ("Crea ticket", "Salva", "Annulla"), sentence case.
- **Italian type names recognised by the backend** (small, so a team that names types in Italian
  gets the right behaviour): `levelForTypeName`/`TypeIDByName` treat `sottotask`, `sotto-task`,
  `sotto-attività`, `sottoattività` as subtask (level −1, `is_subtask`, subtask icon) and `epica` as
  epic (level 1). Existing data unchanged (no migration).

## Goals
1. `lib/i18n`: catalogues per area (`messages/en/<area>.ts` is the source of truth; `messages/it/<area>.ts`
   is typed against it so **a missing or extra Italian key fails `npm run build`**), `I18nProvider`,
   `useT()` → `t(key, params?)` with `{name}` interpolation and `{count}` plural forms,
   `useFormat()` for dates/relative times/numbers in the active locale, `useLocale()`/`setLocale()`.
2. Every user-visible string in `frontend-next/app/**` and `frontend-next/components/**` goes through
   `t()`; every date/number shown goes through `useFormat()`.
3. Default experience unchanged for English users (all existing e2e specs keep passing untouched,
   since Playwright's browser is `en-US` and the seeded users have an empty locale).
4. An Italian user (profile `Italiano`, or empty locale + Italian browser) sees the whole app —
   login, navigation, issue detail, create modal, backlog, board, settings, search, dashboards,
   reports, notifications, profile — in Italian, with Italian dates (`3 nov 2026`, `2 ore fa`).

## Non-goals
- Locales beyond en/it; right-to-left; translating user data; translating API error strings;
  e-mail templates (none ship today); per-instance default locale config (zero-config via the browser
  instead; anti-pattern #13 — no speculative config).

## Anti-goals (would make this a failure even if it "works")
- A half-Italian screen: Italian chrome with English leftovers on a main page.
- A flash of English on every load for Italian users, or hydration warnings.
- Breaking the English UI or the existing e2e suite; adding a dependency.

## Design notes
- **Key style:** namespaced dotted keys per area (`issues.create.title`), values are plain strings;
  plurals as `{ one: "...", other: "..." }` objects resolved with `Intl.PluralRules`.
- **Typing:** `en` catalogues are `as const` objects; the `it` ones are declared with a
  `Messages<typeof en>` mapped type (same keys, string leaves, same plural shape). `t()`'s key
  parameter is the union of dotted paths, so a typo is a type error.
- **Server components:** pages under `/app` are client components; the few server-rendered bits
  (root layout `<html lang>`, metadata title) read the cookie. Login is client-side too.
- **Testing:** Playwright `e2e/i18n.spec.ts` — Italian via profile switch and via `locale: 'it-IT'`
  browser context with an empty profile locale; asserts Italian labels on each main page, Italian
  date formatting, a localized notification, and switching back to English. A catalogue guard script
  (`scripts/check-i18n.mjs`, run in `npm run build` via `prebuild`) is **not** required — the type
  system already enforces catalogue parity; leftover hard-coded strings are caught by review + the
  e2e sweep.

## Review focus
- Any main page in Italian with an English leftover (reviewers sweep each page in `it`).
- Hydration warnings in the browser console on first load in `it`.
- Existing English e2e specs untouched and green.
- Italian copy: consistent with the glossary, natural, no machine-translation artefacts.
- Dates/numbers locale-aware everywhere they are shown (no `toLocaleDateString()` without the
  active locale, no hand-formatted dates).
