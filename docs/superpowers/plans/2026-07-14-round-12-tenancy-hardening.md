# Round 12 — Tenancy hardening (Fase A: instance-per-tenant · Fase B: enforcement letture) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rendere Heureum realmente utilizzabile in scenari multi-tenant "un'istanza per tenant" (Fase A: fix packaging Helm/compose, storage allegati configurabile+durevole, registrazione chiudibile, docs) e chiudere il buco di confidenzialità delle letture (Fase B: membership-gating su GET project-scoped, liste filtrate, JQL scoped, directory utenti senza email per non-admin, 404 senza leak di esistenza).

**Architecture:** Fase A è packaging/config puro (nessun cambio di comportamento di default: signup resta open, uploads dir default invariata). Fase B riusa il pacchetto `internal/api/authz` del Round 11: nuova variante decorator **`EnforceNotFound`** (risponde **404**, non 403, per non rivelare l'esistenza delle risorse — semantica Jira) applicata ai GET project-scoped; per le liste e la ricerca JQL il filtro è **in-query** (subquery membership `project_id IN (SELECT project_id FROM project_members WHERE user_id = ?)`, bypass per global admin). Ordine: A prima di B (A è a rischio zero; B tocca la suite test).

**Tech Stack:** Go 1.25, GORM (subquery), `internal/api/authz` esistente, Helm/docker-compose, harness `internal/contract` (helper R11: `newTestServerDB`, `registerUserAndLogin`, `promoteAdmin`).

---

## Decisioni di scope (esplicite)

- **Multi-tenancy = instance-per-tenant.** La multi-tenancy applicativa (org_id su tutte le entità, JWT claim, namespace per-org) NON è in questo round — la tabella `organizations` resta vestigiale (dichiarato).
- **Semantica letture negate: 404** sulle risorse singole (issue/progetto/board/…): "does not exist or you do not have permission". Le **liste** vengono **filtrate** (200 con meno risultati), mai 403.
- **Global admin** (`is_admin`) vede tutto (bypass di ogni filtro lettura) — è così che E2E resta verde (l'admin demo è global admin).
- **Restano aperte a ogni utente loggato**: reference data non-progetto (priority, issuetype, resolution, status catalog, statuscategory, project/type, projectCategory GET, autocompletedata), `/permissions`, `/mypermissions`, `/myself`. **`/label` e `/field` invece vengono filtrati** (oggi leakano nomi label/custom-field cross-progetto).
- **Email utenti**: esposte solo a se stessi e ai global admin. La UI non consuma i search endpoint utenti (verificato: zero consumer; le uniche email in UI sono la propria via `/myself`/`/users/me` e il fallback lead in `/project/search` — il lead di un progetto visibile è accettabile). `emailAddress` è `omitempty` nel DTO v3 → ometterla è contract-safe.
- **Signup flag default `open`**: nessun cambio di comportamento di default; `closed` serve alle istanze per-tenant.
- **Non-goal**: multi-tenancy applicativa; object storage S3 per allegati (solo dir configurabile + PVC); condivisione filtri/dashboard granulare (resta owner-or-shared/public); client WS nel frontend.

## Contesto verificato (dagli audit — leggere una volta)

**Fase A (deployment):**
- `deploy/helm/heureum/templates/_helpers.tpl:1-3` hardcoda `printf "heureum"` (ignora `.Release.Name`) → due release nello stesso namespace collidono. Risorse: deployment.yaml (api:4, worker:54, frontend:90), service.yaml (:4,:22), configmap.yaml:4, secret.yaml:4, ingress.yaml:5.
- **Probe rotte**: deployment.yaml:41,47 puntano a `/api/v1/projects` che NON esiste (il router ha `/rest/api/3/*` e `GET /healthz` — router.go:117) → crash-loop garantito. Verificare anche le probe di worker (nessun server HTTP!) e frontend (Next su :3000).
- **Allegati IMPLEMENTATI** (le docs R10/R11 dicono il falso): `internal/domain/issue/attachment_service.go:22` `uploadDir = "./data/uploads"` (hardcoded, relativo a CWD), `os.MkdirAll`:23, upload handler su `POST /issue/{issueIdOrKey}/attachments` (router.go:227), delete `os.Remove`:76. Il chart NON monta volumi → upload persi al restart.
- compose dev (`deploy/docker/docker-compose.yml`): `name: open-jira` hardcoded (:1), api pubblica `"8080:8080"` (:52) → due stack collidono. Prod ok (solo nginx `${APP_PORT:-80}`).
- Config (`internal/config/config.go`): tutto da env; `Load()` legge APP_PORT/APP_ENV/APP_SECRET/APP_BASE_URL/DB_DRIVER/DB_DSN/REDIS_URL. Migrazioni auto al boot del server (`cmd/server/main.go:26`); il worker NON le esegue. Redis dichiarato ma MAI usato (nessuna dep in go.mod).
- Registrazione: `POST /auth/register` pubblica, nessun gate (`auth_handler.go:21-43`, `auth/service.go:26-40`).

**Fase B (letture):**
- Search: `search.Service.Search(jqlStr string, r jql.Resolver, offset, limit int) (SearchResult, error)` — service.go:31; query base a service.go:46 `s.db.Model(&issue.Issue{}).Where("is_archived = ?", false)` → **il predicato membership va lì** (vale sia per Count sia per Find; `limit==0` = count-only). Handler (`search_handler.go:72,105,133`) leggono già `uid` e lo passano solo al resolver.
- Project list: `ListWithFilters(f ListFilter, userID string)` — service.go:192; query a :197 → predicato lì. `GET /project` e `GET /project/search` la chiamano entrambe (project_handler.go:173,201). Frontend usa `/project/search` (lib/api.ts:210).
- Inventario GET project-scoped da gatare (router.go, param tra `{}`): project `{key}` Get:173, members:178, git/providers:184, webhooks:187, issues:191, issues/export:366, workflow:243,248, sprints:268,270, board:275, reports:330-336; `{projectID}` custom-fields:368, automation:380, timeline:388, calendar:389; automation `{ruleID}` GetRule:382, runs:386; issue `{issueKey}` Get:197, editmeta:198, changelog:202, git:203, transitions:252; `{issueIdOrKey}` watchers:204, comment:208,210, worklog:215, votes:219, remotelink:223; `{issueID}` custom-values:377; attachment `{id}`:228,232 (two-hop); issueLink `{linkId}`:235 (two-hop); custom-fields `{fieldID}`/options:371; agile board list:281, `{boardId}` get/config/backlog/issue/sprint/epic:285-291, sprint `{sprintId}`:296,300, agile issue `{issueIdOrKey}`:307, epic `{epicIdOrKey}`:309.
- `createmeta`/`editmeta` sono **stub** (tornano vuoto, non leggono il progetto — reference_handler.go:161-170) → editmeta va comunque gatato (path issue), createmeta resta aperto (stub senza dati).
- Leak reference: `/label` (reference_handler.go:205-212, `Pluck` distinct su TUTTE le label) e `/field` (:196, enumera TUTTI i custom_fields cross-progetto).
- Directory utenti: `GET /users/search` → **raw `user.User` con email** (user_handler.go:55-69); `GET /user/search` → `v3.JiraUser` con `EmailAddress` (v3/user.go:14,39 — `omitempty`); `GET /user?accountId=` → raw user (user_handler.go:130-143); `GET /user/assignable/search` già membership-scoped sul progetto target MA senza check sul chiamante (user_handler.go:87-105).
- Filtri: `GET /filter/{id}` NON owner-filtered (filter_handler.go:88 → `svc.Get(id)`); search/my/favourite già `owner_id = ? OR is_shared` (saved_filter.go:30,57,117). Dashboard: `GET /dashboards|dashboard/{id}` NON owner-filtered (dashboard_handler.go:76); List/Search già per-uid. Il modello Dashboard ha `is_public` (usato da CopyDashboard).
- WS: `GET /ws/v1/projects/{key}/board` registrata `mux.HandleFunc` SENZA authMw (router.go:355-357); hub broadcast a TUTTI i client, `{key}` mai letto; **zero client frontend** (grep ws:// = nulla).
- Notifications: già per-uid. `/mypermissions`,`/permissions`: informativi, invariati.
- authz esistente: `chk.Enforce(perm, resolver, next)` (403), resolver `ByKey/ByProjectID/ByIssueParam/ByIssueUUID/ByBoardSeq/BySprintSeq/ByAutomationRule/ByCustomField`, `EnforceGlobalAdmin`, `IsGlobalAdmin`, `WriteForbidden`. `permission.BrowseProjects` esiste. Manca: variante 404, resolver two-hop attachment/issueLink (R11 li fece in-handler sulle DELETE).
- Test: alice (non-admin) crea i propri progetti → membership admin (R11 creator=admin) → le letture sue restano verdi. Helper: `newTestServerDB`, `registerUserAndLogin`, `promoteAdmin`. ATTENZIONE: `TestUserSearch_Conformant` (users_perms_test.go) potrebbe validare la shape utente — se assert su `emailAddress`, promuovere admin il chiamante o adeguare.

---

## Struttura dei file

**Fase A:** `deploy/helm/heureum/templates/{_helpers.tpl,deployment.yaml,service.yaml,configmap.yaml,secret.yaml,ingress.yaml,pvc.yaml(nuovo)}`, `deploy/helm/heureum/values.yaml`, `internal/config/config.go`, `internal/domain/issue/attachment_service.go`, `internal/api/handlers/auth_handler.go` (+router), `deploy/docker/docker-compose.yml`, `.env.example`, `README.md`, `SECURITY.md`, `CHANGELOG.md`, `docs/RELEASE.md`.

**Fase B:** `internal/api/authz/enforce.go` (+`resolvers.go`), `internal/domain/project/service.go` (subquery membership), `internal/domain/search/service.go`, `internal/api/handlers/{search_handler,project_handler,user_handler,reference_handler,filter_handler,dashboard_handler,attachment_handler,issuelink_handler}.go`, `internal/api/router.go`, `internal/contract/authz_read_test.go` (nuovo), docs.

---

## FASE A — Instance-per-tenant

### Task 1: Helm — fullname release-aware, probe corrette, PVC uploads

**Files:**
- Modify: `deploy/helm/heureum/templates/_helpers.tpl`, `deployment.yaml`, `service.yaml`, `configmap.yaml`, `secret.yaml`, `ingress.yaml`, `values.yaml`
- Create: `deploy/helm/heureum/templates/pvc.yaml`

- [ ] **Step 1: Fullname col release name**

In `_helpers.tpl` sostituire il template hardcoded con la convenzione standard:
```yaml
{{- define "heureum.fullname" -}}
{{- if contains "heureum" .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-heureum" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
```
(Verificare il nome reale del define esistente — probabilmente `heureum.fullname` — e mantenerlo.) Poi verificare che TUTTI i template usino `include "heureum.fullname"` per i nomi risorsa (deployment/service/configmap/secret/ingress + i suffissi `-api`/`-worker`/`-frontend`/`-config`/`-secret`): dove il nome è hardcoded `heureum-*`, sostituire con `{{ include "heureum.fullname" . }}-api` ecc. Aggiornare coerentemente i riferimenti incrociati (selector/labels, `envFrom` verso configmap/secret, service name usato dall'ingress).

- [ ] **Step 2: Probe corrette**

In `deployment.yaml`:
- api: liveness+readiness → `httpGet: {path: /healthz, port: 8080}`.
- worker: NON espone HTTP → rimuovere ogni probe httpGet (nessuna probe, o exec banale `["/bin/sh","-c","true"]` — preferire nessuna).
- frontend: probe → `httpGet: {path: /, port: 3000}`.

- [ ] **Step 3: PVC per gli allegati**

`templates/pvc.yaml` (condizionale):
```yaml
{{- if .Values.uploads.persistence.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ include "heureum.fullname" . }}-uploads
  labels: {{- include "heureum.labels" . | nindent 4 }}
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: {{ .Values.uploads.persistence.size }}
{{- end }}
```
In `values.yaml` aggiungere:
```yaml
uploads:
  dir: /data/uploads
  persistence:
    enabled: true
    size: 5Gi
```
Nel deployment api: `volumeMounts: [{name: uploads, mountPath: {{ .Values.uploads.dir }}}]` + `volumes: [{name: uploads, persistentVolumeClaim: {claimName: <fullname>-uploads}}]` (condizionali su `.Values.uploads.persistence.enabled`; se disabled → `emptyDir`). Aggiungere env `APP_UPLOADS_DIR: {{ .Values.uploads.dir }}` alla configmap (la chiave env nasce nel Task 2 — coordinare il nome esatto).

- [ ] **Step 4: Lint/template**

Run: `helm lint deploy/helm/heureum && helm template t1 deploy/helm/heureum >/dev/null && helm template t2 deploy/helm/heureum --namespace other >/dev/null && echo HELM_OK`
Se `helm` non è installato: validare `Chart.yaml`/`values.yaml` con un parser YAML, eyeball dei template, e annotare che il lint va rifatto dove helm esiste. Verificare a mano che due release name diversi producano nomi risorsa diversi (grep del rendered output se helm c'è).

- [ ] **Step 5: Commit**

```bash
git add deploy/helm/heureum/
git commit -m "fix(helm): release-aware names, real probe endpoints, uploads PVC"
```

---

### Task 2: Config — APP_UPLOADS_DIR + APP_SIGNUP (registrazione chiudibile)

**Files:**
- Modify: `internal/config/config.go`, `internal/domain/issue/attachment_service.go`, `internal/api/handlers/auth_handler.go`, `internal/api/router.go`
- Test: `internal/config/config_test.go` (se esiste — altrimenti test nel dominio), `internal/api/handlers/auth_handler_test.go` o contract

- [ ] **Step 1: Leggere i file reali**

Leggere `internal/config/config.go` (struct Config + Load), `attachment_service.go` (com'è costruito `NewAttachmentService(db)` e dove usa `uploadDir`), `auth_handler.go` (Register), `router.go` (costruzione attachmentSvc e authH, e come `cfg` è disponibile).

- [ ] **Step 2: Config**

In `config.Config` aggiungere `UploadsDir string` e `SignupOpen bool`. In `Load()`:
```go
	cfg.UploadsDir = getEnv("APP_UPLOADS_DIR", "./data/uploads")
	cfg.SignupOpen = getEnv("APP_SIGNUP", "open") != "closed"
```
(Adeguare allo stile reale del file — se non esiste un helper `getEnv`, usare il pattern presente.)

- [ ] **Step 3: Uploads dir iniettata**

`attachment_service.go`: sostituire la costante `uploadDir` con un campo del service; `NewAttachmentService(db, uploadsDir string)` (default se stringa vuota → "./data/uploads" per compat). Aggiornare la costruzione in `router.go` (`issue.NewAttachmentService(db, cfg.UploadsDir)`) e ogni altro chiamante (grep `NewAttachmentService`). I path scritti nel DB restano assoluti/relativi come oggi.

- [ ] **Step 4: Signup flag**

`Register` handler: se signup chiuso → `403` con messaggio v3 `{"errorMessages":["signup is disabled on this instance"]}`. Il flag arriva dall'handler (campo nel struct AuthHandler o closure dal router — seguire il pattern del file). Il login resta sempre aperto.

- [ ] **Step 5: Test**

- Test unit (o contract): con `APP_SIGNUP=closed` la register torna 403 e non crea l'utente; default open → 201. Nel contract harness il server di test è costruito con una `config.Config` esplicita (myself_test.go) → aggiungere un test che costruisce il server con `SignupOpen=false` (verificare come newTestServer costruisce cfg e replicare con il flag).
- Test attachment: upload scrive nella dir configurata (t.TempDir()).

- [ ] **Step 6: Gate + commit**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | grep -vE '^ok|no test files'; echo DONE` → verde.
```bash
git add internal/config/ internal/domain/issue/ internal/api/ internal/contract/
git commit -m "feat(config): configurable uploads dir and closable signup (APP_UPLOADS_DIR, APP_SIGNUP)"
```

---

### Task 3: Compose fixes + .env.example + README config

**Files:**
- Modify: `deploy/docker/docker-compose.yml`, `deploy/docker/docker-compose.prod.yml`, `.env.example`, `README.md`

- [ ] **Step 1: Dev compose**

- Rimuovere `name: open-jira` (torna allo scoping per directory/`-p`).
- API: pubblicare con `"${API_PORT:-8080}:8080"` invece di `"8080:8080"` (o rimuovere il publish: nginx già proxya — scegliere il publish parametrizzato per non rompere il dev workflow).
- Aggiungere ad api il volume uploads: `- uploads:/app/data/uploads` (verificare la WORKDIR del Dockerfile api per il path giusto; se WORKDIR è `/`, il path è `./data/uploads` → montare lì) + `APP_UPLOADS_DIR` env coerente + volume `uploads` dichiarato.

- [ ] **Step 2: Prod compose**

Stesso volume uploads su api + `APP_UPLOADS_DIR`. Verificare che non ci sia altro storage locale.

- [ ] **Step 3: .env.example + README**

Aggiungere `APP_UPLOADS_DIR` e `APP_SIGNUP` alle sezioni Optional (con default e una riga di spiegazione). README: tabella env aggiornata + nota che gli allegati sono su disco locale (volume/PVC consigliato).

- [ ] **Step 4: Validare + commit**

Run: `docker compose -f deploy/docker/docker-compose.yml config >/dev/null && echo DEV_OK; docker compose -f deploy/docker/docker-compose.prod.yml config >/dev/null && echo PROD_OK` (con env fittizie se richieste; fallback YAML parse se docker assente).
```bash
git add deploy/docker/ .env.example README.md
git commit -m "chore(deploy): parameterized ports/volumes for side-by-side stacks; uploads volume; document new env"
```

---

### Task 4: Docs — multi-tenancy honesty + riconciliazione allegati

**Files:**
- Modify: `README.md`, `SECURITY.md`, `CHANGELOG.md`, `docs/RELEASE.md`

- [ ] **Step 1: README — sezione "Multi-tenancy"**

Aggiungere una sezione breve e onesta: Heureum è **single-tenant per istanza**; per più organizzazioni usare **un'istanza per tenant** (un DB + un APP_SECRET + un hostname per tenant; Helm: una release per namespace/tenant con `ingress.host` proprio; compose: un project name per stack). La multi-tenancy applicativa non è supportata.

- [ ] **Step 2: Riconciliare gli allegati (le docs mentono)**

In `SECURITY.md` e `CHANGELOG.md`: sostituire "Attachments are not implemented" con la verità: **implementati**, storage su disco locale configurabile via `APP_UPLOADS_DIR` (default `./data/uploads`); in container serve un volume/PVC (il chart lo monta di default da questo round); niente object storage ancora (follow-up S3). In CHANGELOG spostare la voce da Known limitations ad Added (upload/download/delete allegati) — siamo pre-tag, si folda nella `[1.0.0]`.

- [ ] **Step 3: RELEASE.md checklist**

Aggiornare: probe fix fatte; aggiungere check "una release Helm per tenant/namespace" alla nota multi-tenant; `APP_SIGNUP=closed` consigliato per istanze esposte.

- [ ] **Step 4: Commit**

```bash
git add README.md SECURITY.md CHANGELOG.md docs/RELEASE.md
git commit -m "docs: instance-per-tenant guidance; reconcile attachment reality"
```

---

## FASE B — Enforcement letture

### Task 5: authz — EnforceNotFound + resolver two-hop + subquery membership

**Files:**
- Modify: `internal/api/authz/enforce.go`, `internal/api/authz/resolvers.go`, `internal/domain/project/service.go`
- Test: `internal/api/authz/enforce_test.go`, `internal/domain/project/service_test.go`

- [ ] **Step 1: Test (falliscono)**

- `EnforceNotFound`: come `Enforce` ma un utente senza permesso riceve **404** (body v3 "not found"); con permesso → next; resolver ok=false → next (invariato).
- Resolver two-hop: `ByAttachment("id")` (attachment→issue→project) e `ByIssueLink("linkId")` (link→source issue→project) risolvono il progetto; id inesistente → ok=false.
- `project.Service.MembershipSubquery(userID) *gorm.DB`: subquery `SELECT project_id FROM project_members WHERE user_id = ?` usabile in `Where("project_id IN (?)", sub)` — test: due progetti, utente membro di uno solo → il filtro ne ritorna uno.

- [ ] **Step 2: Implementare**

`enforce.go`:
```go
// EnforceNotFound: come Enforce ma nega con 404 (semantica Jira: non rivelare
// l'esistenza di risorse non visibili). Da usare sulle letture.
func (c *Checker) EnforceNotFound(permKey string, resolve Resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := middleware.UserIDFromContext(r.Context())
		projectID, ok := resolve(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if err := c.RequireProject(uid, projectID, permKey); err != nil {
			v3.WriteError(w, http.StatusNotFound, []string{"the resource does not exist or you do not have permission to view it"}, nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```
`resolvers.go`: `ByAttachment(param)` (usa il servizio attachment o `issues.DB()` per caricare attachment→IssueID→issue.ProjectID — il Checker ha `issues`; per l'attachment leggere la tabella via `c.issues.DB().Table("issue_attachments")...` o aggiungere il service al Checker: scegliere la via più pulita leggendo `attachment_service.go`), `ByIssueLink(param)` (link via `issues.DB()` → SourceID → issue → ProjectID; pattern già in `issuelink_handler.go:78`).
`project/service.go`:
```go
// MembershipSubquery: subquery dei project_id di cui userID è membro (per filtri lettura).
func (s *Service) MembershipSubquery(userID string) *gorm.DB {
	return s.db.Model(&ProjectMember{}).Select("project_id").Where("user_id = ?", userID)
}
```

- [ ] **Step 3: Eseguire + commit**

Run: `go test ./internal/api/authz/ ./internal/domain/project/ -v 2>&1 | tail -20 && go build ./... && echo OK`
```bash
git add internal/api/authz/ internal/domain/project/
git commit -m "feat(authz): EnforceNotFound (404 reads), two-hop resolvers, membership subquery"
```

---

### Task 6: Liste progetti + ricerca JQL scoped per membership

**Files:**
- Modify: `internal/domain/project/service.go` (ListWithFilters), `internal/domain/search/service.go` (Search), `internal/api/handlers/search_handler.go`, `internal/api/handlers/project_handler.go`, `internal/api/router.go` (se serve passare il Checker/flag admin)
- Test: `internal/domain/search/service_test.go`, `internal/domain/project/service_test.go`

- [ ] **Step 1: Test (falliscono)**

- `ListWithFilters` con un nuovo campo filtro `MemberUserID string` (vuoto = nessun filtro): due progetti, utente membro di uno → torna solo quello; vuoto → entrambi.
- `search.Service.Search` con nuovo parametro scope: firma `Search(jqlStr string, r jql.Resolver, offset, limit int, scope *gorm.DB)` dove `scope==nil` = nessun filtro (admin), altrimenti `base.Where("project_id IN (?)", scope)`. Test: issue in due progetti, scope sul primo → trova solo quelle; count-only rispetta lo scope.

- [ ] **Step 2: Implementare**

- `ListWithFilters`: se `f.MemberUserID != ""` → `query = query.Where("projects.id IN (?)", s.MembershipSubquery(f.MemberUserID))` (a service.go:197, prima del Count).
- `search.Service.Search`: parametro `scope *gorm.DB`; a service.go:46 dopo `is_archived`: `if scope != nil { base = base.Where("issues.project_id IN (?)", scope) }`. Aggiornare TUTTI i chiamanti (grep `svc.Search(` — search_handler 3 punti + eventuale filter run/renderer: verificare come la pagina filtri esegue le ricerche, probabilmente sempre via search handler).
- Handler: nei 3 metodi search e nei 2 project-list, calcolare lo scope: `var scope *gorm.DB; if !h.chk.IsGlobalAdmin(uid) { scope = h.projectSvc.MembershipSubquery(uid) }` — iniettare `chk`+`projectSvc` dove mancano (verificare i campi reali degli handler; SearchHandler avrà bisogno del Checker e del project service o direttamente della subquery via search service — scegliere l'iniezione più pulita e uniforme).

- [ ] **Step 3: Suite contract**

Run: `go test ./internal/contract/ 2>&1 | grep -E '^(--- FAIL|ok|FAIL)'` → verde (alice è membro dei progetti che crea; i test search/list operano sui suoi progetti).

- [ ] **Step 4: Commit**

```bash
git add internal/domain/ internal/api/
git commit -m "feat(authz): membership-scoped project lists and JQL search (global admin sees all)"
```

---

### Task 7: Gating GET project-scoped col decorator (404)

**Files:**
- Modify: `internal/api/router.go`

- [ ] **Step 1: Avvolgere i GET**

Per OGNI rotta GET dell'inventario in "Contesto verificato → Fase B" (project `{key}`/`{projectID}`, issue `{issueKey}`/`{issueIdOrKey}`/`{issueID}`, agile `{boardId}`/`{sprintId}`/issue/epic, automation `{ruleID}` GET, custom-fields `{fieldID}` options, attachment `{id}` GET/content, issueLink `{linkId}` GET):
```go
mux.Handle("GET /rest/api/3/project/{key}", authMw(chk.EnforceNotFound(permission.BrowseProjects, chk.ByKey, http.HandlerFunc(projectH.Get))))
```
Resolver per classe come nel Round 11 (ByKey/ByProjectID/ByIssueParam col param esatto/ByIssueUUID/ByBoardSeq/BySprintSeq/ByAutomationRule/ByCustomField) + i nuovi ByAttachment/ByIssueLink. Il permesso è sempre `permission.BrowseProjects`.
- `GET /rest/agile/1.0/board` (lista board globale): NON ha resolver — filtrare in-handler o (semplice) lasciare che la lista board venga filtrata per membership: leggere `agileBoardH.List` e applicare il filtro membership come nel Task 6 (subquery su `boards.project_id`). Scegliere in base al codice reale; documentare.
- `GET /rest/agile/1.0/epic/{epicIdOrKey}` e agile issue get → `ByIssueParam` (l'epic è una issue).
- NON toccare: reference data globali, `/search*` (già scoped T6), `/filter*`/`/dashboard*` (T9), notifications, myself, permissions, groups GET (restano aperti), createmeta (stub).

- [ ] **Step 2: Build + suite contract completa**

Run: `go build ./... && go vet ./... && go test ./internal/contract/ -v 2>&1 | grep -cE '^--- PASS'; go test ./internal/contract/ 2>&1 | grep -E 'FAIL|ok'` → tutto verde. Se un test legge una risorsa senza che l'utente sia membro → capire se è un buco del test (aggiungere membership/promoteAdmin) o un resolver sbagliato. NON allentare.

- [ ] **Step 3: Commit**

```bash
git add internal/api/router.go
git commit -m "feat(authz): membership-gate project-scoped reads with 404 semantics"
```

---

### Task 8: Directory utenti + leak reference + websocket

**Files:**
- Modify: `internal/api/handlers/user_handler.go`, `internal/api/handlers/reference_handler.go`, `internal/api/router.go`
- Test: aggiornare eventuali test toccati

- [ ] **Step 1: Email solo a sé stessi e agli admin**

- `SearchUsers` (`/users/search`): mai serializzare `user.User` raw. Costruire una shape sanitizzata (id, username, display_name, avatar_url, is_active); includere `email` SOLO se il chiamante è global admin. (Verificare i consumer frontend di `/users/search`: la TopBar usa `/users/me` — rotta diversa, invariata.)
- `GetUser` (`/user?accountId=`): stessa sanitizzazione (email solo se admin o se accountId == caller).
- `SearchV3` (`/user/search`): passare al mapper v3 un flag; se non admin → azzerare `Email` prima del mapping (il DTO ha `omitempty` → il campo sparisce). NON toccare `/myself` (torna sé stesso, email inclusa).
- `AssignableSearch`: prima di rispondere, `RequireProject(uid, p.ID, permission.BrowseProjects)` → 404 se il chiamante non è membro (iniettare chk nel UserHandler).

- [ ] **Step 2: /label e /field filtrati**

`reference_handler.go`:
- `Labels`: filtrare per progetti browsable: `WHERE project_id IN (subquery)` se non admin (le label hanno project_id — verificare lo schema; se la tabella labels non ha project_id diretto, usare il join reale).
- `Fields`: stessa cosa sui custom_fields enumerati (i field di sistema restano).
Iniettare chk+projectSvc (o la subquery) nel ReferenceHandler; global admin invariato.

- [ ] **Step 3: Websocket sotto auth**

In `router.go`: la rotta `GET /ws/v1/projects/{key}/board` passa sotto `authMw` (`mux.Handle(... , authMw(http.HandlerFunc(...)))` — adattare la closure esistente). Nessun client la usa (verificato) → zero breakage. Nota nel codice: per client browser servirà un token in query/subprotocol (follow-up).

- [ ] **Step 4: Suite + commit**

Run: `go build ./... && go test ./... 2>&1 | grep -vE '^ok|no test files'; echo DONE` → verde. ATTENZIONE a `TestUserSearch_Conformant`: se valida `emailAddress`, adeguare (promuovere admin il chiamante o assert senza email — il campo è opzionale nel contratto).
```bash
git add internal/api/
git commit -m "feat(authz): sanitize user directory, scope label/field catalogs, authenticate websocket"
```

---

### Task 9: Filter/Dashboard GET by id — owner-or-shared/public

**Files:**
- Modify: `internal/api/handlers/filter_handler.go`, `internal/api/handlers/dashboard_handler.go`
- Test: estendere `internal/contract/authz_test.go`

- [ ] **Step 1: Filter Get**

`GET /filter/{id}`: dopo `svc.Get(id)`, consentire solo se `OwnerID == uid || f.IsShared || chk.IsGlobalAdmin(uid)` → altrimenti **404**. (Verificare il nome reale del campo shared su SavedFilter — saved_filter.go usa `is_shared`.)

- [ ] **Step 2: Dashboard Get**

`GET /dashboards/{id}` e `GET /dashboard/{id}`: consentire solo se `OwnerID == uid || d.IsPublic || IsGlobalAdmin` → altrimenti 404. (Campo `is_public` verificato dallo scout su CopyDashboard; applicare lo stesso criterio a `CopyDashboard` che oggi copia dashboard private altrui — fixarlo qui.)

- [ ] **Step 3: Test + commit**

Contract: bob non vede il filtro privato di alice (404) né la sua dashboard privata (404); filtro `is_shared` visibile. Suite verde.
```bash
git add internal/api/handlers/ internal/contract/
git commit -m "feat(authz): owner-or-shared read scoping for filters and dashboards"
```

---

### Task 10: Contract test negativi di lettura

**Files:**
- Create: `internal/contract/authz_read_test.go`

- [ ] **Step 1: Test**

Con `newTestServerDB` + alice (`registerAndLogin`) + bob (`registerUserAndLogin`); alice crea progetto "RD" + una issue:
- `TestAuthzRead_NonMemberGets404`: bob su `GET /project/RD` → **404**; su `GET /issue/{key}` → **404**; su `GET /project/RD/board` → 404.
- `TestAuthzRead_ListsAreFiltered`: bob su `GET /project/search` → 200 ma il progetto RD NON compare (`total`/values senza RD); bob su `GET /search/jql?jql=project=RD` (o POST) → 200 con 0 issue.
- `TestAuthzRead_UserDirectoryHidesEmails`: bob su `GET /user/search?query=alice` → la risposta NON contiene `alice@example.com` (grep sul body raw); `promoteAdmin(bob)` → la stessa chiamata ora può contenerla.
- `TestAuthzRead_GlobalAdminSeesAll`: promoteAdmin(bob) → `GET /project/RD` 200, search vede le issue.
- Positivo: alice vede tutto il suo (già coperto dalla suite, ma un assert esplicito su `GET /project/RD` 200 non guasta).

- [ ] **Step 2: Eseguire + suite completa + commit**

Run: `go test ./internal/contract/ -run TestAuthzRead -v 2>&1 | tail -25 && go test ./... 2>&1 | grep -vE '^ok|no test files'; echo DONE` → verde.
```bash
git add internal/contract/authz_read_test.go
git commit -m "test(authz): read isolation negatives (404, filtered lists, hidden emails)"
```

---

### Task 11: Gate finale + docs + STATE.md

**Files:**
- Modify: `SECURITY.md`, `CHANGELOG.md`, `docs/RELEASE.md`, `docs/superpowers/STATE.md`

- [ ] **Step 1: Gate a tre livelli**

```bash
cd /Users/n0r41n/Development/open-jira
go build ./... && echo BUILD_OK && go vet ./... && echo VET_OK
go test ./... 2>&1 | grep -vE '^ok|no test files'; echo GO_DONE
go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md && rm -f seed gapreport
lsof -ti:8080 | xargs kill 2>/dev/null; lsof -ti:3000 | xargs kill 2>/dev/null; sleep 1
cd frontend-next && npx tsc --noEmit && echo TSC_OK && npm run build 2>&1 | tail -3 && npx playwright test --reporter=line 2>&1 | tail -6; cd ..
```
Expected: tutto verde, **20/20 E2E** (admin demo = global admin → vede tutto), gap senza drift.

- [ ] **Step 2: SECURITY.md**

Aggiornare la sezione Authorization model: **anche le letture sono ora membership-gated** (404 su risorse non visibili, liste filtrate, JQL scoped, email visibili solo a sé/admin, WS autenticato). Rimuovere la Known limitation sulle letture; restano: niente permission scheme configurabili, allegati su disco locale (volume richiesto), signup open di default (`APP_SIGNUP=closed` per chiuderlo).

- [ ] **Step 3: CHANGELOG + RELEASE**

CHANGELOG (pre-tag → foldare in `[1.0.0]`): sezione Security estesa (read enforcement + email hiding + WS auth); Added: `APP_UPLOADS_DIR`, `APP_SIGNUP`, PVC uploads, Helm release-aware. Known limitations aggiornate (via la voce letture). RELEASE.md: checklist aggiornata (helm lint ora include il nuovo pvc.yaml).

- [ ] **Step 4: STATE.md**

Round 12 nei completati (due fasi: instance-per-tenant hardening + read enforcement, con l'elenco sintetico); "Prossimo" → Tag 1.0 (tutto chiuso) / post-1.0: multi-tenancy applicativa (org_id), object storage allegati, permission scheme, WS token per browser client, retry webhook.

- [ ] **Step 5: Commit**

```bash
git add SECURITY.md CHANGELOG.md docs/RELEASE.md docs/superpowers/STATE.md docs/contracts/gap-report.md
git commit -m "docs: read-side authorization and instance-per-tenant hardening complete (Round 12)"
```

---

## Note di chiusura round

- **Follow-up:** multi-tenancy applicativa (attivare `organizations`/`org_id`, JWT claim, namespace per-org — round grande, solo se serve un SaaS condiviso); object storage S3 per allegati; token WS per client browser + client WS nel frontend; condivisione filtri/dashboard granulare; `AssignableForProject` per assignee-check sulle mutazioni; rate-limit sulla register.
- **Rischi:** T7 avvolge ~40 GET — la rete di sicurezza è la suite contract (alice-membro) + i negativi di T10. Il cambio firma `search.Service.Search` tocca tutti i chiamanti (grep completo). `TestUserSearch_Conformant` può richiedere adattamento (email opzionale). L'E2E resta verde via global admin. Fase A prima di Fase B: se B si complica, A è comunque shippabile.
- Il round chiude solo con i tre livelli verdi + negativi di lettura verdi + gap senza drift.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura:** Fase A: Helm (fullname/probe/PVC) T1; uploads+signup config T2; compose T3; docs (multi-tenancy + verità allegati) T4 — copre tutti i findings dell'audit deployment. Fase B: meccanica (404/two-hop/subquery) T5; liste+JQL T6; GET gating T7; directory+label/field+WS T8; filter/dashboard get T9; negativi T10; docs T11 — copre tutti i findings dell'audit letture (progetti, issue, JQL, email, gruppi restano aperti by-scope-decision, WS, label/field, filter/dashboard by-id).

**2. Placeholder scan:** codice concreto per EnforceNotFound/MembershipSubquery/fullname helper/pvc.yaml/config; i punti "verificare il nome reale" sono verifiche su firme esistenti con file indicato, non logica mancante. L'inventario GET di T7 è esplicito nel Contesto (righe router precise).

**3. Consistenza tipi:** `EnforceNotFound(permKey, Resolver, next)` stessa shape di `Enforce` (R11). `MembershipSubquery` usata in T6 (liste+search) e T8 (label/field). `Search(..., scope *gorm.DB)` coerente tra service e 3 handler. `ByAttachment`/`ByIssueLink` usati solo in T7. Helper contract R11 riusati in T10. `APP_UPLOADS_DIR` coerente tra config (T2), compose (T3), Helm configmap (T1 — coordinato).
