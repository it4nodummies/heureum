# Round 10 — Release 1.0 (rebrand "Heureum" + release engineering) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Portare il repo a "pronto per la pubblicazione open source 1.0": rebrand completo da "Open Jira" a **Heureum** (senza toccare la compat API v3/JQL), rimozione del vecchio frontend Vite, licenza AGPL-3.0 + documentazione di community (README EN, CONTRIBUTING, CoC, SECURITY, CHANGELOG), packaging coerente (docker-compose + Helm), workflow CI di release verso GHCR, e istruzioni di pubblicazione. Il push finale su GitHub NON è nel piano (lo fa l'utente).

**Architecture:** Round di sole modifiche meccaniche/config/docs, a invariante forte: **il gate (build+vet+test Go, tsc+build+E2E frontend, gap report senza drift) resta verde dopo ogni task**. Nessuna modifica di comportamento runtime tranne rinomini di stringhe/percorsi. La compat con Jira Cloud REST API v3 (`/rest/api/3`, `/rest/agile/1.0`, JQL) è **intoccabile**: sono superfici di compatibilità, non branding.

**Tech Stack:** Go 1.25 (module rename via `go mod edit` + sed sugli import), Next.js 16 / React 19 (frontend-next), Playwright, Docker + docker-compose, Helm, GitHub Actions (GHCR).

---

## Decisioni bloccate (dall'utente)

- **Nome prodotto: `Heureum`.**
- **Owner GitHub: `it4nodummies`** (confermato dall'utente).
- **Module path Go: `github.com/it4nodummies/heureum`.**
- **Immagini container: `ghcr.io/it4nodummies/heureum-{api,worker,frontend}`** (GHCR sotto l'owner). Le compose di prod e i values Helm puntano a questi.
- **Enforcement permessi: NON in questo round** → è il **Round 11** dedicato (middleware 403 + `creator=admin`). Round 10 non tocca l'autorizzazione.
- **Pubblicazione GitHub: preparazione soltanto.** Il piano lascia il repo "pronto"; il `git push`/creazione repo/tag lo esegue l'utente (Task 13 fornisce i comandi esatti, NON li esegue).

## Scelte di scope (esplicite)

- **Compat da NON toccare:** prefissi path `/rest/api/3/`, `/rest/agile/1.0/`; grammatica/parser JQL (`internal/jql/`); valori compat (`AccountType:"atlassian"`, template key `com.atlassian.jira-core-project-templates:*`, tipo JQL `com.atlassian.jira.user.ApplicationUser`); spec vendored `docs/contracts/*.json` (oracolo di conformità). Questi contengono "jira"/"atlassian" per fedeltà API e restano.
- **Identificatori Go `Jira*`** (JiraUser/JiraTime/JiraIssue…, ~160 occorrenze in `internal/api/v3`): interni, non esposti, rispecchiano i DTO v3 → **NON rinominati** (churn senza beneficio trademark). Non-goal dichiarato.
- **Header webhook esterni `X-OpenJira-*`**: visibili nei payload in uscita → **rinominati** in `X-Heureum-*` (Task 3).
- **Prefisso URL UI `/jira`**: è in barra indirizzi (user-facing) → **rinominato** in `/app` (Task 5).
- **Docs storici** (`docs/superpowers/plans/*`, `docs/superpowers/specs/*`, `jira-opensource-spec.md`): artefatti datati, si lasciano com'è (riferimenti a `frontend/` o "Open Jira" al loro interno sono storia, non docs pubblicate). Solo README, `docs/api/openapi.yaml` e i file di community vengono aggiornati.

## Contesto verificato (dallo scout — leggere una volta)

- **Vecchio frontend `frontend/`**: SPA Vite/React tracciata in git (38 file). Unici consumatori funzionali: `Dockerfile.frontend` e la riga 160 del README. Le compose e la CI usano solo `frontend-next`. `docker-compose.prod.yml` è incoerente (referenzia `open-jira/frontend:latest` su porta 80 = vecchia SPA/nginx, mentre `docker-compose.yml` builda `Dockerfile.frontend-next` su 3000).
- **Module path**: `github.com/open-jira/open-jira` in `go.mod`; importato da **74 file .go**.
- **Header webhook**: `internal/domain/webhook/delivery.go` setta `X-OpenJira-Event` / `X-OpenJira-Signature`; verificati in `internal/domain/webhook/delivery_test.go` e `internal/contract/webhook_test.go`.
- **Branding UI "Open Jira"** (4 punti): `frontend-next/app/layout.tsx:6` (`metadata.title`), `frontend-next/app/jira/projects/page.tsx:3` (`metadata.title`), `frontend-next/app/login/page.tsx:62` (heading), `frontend-next/components/layout/Sidebar.tsx:171` (wordmark). Favicon: `frontend-next/app/favicon.ico` (25.9 KB, origine non verificata → sostituire). Starter SVG Vercel in `frontend-next/public/` (`file.svg`, `globe.svg`, `next.svg`, `vercel.svg`, `window.svg`).
- **Route UI**: tutto sotto `frontend-next/app/jira/*` (`boards/[boardId]`, `boards/[boardId]/backlog`, `browse/[key]`, `dashboards`, `dashboards/[id]`, `filters`, `profile`, `projects`, `projects/[key]`, `projects/[key]/reports`, `projects/[key]/settings`). 103 ref a `/jira` in .ts/.tsx; 2 redirect (`app/page.tsx`, `app/login/page.tsx:34`); 10 spec E2E in `frontend-next/e2e/`.
- **Config runtime** (`internal/config/config.go`, unico loader): legge solo `APP_PORT`(8080), `APP_ENV`(development), `APP_SECRET`(**required**), `APP_BASE_URL`, `DB_DRIVER`(postgres), `DB_DSN`(**required**), `REDIS_URL`. **Dead config** (documentata ma non letta da alcun `.go`): `SMTP_*`, `OAUTH_*`, `STORAGE_*`, `S3_*`.
- **Binari `cmd/`**: `server` (runna migrazioni + serve), `worker` (tick 30s), `seed` (migra+seed demo), `gapreport` (genera `docs/contracts/gap-report.md`). Nessun `cmd/migrate`.
- **Docker**: `Dockerfile` (api, :8080, copia `migrations/`), `Dockerfile.worker`, `Dockerfile.frontend-next` (Next standalone, :3000), `Dockerfile.frontend` (SPA vecchia, orfana). `.dockerignore` **assente**. Compose in `deploy/docker/`. Helm in `deploy/helm/open-jira/`.
- **CI**: unico `.github/workflows/ci.yml` (job backend/gap-report/frontend/e2e), trigger push+PR su master/main. **Nessun** release/tag/publish. Nessun issue/PR template, CODEOWNERS, dependabot.
- **Legal/docs mancanti**: `LICENSE`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, `CHANGELOG.md`, `.dockerignore`. README esiste ma è in italiano, descrive il frontend come "Vite SPA" (stale) e documenta env var morte.

---

## Struttura dei file

- **Rimozioni:** `frontend/` (intera dir), `Dockerfile.frontend`.
- **Rename Go module:** `go.mod` + 74 file `.go` (solo blocco import).
- **Webhook header:** `internal/domain/webhook/delivery.go`, `internal/domain/webhook/delivery_test.go`, `internal/contract/webhook_test.go` (+ eventuali ref in `internal/integration`).
- **Branding FE:** `frontend-next/app/layout.tsx`, `frontend-next/app/jira/projects/page.tsx`, `frontend-next/app/login/page.tsx`, `frontend-next/components/layout/Sidebar.tsx`, `frontend-next/app/icon.svg` (nuovo), rimozione `frontend-next/app/favicon.ico` + starter svg inutilizzati; `cmd/seed/main.go` (stringhe demo visibili); `docs/api/openapi.yaml` (title).
- **Route rename:** dir `frontend-next/app/jira/` → `frontend-next/app/app/`; sostituzione `/jira` → `/app` in .ts/.tsx e `frontend-next/e2e/*`.
- **Docs/legal (root):** `README.md` (riscrittura EN), `LICENSE`, `.env.example`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, `CHANGELOG.md`, `docs/RELEASE.md` (nuovo).
- **`.github/`:** `ISSUE_TEMPLATE/bug_report.md`, `ISSUE_TEMPLATE/feature_request.md`, `ISSUE_TEMPLATE/config.yml`, `PULL_REQUEST_TEMPLATE.md`, `CODEOWNERS`, `dependabot.yml`, `workflows/release.yml`.
- **Packaging:** `.dockerignore` (nuovo), `deploy/docker/docker-compose.yml`, `deploy/docker/docker-compose.prod.yml`, dir Helm `deploy/helm/open-jira/` → `deploy/helm/heureum/` (Chart.yaml, values.yaml, templates).
- **Stato:** `docs/superpowers/STATE.md`, memoria di progetto.

---

### Task 1: Rimozione del vecchio frontend Vite

**Files:**
- Delete: `frontend/` (intera dir), `Dockerfile.frontend`

- [ ] **Step 1: Confermare che nulla di funzionale dipenda da `frontend/`**

Run: `cd /Users/n0r41n/Development/open-jira && grep -rn --include='*.yml' --include='*.yaml' --include='Dockerfile*' -e 'frontend/' -e 'Dockerfile.frontend' deploy/ .github/ Dockerfile* nginx.conf 2>/dev/null`
Expected: NESSUN match nelle compose/CI (solo `Dockerfile.frontend` stesso, che rimuoviamo). Se compare un riferimento in `deploy/docker/docker-compose*.yml`, annotarlo: verrà sistemato nel Task 10 (non bloccare qui).

- [ ] **Step 2: Rimuovere**

Run:
```bash
cd /Users/n0r41n/Development/open-jira
git rm -r frontend
git rm Dockerfile.frontend
```

- [ ] **Step 3: Verificare build backend + build frontend-next intatti**

Run: `go build ./... && echo GO_OK && cd frontend-next && npx tsc --noEmit && echo TSC_OK; cd ..`
Expected: `GO_OK`, `TSC_OK` (il backend non dipende dal frontend; `frontend-next` è indipendente da `frontend/`).

- [ ] **Step 4: Commit**

```bash
git commit -m "chore: remove legacy Vite frontend (superseded by frontend-next)"
```

---

### Task 2: Rename module path Go → github.com/heureum/heureum

**Files:**
- Modify: `go.mod` (riga `module`)
- Modify: tutti i `.go` che importano il vecchio path (74 file)

- [ ] **Step 1: Cambiare il module path**

Run:
```bash
cd /Users/n0r41n/Development/open-jira
go mod edit -module github.com/it4nodummies/heureum
```

- [ ] **Step 2: Riscrivere gli import in tutti i .go**

Run (macOS sed):
```bash
cd /Users/n0r41n/Development/open-jira
grep -rl 'github.com/open-jira/open-jira' --include='*.go' . \
  | grep -v '/.claude/' \
  | xargs sed -i '' 's#github.com/open-jira/open-jira#github.com/it4nodummies/heureum#g'
```
(Escludere `.claude/worktrees/` — è una copia stale fuori scope.)

- [ ] **Step 3: Verificare che non resti il vecchio path**

Run: `grep -rn 'open-jira/open-jira' --include='*.go' . | grep -v '/.claude/' || echo NONE_LEFT`
Expected: `NONE_LEFT`.

- [ ] **Step 4: Tidy + gate Go completo**

Run:
```bash
go mod tidy
go build ./... && echo BUILD_OK
go vet ./... && echo VET_OK
go test ./... 2>&1 | grep -vE '^ok|no test files' ; echo GO_TESTS_DONE
```
Expected: `BUILD_OK`, `VET_OK`, nessuna riga di FAIL prima di `GO_TESTS_DONE`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: rename Go module to github.com/it4nodummies/heureum"
```

---

### Task 3: Rinominare gli header webhook esterni X-OpenJira-* → X-Heureum-*

**Files:**
- Modify: `internal/domain/webhook/delivery.go`
- Modify: `internal/domain/webhook/delivery_test.go`
- Modify: `internal/contract/webhook_test.go`

- [ ] **Step 1: Trovare tutte le occorrenze del prefisso header**

Run: `cd /Users/n0r41n/Development/open-jira && grep -rn 'OpenJira' --include='*.go' . | grep -v '/.claude/'`
Expected: occorrenze in `delivery.go` (set header), `delivery_test.go`, `internal/contract/webhook_test.go`. Annotare eventuali altri (es. `internal/integration/dispatcher.go`).

- [ ] **Step 2: Sostituire `X-OpenJira-` → `X-Heureum-` e ogni identificatore/stringa `OpenJira` → `Heureum`**

Run:
```bash
cd /Users/n0r41n/Development/open-jira
grep -rl 'OpenJira' --include='*.go' . | grep -v '/.claude/' \
  | xargs sed -i '' 's/OpenJira/Heureum/g'
```
(Questo cambia `X-OpenJira-Event`→`X-Heureum-Event`, `X-OpenJira-Signature`→`X-Heureum-Signature`, e ogni riferimento nei test.)

- [ ] **Step 3: Verificare coerenza test webhook (delivery + contract)**

Run: `go test ./internal/domain/webhook/ ./internal/contract/ -run 'Webhook|Deliver|Sign' -v 2>&1 | tail -30`
Expected: PASS (il test `TestDeliver_PostsSignedPayload` e `TestWebhook_FiresOnIssueCreate` ora asseriscono `X-Heureum-Event`).

- [ ] **Step 4: Gate Go**

Run: `go build ./... && echo BUILD_OK && go test ./... 2>&1 | grep -vE '^ok|no test files'; echo DONE`
Expected: `BUILD_OK`, nessun FAIL.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(webhook): rename outbound headers to X-Heureum-*"
```

---

### Task 4: Rebrand UI "Open Jira" → "Heureum" + favicon + asset cleanup

**Files:**
- Modify: `frontend-next/app/layout.tsx`, `frontend-next/app/jira/projects/page.tsx`, `frontend-next/app/login/page.tsx`, `frontend-next/components/layout/Sidebar.tsx`
- Create: `frontend-next/app/icon.svg`
- Delete: `frontend-next/app/favicon.ico`, starter SVG inutilizzati in `frontend-next/public/`
- Modify: `cmd/seed/main.go` (solo stringhe demo user-visible), `docs/api/openapi.yaml` (title)

- [ ] **Step 1: Sostituire il wordmark/titolo "Open Jira" → "Heureum"**

Leggere ciascun file e sostituire la stringa visibile. Comando bulk (verificare a mano dopo):
```bash
cd /Users/n0r41n/Development/open-jira
grep -rl 'Open Jira' frontend-next/app frontend-next/components \
  | xargs sed -i '' 's/Open Jira/Heureum/g'
```
Attesi 4 file toccati: `app/layout.tsx` (`title: "Heureum"`), `app/jira/projects/page.tsx` (`title: "Projects – Heureum"`), `app/login/page.tsx` (heading), `components/layout/Sidebar.tsx` (wordmark). Verificare con `grep -rn 'Open Jira' frontend-next || echo NONE`.

- [ ] **Step 2: Creare un'icona Heureum e rimuovere la favicon di origine incerta**

`frontend-next/app/icon.svg` (Next.js genera automaticamente le favicon da `icon.svg`; mark testuale "H", nessun asset di terzi):
```svg
<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
  <rect width="64" height="64" rx="14" fill="#0052cc"/>
  <text x="50%" y="50%" dy="0.35em" text-anchor="middle" font-family="Helvetica,Arial,sans-serif" font-size="40" font-weight="700" fill="#ffffff">H</text>
</svg>
```
Poi:
```bash
cd /Users/n0r41n/Development/open-jira
git rm frontend-next/app/favicon.ico
```
(Rimuove il binario di origine non verificata: `icon.svg` lo rimpiazza.)

- [ ] **Step 3: Rimuovere gli starter SVG Vercel se non referenziati**

Run: `cd /Users/n0r41n/Development/open-jira && for f in file globe next vercel window; do grep -rqn "$f.svg" frontend-next/app frontend-next/components frontend-next/lib && echo "USATO: $f.svg" || echo "orfano: $f.svg"; done`
Per ogni "orfano": `git rm frontend-next/public/<nome>.svg`. NON rimuovere quelli marcati "USATO".

- [ ] **Step 4: Stringhe demo nel seed + title openapi**

- In `cmd/seed/main.go`: cercare stringhe user-visible "Open Jira"/"Jira" nei nomi demo (NON toccare i template key `com.atlassian.jira-core-project-templates:*` — sono compat). Se una stringa di brand appare, rinominarla a "Heureum". `grep -n 'Open Jira\|Jira' cmd/seed/main.go` e valutare caso per caso.
- In `docs/api/openapi.yaml`: `title: Open Jira` → `title: Heureum` (grep e sostituire il campo `info.title`).

- [ ] **Step 5: Type-check + build**

Run: `cd /Users/n0r41n/Development/open-jira && go build ./... && echo GO_OK && cd frontend-next && npx tsc --noEmit && npm run build 2>&1 | tail -3; cd ..`
Expected: `GO_OK`, build Next OK.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(brand): rebrand UI to Heureum; add icon, drop starter assets"
```

---

### Task 5: Rinominare il prefisso URL /jira → /app

**Files:**
- Rename dir: `frontend-next/app/jira/` → `frontend-next/app/app/`
- Modify: ogni `.ts`/`.tsx` in `frontend-next/app`, `frontend-next/components`, `frontend-next/lib` con stringa `/jira`
- Modify: `frontend-next/e2e/*.spec.ts`

> **Nota:** puramente URL-namespace del frontend. NON tocca `/rest/api/3` né `/rest/agile/1.0` (chiamati da `lib/api.ts`, restano). Il segmento diventa `/app` (folder `frontend-next/app/app/` — valido in Next App Router).

- [ ] **Step 1: Spostare la cartella delle route**

Run:
```bash
cd /Users/n0r41n/Development/open-jira/frontend-next
git mv app/jira app/app
```

- [ ] **Step 2: Sostituire le stringhe `/jira` → `/app` (link, redirect, e2e)**

Run:
```bash
cd /Users/n0r41n/Development/open-jira/frontend-next
grep -rl '/jira' app components lib e2e --include='*.ts' --include='*.tsx' \
  | xargs sed -i '' 's#/jira#/app#g'
```

- [ ] **Step 3: Verificare che non resti `/jira` (a parte la compat, che non ne ha)**

Run: `cd /Users/n0r41n/Development/open-jira/frontend-next && grep -rn '/jira' app components lib e2e --include='*.ts' --include='*.tsx' || echo NONE_LEFT`
Expected: `NONE_LEFT`. (Le API restano `/rest/api/3` — non contengono `/jira`.)

- [ ] **Step 4: Type-check + build + E2E completo**

Run:
```bash
cd /Users/n0r41n/Development/open-jira
lsof -ti:8080 | xargs kill 2>/dev/null; lsof -ti:3000 | xargs kill 2>/dev/null; sleep 1
cd frontend-next && npx tsc --noEmit && echo TSC_OK && npm run build 2>&1 | tail -3 && npx playwright test --reporter=line 2>&1 | tail -8; cd ..
```
Expected: `TSC_OK`, build OK, **tutti gli E2E verdi** (le spec ora navigano su `/app/...`).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(frontend): rename UI route prefix /jira -> /app"
```

---

### Task 6: README in inglese, accurato e non fuorviante

**Files:**
- Modify: `README.md` (riscrittura)

- [ ] **Step 1: Riscrivere il README**

Sostituire integralmente `README.md` con un README EN per un progetto open source 1.0. DEVE:
- Titolo **Heureum**, tagline: "A self-hostable, open-source project & issue tracker with a drop-in Jira Cloud REST API v3-compatible surface."
- Badge licenza AGPL-3.0.
- **Compatibility note** esplicita: "Heureum implements a subset of the Jira Cloud REST API v3 (`/rest/api/3`) and Agile API (`/rest/agile/1.0`) for drop-in compatibility. *Jira* and *Atlassian* are trademarks of Atlassian; Heureum is an independent project, not affiliated with or endorsed by Atlassian."
- Stack REALE: backend Go (`net/http`, GORM, golang-migrate), frontend **Next.js (App Router) + React** in `frontend-next/` (NON "Vite SPA"), Postgres/MySQL/SQLite, Redis opzionale.
- Struttura repo aggiornata (NIENTE `frontend/`): `cmd/{server,worker,seed,gapreport}`, `internal/`, `frontend-next/`, `migrations/`, `deploy/{docker,helm}`, `docs/`.
- Quick start locale con SQLite:
  ```bash
  APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/seed
  APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/server
  cd frontend-next && npm install && npm run dev
  # UI su http://localhost:3000/app  — demo: admin@example.com / admin-demo-123
  ```
- **Solo le env var realmente lette**: `APP_SECRET` (required), `APP_PORT`, `APP_ENV`, `APP_BASE_URL`, `DB_DRIVER`, `DB_DSN` (required), `REDIS_URL`. Aggiungere una riga: "SMTP/OAuth/object-storage settings are planned and not yet wired." (NON documentare `SMTP_*`/`OAUTH_*`/`STORAGE_*`/`S3_*` come funzionanti.)
- Docker: `docker compose -f deploy/docker/docker-compose.yml up --build` (dev). Menzione immagini `heureum/*` per la prod.
- Helm: `helm install heureum deploy/helm/heureum`.
- Sezione "API compatibility & gap report" che rimanda a `docs/contracts/gap-report.md`.
- Link a CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, LICENSE, CHANGELOG.

- [ ] **Step 2: Verificare i comandi citati**

Run: `cd /Users/n0r41n/Development/open-jira && rm -f /tmp/rmck.db && APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=/tmp/rmck.db go run ./cmd/seed >/dev/null 2>&1 && echo SEED_OK && rm -f /tmp/rmck.db seed`
Expected: `SEED_OK` (il quick start del README funziona davvero). Rimuovere eventuale binario `seed` orfano.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: rewrite README for Heureum 1.0 (accurate stack, honest config, trademark note)"
```

---

### Task 7: LICENSE (AGPL-3.0) + pulizia .env.example

**Files:**
- Create: `LICENSE`
- Modify: `.env.example`

- [ ] **Step 1: Scaricare il testo canonico AGPL-3.0**

Run: `cd /Users/n0r41n/Development/open-jira && curl -fsSL https://www.gnu.org/licenses/agpl-3.0.txt -o LICENSE && head -3 LICENSE && wc -l LICENSE`
Expected: file `LICENSE` con l'intestazione "GNU AFFERO GENERAL PUBLIC LICENSE / Version 3". Se offline, recuperare il testo AGPL-3.0 ufficiale da altra fonte affidabile (NON riscriverlo a mano).

- [ ] **Step 2: Aggiungere l'header di copyright in fondo (istruzioni "How to Apply")**

Aggiungere in cima al README o in `LICENSE`? → convenzione: il file `LICENSE` resta il testo puro. Verificare solo che esista e sia completo (nessuna modifica al testo di licenza).

- [ ] **Step 3: Ripulire `.env.example`**

Riscrivere `.env.example` in modo che elenchi **solo** le variabili effettivamente lette, con una sezione "planned" chiaramente commentata:
```dotenv
# --- Required ---
APP_SECRET=change-me                     # JWT signing secret (required)
DB_DSN=postgres://user:pass@localhost:5432/heureum?sslmode=disable  # required

# --- Optional (with defaults) ---
APP_PORT=8080
APP_ENV=development
APP_BASE_URL=http://localhost:8080
DB_DRIVER=postgres                       # postgres | mysql | sqlite
REDIS_URL=redis://localhost:6379/0

# --- Planned (NOT yet read by the app) ---
# SMTP_HOST=  SMTP_PORT=  SMTP_USER=  SMTP_PASS=  SMTP_FROM=
# OAuth (Forgejo/GitLab/GitHub) is configured per-project in the DB, not via env.
```

- [ ] **Step 4: Commit**

```bash
git add LICENSE .env.example
git commit -m "docs: add AGPL-3.0 LICENSE; trim .env.example to config actually read"
```

---

### Task 8: CONTRIBUTING + Code of Conduct + SECURITY + CHANGELOG

**Files:**
- Create: `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, `CHANGELOG.md`

- [ ] **Step 1: CONTRIBUTING.md**

Contenuto: prerequisiti (Go 1.25, Node 22), come far girare backend+frontend+seed, **il gate a tre livelli** (`go build/vet/test`, `cd frontend-next && npm run build && npx playwright test`, `go run ./cmd/gapreport` senza drift), stile commit (Conventional Commits), workflow branch/PR, dove vivono i piani (`docs/superpowers/plans/`), nota che le modifiche alla superficie API devono restare conformi ai contract test in `internal/contract/`. Nota AGPL: i contributi sono sotto AGPL-3.0.

- [ ] **Step 2: CODE_OF_CONDUCT.md**

Run: `cd /Users/n0r41n/Development/open-jira && curl -fsSL https://raw.githubusercontent.com/mozilla/inclusion/master/code-of-conduct.md -o /tmp/coc_probe 2>/dev/null; echo done` — in pratica usare il **Contributor Covenant v2.1**. Se non scaricabile, creare `CODE_OF_CONDUCT.md` con il testo del Contributor Covenant 2.1 e un contatto: `ivano.turi@harpaitalia.it`. (Preferire il testo canonico del Contributor Covenant; inserire l'email di contatto nell'apposito placeholder.)

- [ ] **Step 3: SECURITY.md**

Politica di disclosure: versioni supportate (1.x), come segnalare privatamente (email `ivano.turi@harpaitalia.it`, no issue pubbliche per vuln), tempi di risposta indicativi. **Nota onesta sullo stato:** "As of 1.0, project-level permissions are informational (UI-gating) and NOT yet enforced server-side; server-side authorization enforcement is tracked for the next release. Do not expose an instance to untrusted users until enforcement lands." (Riflette la scelta Round 11.)

- [ ] **Step 4: CHANGELOG.md**

Formato "Keep a Changelog". Una voce `## [1.0.0] - <data del tag>` (usare placeholder `YYYY-MM-DD`, l'utente la fissa al tag) che riassume le capability accumulate nei Round 0-9: progetti, issue core, collaborazione (commenti/worklog/watch/vote/link/changelog/remotelink), ricerca & JQL, board/backlog/sprint (Agile 1.0), workflow & transizioni, report & dashboard, utenti/gruppi/permessi (informativi), integrazioni (webhook in uscita firmati, auto-commento Git, automation event-driven). Sezione "Known limitations": permessi non ancora enforced, allegati non implementati, SMTP/OAuth-env non wired.

- [ ] **Step 5: Verificare che siano markdown validi (nessun comando runtime)**

Run: `cd /Users/n0r41n/Development/open-jira && ls -la CONTRIBUTING.md CODE_OF_CONDUCT.md SECURITY.md CHANGELOG.md`
Expected: i 4 file esistono e non sono vuoti.

- [ ] **Step 6: Commit**

```bash
git add CONTRIBUTING.md CODE_OF_CONDUCT.md SECURITY.md CHANGELOG.md
git commit -m "docs: add CONTRIBUTING, Code of Conduct, SECURITY policy, CHANGELOG"
```

---

### Task 9: GitHub community health files

**Files:**
- Create: `.github/ISSUE_TEMPLATE/bug_report.md`, `.github/ISSUE_TEMPLATE/feature_request.md`, `.github/ISSUE_TEMPLATE/config.yml`
- Create: `.github/PULL_REQUEST_TEMPLATE.md`
- Create: `.github/CODEOWNERS`, `.github/dependabot.yml`

- [ ] **Step 1: Issue template — bug**

`.github/ISSUE_TEMPLATE/bug_report.md` con front-matter (`name`, `about`, `labels: bug`) e sezioni: descrizione, passi per riprodurre, comportamento atteso/effettivo, versione/commit, ambiente (OS, DB driver, browser).

- [ ] **Step 2: Issue template — feature + config**

`feature_request.md` (front-matter `labels: enhancement`; problema/soluzione/alternative). `config.yml`: `blank_issues_enabled: false` + eventuale link a Discussions.

- [ ] **Step 3: PR template**

`.github/PULL_REQUEST_TEMPLATE.md`: cosa/perché, checklist (gate a tre livelli verde, gap report aggiornato se cambia l'API, Conventional Commit, docs aggiornate).

- [ ] **Step 4: CODEOWNERS + dependabot**

`.github/CODEOWNERS`: `* @it4nodummies`. `.github/dependabot.yml`: ecosistemi `gomod` (root), `npm` (`/frontend-next`), `github-actions` (root), `docker` — cadenza settimanale.

- [ ] **Step 5: Verificare YAML**

Run: `cd /Users/n0r41n/Development/open-jira && python3 -c "import yaml,sys; [yaml.safe_load(open(f)) for f in ['.github/ISSUE_TEMPLATE/config.yml','.github/dependabot.yml']]; print('YAML_OK')"`
Expected: `YAML_OK`.

- [ ] **Step 6: Commit**

```bash
git add .github/ISSUE_TEMPLATE .github/PULL_REQUEST_TEMPLATE.md .github/CODEOWNERS .github/dependabot.yml
git commit -m "chore(github): add issue/PR templates, CODEOWNERS, dependabot"
```

---

### Task 10: Docker — .dockerignore + coerenza compose + rename immagini

**Files:**
- Create: `.dockerignore`
- Modify: `deploy/docker/docker-compose.yml`, `deploy/docker/docker-compose.prod.yml`

- [ ] **Step 1: `.dockerignore`**

Creare `.dockerignore` a root:
```
.git
.github
.claude
node_modules
frontend-next/node_modules
frontend-next/.next
frontend-next/test-results
frontend-next/playwright-report
*.db
dev.db
docs
deploy
*.md
```
(Riduce il contesto di build; `migrations/` e il sorgente Go NON sono esclusi.)

- [ ] **Step 2: Coerenza compose prod**

In `deploy/docker/docker-compose.prod.yml`:
- rinominare le immagini `open-jira/api|worker|frontend:latest` → `ghcr.io/it4nodummies/heureum-api|heureum-worker|heureum-frontend:latest`;
- il servizio `frontend` deve puntare all'immagine Next (`heureum/frontend`) su **porta 3000** con davanti `nginx` (come il dev), NON su porta 80 (che implicava la vecchia SPA rimossa). Allineare al modello del `docker-compose.yml` (nginx pubblica `${APP_PORT:-80}:80`, proxy verso `api:8080` e `frontend:3000`).
- rimuovere il blocco `SMTP_*` passato ad `api` (dead config) oppure lasciarlo commentato con nota "planned".
- rimuovere `version: '3.8'` (obsoleto in Compose v2) se presente.

- [ ] **Step 3: Rinominare le immagini anche nel dev compose (se taggate) + build service frontend**

In `deploy/docker/docker-compose.yml`: assicurarsi che il servizio `frontend` builda `Dockerfile.frontend-next` (non più `Dockerfile.frontend` rimosso). Se qualche `image:` usa `open-jira/*`, rinominare `ghcr.io/it4nodummies/heureum-*`.

- [ ] **Step 4: Validare le compose**

Run: `cd /Users/n0r41n/Development/open-jira && docker compose -f deploy/docker/docker-compose.yml config >/dev/null && echo DEV_OK && docker compose -f deploy/docker/docker-compose.prod.yml config >/dev/null && echo PROD_OK`
Expected: `DEV_OK`, `PROD_OK` (parsing + risoluzione validi). Se `docker` non è disponibile nell'ambiente, validare almeno lo YAML con `python3 -c "import yaml; yaml.safe_load(open(...))"` e annotare che la validazione compose va fatta dove c'è Docker.

- [ ] **Step 5: Commit**

```bash
git add .dockerignore deploy/docker/
git commit -m "chore(docker): add .dockerignore, fix compose coherence, rename images to heureum/*"
```

---

### Task 11: Helm chart → deploy/helm/heureum

**Files:**
- Rename dir: `deploy/helm/open-jira/` → `deploy/helm/heureum/`
- Modify: `Chart.yaml`, `values.yaml`, `templates/*`

- [ ] **Step 1: Rinominare la cartella del chart**

Run: `cd /Users/n0r41n/Development/open-jira && git mv deploy/helm/open-jira deploy/helm/heureum`

- [ ] **Step 2: Sostituire `open-jira`/`openjira` → `heureum` nel chart**

Run:
```bash
cd /Users/n0r41n/Development/open-jira
grep -rl -e 'open-jira' -e 'openjira' deploy/helm/heureum \
  | xargs sed -i '' -e 's#open-jira#heureum#g' -e 's#openjira#heureum#g'
```
Verificare a mano: `Chart.yaml` `name: heureum`; `values.yaml` `image.repository` per api/worker/frontend = `ghcr.io/it4nodummies/heureum-api|heureum-worker|heureum-frontend` (il sed porta `open-jira/api`→`heureum/api`: correggere a mano nel formato GHCR), DSN `heureum:heureum@.../heureum`; `_helpers.tpl` nome release; `deployment.yaml`/`service.yaml`/`ingress.yaml`/`configmap.yaml`/`secret.yaml` coerenti. Il campo `smtpHost` (dead config) può restare come placeholder commentato o essere rimosso.

- [ ] **Step 3: Lint/template del chart**

Run: `cd /Users/n0r41n/Development/open-jira && helm lint deploy/helm/heureum && helm template heureum deploy/helm/heureum >/dev/null && echo HELM_OK`
Expected: `HELM_OK`. Se `helm` non è installato, validare gli YAML dei template con un parser e annotare che il lint va eseguito dove c'è Helm.

- [ ] **Step 4: Commit**

```bash
git add deploy/helm/
git commit -m "chore(helm): rename chart to heureum and update image/DSN references"
```

---

### Task 12: CI di release verso GHCR

**Files:**
- Create: `.github/workflows/release.yml`
- Modify: `.github/workflows/ci.yml` (aggiornare i branch trigger se serve; nessun rename obbligatorio)

- [ ] **Step 1: Workflow di release**

`.github/workflows/release.yml` — trigger sul tag `v*`, permesso `packages: write`, login GHCR con `GITHUB_TOKEN`, build & push delle 3 immagini con `docker/build-push-action`:
```yaml
name: release
on:
  push:
    tags: ['v*']
permissions:
  contents: read
  packages: write
jobs:
  images:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - name: api
            dockerfile: Dockerfile
          - name: worker
            dockerfile: Dockerfile.worker
          - name: frontend
            dockerfile: Dockerfile.frontend-next
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/metadata-action@v5
        id: meta
        with:
          images: ghcr.io/${{ github.repository_owner }}/heureum-${{ matrix.name }}
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=raw,value=latest
      - uses: docker/build-push-action@v6
        with:
          context: .
          file: ${{ matrix.dockerfile }}
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
```

- [ ] **Step 2: Aggiornare il trigger di ci.yml (facoltativo)**

Se il branch di sviluppo è `feat/frontend-next` e diventerà `main` alla pubblicazione, verificare che `ci.yml` scatti su `main`/`master` (già così). Nessuna modifica se non necessaria.

- [ ] **Step 3: Validare gli YAML dei workflow**

Run: `cd /Users/n0r41n/Development/open-jira && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml')); yaml.safe_load(open('.github/workflows/ci.yml')); print('WF_OK')"`
Expected: `WF_OK`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/
git commit -m "ci: add tag-triggered release workflow publishing images to GHCR"
```

---

### Task 13: Gate finale + RELEASE.md + STATE.md + istruzioni di pubblicazione

**Files:**
- Create: `docs/RELEASE.md`
- Modify: `docs/superpowers/STATE.md`

- [ ] **Step 1: Gate a tre livelli completo**

Run:
```bash
cd /Users/n0r41n/Development/open-jira
go build ./... && echo BUILD_OK
go vet ./... && echo VET_OK
go test ./... 2>&1 | grep -vE '^ok|no test files'; echo GO_DONE
lsof -ti:8080 | xargs kill 2>/dev/null; lsof -ti:3000 | xargs kill 2>/dev/null; sleep 1
cd frontend-next && npx tsc --noEmit && echo TSC_OK && npm run build 2>&1 | tail -3 && npx playwright test --reporter=line 2>&1 | tail -6; cd ..
```
Expected: `BUILD_OK`, `VET_OK`, nessun FAIL Go, `TSC_OK`, build Next OK, **tutti gli E2E verdi**.

- [ ] **Step 2: Gap report senza drift**

Run: `cd /Users/n0r41n/Development/open-jira && go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md && rm -f seed`
Expected: nessun drift inatteso (il rename non cambia le route API); nessun binario `seed`/`gapreport` orfano committato.

- [ ] **Step 3: `docs/RELEASE.md` — istruzioni di pubblicazione (per l'utente)**

Creare `docs/RELEASE.md` con i passi ESATTI che l'utente eseguirà (il piano NON li esegue):
```markdown
# Publishing Heureum

Owner: `it4nodummies` (matches the Go module path `github.com/it4nodummies/heureum` and the image names).

1. Create the public repo (empty) on GitHub as `it4nodummies/heureum`.
2. Point origin and push:
   git remote add github git@github.com:it4nodummies/heureum.git
   git push github <branch>:main
3. On GitHub: set default branch to `main`; add topics; confirm the license shows AGPL-3.0.
4. Cut the release (triggers the GHCR image build):
   git tag -a v1.0.0 -m "Heureum 1.0.0"
   git push github v1.0.0
5. Create the GitHub Release from the tag, pasting the CHANGELOG [1.0.0] section.
6. Verify images at ghcr.io/it4nodummies/heureum-{api,worker,frontend}:1.0.0.
```

- [ ] **Step 4: Aggiornare STATE.md**

In `docs/superpowers/STATE.md`:
- header: "Aggiornato ... (dopo Round 10)";
- aggiungere alla sezione round completati la riga **Round 10 — Release 1.0 (rebrand Heureum + release engineering)**: rimozione vecchio frontend Vite; module path `github.com/heureum/heureum`; header webhook `X-Heureum-*`; rebrand UI + favicon; route UI `/app`; README EN accurato; LICENSE AGPL-3.0; `.env.example` onesto; CONTRIBUTING/CoC/SECURITY/CHANGELOG; template GitHub + dependabot; `.dockerignore` + compose coerenti + immagini `heureum/*`; Helm chart `deploy/helm/heureum`; workflow release GHCR su tag `v*`; `docs/RELEASE.md`. **Enforcement permessi NON incluso (Round 11).** Pubblicazione lasciata all'utente.
- cambiare "Prossimo" in **Round 11 — Enforcement permessi (sicurezza)**: middleware server-side 403 sulle rotte mutanti + `creator=admin` alla creazione progetto + contract test 403, poi tag 1.0/1.1;
- aggiornare la nota di ripresa e le istruzioni (UI su `/app`, brand Heureum).

- [ ] **Step 5: Commit finale**

```bash
git add docs/RELEASE.md docs/superpowers/STATE.md docs/contracts/gap-report.md
git commit -m "docs: add RELEASE guide; mark Round 10 (Heureum 1.0 prep) complete, Round 11 (permission enforcement) next"
```

---

## Note di chiusura round

- **Non-goal dichiarati:** enforcement permessi (Round 11); allegati (storage file); wiring SMTP/OAuth-env/object-storage; rinomina identificatori Go `Jira*` interni; esecuzione del push/tag su GitHub (utente).
- **Rischi:** il rename module path + `/jira`→`/app` sono find-replace ampi → il gate verde (build/test Go + tsc/build/E2E) è la rete di sicurezza; eseguirlo dopo ogni task di rename. Il favicon.ico rimosso è sostituito da `icon.svg` (Next genera le favicon). Le validazioni `docker compose config` e `helm lint` richiedono i tool installati nell'ambiente d'esecuzione: se assenti, validare gli YAML e annotare la verifica come "da fare dove disponibili".
- Il round chiude solo con i tre livelli verdi e il gap report senza drift.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura spec (obiettivi Round 10):**
- Rimozione vecchio `frontend/` → Task 1. ✅
- Rebrand trademark (nome/module/immagini/UI/header/route) → Task 2 (module), 3 (header), 4 (UI+favicon+seed+openapi), 5 (route), 10/11 (immagini docker/helm). ✅
- LICENSE AGPL-3.0 → Task 7. ✅
- README/CONTRIBUTING/CoC (+ SECURITY/CHANGELOG) → Task 6, 8. ✅
- docker-compose demo + Helm → Task 10, 11. ✅
- CI/CD release → Task 12. ✅
- Preparazione pubblicazione (no push) → Task 13 (`docs/RELEASE.md`). ✅
- Security review/enforcement → **rinviato a Round 11** per decisione utente (dichiarato in SECURITY.md e STATE.md). ✅ (scope)

**2. Placeholder scan:** i comandi di rename sono concreti (sed/git mv/grep con path reali). LICENSE e Code of Conduct usano testi canonici scaricati (NON riscritti a mano) — non è un placeholder ma la prassi corretta per testi legali. Le date del CHANGELOG/tag sono `YYYY-MM-DD` perché le fissa l'utente al tag (dichiarato).

**3. Consistenza:** `github.com/it4nodummies/heureum` (module) ↔ `ghcr.io/it4nodummies/heureum-{api,worker,frontend}` (immagini compose prod + Helm values + release CI via `${{ github.repository_owner }}`) ↔ `deploy/helm/heureum` ↔ CODEOWNERS `@it4nodummies` ↔ RELEASE.md `it4nodummies/heureum` — owner unico `it4nodummies`. Header `X-Heureum-*` (Task 3) coerente coi test webhook. Route `/app` (Task 5) coerente con redirect + E2E. Env var documentate (Task 6/7) = esattamente quelle lette da `internal/config/config.go`.
