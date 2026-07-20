# Team associati ai progetti — Piano d'implementazione

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Associare un team (gruppo di utenti) a un progetto con un ruolo, così che i membri del team ereditino quel ruolo/permessi e possano lavorarne le issue.

**Architecture:** Nuova tabella `project_teams(project_id, group_id, role)` che collega i `groups` esistenti ai `projects`. Il ruolo effettivo di un utente su un progetto diventa il più permissivo tra il ruolo individuale (`project_members`) e quelli ereditati dai team. `authz.Checker.RequireProject` e lo scoping letture (`MembershipSubquery`) usano il ruolo effettivo. Nuovi endpoint estensione `/project/{key}/teams`. UI: la pagina Gruppi diventa "Teams" e il tab Access del progetto guadagna una sezione Teams.

**Tech Stack:** Go 1.25 (GORM, golang-migrate, net/http ServeMux), Next.js App Router + React 19 + TanStack Query, Playwright.

## Global Constraints

- Migrazioni numerate `000023_*` (`.up.sql`/`.down.sql`, convenzione golang-migrate, niente `IF NOT EXISTS` non standard).
- **FK nullable → NULL, mai stringa vuota**; timestamp come `time.Time` (lezione SQLite-vs-Postgres: i bug tipo/FK sfuggono ai test SQLite). `project_teams` non ha colonne nullable-FK, ma le scritture usano id reali.
- Tutte le rotte project-scoped keyed su `{key}` (mai UUID interno); gating via `internal/api/authz`.
- **NON toccare** il contratto Jira: `/rest/api/3/group*` restano invariati; i nuovi endpoint `/project/{key}/teams` sono estensioni Heureum.
- `permission.ForRole` e le chiavi permesso restano invariate (nessun permesso nuovo).
- Ruoli: enum esistente `project.MemberRole` = `admin`/`member`/`viewer`. Conflitto individuale/team → più permissivo (`admin > member > viewer`).
- Gate a tre livelli verde prima di considerare finito: `go build ./... && go vet ./... && go test ./...`; `cd frontend-next && npm run build && npx playwright test`; `go run ./cmd/gapreport` senza drift.
- Conventional Commits; branch `feat/teams-project-association` (già creato da origin/main).

## File Structure

- `migrations/000023_project_teams.up.sql` / `.down.sql` — nuova tabella (create).
- `internal/domain/project/team.go` — modello `ProjectTeam` + tipo `ProjectTeamInfo` (create).
- `internal/domain/project/service.go` — `AddTeam`/`RemoveTeam`/`ListTeams`/`EffectiveRole` + estensione membership (modify).
- `internal/domain/project/service_test.go` (o nuovo `team_test.go`) — test dominio (create/modify).
- `internal/api/authz/checker.go` — `RequireProject` usa `EffectiveRole` (modify).
- `internal/api/handlers/project_team_handler.go` — handler dei 4 endpoint (create).
- `internal/api/router.go` — registrazione rotte gated (modify).
- `internal/api/handlers/*_test.go` / `internal/contract/*_test.go` — test handler/authz (create/modify).
- `cmd/seed/main.go` — associazione team demo (modify).
- `frontend-next/lib/api.ts` — wrapper `projectTeams.*` (modify).
- `frontend-next/components/projects/AccessTab.tsx` — sezione Teams (modify).
- `frontend-next/components/layout/Sidebar.tsx` — attiva "Teams" (modify).
- `frontend-next/app/app/groups/page.tsx` — relabel "Teams" (modify).
- `frontend-next/e2e/teams.spec.ts` — E2E (create).
- `docs/superpowers/STATE.md`, `CHANGELOG.md`, `docs/contracts/gap-report.md` — chiusura (modify).

---

## Task 1: Migrazione + modello `ProjectTeam`

**Files:**
- Create: `migrations/000023_project_teams.up.sql`, `migrations/000023_project_teams.down.sql`
- Create: `internal/domain/project/team.go`
- Test: (verificato in Task 2; qui solo build)

**Interfaces:**
- Produces: tabella `project_teams`; tipo Go `project.ProjectTeam` (GORM model) e `project.ProjectTeamInfo{GroupID, GroupName, Role string}`.

- [ ] **Step 1: Scrivere la migrazione up**

`migrations/000023_project_teams.up.sql`:
```sql
CREATE TABLE project_teams (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    group_id   TEXT NOT NULL REFERENCES groups(id)   ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member','viewer')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, group_id)
);
CREATE INDEX idx_project_teams_group ON project_teams (group_id);
```

- [ ] **Step 2: Scrivere la migrazione down**

`migrations/000023_project_teams.down.sql`:
```sql
DROP TABLE IF EXISTS project_teams;
```

- [ ] **Step 3: Modello GORM**

`internal/domain/project/team.go`:
```go
package project

import "time"

// ProjectTeam associa un gruppo (team) a un progetto con un ruolo.
type ProjectTeam struct {
	ProjectID string     `gorm:"primaryKey;type:text" json:"project_id"`
	GroupID   string     `gorm:"primaryKey;type:text" json:"group_id"`
	Role      MemberRole `gorm:"type:text;not null;default:member" json:"role"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (ProjectTeam) TableName() string { return "project_teams" }

// ProjectTeamInfo è la proiezione con il nome del gruppo per l'API/UI.
type ProjectTeamInfo struct {
	GroupID   string     `json:"groupId"`
	GroupName string     `json:"name"`
	Role      MemberRole `json:"role"`
}
```

- [ ] **Step 4: Build**

Run: `go build ./...`
Expected: OK (nessun uso ancora, solo compilazione del nuovo file).

- [ ] **Step 5: Commit**

```bash
git add migrations/000023_project_teams.up.sql migrations/000023_project_teams.down.sql internal/domain/project/team.go
git commit -m "feat(project): project_teams migration + ProjectTeam model"
```

---

## Task 2: Dominio — AddTeam / RemoveTeam / ListTeams

**Files:**
- Modify: `internal/domain/project/service.go`
- Test: `internal/domain/project/team_test.go` (create)

**Interfaces:**
- Consumes: `ProjectTeam`, `ProjectTeamInfo`, `MemberRole` (Task 1); pattern `AddMember` esistente (upsert `ON CONFLICT`).
- Produces: `(*Service) AddTeam(projectID, groupID string, role MemberRole) error`, `RemoveTeam(projectID, groupID string) error`, `ListTeams(projectID string) ([]ProjectTeamInfo, error)`.

- [ ] **Step 1: Test falliti**

In `internal/domain/project/team_test.go`, usando l'helper DB in-memory già presente nel package (vedi gli altri `*_test.go` del package per `newTestService`/apertura SQLite + AutoMigrate). AutoMigrate deve includere `&ProjectTeam{}`, `&group.Group{}`, `&group.GroupMember{}` se non già. Test:
```go
func TestAddTeamIsIdempotentAndListTeams(t *testing.T) {
	s := newTestService(t) // helper del package
	// seed: progetto + gruppo
	// ... crea project p (s.CreateProject o insert diretto) e un group g via group.Service o insert
	if err := s.AddTeam(p.ID, g.ID, RoleMember); err != nil { t.Fatal(err) }
	if err := s.AddTeam(p.ID, g.ID, RoleAdmin); err != nil { t.Fatal(err) } // upsert cambia ruolo
	teams, err := s.ListTeams(p.ID)
	if err != nil { t.Fatal(err) }
	if len(teams) != 1 || teams[0].Role != RoleAdmin || teams[0].GroupID != g.ID {
		t.Fatalf("want 1 team admin, got %+v", teams)
	}
	if teams[0].GroupName != g.Name { t.Fatalf("want group name hydrated, got %q", teams[0].GroupName) }
	if err := s.RemoveTeam(p.ID, g.ID); err != nil { t.Fatal(err) }
	teams, _ = s.ListTeams(p.ID)
	if len(teams) != 0 { t.Fatalf("want 0 after remove, got %d", len(teams)) }
}
```

- [ ] **Step 2: Eseguire il test (fallisce)**

Run: `go test ./internal/domain/project/ -run TestAddTeamIsIdempotentAndListTeams -v`
Expected: FAIL (metodi non definiti).

- [ ] **Step 3: Implementazione**

In `service.go` (modellare su `AddMember`/`ListMembers` esistenti):
```go
// AddTeam associa (o aggiorna il ruolo di) un team su un progetto. Idempotente.
func (s *Service) AddTeam(projectID, groupID string, role MemberRole) error {
	pt := ProjectTeam{ProjectID: projectID, GroupID: groupID, Role: role}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "group_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"role"}),
	}).Create(&pt).Error
}

func (s *Service) RemoveTeam(projectID, groupID string) error {
	return s.db.Where("project_id = ? AND group_id = ?", projectID, groupID).
		Delete(&ProjectTeam{}).Error
}

// ListTeams restituisce i team associati con il nome del gruppo idratato.
func (s *Service) ListTeams(projectID string) ([]ProjectTeamInfo, error) {
	var out []ProjectTeamInfo
	err := s.db.Table("project_teams AS pt").
		Select("pt.group_id AS group_id, g.name AS group_name, pt.role AS role").
		Joins("JOIN groups g ON g.id = pt.group_id").
		Where("pt.project_id = ?", projectID).
		Order("g.name ASC").
		Scan(&out).Error
	return out, err
}
```
(Verificare l'import `gorm.io/gorm/clause`; è già usato altrove nel package? Se `AddMember` usa un pattern diverso, riusare lo STESSO pattern di `AddMember` per coerenza.)

- [ ] **Step 4: Test passa**

Run: `go test ./internal/domain/project/ -run TestAddTeamIsIdempotentAndListTeams -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/project/service.go internal/domain/project/team_test.go
git commit -m "feat(project): AddTeam/RemoveTeam/ListTeams service methods"
```

---

## Task 3: Dominio — `EffectiveRole` (più permissivo individuale vs team)

**Files:**
- Modify: `internal/domain/project/service.go`
- Test: `internal/domain/project/team_test.go`

**Interfaces:**
- Consumes: `GetRole` esistente, `group_members` (join), `MemberRole`.
- Produces: `(*Service) EffectiveRole(userID, projectID string) (MemberRole, bool)` — ruolo più permissivo tra individuale e team; `ok=false` se nessun accesso.

- [ ] **Step 1: Test falliti**

```go
func TestEffectiveRoleMostPermissive(t *testing.T) {
	s := newTestService(t)
	// p progetto; u utente; g gruppo con u dentro, associato a p come member
	// caso solo-team member:
	_ = s.AddTeam(p.ID, g.ID, RoleMember)
	if r, ok := s.EffectiveRole(u.ID, p.ID); !ok || r != RoleMember {
		t.Fatalf("solo team: want member, got %v ok=%v", r, ok)
	}
	// individuale viewer + team member -> member (più permissivo)
	_ = s.AddMember(p.ID, u.ID, RoleViewer)
	if r, ok := s.EffectiveRole(u.ID, p.ID); !ok || r != RoleMember {
		t.Fatalf("viewer+team member: want member, got %v", r)
	}
	// team admin -> admin vince
	_ = s.AddTeam(p.ID, g.ID, RoleAdmin)
	if r, _ := s.EffectiveRole(u.ID, p.ID); r != RoleAdmin {
		t.Fatalf("team admin: want admin, got %v", r)
	}
	// utente senza accesso
	if _, ok := s.EffectiveRole("nobody", p.ID); ok {
		t.Fatal("no access: want ok=false")
	}
}
```

- [ ] **Step 2: Eseguire (fallisce)**

Run: `go test ./internal/domain/project/ -run TestEffectiveRoleMostPermissive -v`
Expected: FAIL (metodo non definito).

- [ ] **Step 3: Implementazione**

```go
// rank ordina i ruoli per permissività (più alto = più permissivo).
func roleRank(r MemberRole) int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleMember:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

// EffectiveRole = ruolo più permissivo tra quello individuale (project_members)
// e quelli ereditati dai team (project_teams ⋈ group_members) dell'utente.
func (s *Service) EffectiveRole(userID, projectID string) (MemberRole, bool) {
	best := MemberRole("")
	bestRank := 0
	// individuale
	if r, ok := s.GetRole(projectID, userID); ok {
		best, bestRank = r, roleRank(r)
	}
	// team
	var teamRoles []MemberRole
	s.db.Table("project_teams AS pt").
		Select("pt.role").
		Joins("JOIN group_members gm ON gm.group_id = pt.group_id").
		Where("pt.project_id = ? AND gm.user_id = ?", projectID, userID).
		Scan(&teamRoles)
	for _, r := range teamRoles {
		if roleRank(r) > bestRank {
			best, bestRank = r, roleRank(r)
		}
	}
	if bestRank == 0 {
		return "", false
	}
	return best, true
}
```
(Verificare la firma reale di `GetRole` — dall'inventario è `GetRole(projectID, userID) (MemberRole, bool)` o simile; adattare l'ordine dei parametri a quello esistente. Nome tabella `group_members` e colonne `group_id`/`user_id` confermati in migrazione 000014.)

- [ ] **Step 4: Test passa**

Run: `go test ./internal/domain/project/ -run TestEffectiveRoleMostPermissive -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/project/service.go internal/domain/project/team_test.go
git commit -m "feat(project): EffectiveRole (most-permissive individual vs team)"
```

---

## Task 4: Dominio — membership scoping include i progetti via team

**Files:**
- Modify: `internal/domain/project/service.go` (dove è definito `MembershipSubquery` o l'equivalente usato per lo scoping letture)
- Test: `internal/domain/project/team_test.go`

**Interfaces:**
- Consumes: `MembershipSubquery` esistente (R12) — individuare la sua definizione e i consumatori (search/reference/agile handlers).
- Produces: la stessa funzione ora restituisce `project_members ∪ (project_teams ⋈ group_members)` per l'utente.

- [ ] **Step 1: Test fallito**

```go
func TestMembershipIncludesTeamProjects(t *testing.T) {
	s := newTestService(t)
	// u NON è membro individuale di p, ma è in g che è team di p
	_ = s.AddTeam(p.ID, g.ID, RoleViewer)
	ids := s.MemberProjectIDs(u.ID) // helper: esegue MembershipSubquery e ritorna gli id (aggiungere se non esiste un getter testabile)
	if !contains(ids, p.ID) {
		t.Fatalf("want project reachable via team in membership, got %v", ids)
	}
}
```
> Se `MembershipSubquery` è un `*gorm.DB` subquery non direttamente testabile, aggiungere un piccolo metodo `MemberProjectIDs(userID string) []string` che la esegue, e testare quello. Mantenere `MembershipSubquery` come fonte unica.

- [ ] **Step 2: Eseguire (fallisce)**

Run: `go test ./internal/domain/project/ -run TestMembershipIncludesTeamProjects -v`
Expected: FAIL (progetto non incluso — oggi solo project_members).

- [ ] **Step 3: Implementazione**

Estendere la subquery membership con UNION dei progetti raggiunti via team. Esempio (adattare alla forma reale esistente):
```go
// MembershipSubquery: id dei progetti di cui l'utente è membro, direttamente
// (project_members) O tramite un team (project_teams ⋈ group_members).
func (s *Service) MembershipSubquery(userID string) *gorm.DB {
	direct := s.db.Table("project_members").Select("project_id").Where("user_id = ?", userID)
	viaTeam := s.db.Table("project_teams AS pt").
		Select("pt.project_id").
		Joins("JOIN group_members gm ON gm.group_id = pt.group_id").
		Where("gm.user_id = ?", userID)
	// UNION: usare una subquery raw o gorm. Se il pattern esistente restituisce
	// un *gorm.DB da innestare con `.Where("id IN (?)", sub)`, produrre l'UNION
	// con `s.db.Raw("SELECT project_id FROM project_members WHERE user_id = ? UNION SELECT pt.project_id FROM project_teams pt JOIN group_members gm ON gm.group_id = pt.group_id WHERE gm.user_id = ?", userID, userID)`.
	_ = direct; _ = viaTeam
	return s.db.Raw(`SELECT project_id FROM project_members WHERE user_id = ?
	                 UNION
	                 SELECT pt.project_id FROM project_teams pt
	                   JOIN group_members gm ON gm.group_id = pt.group_id
	                   WHERE gm.user_id = ?`, userID, userID)
}
```
> IMPORTANTE: mantenere la firma/tipo di ritorno ESISTENTE di `MembershipSubquery` per non rompere i consumatori. Se oggi ritorna un `*gorm.DB` usato come `.Where("projects.id IN (?)", sub)`, la versione Raw sopra funziona come subquery. Verificare i call site e adeguare.

- [ ] **Step 4: Test passa + non-regressione**

Run: `go test ./internal/domain/project/... ./internal/domain/search/... -v`
Expected: PASS (il nuovo test e quelli esistenti di scoping).

- [ ] **Step 5: Commit**

```bash
git add internal/domain/project/service.go internal/domain/project/team_test.go
git commit -m "feat(project): membership scoping includes team-granted projects"
```

---

## Task 5: Authz — `RequireProject` usa il ruolo effettivo

**Files:**
- Modify: `internal/api/authz/checker.go`
- Test: `internal/api/authz/*_test.go` (o contract)

**Interfaces:**
- Consumes: `project.EffectiveRole` (Task 3).
- Produces: `RequireProject` concede il permesso se il ruolo effettivo (individuale ∪ team) lo consente.

- [ ] **Step 1: Test fallito**

Test che un utente membro solo-via-team con ruolo `member` passi `RequireProject(uid, projID, CREATE_ISSUES)` e sia negato su `DELETE_ISSUES`; un team `viewer` sia negato su `EDIT_ISSUES`. Usare l'helper di setup del package authz (vedi test R11/R12).
```go
func TestRequireProjectHonorsTeamRole(t *testing.T) {
	// setup: user u in group g; g team di p come member; u NON in project_members
	if err := chk.RequireProject(u.ID, p.ID, permission.EDIT_ISSUES); err != nil {
		t.Fatalf("team member should EDIT: %v", err)
	}
	if err := chk.RequireProject(u.ID, p.ID, permission.DELETE_ISSUES); err == nil {
		t.Fatal("team member must NOT delete")
	}
}
```

- [ ] **Step 2: Eseguire (fallisce)**

Run: `go test ./internal/api/authz/ -run TestRequireProjectHonorsTeamRole -v`
Expected: FAIL (RequireProject usa ancora solo GetRole individuale).

- [ ] **Step 3: Implementazione**

In `checker.go`, sostituire la risoluzione del ruolo dentro `RequireProject`: dove oggi chiama `s.projects.GetRole(projectID, userID)`, usare `s.projects.EffectiveRole(userID, projectID)`. Mantenere il bypass global-admin e la successiva `permission.ForRole`.

- [ ] **Step 4: Test passa + non-regressione**

Run: `go test ./internal/api/authz/... ./internal/contract/...`
Expected: PASS (inclusi i test 403 esistenti di R11/R12).

- [ ] **Step 5: Commit**

```bash
git add internal/api/authz/checker.go internal/api/authz/*_test.go
git commit -m "feat(authz): RequireProject uses effective (individual+team) role"
```

---

## Task 6: Endpoint `/project/{key}/teams` + wiring router

**Files:**
- Create: `internal/api/handlers/project_team_handler.go`
- Modify: `internal/api/router.go`
- Test: `internal/api/handlers/project_team_handler_test.go` o `internal/contract/*`

**Interfaces:**
- Consumes: `project.AddTeam/RemoveTeam/ListTeams`, `authz.Enforce(ADMINISTER_PROJECTS, ByKey)`.
- Produces: `GET/POST /project/{key}/teams`, `PUT/DELETE /project/{key}/teams/{groupId}`.

- [ ] **Step 1: Handler**

`project_team_handler.go` — modellare su `project_member_handler` esistente. Risolvere il progetto via key (helper esistente), poi:
- `List`: `svc.ListTeams(proj.ID)` → `200 [{groupId,name,role}]`.
- `Add`: body `{groupId string, role string}`; validare role ∈ {admin,member,viewer} (400 altrimenti) e che il gruppo esista; `svc.AddTeam(proj.ID, groupId, role)` → `204`/`200`.
- `UpdateRole`: `{role}` su `{groupId}` → `svc.AddTeam` (upsert) → `204`.
- `Remove`: `svc.RemoveTeam(proj.ID, groupId)` → `204`.
Usare `v3.WriteError` per gli errori, coerente con gli altri handler.

- [ ] **Step 2: Router**

In `router.go`, accanto alle rotte `/project/{key}/members`, registrare le 4 rotte gated con `Enforce(permission.ADMINISTER_PROJECTS, authz.ByKey(...))` (stesso decoratore usato dalle rotte membri/settings). Keyed su `{key}` e `{groupId}`.

- [ ] **Step 3: Test (fallito→passa)**

Contract/handler test: project admin associa un team (200/204) e lo rivede in GET; un utente `member` riceve 403 sull'associazione. Eseguire:
Run: `go test ./internal/api/handlers/... ./internal/contract/...`
Expected: PASS

- [ ] **Step 4: Build+vet**

Run: `go build ./... && go vet ./...`
Expected: OK

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/project_team_handler.go internal/api/router.go internal/api/handlers/project_team_handler_test.go
git commit -m "feat(api): project teams endpoints gated by ADMINISTER_PROJECTS"
```

---

## Task 7: Gap report

**Files:**
- Modify: `docs/contracts/gap-report.md`

- [ ] **Step 1: Rigenerare**

Run: `go run ./cmd/gapreport`

- [ ] **Step 2: Verificare no-drift residuo**

Run: `git diff --stat docs/contracts/gap-report.md` (i nuovi endpoint compaiono come extension).

- [ ] **Step 3: Commit**

```bash
git add docs/contracts/gap-report.md
git commit -m "docs(contracts): regenerate gap report (project teams endpoints)"
```

---

## Task 8: Frontend — client API + sezione Teams nel tab Access

**Files:**
- Modify: `frontend-next/lib/api.ts`
- Modify: `frontend-next/components/projects/AccessTab.tsx`

**Interfaces:**
- Consumes: nuovi endpoint (Task 6); `groups.picker` esistente per elencare i team disponibili.
- Produces: `projectTeams.list(key)`, `projectTeams.add(key, groupId, role)`, `projectTeams.updateRole(key, groupId, role)`, `projectTeams.remove(key, groupId)`.

- [ ] **Step 1: Client wrappers**

In `lib/api.ts`, aggiungere il gruppo `projectTeams` (mirror di `projects.members`):
```ts
export interface ProjectTeam { groupId: string; name: string; role: "admin" | "member" | "viewer"; }
export const projectTeams = {
  list: (key: string) => apiFetch<ProjectTeam[]>(`/rest/api/3/project/${key}/teams`),
  add: (key: string, groupId: string, role: string) =>
    apiFetch<void>(`/rest/api/3/project/${key}/teams`, { method: "POST", body: JSON.stringify({ groupId, role }) }),
  updateRole: (key: string, groupId: string, role: string) =>
    apiFetch<void>(`/rest/api/3/project/${key}/teams/${groupId}`, { method: "PUT", body: JSON.stringify({ role }) }),
  remove: (key: string, groupId: string) =>
    apiFetch<void>(`/rest/api/3/project/${key}/teams/${groupId}`, { method: "DELETE" }),
};
```
(Adeguare all'export style reale del file — se le risorse sono raggruppate in un unico oggetto, seguirlo.)

- [ ] **Step 2: UI sezione Teams**

In `AccessTab.tsx`, sotto la lista membri, aggiungere una sezione "Teams": TanStack Query `["projectTeams", key]` → `projectTeams.list`; tabella con nome team + dropdown ruolo (`admin/member/viewer`, `onChange` → `updateRole` + invalidate) + bottone rimuovi (`remove` + invalidate); riga "aggiungi": select popolata da `groups.picker("")` (escludendo i già associati) + select ruolo + bottone Aggiungi (`add` + invalidate). Riusare gli stili/pattern della lista membri già presente nel file.

- [ ] **Step 3: Build**

Run: `cd frontend-next && npm run build`
Expected: compila senza errori di tipo.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/lib/api.ts frontend-next/components/projects/AccessTab.tsx
git commit -m "feat(frontend): project Teams section in Access tab + api client"
```

---

## Task 9: Frontend — attiva "Teams" in sidebar + relabel pagina gruppi

**Files:**
- Modify: `frontend-next/components/layout/Sidebar.tsx`
- Modify: `frontend-next/app/app/groups/page.tsx`

- [ ] **Step 1: Sidebar**

In `Sidebar.tsx`, la voce `{label:"Teams", comingSoon:true}` → rimuovere `comingSoon` e renderla link a `/app/groups` (mantengo la route esistente per non rompere link; cambio solo la label visibile). Rimuovere l'eventuale voce "Groups" duplicata se presente, oppure lasciare solo "Teams" che punta a `/app/groups`.

- [ ] **Step 2: Relabel pagina**

In `app/app/groups/page.tsx`, cambiare titolo/heading e copy da "Groups" a "Teams" (l'entità sotto resta gruppo; nessun cambio di endpoint). Aggiornare eventuali testi/aria-label.

- [ ] **Step 3: Build**

Run: `cd frontend-next && npm run build`
Expected: OK

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/layout/Sidebar.tsx frontend-next/app/app/groups/page.tsx
git commit -m "feat(frontend): activate Teams in sidebar; relabel groups page as Teams"
```

---

## Task 10: Seed demo + E2E + chiusura (gate, STATE, CHANGELOG)

**Files:**
- Modify: `cmd/seed/main.go`
- Create: `frontend-next/e2e/teams.spec.ts`
- Modify: `docs/superpowers/STATE.md`, `CHANGELOG.md`

**Interfaces:**
- Consumes: tutto il precedente.

- [ ] **Step 1: Seed demo (idempotente)**

In `cmd/seed/main.go`, dopo la creazione del gruppo "developers" demo e del progetto DEMO, associare il team al progetto con ruolo `member` in modo idempotente (via `project.Service.AddTeam(demoProject.ID, devGroup.ID, project.RoleMember)`), così il percorso di scrittura `project_teams` è esercitato dal seed → coperto anche dal job CI `postgres-smoke`.

- [ ] **Step 2: E2E**

`frontend-next/e2e/teams.spec.ts` (seguire il pattern degli altri spec, helper `login()`):
1. login admin globale → `/app/groups` (Teams): crea un team, aggiungi un utente esistente.
2. vai in Project Settings → Access del progetto DEMO → sezione Teams → associa il team con ruolo `member`.
3. login come quell'utente → verifica che il progetto DEMO compaia nella lista progetti e che riesca ad aprire e modificare (Save) una sua issue (nessun 403).

- [ ] **Step 3: Gate a tre livelli**

Run:
```bash
go build ./... && go vet ./... && go test ./...
cd frontend-next && npm run build && npx playwright test
cd .. && go run ./cmd/gapreport && git diff --exit-code docs/contracts/gap-report.md
```
Expected: tutto verde, nessun drift.

- [ ] **Step 4: Docs**

- `CHANGELOG.md`: `### Added` sotto `## [Unreleased]` — Team associabili ai progetti con un ruolo; i membri del team ereditano i permessi (ruolo effettivo = più permissivo tra individuale e team); pagina Teams + sezione Teams nel tab Access; migrazione 000023.
- `docs/superpowers/STATE.md`: nuova voce round con cosa è cambiato, la tabella `project_teams`, il punto di innesto authz (`EffectiveRole`), e i gap residui (permessi configurabili / team lead / workload ancora fuori scopo).

- [ ] **Step 5: Commit**

```bash
git add cmd/seed/main.go frontend-next/e2e/teams.spec.ts docs/superpowers/STATE.md CHANGELOG.md
git commit -m "feat(teams): demo seed association, e2e, docs; close round"
```

---

## Self-Review (controllo finale del piano)

- **Copertura spec**: modello dati (T1), gestione team↔progetto (T2, T6, T8), ruolo effettivo (T3, T5), visibilità via team (T4), UI Teams (T8, T9), test/authz/contract/E2E (T3–T6, T10), seed+Postgres-smoke (T10). ✓
- **Tipi coerenti**: `MemberRole` (admin/member/viewer) usato ovunque; `EffectiveRole(userID, projectID)` firma unica; `ProjectTeamInfo{GroupID,GroupName,Role}`. ⚠️ Verificare a implementazione l'ordine parametri reale di `GetRole` e la firma/tipo di `MembershipSubquery` esistenti (annotato nei task).
- **Niente placeholder**: ogni task ha codice o istruzioni concrete con path esatti.
- **Fuori scopo** ribadito: permessi configurabili, team lead, workload, team annidati.
