# Round 11 — Enforcement permessi (autorizzazione server-side) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Chiudere l'unico buco di sicurezza rimasto prima del tag 1.0: introdurre l'**autorizzazione lato server** (403 sulle rotte MUTANTI) basata sul ruolo dell'utente nel progetto (o `is_admin` globale), col fix "il creatore del progetto diventa admin", e i contract test 403 (negativi) + positivi.

**Architecture:** Oggi l'autenticazione (401) è l'unico gate; l'autorizzazione è solo informativa (`/mypermissions`). Round 11 aggiunge un pacchetto `internal/api/authz` con un `Checker` che, dato (userID, projectID, permKey), consulta `is_admin` + il ruolo `project_members` via `permission.ForRole` e concede/nega. L'enforcement si applica come **decorator di rotta** (`authMw(chk.Enforce(permKey, resolver, handler))`) per le rotte il cui progetto è risolvibile dal path (project key / projectID / issue / board / sprint / automation / custom-field); per le poche rotte in cui il progetto è nel **body** (issue create, issueLink, agile create/rank/backlog) il check è **in-handler** (il handler già risolve il progetto). Le rotte **globali-admin** (gruppi, projectCategory) usano `EnforceGlobalAdmin`. Le rotte **owner-scoped** (filtri, dashboard) ottengono un check di proprietà. Le **letture restano aperte** in 1.0 (enforcement solo sulle mutazioni — follow-up dichiarato).

**Tech Stack:** Go 1.25 (net/http, GORM, `clause.OnConflict` per upsert), domini `internal/domain/{permission,project,user,issue,board,sprint,automation,customfield}`, harness `internal/contract`. Nessuna modifica frontend (l'admin demo è global-admin → E2E verdi; la UI già fa gating informativo via `/mypermissions`).

---

## Decisioni di scope (esplicite)

- **Solo MUTAZIONI** (POST/PUT/PATCH/DELETE) sono enforced in 1.0. Le letture (GET, e i POST-di-lettura `/search`, `/search/jql`, `/comment/list`) restano aperte a ogni utente loggato. **Follow-up**: enforcement `BROWSE_PROJECTS` sulle letture.
- **`POST /project` (creazione progetto)**: consentito a **ogni utente autenticato** (modello self-serve, coerente coi test); al successo il **creatore diventa `admin`** del progetto. (Jira lo gaterebbe dietro admin globale — scelta pragmatica dichiarata.)
- **Rotte globali-admin** (`/group*`, `POST /projectCategory`): richiedono `is_admin`.
- **Owner-scoped** (`/filter*`, `/dashboard*`): l'utente può mutare solo le proprie risorse (o admin globale). `/notifications*` e `/myself` sono già per-utente sul soggetto autenticato → verifica, non ridisegno.
- **Non-goal**: enforcement sulle letture; permission scheme/grant configurabili; ruoli oltre admin/member/viewer; gating UI lato frontend oltre l'informativo esistente (follow-up: la UI dovrebbe gestire i 403 con grazia). Allegati/SMTP restano rinviati.

## Policy — mappa Rotta → permesso richiesto

Ruoli (da `permission.ForRole`): **global admin** = tutti; **admin** = ADMINISTER_PROJECTS, BROWSE_PROJECTS, CREATE/EDIT/TRANSITION/DELETE_ISSUES, MANAGE_SPRINTS; **member** = BROWSE, CREATE/EDIT/TRANSITION_ISSUES, MANAGE_SPRINTS; **viewer** = BROWSE.

| Rotta (mutante) | Classe | Permesso | Dove |
|---|---|---|---|
| `POST /issue` | D(body) | CREATE_ISSUES | in-handler |
| `PUT /issue/{issueKey}` | C | EDIT_ISSUES | decorator |
| `DELETE /issue/{issueKey}` | C | DELETE_ISSUES | decorator |
| `POST /issue/{issueKey}/labels` | C | EDIT_ISSUES | decorator |
| `POST/DELETE /issue/{idOrKey}/watchers` | C | BROWSE_PROJECTS | decorator |
| `POST/DELETE /issue/{idOrKey}/votes` | C | BROWSE_PROJECTS | decorator |
| `POST /issue/{idOrKey}/comment`, `PUT/DELETE .../comment/{id}` | C | EDIT_ISSUES | decorator |
| `POST /issue/{idOrKey}/worklog`, `DELETE .../worklog/{id}` | C | EDIT_ISSUES | decorator |
| `POST /issue/{idOrKey}/remotelink`, `DELETE .../remotelink/{id}` | C | EDIT_ISSUES | decorator |
| `POST /issue/{idOrKey}/attachments` | C | EDIT_ISSUES | decorator |
| `DELETE /attachment/{id}` | E | EDIT_ISSUES | decorator (attachment→issue→project) |
| `POST /issue/{issueKey}/transitions` | C | TRANSITION_ISSUES | decorator |
| `PUT /issue/{issueID}/custom-values/{fieldID}` | C(uuid) | EDIT_ISSUES | decorator (issue by uuid) |
| `POST /issueLink`, `DELETE /issueLink/{linkId}` | D/E | EDIT_ISSUES | in-handler / decorator |
| `PUT /project/{key}`, `DELETE /project/{key}`, `POST .../archive`, `.../restore` | A | ADMINISTER_PROJECTS | decorator |
| `POST /project/{key}/members`, `DELETE .../members/{userId}`, `POST .../invites` | A | ADMINISTER_PROJECTS | decorator |
| `PUT/DELETE /project/{key}/star` | A | BROWSE_PROJECTS | decorator |
| `POST/DELETE /project/{key}/git/providers` | A | ADMINISTER_PROJECTS | decorator |
| `POST /project/{key}/webhooks`, `DELETE .../webhooks/{id}` | A | ADMINISTER_PROJECTS | decorator |
| `POST/PATCH/DELETE /project/{key}/workflow/*`, `PUT .../statuses/order` | A | ADMINISTER_PROJECTS | decorator |
| `POST /project/{key}/sprints`, `PATCH .../sprints/{id}`, `.../start`, `.../complete` | A | MANAGE_SPRINTS | decorator |
| `POST /project/{projectID}/custom-fields` | B | ADMINISTER_PROJECTS | decorator |
| `POST /project/{projectID}/automation` | B | ADMINISTER_PROJECTS | decorator |
| `DELETE /custom-fields/{fieldID}`, `POST .../{fieldID}/options` | E | ADMINISTER_PROJECTS | decorator (field→project) |
| `PATCH/DELETE /automation/{ruleID}`, `POST .../execute` | E | ADMINISTER_PROJECTS | decorator (rule→project) |
| `POST/DELETE /rest/agile/1.0/board[/{boardId}]` | D/E | ADMINISTER_PROJECTS | in-handler(create) / decorator(delete) |
| `POST /rest/agile/1.0/sprint`, `POST/PUT/DELETE .../sprint/{id}`, `.../{id}/issue` | D/E | MANAGE_SPRINTS | in-handler(create) / decorator |
| `PUT /rest/agile/1.0/issue/rank`, `POST /rest/agile/1.0/backlog/issue`, `POST /issues/rank` | D(body) | MANAGE_SPRINTS | in-handler |
| `POST /group`, `DELETE /group`, `POST/DELETE /group/user` | Global | is_admin | decorator (global) |
| `POST /projectCategory` | Global | is_admin | decorator (global) |
| `POST /filter`, `PUT/DELETE /filter/{id}`, `.../favourite` | Owner | owner==uid | in-handler |
| `POST /dashboards`/`dashboard`, `PATCH/PUT/DELETE .../{id}`, widgets/gadget, copy | Owner | owner==uid | in-handler |
| `PATCH /notifications/*`, `PUT /myself`, `POST /auth/api-tokens` | Self | (già per-utente) | verifica |
| `POST /project` | — | autenticato; creator→admin | in-handler (T1) |
| `POST /webhooks/git/{token}` | token | (non autenticato, token-scoped) | invariato |

**Nota "not found":** i resolver del decorator, se la risorsa target non esiste, ritornano `ok=false` → il decorator **passa al handler** (che risponde 404 col suo comportamento attuale). Nessun info-leak (un utente non autorizzato su risorsa inesistente vede 404, non 403). L'enforcement scatta solo quando il progetto è risolvibile.

## Contesto verificato (dallo scout — leggere una volta)

- `internal/api/middleware/auth.go`: `Auth(secret, verifyBasic) func(http.Handler) http.Handler` (`:21`); `UserIDFromContext(ctx) string` (`:68`). Il decorator di autz va **dentro** `authMw`: `authMw(chk.Enforce(...))`.
- `internal/domain/permission/permission.go`: `Def{Key,Name,Description,Type}`, `var defs`, `All() []Def`, `ForRole(role string, isGlobalAdmin bool) map[string]bool` (`:31`). Chiavi: ADMINISTER(GLOBAL), ADMINISTER_PROJECTS, BROWSE_PROJECTS, CREATE_ISSUES, EDIT_ISSUES, TRANSITION_ISSUES, DELETE_ISSUES, MANAGE_SPRINTS. **Non ci sono const esportate per le chiavi** → le aggiungiamo (T2).
- `internal/domain/project/member.go`: `MemberRole` (RoleAdmin/RoleMember/RoleViewer), `ProjectMember{ProjectID,UserID,Role}` PK composita, table `project_members`. `service.go`: `AddMember(projectID,userID,role) error` (`:309`), `ListMembers` (`:317`), `DB()` (`:325`). **Manca `GetRole`**.
- `internal/domain/project/service.go`: `Create(name,key,description,pType)` (`:44`) e `CreateProject(in CreateInput)` (`:88`) — **NON** inseriscono `project_members`; `CreateInput` ha `LeadUserID *string` ma non un creator id. `ProjectHandler.Create` (`project_handler.go:50`) **non** legge `UserIDFromContext` e chiama `CreateProject(in)` (`:87`).
- `internal/domain/user/service.go`: `GetByID(id) (*User, error)` (`:11`) carica `IsAdmin`. `user/model.go:10`: `IsAdmin bool`.
- Resolver issue esistenti negli handler: `IssueHandler.resolveIssue` (`issue_handler.go:32`), `CommentHandler.resolve`/`WorklogHandler.resolve`/`VotesHandler.resolve`/`RemoteLinkHandler.resolve` (numerico→`GetBySeqID`, else `GetByKey`). `issue.Service` **non** ha `GetByID(uuid)` → per id interno si usa `issueSvc.DB().First(&iss,"id = ?",id)`.
- Load-by-id per E: `board.Service.GetByID/GetBySeqID` → `Board.ProjectID`; `sprint.Service.GetByID/GetBySeqID` → `Sprint.ProjectID`; `automation.Service.GetRule(id)` → `AutomationRule.ProjectID`; `customfield.Service.GetField(id)` → `CustomField.ProjectID`. `attachment` → `IssueID` (no ProjectID, via issue); `issueLink` → Source/Target issue → project; `filter` è owner-scoped (`SavedFilter.OwnerID`, `ProjectID *string` nullable); `dashboard` è owner-scoped (`Dashboard.OwnerID`, no project).
- Servizi già costruiti in `router.go` (variabili locali): `projectSvc, issueSvc, boardSvc, sprintSvc, autoSvc, cfSvc, filterSvc, dashboardSvc, userSvc?`… (verificare il nome del `user.Service`: se non c'è già una var, costruirla `userSvc := user.NewService(db)` — è leggera).
- Harness contract: `newTestServer(t) (*httptest.Server, *auth.Service)` (`myself_test.go:18`); `registerAndLogin(t, authSvc) string` registra **alice non-admin** (`project_test.go:16`); `createProjectViaAPI(t,srv,tok,key,name)` (crea come alice); `createIssueViaAPI(t,srv,tok,key,summary)`; `doJSON`, `decodeBody`. **`auth.Register` non setta is_admin** (default false). Nessun test multi-utente né 403 oggi.
- Seed (`cmd/seed/main.go`): admin `is_admin=true` (`:71`) e aggiunto a DEMO come `admin` manualmente (`:105`, compensa il gap). Usa `project.NewService(db,&admin)` + `Create` (path che NON auto-aggiunge il membro).

---

## Struttura dei file

- **Dominio:**
  - `internal/domain/project/service.go` — nuovo `GetRole`; `AddMember` idempotente (upsert ruolo); `CreateProject` aggiunge il creatore come admin.
  - `internal/domain/project/service_test.go` — test GetRole + creator-admin + idempotenza.
  - `internal/domain/permission/permission.go` — const esportate delle chiavi.
- **Authz (nuovo pacchetto):**
  - `internal/api/authz/checker.go` — `Checker`, `RequireProject`, `RequireGlobalAdmin`, `ErrForbidden`.
  - `internal/api/authz/resolvers.go` — resolver di progetto dal request.
  - `internal/api/authz/enforce.go` — decorator `Enforce`, `EnforceGlobalAdmin`.
  - `internal/api/authz/*_test.go` — unit test.
- **Handler (in-handler checks):**
  - `internal/api/handlers/project_handler.go` — `Create` passa l'uid (creator).
  - `internal/api/handlers/issue_handler.go` (create → CREATE_ISSUES), `issuelink_handler.go`, `agile_board_handler.go`/`agile_sprint_handler.go`/`agile_misc_handler.go`/`board_handler.go` (body routes), `saved filter`/`dashboard` handlers (owner check).
- **Router:** `internal/api/router.go` — costruzione `Checker` + wrapping delle rotte.
- **Test contract:** `internal/contract/harness_authz_test.go` (nuovo: `newTestServerDB`, `registerUserAndLogin`, `promoteAdmin`), `internal/contract/authz_test.go` (nuovo: negativi/positivi), aggiornamento `users_perms_test.go`/`project_test.go` (gruppi/category → admin).
- **Docs:** `SECURITY.md`, `CHANGELOG.md`, `docs/RELEASE.md`, `docs/superpowers/STATE.md`.

---

### Task 1: project.GetRole + creator-diventa-admin + AddMember idempotente

**Files:**
- Modify: `internal/domain/project/service.go`
- Modify: `internal/api/handlers/project_handler.go`
- Test: `internal/domain/project/service_test.go`

- [ ] **Step 1: Test (falliscono)**

Aggiungere in `internal/domain/project/service_test.go` (creare il file se assente; usare un DB sqlite in-memory con AutoMigrate di `Project`, `ProjectMember`; seguire il pattern degli altri `*_test.go` del dominio):

```go
func TestGetRole_ReturnsEmptyForNonMember(t *testing.T) {
	svc := newTestProjectService(t) // helper locale: NewService(db, nil) su :memory:
	role, err := svc.GetRole("proj-x", "user-x")
	if err != nil {
		t.Fatalf("GetRole err: %v", err)
	}
	if role != "" {
		t.Errorf("atteso ruolo vuoto per non-membro, ottenuto %q", role)
	}
}

func TestAddMember_IsIdempotentAndUpdatesRole(t *testing.T) {
	svc := newTestProjectService(t)
	if err := svc.AddMember("p1", "u1", RoleMember); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember("p1", "u1", RoleAdmin); err != nil {
		t.Fatalf("re-add deve essere idempotente: %v", err)
	}
	role, _ := svc.GetRole("p1", "u1")
	if role != RoleAdmin {
		t.Errorf("atteso admin dopo upsert, ottenuto %q", role)
	}
	var cnt int64
	svc.DB().Model(&ProjectMember{}).Where("project_id = ? AND user_id = ?", "p1", "u1").Count(&cnt)
	if cnt != 1 {
		t.Errorf("atteso 1 riga membro, %d", cnt)
	}
}

func TestCreateProject_AddsCreatorAsAdmin(t *testing.T) {
	svc := newTestProjectService(t)
	p, err := svc.CreateProject(CreateInput{Name: "P", Key: "P", Type: TypeSoftware, CreatorID: "creator-1"})
	if err != nil {
		t.Fatal(err)
	}
	role, _ := svc.GetRole(p.ID, "creator-1")
	if role != RoleAdmin {
		t.Errorf("il creatore deve essere admin, ottenuto %q", role)
	}
}
```

> **Nota:** verificare il nome reale del valore `Type` (es. `TypeSoftware`/`TypeBusiness`) leggendo `project/model.go`; usare quello. Il helper `newTestProjectService` apre `sqlite :memory:` + `AutoMigrate(&Project{}, &ProjectMember{})` e ritorna `NewService(db, nil)`.

- [ ] **Step 2: Eseguire (falliscono)**

Run: `go test ./internal/domain/project/ -run 'GetRole|AddMember_IsIdempotent|CreateProject_AddsCreator' -v`
Expected: FAIL (undefined GetRole / CreatorID / comportamento).

- [ ] **Step 3: Implementare**

In `internal/domain/project/service.go`:

```go
import (
	"errors"
	// ... esistenti ...
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetRole restituisce il ruolo dell'utente nel progetto, o "" se non è membro.
func (s *Service) GetRole(projectID, userID string) (MemberRole, error) {
	var m ProjectMember
	err := s.db.Where("project_id = ? AND user_id = ?", projectID, userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return m.Role, nil
}
```

Sostituire `AddMember` con la versione idempotente (upsert del ruolo):

```go
func (s *Service) AddMember(projectID, userID string, role MemberRole) error {
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"role"}),
	}).Create(&ProjectMember{ProjectID: projectID, UserID: userID, Role: role}).Error
}
```

In `CreateInput` (struct) aggiungere `CreatorID string`. In `CreateProject`, dopo l'inserimento riuscito del progetto (dopo `s.db.Create(p)`), aggiungere:

```go
	if in.CreatorID != "" {
		if err := s.AddMember(p.ID, in.CreatorID, RoleAdmin); err != nil {
			return nil, err
		}
	}
```

- [ ] **Step 4: Handler passa l'uid**

In `internal/api/handlers/project_handler.go`, dentro `Create`, prima di costruire `in`:
```go
	uid := middleware.UserIDFromContext(r.Context())
```
e impostare `in.CreatorID = uid` nel `project.CreateInput`. (Import `middleware` se non presente — è già usato altrove nel file.)

- [ ] **Step 5: Eseguire (passano) + build**

Run: `go test ./internal/domain/project/ -v 2>&1 | tail -20 && go build ./... && echo BUILD_OK`
Expected: PASS + BUILD_OK.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/project/ internal/api/handlers/project_handler.go
git commit -m "feat(project): GetRole, idempotent AddMember, creator becomes project admin on create"
```

---

### Task 2: permission const + authz.Checker (core)

**Files:**
- Modify: `internal/domain/permission/permission.go`
- Create: `internal/api/authz/checker.go`
- Test: `internal/api/authz/checker_test.go`

- [ ] **Step 1: Const chiavi permesso**

In `internal/domain/permission/permission.go` aggiungere const esportate e usarle nei `defs` (single source of truth):

```go
const (
	Administer         = "ADMINISTER"
	AdministerProjects = "ADMINISTER_PROJECTS"
	BrowseProjects     = "BROWSE_PROJECTS"
	CreateIssues       = "CREATE_ISSUES"
	EditIssues         = "EDIT_ISSUES"
	TransitionIssues   = "TRANSITION_ISSUES"
	DeleteIssues       = "DELETE_ISSUES"
	ManageSprints      = "MANAGE_SPRINTS"
)
```
(Aggiornare i letterali nei `defs` e in `ForRole` per riferire queste const; comportamento invariato — girare `go test ./internal/domain/permission/` per confermare.)

- [ ] **Step 2: Test checker (falliscono)**

`internal/api/authz/checker_test.go` — DB sqlite :memory:, migrate `user.User`, `project.Project`, `project.ProjectMember`; creare un utente admin globale, uno member, uno estraneo; costruire `Checker` con `user.NewService(db)` e `project.NewService(db, nil)`:

```go
func TestRequireProject(t *testing.T) {
	// setup: db, users (globalAdmin is_admin=true; alice member su P1; bob nessun ruolo), project P1
	// chk := New(userSvc, projSvc, issueSvc, boardSvc, sprintSvc, autoSvc, cfSvc)  // vedi firma T2/T3
	cases := []struct {
		uid, permKey string
		want         bool
	}{
		{globalAdminID, permission.DeleteIssues, true},   // admin globale: tutto
		{aliceID, permission.CreateIssues, true},         // member: create sì
		{aliceID, permission.DeleteIssues, false},        // member: delete no
		{aliceID, permission.AdministerProjects, false},  // member: admin-proj no
		{bobID, permission.BrowseProjects, false},        // estraneo: niente
	}
	for _, c := range cases {
		err := chk.RequireProject(c.uid, p1ID, c.permKey)
		if (err == nil) != c.want {
			t.Errorf("RequireProject(%s,%s)=%v, want allow=%v", c.uid, c.permKey, err, c.want)
		}
	}
}

func TestRequireGlobalAdmin(t *testing.T) {
	if err := chk.RequireGlobalAdmin(globalAdminID); err != nil {
		t.Errorf("admin globale deve passare: %v", err)
	}
	if err := chk.RequireGlobalAdmin(aliceID); !errors.Is(err, ErrForbidden) {
		t.Errorf("non-admin deve essere ErrForbidden, %v", err)
	}
}
```

- [ ] **Step 3: Implementare checker**

`internal/api/authz/checker.go`:

```go
// Package authz applica l'autorizzazione lato server: dato l'utente autenticato,
// il progetto risolto dalla richiesta e il permesso richiesto, concede o nega.
package authz

import (
	"errors"

	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	// gli altri servizi (issue/board/sprint/automation/customfield) servono ai resolver (T3)
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/board"
	"github.com/it4nodummies/heureum/internal/domain/sprint"
	"github.com/it4nodummies/heureum/internal/domain/automation"
	"github.com/it4nodummies/heureum/internal/domain/customfield"
)

var ErrForbidden = errors.New("forbidden")

type Checker struct {
	users    *user.Service
	projects *project.Service
	issues   *issue.Service
	boards   *board.Service
	sprints  *sprint.Service
	autos    *automation.Service
	cfs      *customfield.Service
}

func New(users *user.Service, projects *project.Service, issues *issue.Service, boards *board.Service, sprints *sprint.Service, autos *automation.Service, cfs *customfield.Service) *Checker {
	return &Checker{users: users, projects: projects, issues: issues, boards: boards, sprints: sprints, autos: autos, cfs: cfs}
}

// RequireProject: l'utente deve avere permKey sul progetto (o essere admin globale).
func (c *Checker) RequireProject(userID, projectID, permKey string) error {
	if c.isGlobalAdmin(userID) {
		return nil
	}
	role, err := c.projects.GetRole(projectID, userID)
	if err != nil {
		return ErrForbidden
	}
	if permission.ForRole(string(role), false)[permKey] {
		return nil
	}
	return ErrForbidden
}

// RequireGlobalAdmin: l'utente deve avere is_admin.
func (c *Checker) RequireGlobalAdmin(userID string) error {
	if c.isGlobalAdmin(userID) {
		return nil
	}
	return ErrForbidden
}

func (c *Checker) isGlobalAdmin(userID string) bool {
	u, err := c.users.GetByID(userID)
	return err == nil && u.IsAdmin
}
```

> **Nota firma:** verificare i costruttori reali dei servizi (`user.NewService(db)`, `board.NewService`, `sprint.NewService`, `automation.NewService`, `customfield.NewService`) e i tipi esatti; adeguare gli import/campi. Se un servizio non è ancora costruito in router, lo si costruirà in T4.

- [ ] **Step 4: Eseguire (passano)**

Run: `go test ./internal/api/authz/ ./internal/domain/permission/ -v 2>&1 | tail -20`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/permission/permission.go internal/api/authz/checker.go internal/api/authz/checker_test.go
git commit -m "feat(authz): permission key constants and Checker (RequireProject/RequireGlobalAdmin)"
```

---

### Task 3: authz resolvers + decorator Enforce

**Files:**
- Create: `internal/api/authz/resolvers.go`
- Create: `internal/api/authz/enforce.go`
- Test: `internal/api/authz/enforce_test.go`

- [ ] **Step 1: Resolver**

`internal/api/authz/resolvers.go` — un `Resolver` estrae `(projectID string, ok bool)` dalla richiesta. `ok=false` ⇒ target non trovato ⇒ il decorator passa al handler (404 naturale).

```go
package authz

import (
	"net/http"
	"strconv"
)

// Resolver risolve il progetto rilevante per la richiesta.
type Resolver func(r *http.Request) (projectID string, ok bool)

// ByKey: path {key} → project.GetByKey.
func (c *Checker) ByKey(r *http.Request) (string, bool) {
	p, err := c.projects.GetByKey(r.PathValue("key"))
	if err != nil {
		return "", false
	}
	return p.ID, true
}

// ByProjectID: path {projectID} è già l'UUID del progetto (verificare che esista).
func (c *Checker) ByProjectID(r *http.Request) (string, bool) {
	id := r.PathValue("projectID")
	if id == "" {
		return "", false
	}
	if _, err := c.projects.GetByID(id); err != nil {
		return "", false
	}
	return id, true
}

// ByIssueParam: risolve un path param issue (numerico→GetBySeqID, else GetByKey) → ProjectID.
func (c *Checker) ByIssueParam(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		v := r.PathValue(param)
		if v == "" {
			return "", false
		}
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			if iss, err := c.issues.GetBySeqID(n); err == nil {
				return iss.ProjectID, true
			}
		}
		if iss, err := c.issues.GetByKey(v); err == nil {
			return iss.ProjectID, true
		}
		return "", false
	}
}

// ByIssueUUID: path {issueID} è l'UUID interno della issue (custom-values) → ProjectID via DB.
func (c *Checker) ByIssueUUID(r *http.Request) (string, bool) {
	id := r.PathValue("issueID")
	if id == "" {
		return "", false
	}
	var iss issue.Issue
	if err := c.issues.DB().First(&iss, "id = ?", id).Error; err != nil {
		return "", false
	}
	return iss.ProjectID, true
}

// ByBoardSeq / BySprintSeq / ByAutomationRule / ByCustomField: load-by-id → ProjectID.
func (c *Checker) ByBoardSeq(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		n, err := strconv.ParseInt(r.PathValue(param), 10, 64)
		if err != nil {
			return "", false
		}
		b, err := c.boards.GetBySeqID(n)
		if err != nil {
			return "", false
		}
		return b.ProjectID, true
	}
}
// (analoghi BySprintSeq(param), ByAutomationRule(param="ruleID") via autos.GetRule, ByCustomField(param="fieldID") via cfs.GetField)
```

> **Nota implementatore (CRITICO):** verificare le firme reali: `project.GetByID`, `board.GetBySeqID`/`GetByID`, `sprint.GetBySeqID`/`GetByID`, `automation.GetRule`, `customfield.GetField`, `issue.GetBySeqID`/`GetByKey`/`DB()`, e i **nomi reali dei path param** in `router.go` (es. `{boardId}`, `{sprintId}`, `{ruleID}`, `{fieldID}`, `{issueKey}` vs `{issueIdOrKey}`). Implementare i resolver realmente necessari alla tabella policy; per attachment/issueLink (E a due salti) vedere T4 (o gestirli in-handler se il resolver è troppo intricato).

- [ ] **Step 2: Test decorator (falliscono)**

`internal/api/authz/enforce_test.go` — costruire un `Checker` su DB in-memory (come T2), un handler finto che scrive 200, e verificare: utente con permesso → 200; senza → 403; resolver `ok=false` → il handler viene comunque chiamato (200/404 del handler). Usare `httptest.NewRequest` con `SetPathValue` e un contesto che porti l'uid (usare `middleware`—vedi sotto). Per iniettare l'uid nel contesto nel test, esporre un helper di test o usare `middleware` (l'`Enforce` legge `middleware.UserIDFromContext`).

- [ ] **Step 3: Implementare decorator**

`internal/api/authz/enforce.go`:

```go
package authz

import (
	"net/http"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
)

func forbidden(w http.ResponseWriter) {
	v3.WriteError(w, http.StatusForbidden, []string{"you do not have permission to perform this action"}, nil)
}

// Enforce: richiede permKey sul progetto risolto da resolve. Se il target non è
// risolvibile (ok=false) passa al handler (404 naturale, nessun info-leak).
func (c *Checker) Enforce(permKey string, resolve Resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := middleware.UserIDFromContext(r.Context())
		projectID, ok := resolve(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if err := c.RequireProject(uid, projectID, permKey); err != nil {
			forbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// EnforceGlobalAdmin: richiede is_admin.
func (c *Checker) EnforceGlobalAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := c.RequireGlobalAdmin(middleware.UserIDFromContext(r.Context())); err != nil {
			forbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

> **Nota:** verificare la firma reale di `v3.WriteError` (usata ovunque negli handler) e adeguare la chiamata.

- [ ] **Step 4: Eseguire (passano) + build**

Run: `go test ./internal/api/authz/ -v 2>&1 | tail -20 && go build ./... && echo BUILD_OK`
Expected: PASS + BUILD_OK.

- [ ] **Step 5: Commit**

```bash
git add internal/api/authz/
git commit -m "feat(authz): request→project resolvers and Enforce/EnforceGlobalAdmin decorators"
```

---

### Task 4: Wiring decorator sulle rotte project-scoped (A/B/C/E)

**Files:**
- Modify: `internal/api/router.go`

- [ ] **Step 1: Costruire il Checker**

In `internal/api/router.go`, dopo la costruzione dei servizi (assicurarsi che esista `userSvc := user.NewService(db)`; se assente, aggiungerlo), costruire:
```go
	chk := authz.New(userSvc, projectSvc, issueSvc, boardSvc, sprintSvc, autoSvc, cfSvc)
```
Import `"github.com/it4nodummies/heureum/internal/api/authz"` e `"github.com/it4nodummies/heureum/internal/domain/user"` se serve.

- [ ] **Step 2: Avvolgere le rotte A/B/C/E**

Per ciascuna rotta MUTANTE project-scoped della tabella policy, cambiare da:
```go
mux.Handle("PUT /rest/api/3/project/{key}", authMw(http.HandlerFunc(projectH.Update)))
```
a:
```go
mux.Handle("PUT /rest/api/3/project/{key}", authMw(chk.Enforce(permission.AdministerProjects, chk.ByKey, http.HandlerFunc(projectH.Update))))
```
Applicare a TUTTE le rotte A/B/C/E della tabella (project settings→AdministerProjects; star→BrowseProjects; sprints per-key→ManageSprints; issue C→CreateIssues/EditIssues/DeleteIssues/TransitionIssues/BrowseProjects come da tabella; watchers/votes→BrowseProjects; attachment DELETE, issueLink DELETE, custom-fields/{fieldID}, automation/{ruleID}, agile board/{id}, sprint/{id} → col resolver E appropriato).

> **Attachment / issueLink DELETE (E a due salti):** se un resolver diretto è troppo intricato (attachment→issue→project; link→issue→project), è ammesso gestirli **in-handler** in T5 invece che col decorator — annotarlo e spostarli lì. Preferire il decorator quando il resolver è semplice.
>
> **NON avvolgere:** letture GET, `/search*`, `/comment/list`, `POST /issue` (T5), `POST /issueLink` (T5), agile create/rank/backlog (T5), rotte owner/global/self (T6/T7), `POST /project` (già gestito in T1), `POST /webhooks/git/{token}` (non autenticato). Preservare le closure inline esistenti (`router.go:150/154`) avvolgendole allo stesso modo se sono rotte mutanti (verificare cosa fanno).

- [ ] **Step 3: Build + vet + suite contract completa**

Run:
```bash
go build ./... && echo BUILD_OK && go vet ./... && echo VET_OK
go test ./internal/contract/ -v 2>&1 | grep -E '^(--- FAIL|FAIL|ok|PASS)' | tail -40
```
Expected: BUILD_OK, VET_OK, **tutti i contract test PASS** — perché alice, creando il progetto, ora è admin (T1) e ha tutti i permessi di progetto. Se qualche test va in 403, verificare: (a) il resolver risolve il progetto giusto? (b) il permesso mappato è coerente con la tabella? (c) alice è davvero admin del progetto creato? NON allentare l'enforcement per far passare i test — correggere resolver/mappa.

- [ ] **Step 4: Commit**

```bash
git add internal/api/router.go
git commit -m "feat(authz): enforce project permissions on path-scoped mutating routes"
```

---

### Task 5: Enforcement in-handler per le rotte body-based (D)

**Files:**
- Modify: `internal/api/handlers/issue_handler.go` (Create), `issuelink_handler.go`, `agile_board_handler.go`, `agile_sprint_handler.go`, `agile_misc_handler.go`, `board_handler.go`
- Modify: `internal/api/router.go` (iniettare il Checker in questi handler o passarlo al costruttore)

- [ ] **Step 1: Iniettare il Checker negli handler interessati**

Aggiungere un campo `authz *authz.Checker` (o `chk`) ai struct degli handler che gestiscono rotte D, e passarlo dai rispettivi costruttori in `router.go`. (Verificare i costruttori reali.)

- [ ] **Step 2: Check dopo la risoluzione del progetto**

In ciascun handler D, DOPO aver risolto il progetto dal body e PRIMA di mutare:
- `IssueHandler.Create`: risolve `projectSvc.GetByKey/GetByID` dal body → `if err := h.authz.RequireProject(uid, projectID, permission.CreateIssues); err != nil { forbid(w); return }`. `uid := middleware.UserIDFromContext(r.Context())`.
- `issueLink` create (POST /issueLink): risolto l'issue sorgente → `RequireProject(uid, srcProjectID, permission.EditIssues)`.
- Agile board create: risolto il progetto dal `projectKeyOrId` → `RequireProject(uid, pid, permission.AdministerProjects)`.
- Agile sprint create: risolto il progetto via `originBoardId`→board.ProjectID → `permission.ManageSprints`.
- Rank / backlog / issues rank: risolto il progetto della/e issue coinvolte → `permission.ManageSprints`. (Per liste multi-issue: verificare che tutte appartengano a progetti su cui l'utente ha il permesso; per semplicità 1.0, controllare il progetto della prima issue risolta e — se le issue possono attraversare progetti — negare se una non è consentita. Documentare la semplificazione.)

Fornire un helper `forbid(w)` negli handler (o riusare quello di `authz` esportando `authz.WriteForbidden(w)`), coerente con `v3.WriteError` 403.

- [ ] **Step 3: (Se spostati da T4) attachment/issueLink DELETE in-handler**

Se in T4 sono stati lasciati qui: in `AttachmentHandler` delete e `issueLink` delete, dopo aver caricato la riga e risolto il progetto via issue, applicare `RequireProject(uid, projectID, permission.EditIssues)`.

- [ ] **Step 4: Build + suite contract**

Run: `go build ./... && go test ./internal/contract/ 2>&1 | grep -E '^(--- FAIL|FAIL|ok)' | tail -30`
Expected: verde (alice admin del proprio progetto → create issue/link/board/sprint OK).

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/ internal/api/router.go
git commit -m "feat(authz): enforce permissions on body-scoped routes (issue/link/agile create, rank, backlog)"
```

---

### Task 6: Enforcement globale-admin (gruppi, projectCategory) + helper test

**Files:**
- Modify: `internal/api/router.go`
- Create: `internal/contract/harness_authz_test.go`
- Modify: `internal/contract/users_perms_test.go`, `internal/contract/project_test.go`

- [ ] **Step 1: Avvolgere le rotte globali-admin**

In `router.go`, avvolgere con `chk.EnforceGlobalAdmin(...)`:
- `POST /rest/api/3/group`, `DELETE /rest/api/3/group`, `POST /rest/api/3/group/user`, `DELETE /rest/api/3/group/user`
- `POST /rest/api/3/projectCategory`

- [ ] **Step 2: Helper di test per utente admin + DB**

`internal/contract/harness_authz_test.go`:

```go
package contract

import (
	"testing"

	"github.com/it4nodummies/heureum/internal/domain/auth"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"gorm.io/gorm"
	"net/http/httptest"
)

// newTestServerDB come newTestServer ma restituisce anche il *gorm.DB per i test
// che devono impostare ruoli/flag (es. promuovere ad admin globale).
func newTestServerDB(t *testing.T) (*httptest.Server, *auth.Service, *gorm.DB) {
	// duplicare il corpo di newTestServer (myself_test.go:18) tenendo il riferimento al db
	// ... apri store/DB, migra, srv := httptest.NewServer(api.NewRouter(cfg, db)); return srv, authSvc, db
}

// registerUserAndLogin registra un utente arbitrario e ritorna il jwt.
func registerUserAndLogin(t *testing.T, authSvc *auth.Service, email, username string) string {
	t.Helper()
	if _, err := authSvc.Register(email, username, username, "password-123"); err != nil {
		t.Fatal(err)
	}
	jwt, err := authSvc.Login(email, "password-123")
	if err != nil {
		t.Fatal(err)
	}
	return jwt
}

// promoteAdmin imposta is_admin=true sull'utente con quell'email.
func promoteAdmin(t *testing.T, db *gorm.DB, email string) {
	t.Helper()
	if err := db.Model(&user.User{}).Where("email = ?", email).Update("is_admin", true).Error; err != nil {
		t.Fatal(err)
	}
}
```

> **Nota:** leggere `myself_test.go:18` per replicare esattamente il corpo di `newTestServer` (stesso `api.NewRouter(cfg, db)` reale) restituendo il `db`. Verificare la firma di `auth.Register` (ordine parametri email/username/displayName/password) dallo scout/handler.

- [ ] **Step 3: Aggiornare i 2 test globali-admin**

`TestGroups_CRUDConformant` (users_perms_test.go) e `TestProjectCategories_ConformsToContract` (project_test.go): usare `newTestServerDB`, registrare l'utente, `promoteAdmin(t, db, email)`, poi ri-login (o loggare dopo la promozione) così il token è di un admin globale → le mutazioni gruppi/category tornano 200/201.

> **Nota:** `is_admin` è letto dal DB a ogni richiesta (via `user.GetByID` nel Checker), quindi NON serve un nuovo login dopo `promoteAdmin` — il JWT porta solo l'user id; basta promuovere prima della chiamata mutante. Confermare che il Checker rilegge `is_admin` dal DB (sì: `users.GetByID`).

- [ ] **Step 4: Build + suite contract**

Run: `go build ./... && go test ./internal/contract/ 2>&1 | grep -E '^(--- FAIL|FAIL|ok)' | tail -30`
Expected: verde (gruppi/category ora richiedono admin; i test li promuovono).

- [ ] **Step 5: Commit**

```bash
git add internal/api/router.go internal/contract/
git commit -m "feat(authz): global-admin enforcement on groups and project categories; test helpers"
```

---

### Task 7: Enforcement owner-scoped (filtri, dashboard)

**Files:**
- Modify: gli handler di filtri (`internal/api/handlers/*filter*`) e dashboard (`*dashboard*`)

- [ ] **Step 1: Verificare il comportamento attuale**

Leggere gli handler di mutazione filtri (`PUT/DELETE /filter/{id}`, `.../favourite`) e dashboard (`PATCH/PUT/DELETE /dashboards|dashboard/{id}`, widgets/gadget, copy). Determinare se già filtrano per proprietario (`WHERE owner_id = uid`) o se caricano per id senza controllo owner. **Riportare cosa si trova**: se già owner-scoped, nessun buco → documentare e passare oltre (nessuna modifica). Se NO, procedere.

- [ ] **Step 2: Aggiungere il check di proprietà dove manca**

Per ogni mutazione filtro/dashboard priva di controllo: caricare la riga (`filterSvc.Get(id)` → `OwnerID`; `dashboardSvc.GetDashboard(id)` → `OwnerID`), e:
```go
	uid := middleware.UserIDFromContext(r.Context())
	if row.OwnerID != uid && !isGlobalAdmin(uid) {
		forbid(w); return
	}
```
(Per l'admin globale usare `chk.RequireGlobalAdmin(uid) == nil`, oppure iniettare il Checker in questi handler e usare un metodo `IsGlobalAdmin`. Se preferibile, aggiungere `authz.Checker.IsGlobalAdmin(uid) bool` esportato.)

- [ ] **Step 3: Test**

Aggiungere/estendere un contract test: alice crea un filtro; bob (secondo utente, via `registerUserAndLogin`) tenta `DELETE /filter/{id}` di alice → **403**; alice → 204. (Idem 1 caso dashboard se applicabile.)

- [ ] **Step 4: Build + suite**

Run: `go build ./... && go test ./internal/contract/ 2>&1 | grep -E '^(--- FAIL|FAIL|ok)' | tail -30`
Expected: verde.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/ internal/contract/
git commit -m "feat(authz): owner-scoped enforcement on filter/dashboard mutations"
```

---

### Task 8: Contract test negativi/positivi (403)

**Files:**
- Create: `internal/contract/authz_test.go`

- [ ] **Step 1: Test**

`internal/contract/authz_test.go` — usare `newTestServerDB`, `registerAndLogin` (alice) e `registerUserAndLogin` (bob), `createProjectViaAPI` (alice crea "AZ"):

```go
func TestAuthz_NonMemberForbiddenOnMutations(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob@example.com", "bob")
	createProjectViaAPI(t, srv, alice, "AZ", "Alice Proj")

	// bob non è membro → create issue nel progetto di alice = 403
	resp := doJSON(t, srv, http.MethodPost, bob, "/rest/api/3/issue", map[string]any{
		"fields": map[string]any{
			"project":   map[string]any{"key": "AZ"},
			"summary":   "hack",
			"issuetype": map[string]any{"name": "Task"},
		},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("atteso 403 per non-membro, ottenuto %d", resp.StatusCode)
	}

	// bob non può amministrare il progetto di alice = 403
	resp = doJSON(t, srv, http.MethodPut, bob, "/rest/api/3/project/AZ", map[string]any{"name": "Hijacked"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("atteso 403 PUT project, ottenuto %d", resp.StatusCode)
	}
}

func TestAuthz_CreatorCanMutateOwnProject(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, alice, "AZ", "Alice Proj")
	// alice è creator→admin: create issue OK
	key := createIssueViaAPI(t, srv, alice, "AZ", "legit")
	if key == "" {
		t.Fatal("il creatore deve poter creare issue")
	}
}

func TestAuthz_NonAdminForbiddenOnGroups(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	resp := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/group", map[string]any{"name": "x"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("atteso 403 group create per non-admin, ottenuto %d", resp.StatusCode)
	}
}
```

> **Nota:** verificare la firma reale di `doJSON`/`createIssueViaAPI`/`createProjectViaAPI` (dallo scout) e adeguare. `createIssueViaAPI` fa `t.Fatal` se non 201 — per il caso positivo va bene; per i casi 403 usare `doJSON` diretto (non gli helper che pretendono 201).

- [ ] **Step 2: Eseguire**

Run: `go test ./internal/contract/ -run TestAuthz -v 2>&1 | tail -30`
Expected: 3 PASS.

- [ ] **Step 3: Suite completa**

Run: `go test ./... 2>&1 | grep -vE '^ok|no test files'; echo DONE`
Expected: nessun FAIL.

- [ ] **Step 4: Commit**

```bash
git add internal/contract/authz_test.go
git commit -m "test(authz): 403 negative paths (non-member, non-admin) and creator positive"
```

---

### Task 9: Gate finale + docs sicurezza + STATE.md → 1.0 ready

**Files:**
- Modify: `SECURITY.md`, `CHANGELOG.md`, `docs/RELEASE.md`, `docs/superpowers/STATE.md`

- [ ] **Step 1: Gate a tre livelli**

Run:
```bash
cd /Users/n0r41n/Development/open-jira
go build ./... && echo BUILD_OK
go vet ./... && echo VET_OK
go test ./... 2>&1 | grep -vE '^ok|no test files'; echo GO_DONE
lsof -ti:8080 | xargs kill 2>/dev/null; lsof -ti:3000 | xargs kill 2>/dev/null; sleep 1
cd frontend-next && npx tsc --noEmit && echo TSC_OK && npm run build 2>&1 | tail -3 && npx playwright test --reporter=line 2>&1 | tail -6; cd ..
```
Expected: BUILD_OK, VET_OK, nessun FAIL Go, TSC_OK, build OK, **tutti gli E2E verdi** (l'admin demo è global-admin → passa l'enforcement).

- [ ] **Step 2: Gap report senza drift + verifica seed**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md && rm -f /tmp/s11.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/s11.db go run ./cmd/seed >/dev/null 2>&1 && echo SEED_OK && rm -f /tmp/s11.db seed gapreport`
Expected: nessun drift (l'enforcement non aggiunge/rimuove rotte); `SEED_OK` (il seed continua a funzionare; l'admin resta global-admin + membro DEMO).

- [ ] **Step 3: Aggiornare SECURITY.md**

Rimuovere/riformulare la nota "permessi non enforced": ora l'autorizzazione è applicata lato server (403) sulle **mutazioni**. Aggiungere la nota residua onesta: "Reads are not yet permission-gated (any authenticated user can read any project's data); read-side authorization is a planned enhancement." Aggiornare le versioni supportate se serve.

- [ ] **Step 4: CHANGELOG + RELEASE**

- `CHANGELOG.md`: nella voce `[1.0.0]` aggiungere sotto "Security" (o una nuova voce se già rilasciato) l'enforcement dei permessi (403 su mutazioni project-scoped, global-admin su gruppi/category, owner-scoped su filtri/dashboard, creator→admin) e spostare "permissions not enforced" da "Known limitations" a fatto; lasciare in "Known limitations" solo: letture non gated, allegati, SMTP/OAuth non wired.
- `docs/RELEASE.md`: nella checklist, rimuovere la riga "permissions informational only"; aggiornare la "Known limitation to disclose" a "reads not yet permission-gated".

- [ ] **Step 5: Aggiornare STATE.md**

- header: "dopo Round 11".
- aggiungere la riga **Round 11 — Enforcement permessi** ai round completati (pacchetto `internal/api/authz`: `Checker` + resolver + decorator `Enforce`/`EnforceGlobalAdmin`; `project.GetRole` + `AddMember` idempotente + creator→admin; mappa rotta→permesso; enforcement in-handler per rotte body; global-admin su gruppi/category; owner-scoped su filtri/dashboard; contract test 403 negativi/positivi; letture non ancora gated).
- cambiare "Prossimo" in **Tag 1.0 / post-1.0** (il blocco di sicurezza è chiuso): opzioni post-1.0 = enforcement letture (`BROWSE_PROJECTS`), permission scheme configurabili, allegati, SMTP/OAuth wiring, gating UI dei 403 lato frontend.
- spostare il follow-up **(R8) CRITICO enforcement** in "risolto (R11)".

- [ ] **Step 6: Commit**

```bash
git add SECURITY.md CHANGELOG.md docs/RELEASE.md docs/superpowers/STATE.md docs/contracts/gap-report.md
git commit -m "docs: server-side permission enforcement complete; 1.0 security blocker closed"
```

---

## Note di chiusura round

- **Follow-up (post-1.0):** enforcement sulle **letture** (`BROWSE_PROJECTS` su GET — oggi ogni utente loggato legge tutto); permission scheme + grant configurabili (oltre admin/member/viewer fissi); gating UI dei 403 lato frontend (nascondere azioni non permesse oltre all'informativo `/mypermissions`); ruoli progetto configurabili; rank/backlog multi-progetto (T5 controlla il progetto della prima issue — irrobustire per liste cross-progetto).
- **Rischi:** il wiring di T4 avvolge ~34 rotte — la rete di sicurezza è la **suite contract** (deve restare verde grazie a creator→admin di T1) più i **test negativi** di T8 (provano che l'enforcement morde davvero). Se un contract test va in 403 dopo T4, è un segnale di resolver/mappa errati, NON di enforcement da allentare. L'E2E resta verde perché l'admin demo è global-admin.
- Il round chiude solo con i tre livelli verdi, i test negativi 403 verdi, e il gap report senza drift.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura:** buco critico (403 su mutazioni) → T2/T3 (meccanismo) + T4/T5 (wiring path+body). Creator→admin → T1 (sblocca i test + corretto). Global-admin (gruppi/category) → T6. Owner-scoped (filtri/dashboard) → T7. Test negativi/positivi → T8. Docs+gate → T9. Letture fuori scope (dichiarato).

**2. Placeholder scan:** codice concreto per GetRole/AddMember/CreateProject (T1), Checker (T2), resolver+decorator (T3), test negativi (T8). Le "Note implementatore" indicano le firme reali da verificare (costruttori servizi, nomi path param, `v3.WriteError`, corpo di `newTestServer`) coi file da leggere — non placeholder di logica. La tabella policy è completa e mappa ogni rotta mutante.

**3. Consistenza:** `permission.*` const (T2) usate in tabella/handler/test. `authz.Checker` (T2) + resolver (T3) + `Enforce` (T3) usati nel router (T4) e in-handler (T5/T6/T7). `project.GetRole` (T1) usato dal Checker (T2). `newTestServerDB`/`registerUserAndLogin`/`promoteAdmin` (T6) usati in T7/T8. Il modello "creator→admin" (T1) è la precondizione che tiene verdi i contract test dopo T4/T5.
