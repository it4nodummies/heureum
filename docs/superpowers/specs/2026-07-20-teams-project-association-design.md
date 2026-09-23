# Team associati ai progetti — Design

> Stato: approvato (brainstorming 2026-07-20). Prossimo passo: piano d'implementazione con `writing-plans`, esecuzione Subagent-Driven.

## Obiettivo

Permettere di raggruppare utenti in **Team** e **associare un team a un progetto con un ruolo**, così che tutti i membri del team ottengano quel ruolo (e quindi i relativi permessi) sul progetto e possano lavorarne le issue. Il tutto **estendendo** il modello esistente (ruoli/permessi/gruppi già implementati), senza duplicare concetti.

## Contesto (stato attuale, verificato)

- **Ruoli di progetto**: `project_members(project_id, user_id, role)` con `role ∈ {admin, member, viewer}` (`internal/domain/project/member.go`, migrazione 000001). Il creatore del progetto è admin.
- **Permessi**: `internal/domain/permission/permission.go` — 8 chiavi, mappa `ForRole(role, isGlobalAdmin)` **hardcoded** (non configurabile). Enforcement reale in `internal/api/authz` (`Checker.RequireProject`, decoratori `Enforce`/`EnforceNotFound`/`EnforceGlobalAdmin`, resolver richiesta→progetto).
- **Gruppi**: `groups`/`group_members` (`internal/domain/group`, migrazione 000014) — rubrica di utenti **non collegata** a progetti o permessi. Endpoint Jira-compat `/rest/api/3/group*`.
- **Visibilità**: le letture project-scoped usano `MembershipSubquery` (progetti di cui l'utente è membro) per liste/ricerca; `EnforceNotFound` gate le GET.
- **Team**: inesistente. Solo etichetta "Teams SOON" in `frontend-next/components/layout/Sidebar.tsx`.

## Decisioni di design (dal brainstorming)

1. **Team = evoluzione dei Gruppi**: l'entità sotto resta `group` (API `/rest/api/3/group` invariata per compat Jira). "Team" è il concetto esteso = gruppo + associazione a progetto con ruolo. Nell'UI l'utente vede solo "Team".
2. **Team→Progetto con ruolo**: si associa un team a un progetto assegnandogli un ruolo (`admin`/`member`/`viewer`); tutti i membri ottengono quel ruolo sul progetto.
3. **UI**: la pagina `/app/groups` diventa "Teams" e si attiva lo slot in sidebar (rimosso `comingSoon`).
4. **Chi gestisce**: creare team e gestirne i membri = **admin globale** (come i gruppi oggi); associare un team esistente a un progetto con un ruolo = **admin del progetto** (`ADMINISTER_PROJECTS`).
5. **Conflitto ruoli**: se un utente ha sia un ruolo individuale (`project_members`) sia uno ereditato da un team, **vince il più permissivo** (`admin > member > viewer`).

## Architettura

### Modello dati

Nuova tabella (migrazione **000023**):

```sql
CREATE TABLE project_teams (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    group_id   TEXT NOT NULL REFERENCES groups(id)   ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member','viewer')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, group_id)
);
CREATE INDEX idx_project_teams_group ON project_teams(group_id);
```

`groups`/`group_members` restano invariati. `project_teams` è l'unica nuova struttura.

> **Lezione SQLite-vs-Postgres**: le scritture su `project_teams` usano sempre id reali (mai stringa vuota) e `created_at` è `time.Time`. Il job CI `postgres-smoke` esercita il percorso (seed che associa un team a un progetto), così un eventuale mismatch tipo/colonna o FK vuota fallisce in CI, non in produzione.

### Dominio

Estendere `internal/domain/project` (il team↔progetto è una relazione del progetto, come i membri):

- `AddTeam(projectID, groupID string, role MemberRole) error` — upsert idempotente (`ON CONFLICT (project_id, group_id) DO UPDATE role`), come `AddMember`.
- `RemoveTeam(projectID, groupID string) error`.
- `ListTeams(projectID string) ([]ProjectTeam, error)` — team associati + ruolo (con nome del gruppo).
- **Ruolo effettivo**: `EffectiveRole(userID, projectID string) (MemberRole, bool)` — restituisce il ruolo più permissivo tra quello individuale (`project_members`) e quelli ereditati dai team a cui l'utente appartiene (`project_teams ⋈ group_members`). Ordine: `admin > member > viewer`. `ok=false` se l'utente non ha alcun accesso.
- **Membership**: aggiornare `MembershipSubquery` (o l'equivalente usato per lo scoping letture) affinché includa `project_members ∪ (project_teams ⋈ group_members WHERE user_id = ?)`.

### Authz

- `Checker.RequireProject` deve usare il **ruolo effettivo** invece del solo `project.GetRole`. Punto di innesto unico: sostituire la risoluzione del ruolo con `project.EffectiveRole`. `permission.ForRole` invariato. Global admin resta bypass.
- Nessun nuovo permesso key. L'associazione team↔progetto è gated con `ADMINISTER_PROJECTS` (project admin); la gestione team/membri con `EnforceGlobalAdmin` (come i gruppi).

### Endpoint (estensioni Heureum, non contratto Jira)

- `GET  /rest/api/3/project/{key}/teams` — lista team associati + ruolo. Gate: `BROWSE_PROJECTS` (lettura) o `ADMINISTER_PROJECTS`; scegliere coerentemente con `AccessTab` (che è admin-only). → `ADMINISTER_PROJECTS`.
- `POST /rest/api/3/project/{key}/teams` — body `{groupId, role}`, associa/aggiorna. Gate: `ADMINISTER_PROJECTS`.
- `PUT  /rest/api/3/project/{key}/teams/{groupId}` — body `{role}`, cambia ruolo. Gate: `ADMINISTER_PROJECTS`.
- `DELETE /rest/api/3/project/{key}/teams/{groupId}` — dissocia. Gate: `ADMINISTER_PROJECTS`.
- Creazione team + membri: riuso `/rest/api/3/group*` esistenti (già `EnforceGlobalAdmin`).
- Tutte keyed su `{key}` (project key), risolte via `authz` resolver `ByKey` esistente. Nessuna rotta keyed su UUID interno.

### Frontend

- `frontend-next/lib/api.ts`: wrapper `projectTeams.list/add/updateRole/remove` per i nuovi endpoint; riuso `groups.*` per team/membri.
- `Sidebar.tsx`: la voce "Teams" diventa un link attivo a `/app/groups` (o rinominare la route a `/app/teams`; scelta nel piano — preferire mantenere `/app/groups` come path e cambiare solo la label, per non toccare i link esistenti, salvo decidere diversamente nel piano). Rimuovere `comingSoon`.
- `frontend-next/app/app/groups/page.tsx`: rinominare il titolo/copy in "Teams" (l'entità resta gruppo). Gestione membri invariata (admin globale).
- `frontend-next/components/projects/AccessTab.tsx`: nuova sezione **"Teams"** sotto la lista membri — elenco team associati con dropdown ruolo (`admin/member/viewer`), aggiungi (picker dei team esistenti via `groups.picker`), rimuovi. Visibile ai project admin.

## Regole & edge case

- **Più permissivo vince** su conflitto individuale/team.
- Dissociare un team (o rimuovere un utente dal team, o `DELETE` del gruppo → cascade) revoca l'accesso a chi lo aveva solo tramite quel team.
- Global admin: sempre accesso pieno (bypass), indipendente da team.
- Un utente in più team associati allo stesso progetto → ruolo più alto tra questi.
- Associare un team già associato = aggiorna il ruolo (idempotente), non errore.

## Testing

- **Domain** (`internal/domain/project/*_test.go`, SQLite in-memory):
  - `EffectiveRole`: solo individuale; solo team; entrambi → più permissivo; nessuno → `ok=false`; più team → più alto.
  - `AddTeam` idempotente aggiorna il ruolo; `RemoveTeam`; `ListTeams`.
  - `MembershipSubquery` include i progetti raggiunti via team.
- **Authz** (contract/handler test): membro solo-via-team `member` → può `EDIT_ISSUES`, 403 su `DELETE_ISSUES`; team `viewer` → sola lettura (403 su create/edit); associazione team↔progetto → 200 per project admin, 403 per member.
- **Contract**: nuovi endpoint restituiti in shape coerente; `go run ./cmd/gapreport` rigenerato (compaiono come extension).
- **Postgres-smoke**: estendere il seed/CI così che il percorso di scrittura `project_teams` (con FK reali) sia esercitato su Postgres.
- **E2E** (`frontend-next/e2e/teams.spec.ts`): admin globale crea un team e aggiunge un utente; project admin associa il team al progetto con ruolo `member`; l'utente del team fa login, vede il progetto nella lista e modifica una issue.

## Gate a tre livelli

1. `go build ./... && go vet ./... && go test ./...`
2. `cd frontend-next && npm run build && npx playwright test`
3. `go run ./cmd/gapreport` — nessun drift non committato.

## Fuori scopo (YAGNI)

Permessi/ruoli **configurabili** (permission scheme, ruoli custom), ruolo "team lead", report di **carico di lavoro** per team, team annidati/gerarchici, sincronizzazione con IdP/SSO. Ognuno eventualmente in un round successivo.
