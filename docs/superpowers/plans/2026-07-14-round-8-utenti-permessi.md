# Round 8 — Utenti & Permessi Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dare a open-jira i gruppi utente, i permessi (`/permissions` + `/mypermissions` derivati), il profilo utente e la UI delle notifiche (campanella + preferenze) collegata al backend notifiche già esistente.

**Architecture:** Nuovo dominio `internal/domain/group` (tabelle `groups`/`group_members`) con endpoint conformi (`/group`, `/group/member`, `/group/user`, `/groups/picker`). I **permessi** sono pragmatici: `GET /permissions` elenca le chiavi supportate; `GET /mypermissions` calcola `havePermission` da `user.is_admin` (globale) + `project_members.role` (admin/member/viewer già esistente) — abbastanza per il gating UI, senza un permission-scheme engine completo (rinviato). Il dominio `notification` è **già completo** (store persistito + preferenze per (utente,progetto,evento) + broadcast websocket): il frontend collega la campanella morta in TopBar e aggiunge profilo + preferenze. Il profilo estende `user` (timezone/locale) con un service e un update `PUT /myself` (estensione).

**Tech Stack:** Go 1.25 (net/http, GORM, golang-migrate, SQLite in test), domini `internal/domain/{user,group,notification,project}`, `internal/api/v3`, harness `internal/contract`. Frontend Next.js 16 + React 19 + TanStack Query + Tailwind + Playwright.

---

## Contesto per l'implementatore (leggere una volta)

**Contratto v3** (`docs/contracts/jira-platform-v3.json`) — proprietà ESATTE:
- **Group**: `name`, `groupId`, `self`, `users`(paginato), `expand`. **AddGroupBean** (POST /group): `{name}`. **UpdateUserToGroupBean** (POST /group/user): `{accountId, name}`.
- **PageBeanUserDetails** (GET /group/member): `{isLast, maxResults, startAt, total, values:[UserDetails]}`. **UserDetails**: `accountId, accountType, active, avatarUrls, displayName, emailAddress, self, timeZone`.
- **FoundGroups** (GET /groups/picker): `{groups:[FoundGroup{name, groupId, html?}], header, total}`.
- **Permissions** (GET /permissions e /mypermissions): `{permissions: <object map> key→UserPermission}` (mappa, NON array). **UserPermission**: `id, key, name, description, type, havePermission`.
- **User** (GET /myself, /user): `accountId, accountType, active, avatarUrls, displayName, emailAddress, self, timeZone, locale`.

**Codice ESISTENTE (verificato):**
- `internal/domain/user/model.go`: `User{ID, Email, Username, DisplayName, AvatarURL, PasswordHash(json:"-"), IsAdmin bool, IsActive bool}`. **Nessun timezone/locale, nessun service** (la logica HTTP è in `user_handler.go` con gorm raw).
- `internal/api/handlers/user_handler.go`: `UserHandler{DB *gorm.DB, BaseURL string}` con `GetMe`, `GetMyself` (→ `v3.JiraUser(u, h.BaseURL)`), `SearchUsers`, `GetUser`. Rotte: `GET /rest/api/3/users/me`, `/myself`, `/users/search`, `/user`.
- `internal/api/v3`: `JiraUser(u user.User, baseURL string) User` (v3.User); `WriteJSON`, `WriteError`, `WritePage[T]`, `ParsePagination`. VERIFICARE se `v3.User` ha `TimeZone`/`Locale` (aggiungerli se assenti, con `omitempty`).
- `internal/domain/project`: `ProjectMember{ProjectID, UserID, Role MemberRole}` (enum `admin`/`member`/`viewer`, tabella `project_members`); `project.Service.ListMembers(projectID)`, `AddMember`, `GetByKey`, `GetBySeqID`. Serve per `/mypermissions`.
- `internal/domain/notification` (**COMPLETO, riusare**): `Notification{ID, UserID, Type, Title, Body, Link, IsRead bool, CreatedAt int64}` (tabella `notifications`); `NotificationSetting{UserID, ProjectID, EventType, ViaEmail, ViaApp}` (tabella `notification_settings`). Handler `NotificationHandler`: `List` (→ `[]Notification` json), `MarkRead`, `MarkAllRead`, `UnreadCount` (→ `{count:int64}`), `GetSettings` (→ `[]NotificationSetting`), `UpdateSettings`. Rotte già montate: `GET /rest/api/3/notifications`, `GET .../notifications/unread-count`, `PATCH .../notifications/read-all`, `PATCH .../notifications/{id}/read`, `GET/PATCH .../notifications/settings`.
- `internal/api/middleware/auth.go`: `middleware.UserIDFromContext(ctx) string`. **Nessun middleware di autorizzazione** (tutte le rotte autenticate sono aperte a ogni utente loggato).
- Frontend: `apiFetch<T>(path, opts)`; `buildQuery`. `components/layout/TopBar.tsx` ha un bottone campanella **statico non collegato** (nessun onClick/fetch). Nessuna pagina profilo/notifiche.

**Migrazioni:** ultima `000013`. Prossima **`000014`**.

**Scope escluso (follow-up, dichiarato):** ruoli progetto configurabili + role actor (`/project/{key}/role/{id}` con actor); **permission scheme + grant + enforcement** su tutte le rotte (ora `/mypermissions` è derivato, non applicato lato server); notification scheme (`/notificationscheme`); **email SMTP reale** (il worker `cmd/worker` stub-loga); **Redis** (config presente, non usato); global roles (`/role`).

**Harness contract:** `newTestServer`, `registerAndLogin`, `createProjectViaAPI`, `createIssueViaAPI`, `MustLoad`, `doJSON`, `decodeBody` (vedi `search_test.go`/`agile_test.go`). `registerAndLogin` crea un utente e ritorna il JWT; per i test servono più utenti → potrebbe servire un helper `registerUser(t, authSvc, email)` (aggiungere se assente, riusando `authSvc.Register`).

---

## Struttura dei file

**Migrazioni:** `migrations/000014_groups_and_profile.up.sql` / `.down.sql`.

**Backend:**
- `internal/domain/group/model.go` + `service.go` (+ test).
- `internal/domain/user/service.go` (nuovo: Get/Update/Search/Assignable) (+ test).
- `internal/domain/permission/permission.go` (chiavi + derivazione) (+ test).
- `internal/api/v3/group.go` (mapper Group/UserDetails/FoundGroups) + `permissions.go` (UserPermission/Permissions).
- `internal/api/handlers/group_handler.go`, `permission_handler.go`; estendere `user_handler.go` (search v3 + assignable + `PUT /myself`).
- `internal/api/router.go` — rotte gruppi/permessi/assignable/myself-update.
- `internal/contract/users_perms_test.go`.

**Frontend:**
- `frontend-next/lib/api.ts` — client `notifications`, `profile`, `permissions`, `groups`.
- `frontend-next/components/notifications/NotificationBell.tsx` + montaggio in `TopBar.tsx`.
- `frontend-next/app/jira/profile/page.tsx` (profilo + preferenze notifiche).
- `frontend-next/e2e/users.spec.ts`.

**Seed:** `cmd/seed/main.go` — un gruppo demo + una notifica demo per admin.

---

### Task 1: Migrazione 000014 — gruppi + campi profilo

**Files:**
- Create: `migrations/000014_groups_and_profile.up.sql`
- Create: `migrations/000014_groups_and_profile.down.sql`

- [ ] **Step 1: Migrazione up**

`migrations/000014_groups_and_profile.up.sql`:

```sql
CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE group_members (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);

ALTER TABLE users ADD COLUMN time_zone TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN locale TEXT DEFAULT '';
```

- [ ] **Step 2: Migrazione down**

`migrations/000014_groups_and_profile.down.sql`:

```sql
ALTER TABLE users DROP COLUMN locale;
ALTER TABLE users DROP COLUMN time_zone;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
```

- [ ] **Step 3: Verificare a pulito**

Run: `rm -f /tmp/mig14.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/mig14.db go run ./cmd/seed && rm -f /tmp/mig14.db`
Expected: `seed complete`, exit 0.

- [ ] **Step 4: Commit**

```bash
git add migrations/000014_groups_and_profile.up.sql migrations/000014_groups_and_profile.down.sql
git commit -m "feat(migrations): groups, group_members, user time_zone/locale"
```

---

### Task 2: Dominio group

**Files:**
- Create: `internal/domain/group/model.go`
- Create: `internal/domain/group/service.go`
- Test: `internal/domain/group/service_test.go`

- [ ] **Step 1: Modello**

`internal/domain/group/model.go`:

```go
package group

import "time"

type Group struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	Name      string    `gorm:"type:text;not null;uniqueIndex" json:"name"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type GroupMember struct {
	GroupID string `gorm:"primaryKey;type:text" json:"group_id"`
	UserID  string `gorm:"primaryKey;type:text" json:"user_id"`
}

func (GroupMember) TableName() string { return "group_members" }
```

- [ ] **Step 2: Test**

`internal/domain/group/service_test.go`:

```go
package group

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&Group{}, &GroupMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateAndFind(t *testing.T) {
	svc := NewService(newDB(t))
	g, err := svc.Create("developers")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if g.ID == "" || g.Name != "developers" {
		t.Errorf("gruppo errato: %+v", g)
	}
	got, err := svc.FindByName("developers")
	if err != nil {
		t.Fatalf("FindByName: %v", err)
	}
	if got.ID != g.ID {
		t.Error("id mismatch")
	}
	if _, err := svc.Create("developers"); err == nil {
		t.Error("atteso errore per nome duplicato")
	}
}

func TestMembers(t *testing.T) {
	svc := NewService(newDB(t))
	g, _ := svc.Create("qa")
	if err := svc.AddUser(g.ID, "user-1"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	svc.AddUser(g.ID, "user-2")
	svc.AddUser(g.ID, "user-1") // idempotente: nessun errore/duplicato
	ids, total, err := svc.MemberIDs(g.ID, 0, 50)
	if err != nil {
		t.Fatalf("MemberIDs: %v", err)
	}
	if total != 2 || len(ids) != 2 {
		t.Errorf("attesi 2 membri, total=%d len=%d", total, len(ids))
	}
	if err := svc.RemoveUser(g.ID, "user-1"); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
	_, total, _ = svc.MemberIDs(g.ID, 0, 50)
	if total != 1 {
		t.Errorf("atteso 1 membro dopo rimozione, %d", total)
	}
}

func TestSearchAndDelete(t *testing.T) {
	svc := NewService(newDB(t))
	svc.Create("developers")
	svc.Create("designers")
	svc.Create("qa")
	found, err := svc.Search("des", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(found) != 1 || found[0].Name != "designers" {
		t.Errorf("search errata: %+v", found)
	}
	g, _ := svc.FindByName("qa")
	if err := svc.Delete(g.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.FindByName("qa"); err == nil {
		t.Error("gruppo dovrebbe essere eliminato")
	}
}
```

- [ ] **Step 3: Eseguire (falliscono)**

Run: `go test ./internal/domain/group/ -v`
Expected: FAIL con "undefined: NewService".

- [ ] **Step 4: Service**

`internal/domain/group/service.go`:

```go
package group

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) Create(name string) (*Group, error) {
	g := &Group{ID: uuid.NewString(), Name: name}
	if err := s.db.Create(g).Error; err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Service) FindByName(name string) (*Group, error) {
	var g Group
	if err := s.db.Where("name = ?", name).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Service) Delete(id string) error {
	return s.db.Where("id = ?", id).Delete(&Group{}).Error
}

// AddUser è idempotente (ON CONFLICT DO NOTHING via clause).
func (s *Service) AddUser(groupID, userID string) error {
	return s.db.
		Where("group_id = ? AND user_id = ?", groupID, userID).
		FirstOrCreate(&GroupMember{GroupID: groupID, UserID: userID}).Error
}

func (s *Service) RemoveUser(groupID, userID string) error {
	return s.db.Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&GroupMember{}).Error
}

// MemberIDs restituisce gli id utente membri del gruppo (paginati) + il totale.
func (s *Service) MemberIDs(groupID string, offset, limit int) ([]string, int, error) {
	var total int64
	if err := s.db.Model(&GroupMember{}).Where("group_id = ?", groupID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var members []GroupMember
	q := s.db.Where("group_id = ?", groupID).Order("user_id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&members).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	return ids, int(total), nil
}

// Search trova gruppi il cui nome contiene la query (case-insensitive).
func (s *Service) Search(query string, limit int) ([]Group, error) {
	var groups []Group
	q := s.db.Where("LOWER(name) LIKE LOWER(?)", "%"+query+"%").Order("name ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}
```

- [ ] **Step 5: Eseguire (passano)**

Run: `go test ./internal/domain/group/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/group/
git commit -m "feat(group): group domain model and service"
```

---

### Task 3: Handler gruppi + mapper v3 + rotte

**Files:**
- Create: `internal/api/v3/group.go`
- Create: `internal/api/handlers/group_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Mapper v3**

`internal/api/v3/group.go`:

```go
package v3

import "fmt"

// GroupRef è lo shape conforme di un gruppo Jira.
type GroupRef struct {
	Name    string `json:"name"`
	GroupID string `json:"groupId"`
	Self    string `json:"self"`
}

func JiraGroup(id, name, baseURL string) GroupRef {
	return GroupRef{Name: name, GroupID: id, Self: fmt.Sprintf("%s/rest/api/3/group?groupId=%s", baseURL, id)}
}

// FoundGroup / FoundGroups per /groups/picker.
type FoundGroup struct {
	Name    string `json:"name"`
	GroupID string `json:"groupId"`
}
type FoundGroups struct {
	Header string       `json:"header"`
	Total  int          `json:"total"`
	Groups []FoundGroup `json:"groups"`
}
```

> **Nota:** `UserDetails` per `/group/member` = la stessa shape di `v3.User` (o un sottoinsieme). Riusare `v3.JiraUser(u, baseURL)` per gli elementi e impacchettarli in `v3.Page[User]` (WritePage emette `startAt/maxResults/total/isLast/values`) — che combacia con `PageBeanUserDetails`.

- [ ] **Step 2: Handler**

`internal/api/handlers/group_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/group"
	"github.com/open-jira/open-jira/internal/domain/user"
	"gorm.io/gorm"
)

type GroupHandler struct {
	svc     *group.Service
	db      *gorm.DB
	baseURL string
}

func NewGroupHandler(svc *group.Service, db *gorm.DB, baseURL string) *GroupHandler {
	return &GroupHandler{svc: svc, db: db, baseURL: baseURL}
}

// Get: GET /rest/api/3/group?groupname=... → GroupRef.
func (h *GroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("groupname")
	g, err := h.svc.FindByName(name)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraGroup(g.ID, g.Name, h.baseURL))
}

// Create: POST /rest/api/3/group {name} → 201 GroupRef.
func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"name is required"}, nil)
		return
	}
	g, err := h.svc.Create(req.Name)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"failed to create group (duplicate?)"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, v3.JiraGroup(g.ID, g.Name, h.baseURL))
}

// Delete: DELETE /rest/api/3/group?groupname=... → 200.
func (h *GroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.FindByName(r.URL.Query().Get("groupname"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	if err := h.svc.Delete(g.ID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete group"}, nil)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Members: GET /rest/api/3/group/member?groupname=... → PageBeanUserDetails.
func (h *GroupHandler) Members(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.FindByName(r.URL.Query().Get("groupname"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	ids, total, err := h.svc.MemberIDs(g.ID, startAt, maxResults)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list members"}, nil)
		return
	}
	values := make([]v3.User, 0, len(ids))
	for _, id := range ids {
		var u user.User
		if h.db.First(&u, "id = ?", id).Error == nil {
			values = append(values, v3.JiraUser(u, h.baseURL))
		}
	}
	v3.WritePage(w, http.StatusOK, v3.Page[v3.User]{StartAt: startAt, MaxResults: maxResults, Total: total, Values: values})
}

// AddUser: POST /rest/api/3/group/user?groupname=... {accountId} → 201 GroupRef.
func (h *GroupHandler) AddUser(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.FindByName(r.URL.Query().Get("groupname"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	var req struct {
		AccountID string `json:"accountId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AccountID == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"accountId is required"}, nil)
		return
	}
	if err := h.svc.AddUser(g.ID, req.AccountID); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to add user"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, v3.JiraGroup(g.ID, g.Name, h.baseURL))
}

// RemoveUser: DELETE /rest/api/3/group/user?groupname=...&accountId=... → 200.
func (h *GroupHandler) RemoveUser(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.FindByName(r.URL.Query().Get("groupname"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"group not found"}, nil)
		return
	}
	if err := h.svc.RemoveUser(g.ID, r.URL.Query().Get("accountId")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to remove user"}, nil)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Picker: GET /rest/api/3/groups/picker?query=... → FoundGroups.
func (h *GroupHandler) Picker(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("query")
	groups, err := h.svc.Search(q, 20)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to search groups"}, nil)
		return
	}
	out := v3.FoundGroups{Header: "Showing groups", Total: len(groups), Groups: make([]v3.FoundGroup, 0, len(groups))}
	for _, g := range groups {
		out.Groups = append(out.Groups, v3.FoundGroup{Name: g.Name, GroupID: g.ID})
	}
	out.Header = fmt.Sprintf("Showing %d of %d matching groups", len(groups), len(groups))
	v3.WriteJSON(w, http.StatusOK, out)
}
```

> **Nota implementatore:** aggiungere import `fmt`. VERIFICARE che `v3.Page[T]`/`v3.WritePage` e `v3.User`/`v3.JiraUser(u, baseURL)` esistano con queste firme (dai round precedenti). L'`accountId` nel nostro sistema è l'`user.ID` (UUID) — coerente con come `JiraUser` espone `accountId`.

- [ ] **Step 3: Rotte**

In `internal/api/router.go` (costruzione + blocco rotte):

```go
	groupSvc := group.NewService(db)
	groupH := handlers.NewGroupHandler(groupSvc, db, cfg.BaseURL)
```
```go
	// --- Gruppi (Round 8) ---
	mux.Handle("GET /rest/api/3/group", authMw(http.HandlerFunc(groupH.Get)))
	mux.Handle("POST /rest/api/3/group", authMw(http.HandlerFunc(groupH.Create)))
	mux.Handle("DELETE /rest/api/3/group", authMw(http.HandlerFunc(groupH.Delete)))
	mux.Handle("GET /rest/api/3/group/member", authMw(http.HandlerFunc(groupH.Members)))
	mux.Handle("POST /rest/api/3/group/user", authMw(http.HandlerFunc(groupH.AddUser)))
	mux.Handle("DELETE /rest/api/3/group/user", authMw(http.HandlerFunc(groupH.RemoveUser)))
	mux.Handle("GET /rest/api/3/groups/picker", authMw(http.HandlerFunc(groupH.Picker)))
```
Aggiungere import `"github.com/open-jira/open-jira/internal/domain/group"`.

> **Nota ServeMux:** `GET /group` e `GET /group/member` sono pattern distinti (segmento in più) → nessun conflitto. `/groups/picker` è un path separato.

- [ ] **Step 4: Build + vet**

Run: `go build ./... && go vet ./internal/api/...`
Expected: compila, vet pulito.

- [ ] **Step 5: Commit**

```bash
git add internal/api/v3/group.go internal/api/handlers/group_handler.go internal/api/router.go
git commit -m "feat(api): group endpoints (CRUD, membership, picker)"
```

---

### Task 4: Permessi — /permissions + /mypermissions

**Files:**
- Create: `internal/domain/permission/permission.go`
- Create: `internal/api/handlers/permission_handler.go`
- Modify: `internal/api/router.go`
- Test: `internal/domain/permission/permission_test.go`

- [ ] **Step 1: Test dominio**

`internal/domain/permission/permission_test.go`:

```go
package permission

import "testing"

func TestForRole(t *testing.T) {
	admin := ForRole("admin", false)
	if !admin["ADMINISTER_PROJECTS"] || !admin["CREATE_ISSUES"] || !admin["BROWSE_PROJECTS"] {
		t.Errorf("admin deve avere tutti i permessi progetto: %v", admin)
	}
	member := ForRole("member", false)
	if !member["CREATE_ISSUES"] || member["ADMINISTER_PROJECTS"] {
		t.Errorf("member: create sì, administer no: %v", member)
	}
	viewer := ForRole("viewer", false)
	if !viewer["BROWSE_PROJECTS"] || viewer["CREATE_ISSUES"] {
		t.Errorf("viewer: solo browse: %v", viewer)
	}
	// admin globale (is_admin) → tutto, anche senza ruolo progetto
	glob := ForRole("", true)
	if !glob["ADMINISTER"] || !glob["ADMINISTER_PROJECTS"] {
		t.Errorf("global admin deve avere tutto: %v", glob)
	}
}

func TestAllKeys(t *testing.T) {
	if len(All()) < 5 {
		t.Error("attese >=5 chiavi permesso")
	}
	for _, p := range All() {
		if p.Key == "" || p.Name == "" {
			t.Errorf("permesso senza key/name: %+v", p)
		}
	}
}
```

- [ ] **Step 2: Eseguire (falliscono)**

Run: `go test ./internal/domain/permission/ -v`
Expected: FAIL con "undefined: ForRole".

- [ ] **Step 3: Implementare**

`internal/domain/permission/permission.go`:

```go
// Package permission definisce le chiavi di permesso supportate e la loro
// derivazione dal ruolo di progetto (project_members.role) e dal flag globale
// is_admin. NB: è un modello PRAGMATICO per il gating UI — non un permission
// scheme configurabile (rinviato a un round successivo).
package permission

// Key è una chiave di permesso stile Jira.
type Def struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

var defs = []Def{
	{"ADMINISTER", "Administer Jira", "Global administration", "GLOBAL"},
	{"ADMINISTER_PROJECTS", "Administer Projects", "Manage project settings/workflow", "PROJECT"},
	{"BROWSE_PROJECTS", "Browse Projects", "View project and issues", "PROJECT"},
	{"CREATE_ISSUES", "Create Issues", "Create issues in the project", "PROJECT"},
	{"EDIT_ISSUES", "Edit Issues", "Edit issues in the project", "PROJECT"},
	{"TRANSITION_ISSUES", "Transition Issues", "Move issues through workflow", "PROJECT"},
	{"DELETE_ISSUES", "Delete Issues", "Delete issues", "PROJECT"},
	{"MANAGE_SPRINTS", "Manage Sprints", "Create/start/complete sprints", "PROJECT"},
}

// All restituisce tutte le definizioni di permesso.
func All() []Def { return defs }

// ForRole calcola l'insieme dei permessi per un ruolo di progetto
// (admin/member/viewer, stringa vuota = nessun ruolo) + flag is_admin globale.
func ForRole(role string, isGlobalAdmin bool) map[string]bool {
	out := map[string]bool{}
	set := func(keys ...string) {
		for _, k := range keys {
			out[k] = true
		}
	}
	if isGlobalAdmin {
		for _, d := range defs {
			out[d.Key] = true
		}
		return out
	}
	switch role {
	case "admin":
		set("ADMINISTER_PROJECTS", "BROWSE_PROJECTS", "CREATE_ISSUES", "EDIT_ISSUES", "TRANSITION_ISSUES", "DELETE_ISSUES", "MANAGE_SPRINTS")
	case "member":
		set("BROWSE_PROJECTS", "CREATE_ISSUES", "EDIT_ISSUES", "TRANSITION_ISSUES", "MANAGE_SPRINTS")
	case "viewer":
		set("BROWSE_PROJECTS")
	}
	return out
}
```

- [ ] **Step 4: Handler + rotte**

`internal/api/handlers/permission_handler.go`:

```go
package handlers

import (
	"net/http"

	"github.com/open-jira/open-jira/internal/api/middleware"
	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/permission"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
	"gorm.io/gorm"
)

type PermissionHandler struct {
	db         *gorm.DB
	projectSvc *project.Service
}

func NewPermissionHandler(db *gorm.DB, projectSvc *project.Service) *PermissionHandler {
	return &PermissionHandler{db: db, projectSvc: projectSvc}
}

// userPermission è lo shape UserPermission del contratto.
type userPermission struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Type           string `json:"type"`
	HavePermission bool   `json:"havePermission,omitempty"`
}

// Permissions: GET /rest/api/3/permissions → tutte le chiavi (senza havePermission).
func (h *PermissionHandler) Permissions(w http.ResponseWriter, r *http.Request) {
	perms := map[string]userPermission{}
	for i, d := range permission.All() {
		perms[d.Key] = userPermission{ID: itoa(i + 1), Key: d.Key, Name: d.Name, Description: d.Description, Type: d.Type}
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

// MyPermissions: GET /rest/api/3/mypermissions?projectKey=... → havePermission
// derivato da is_admin globale + ruolo nel progetto (se projectKey presente).
func (h *PermissionHandler) MyPermissions(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	var u user.User
	isAdmin := false
	if h.db.First(&u, "id = ?", uid).Error == nil {
		isAdmin = u.IsAdmin
	}
	role := ""
	if key := r.URL.Query().Get("projectKey"); key != "" {
		if p, err := h.projectSvc.GetByKey(key); err == nil {
			var m struct{ Role string }
			h.db.Table("project_members").Select("role").Where("project_id = ? AND user_id = ?", p.ID, uid).Scan(&m)
			role = m.Role
		}
	}
	have := permission.ForRole(role, isAdmin)
	perms := map[string]userPermission{}
	for i, d := range permission.All() {
		perms[d.Key] = userPermission{ID: itoa(i + 1), Key: d.Key, Name: d.Name, Description: d.Description, Type: d.Type, HavePermission: have[d.Key]}
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}
```

In `internal/api/router.go`:

```go
	permH := handlers.NewPermissionHandler(db, projectSvc)
	mux.Handle("GET /rest/api/3/permissions", authMw(http.HandlerFunc(permH.Permissions)))
	mux.Handle("GET /rest/api/3/mypermissions", authMw(http.HandlerFunc(permH.MyPermissions)))
```

> **Nota implementatore:** `itoa` — usare `strconv.Itoa` (import `strconv`) invece dell'helper immaginario; adeguare. Verificare il nome della variabile `projectSvc` in router.go. `project.Service.GetByKey` esiste. La colonna ruolo in `project_members` è `role`.

- [ ] **Step 5: Eseguire test + build**

Run: `go test ./internal/domain/permission/ -v && go build ./... && go vet ./...`
Expected: PASS + build/vet OK.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/permission/ internal/api/handlers/permission_handler.go internal/api/router.go
git commit -m "feat(api): /permissions and derived /mypermissions"
```

---

### Task 5: Utenti — service, search v3, assignable, profilo

**Files:**
- Create: `internal/domain/user/service.go`
- Modify: `internal/api/handlers/user_handler.go` (search v3 + assignable + PUT /myself)
- Modify: `internal/api/v3` (aggiungere TimeZone/Locale a v3.User se assenti)
- Modify: `internal/api/router.go`
- Test: `internal/domain/user/service_test.go`

- [ ] **Step 1: Verificare v3.User + aggiungere TimeZone/Locale**

Leggere il mapper `v3.JiraUser`/`v3.User` (`internal/api/v3/user.go` o reference.go). Se `v3.User` non ha `TimeZone`/`Locale`, aggiungerli:
```go
	TimeZone string `json:"timeZone,omitempty"`
	Locale   string `json:"locale,omitempty"`
```
e popolarli in `JiraUser` da `u.TimeZone`/`u.Locale` (aggiungere questi campi al modello `user.User`: `TimeZone string` col `time_zone`, `Locale string` col `locale`).

- [ ] **Step 2: Test service**

`internal/domain/user/service_test.go`:

```go
package user

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func mk(db *gorm.DB, id, email, name string) {
	db.Create(&User{ID: id, Email: email, Username: id, DisplayName: name, IsActive: true})
}

func TestSearch(t *testing.T) {
	db := newDB(t)
	mk(db, "u1", "ada@x.io", "Ada Admin")
	mk(db, "u2", "dev@x.io", "Devi Dev")
	svc := NewService(db)
	res, err := svc.Search("ada", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 || res[0].ID != "u1" {
		t.Errorf("search per nome errata: %+v", res)
	}
	res, _ = svc.Search("x.io", 10)
	if len(res) != 2 {
		t.Errorf("search per email deve trovare 2, %d", len(res))
	}
}

func TestUpdateProfile(t *testing.T) {
	db := newDB(t)
	mk(db, "u1", "ada@x.io", "Ada")
	svc := NewService(db)
	dn, tz := "Ada Lovelace", "Europe/Rome"
	u, err := svc.UpdateProfile("u1", &dn, &tz, nil, nil)
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if u.DisplayName != "Ada Lovelace" || u.TimeZone != "Europe/Rome" {
		t.Errorf("profilo non aggiornato: %+v", u)
	}
}
```

- [ ] **Step 3: Eseguire (falliscono)**

Run: `go test ./internal/domain/user/ -v`
Expected: FAIL (NewService/Search/UpdateProfile undefined, o TimeZone field mancante).

- [ ] **Step 4: Service**

`internal/domain/user/service.go`:

```go
package user

import "gorm.io/gorm"

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) DB() *gorm.DB { return s.db }

func (s *Service) GetByID(id string) (*User, error) {
	var u User
	if err := s.db.First(&u, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// Search cerca utenti attivi per displayName o email (case-insensitive).
func (s *Service) Search(query string, limit int) ([]User, error) {
	var users []User
	like := "%" + query + "%"
	q := s.db.Where("is_active = ? AND (LOWER(display_name) LIKE LOWER(?) OR LOWER(email) LIKE LOWER(?))", true, like, like).Order("display_name ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// AssignableForProject: utenti membri del progetto (assegnabili).
func (s *Service) AssignableForProject(projectID string, query string, limit int) ([]User, error) {
	var users []User
	q := s.db.
		Joins("JOIN project_members pm ON pm.user_id = users.id").
		Where("pm.project_id = ? AND users.is_active = ?", projectID, true)
	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(users.display_name) LIKE LOWER(?) OR LOWER(users.email) LIKE LOWER(?)", like, like)
	}
	q = q.Order("users.display_name ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateProfile aggiorna i campi profilo (nil = invariato).
func (s *Service) UpdateProfile(id string, displayName, timeZone, locale, avatarURL *string) (*User, error) {
	updates := map[string]any{}
	if displayName != nil {
		updates["display_name"] = *displayName
	}
	if timeZone != nil {
		updates["time_zone"] = *timeZone
	}
	if locale != nil {
		updates["locale"] = *locale
	}
	if avatarURL != nil {
		updates["avatar_url"] = *avatarURL
	}
	if len(updates) > 0 {
		if err := s.db.Model(&User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetByID(id)
}
```

Aggiungere al modello `user.User` (se non presenti): `TimeZone string \`gorm:"column:time_zone;default:''" json:"time_zone"\`` e `Locale string \`gorm:"column:locale;default:''" json:"locale"\``.

- [ ] **Step 5: Handler — search v3, assignable, PUT /myself**

In `internal/api/handlers/user_handler.go` aggiungere (l'`UserHandler` ha `DB`/`BaseURL`; aggiungere un `Svc *user.Service` o costruirlo al volo con `user.NewService(h.DB)`):

```go
// SearchV3: GET /rest/api/3/user/search?query=... → []v3.User (shape contratto).
func (h *UserHandler) SearchV3(w http.ResponseWriter, r *http.Request) {
	svc := user.NewService(h.DB)
	users, err := svc.Search(r.URL.Query().Get("query"), 50)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"search failed"}, nil)
		return
	}
	out := make([]v3.User, 0, len(users))
	for _, u := range users {
		out = append(out, v3.JiraUser(u, h.BaseURL))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// AssignableSearch: GET /rest/api/3/user/assignable/search?project=KEY&query=...
func (h *UserHandler) AssignableSearch(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("project")
	var p project.Project
	if h.DB.First(&p, "key = ?", key).Error != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	svc := user.NewService(h.DB)
	users, err := svc.AssignableForProject(p.ID, r.URL.Query().Get("query"), 50)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"search failed"}, nil)
		return
	}
	out := make([]v3.User, 0, len(users))
	for _, u := range users {
		out = append(out, v3.JiraUser(u, h.BaseURL))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// UpdateMyself: PUT /rest/api/3/myself {displayName?, timeZone?, locale?, avatarUrl?}
// (estensione: Jira non ha PUT /myself, ma serve per l'editing profilo).
func (h *UserHandler) UpdateMyself(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	var req struct {
		DisplayName *string `json:"displayName"`
		TimeZone    *string `json:"timeZone"`
		Locale      *string `json:"locale"`
		AvatarURL   *string `json:"avatarUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	svc := user.NewService(h.DB)
	u, err := svc.UpdateProfile(uid, req.DisplayName, req.TimeZone, req.Locale, req.AvatarURL)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to update profile"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraUser(*u, h.BaseURL))
}
```

In `internal/api/router.go`:

```go
	mux.Handle("GET /rest/api/3/user/search", authMw(http.HandlerFunc(userH.SearchV3)))
	mux.Handle("GET /rest/api/3/user/assignable/search", authMw(http.HandlerFunc(userH.AssignableSearch)))
	mux.Handle("PUT /rest/api/3/myself", authMw(http.HandlerFunc(userH.UpdateMyself)))
```

> **Nota implementatore:** verificare gli import in user_handler.go (aggiungere `project`, `user`, `middleware`, `encoding/json`, `v3` se assenti). Confermare `project.Project` ha `Key`/`ID`. Il route esistente `GET /rest/api/3/users/search` resta; il nuovo conforme è `/user/search` (singolare, come il contratto). Non rompere `GetMyself`/`GetMe`.

- [ ] **Step 6: Eseguire test + build**

Run: `go test ./internal/domain/user/ -v && go build ./... && go vet ./...`
Expected: PASS + build/vet OK.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/user/ internal/api/handlers/user_handler.go internal/api/v3/ internal/api/router.go
git commit -m "feat(api): user service, v3 user/search, assignable, profile update"
```

---

### Task 6: Contract test — gruppi, permessi, utenti

**Files:**
- Create: `internal/contract/users_perms_test.go`

- [ ] **Step 1: Scrivere i contract test**

`internal/contract/users_perms_test.go` (usare gli helper reali dell'harness — leggere `search_test.go`/`agile_test.go`; adattare):

```go
package contract

import (
	"net/http"
	"testing"
)

func TestGroups_CRUDConformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")

	// create
	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/group", map[string]any{"name": "developers"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create group %d", resp.StatusCode)
	}
	v.ValidateResponse(http.MethodPost, "/rest/api/3/group", http.StatusCreated, resp.Header, bodyOf(resp))
	// get
	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/group?groupname=developers", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get group %d", resp.StatusCode)
	}
	// members (empty page)
	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/group/member?groupname=developers", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("members %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if _, ok := body["values"]; !ok {
		t.Error("member page deve avere values")
	}
	// picker
	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/groups/picker?query=dev", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("picker %d", resp.StatusCode)
	}
}

func TestPermissions_Conformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")

	resp := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/permissions", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("permissions %d", resp.StatusCode)
	}
	v.ValidateResponse(http.MethodGet, "/rest/api/3/permissions", http.StatusOK, resp.Header, bodyOf(resp))

	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/mypermissions", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mypermissions %d", resp.StatusCode)
	}
	v.ValidateResponse(http.MethodGet, "/rest/api/3/mypermissions", http.StatusOK, resp.Header, bodyOf(resp))
	body := decodeBody(t, resp)
	perms, ok := body["permissions"].(map[string]any)
	if !ok || len(perms) == 0 {
		t.Error("mypermissions deve avere una mappa permissions non vuota")
	}
}

func TestUserSearch_Conformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")

	resp := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/user/search?query=a", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("user search %d", resp.StatusCode)
	}
	v.ValidateResponse(http.MethodGet, "/rest/api/3/user/search", http.StatusOK, resp.Header, bodyOf(resp))
}
```

> **Nota implementatore:** `bodyOf(resp)` è indicativo — usare l'helper reale (in `agile_test.go`/`search_test.go` la validazione avviene passando il body a `ValidateResponse`; se `decodeBody` restituisce anche i raw bytes, ricreare un reader con `strings.NewReader(string(raw))` come già fanno gli altri test). Adattare tutte le chiamate agli helper reali dell'harness. Se `registerAndLogin` non basta per popolare utenti cercabili, va bene: `user/search?query=a` può tornare l'utente registrato (il cui displayName contiene 'a') o lista vuota — l'importante è lo status 200 + conformità di shape. `/permissions` e `/mypermissions`: se lo schema `Permissions` del contratto è `object map` (additionalProperties), `ValidateResponse` accetta la nostra mappa.

- [ ] **Step 2: Eseguire i contract test**

Run: `go test ./internal/contract/ -run 'TestGroups|TestPermissions|TestUserSearch' -v`
Expected: PASS. Se una validazione fallisce, correggere il mapper.

- [ ] **Step 3: Suite completa**

Run: `go test ./...`
Expected: verde.

- [ ] **Step 4: Commit**

```bash
git add internal/contract/users_perms_test.go
git commit -m "test(contract): groups, permissions, user search conformance"
```

---

### Task 7: Frontend — client notifiche/profilo/permessi/gruppi

**Files:**
- Modify: `frontend-next/lib/api.ts`

- [ ] **Step 1: Aggiungere i client**

In `frontend-next/lib/api.ts` (adattare `apiFetch`, verificare gli shape reali):

```ts
export interface AppNotification {
  id: string;
  type: string;
  title: string;
  body: string;
  link: string;
  is_read: boolean;
  created_at: number;
}
export interface NotificationSetting {
  user_id: string;
  project_id: string;
  event_type: string;
  via_email: boolean;
  via_app: boolean;
}

export const notifications = {
  list: () => apiFetch<AppNotification[]>("/rest/api/3/notifications"),
  unreadCount: () => apiFetch<{ count: number }>("/rest/api/3/notifications/unread-count"),
  markRead: (id: string) => apiFetch<void>(`/rest/api/3/notifications/${id}/read`, { method: "PATCH" }),
  markAllRead: () => apiFetch<void>("/rest/api/3/notifications/read-all", { method: "PATCH" }),
  settings: () => apiFetch<NotificationSetting[]>("/rest/api/3/notifications/settings"),
  updateSettings: (s: Partial<NotificationSetting>) =>
    apiFetch<void>("/rest/api/3/notifications/settings", { method: "PATCH", body: JSON.stringify(s) }),
};

export interface JiraUser {
  accountId: string;
  displayName: string;
  emailAddress: string;
  timeZone?: string;
  locale?: string;
  avatarUrls?: Record<string, string>;
}

export const profile = {
  me: () => apiFetch<JiraUser>("/rest/api/3/myself"),
  update: (patch: { displayName?: string; timeZone?: string; locale?: string; avatarUrl?: string }) =>
    apiFetch<JiraUser>("/rest/api/3/myself", { method: "PUT", body: JSON.stringify(patch) }),
  searchUsers: (query: string) => apiFetch<JiraUser[]>(`/rest/api/3/user/search?query=${encodeURIComponent(query)}`),
};

export const permissions = {
  mine: (projectKey?: string) =>
    apiFetch<{ permissions: Record<string, { key: string; name: string; havePermission: boolean }> }>(
      `/rest/api/3/mypermissions${projectKey ? `?projectKey=${projectKey}` : ""}`,
    ),
};

export const groups = {
  picker: (query: string) =>
    apiFetch<{ groups: { name: string; groupId: string }[] }>(`/rest/api/3/groups/picker?query=${encodeURIComponent(query)}`),
  create: (name: string) => apiFetch<{ name: string; groupId: string }>("/rest/api/3/group", { method: "POST", body: JSON.stringify({ name }) }),
};
```

> **Nota:** verificare che `UpdateSettings` accetti il body così com'è (leggere `notification_handler.go` `UpdateSettings`). Confermare shape `/myself` = `JiraUser` (v3.User). Confermare `apiFetch`.

- [ ] **Step 2: Type-check**

Run: `cd frontend-next && npx tsc --noEmit`
Expected: nessun errore.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): notifications, profile, permissions, groups API client"
```

---

### Task 8: Frontend — campanella notifiche in TopBar

**Files:**
- Create: `frontend-next/components/notifications/NotificationBell.tsx`
- Modify: `frontend-next/components/layout/TopBar.tsx`

- [ ] **Step 1: Componente campanella**

`frontend-next/components/notifications/NotificationBell.tsx`:

```tsx
"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { notifications, type AppNotification } from "@/lib/api";

export function NotificationBell() {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);

  const count = useQuery({
    queryKey: ["notif", "count"],
    queryFn: notifications.unreadCount,
    refetchInterval: 30000, // polling ogni 30s (nessun push nativo lato bell)
  });
  const list = useQuery({ queryKey: ["notif", "list"], queryFn: notifications.list, enabled: open });

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ["notif", "count"] });
    qc.invalidateQueries({ queryKey: ["notif", "list"] });
  };
  const markRead = useMutation({ mutationFn: (id: string) => notifications.markRead(id), onSuccess: invalidate });
  const markAll = useMutation({ mutationFn: () => notifications.markAllRead(), onSuccess: invalidate });

  const unread = count.data?.count ?? 0;

  return (
    <div className="relative">
      <button
        aria-label="Notifications"
        onClick={() => setOpen((o) => !o)}
        className="relative rounded p-2 text-slate-500 hover:bg-slate-100"
      >
        <span aria-hidden>🔔</span>
        {unread > 0 && (
          <span data-testid="notif-badge" className="absolute -right-0.5 -top-0.5 rounded-full bg-[#de350b] px-1 text-[10px] font-semibold text-white">
            {unread}
          </span>
        )}
      </button>
      {open && (
        <div className="absolute right-0 z-20 mt-2 w-80 rounded border border-slate-200 bg-white shadow-lg" data-testid="notif-dropdown">
          <div className="flex items-center justify-between border-b px-3 py-2">
            <span className="text-sm font-semibold text-[#1a1f36]">Notifications</span>
            <button onClick={() => markAll.mutate()} className="text-xs text-[#0052cc] hover:underline">Mark all read</button>
          </div>
          <ul className="max-h-96 overflow-auto">
            {(list.data ?? []).map((n: AppNotification) => (
              <li key={n.id} className={`border-b border-slate-100 px-3 py-2 text-sm ${n.is_read ? "opacity-60" : ""}`}>
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <div className="font-medium text-[#1a1f36]">{n.title}</div>
                    {n.body && <div className="text-xs text-slate-500">{n.body}</div>}
                  </div>
                  {!n.is_read && (
                    <button onClick={() => markRead.mutate(n.id)} aria-label={`Mark ${n.title} read`} className="text-xs text-[#0052cc] hover:underline">
                      Read
                    </button>
                  )}
                </div>
              </li>
            ))}
            {list.data && list.data.length === 0 && <li className="px-3 py-4 text-sm text-slate-400">No notifications</li>}
          </ul>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Montare in TopBar**

In `frontend-next/components/layout/TopBar.tsx`, sostituire il bottone campanella statico con `<NotificationBell />` (import `@/components/notifications/NotificationBell`). Mantenere gli altri elementi (help, settings, avatar).

> **Nota:** leggere TopBar.tsx per il markup esatto del bottone campanella (righe ~74-79 secondo lo scout) e rimpiazzarlo. Se TopBar è server component, `NotificationBell` è un client island che va bene inserito direttamente.

- [ ] **Step 3: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/notifications/NotificationBell.tsx frontend-next/components/layout/TopBar.tsx
git commit -m "feat(frontend): notification bell wired to notifications API"
```

---

### Task 9: Frontend — pagina profilo + preferenze notifiche

**Files:**
- Create: `frontend-next/app/jira/profile/page.tsx`

- [ ] **Step 1: Pagina profilo**

`frontend-next/app/jira/profile/page.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { profile, notifications, type NotificationSetting } from "@/lib/api";

export default function ProfilePage() {
  const qc = useQueryClient();
  const me = useQuery({ queryKey: ["profile", "me"], queryFn: profile.me });
  const [displayName, setDisplayName] = useState("");
  const [timeZone, setTimeZone] = useState("");

  useEffect(() => {
    if (me.data) {
      setDisplayName(me.data.displayName ?? "");
      setTimeZone(me.data.timeZone ?? "");
    }
  }, [me.data]);

  const save = useMutation({
    mutationFn: () => profile.update({ displayName, timeZone }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["profile", "me"] }),
  });

  const settings = useQuery({ queryKey: ["notif", "settings"], queryFn: notifications.settings });
  const updateSetting = useMutation({
    mutationFn: (s: Partial<NotificationSetting>) => notifications.updateSettings(s),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notif", "settings"] }),
  });

  return (
    <div className="mx-auto max-w-2xl p-6">
      <h1 className="mb-4 text-xl font-semibold text-[#1a1f36]">Profile</h1>

      <section className="mb-6 rounded border border-slate-200 bg-white p-4">
        <label className="mb-1 block text-xs font-semibold text-slate-500">Display name</label>
        <input aria-label="Display name" value={displayName} onChange={(e) => setDisplayName(e.target.value)} className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm" />
        <label className="mb-1 block text-xs font-semibold text-slate-500">Time zone</label>
        <input aria-label="Time zone" value={timeZone} onChange={(e) => setTimeZone(e.target.value)} placeholder="Europe/Rome" className="mb-3 w-full rounded border border-slate-300 px-3 py-1.5 text-sm" />
        <button onClick={() => save.mutate()} disabled={save.isPending} className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60">Save profile</button>
        <p className="mt-2 text-xs text-slate-500">{me.data?.emailAddress}</p>
      </section>

      <section className="rounded border border-slate-200 bg-white p-4" data-testid="notif-prefs">
        <h2 className="mb-2 text-sm font-semibold text-[#1a1f36]">Notification preferences</h2>
        <ul className="space-y-1 text-sm">
          {(settings.data ?? []).map((s) => (
            <li key={`${s.project_id}:${s.event_type}`} className="flex items-center justify-between border-b border-slate-100 py-1">
              <span className="text-[#1a1f36]">{s.event_type}{s.project_id ? ` · ${s.project_id}` : ""}</span>
              <span className="flex gap-3">
                <label className="flex items-center gap-1 text-xs"><input type="checkbox" checked={s.via_app} onChange={(e) => updateSetting.mutate({ ...s, via_app: e.target.checked })} /> app</label>
                <label className="flex items-center gap-1 text-xs"><input type="checkbox" checked={s.via_email} onChange={(e) => updateSetting.mutate({ ...s, via_email: e.target.checked })} /> email</label>
              </span>
            </li>
          ))}
          {settings.data && settings.data.length === 0 && <li className="py-2 text-slate-400">Default preferences (all channels on)</li>}
        </ul>
      </section>
    </div>
  );
}
```

> **Nota implementatore:** verificare che `notifications.updateSettings` mandi la chiave (user_id/project_id/event_type + via_app/via_email) attesa da `UpdateSettings` handler (leggerlo). Se l'handler richiede tutti i campi identificativi, spread `...s` li include. Aggiungere un link "Profile" al menu avatar in TopBar è opzionale (l'E2E naviga direttamente).

- [ ] **Step 2: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK; route `/jira/profile` generata.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/app/jira/profile/
git commit -m "feat(frontend): profile page with notification preferences"
```

---

### Task 10: E2E — campanella + profilo

**Files:**
- Create: `frontend-next/e2e/users.spec.ts`

- [ ] **Step 1: Scrivere l'E2E**

`frontend-next/e2e/users.spec.ts` (login helper reale da `board.spec.ts`):

```ts
import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira/);
}

test("notification bell opens dropdown and shows seeded notification", async ({ page }) => {
  await login(page);
  await page.getByRole("button", { name: "Notifications" }).click();
  await expect(page.getByTestId("notif-dropdown")).toBeVisible();
  // il seed crea una notifica demo per admin
  await expect(page.getByText(/Notifications/).first()).toBeVisible();
});

test("profile page loads and saves display name", async ({ page }) => {
  await login(page);
  await page.goto("/jira/profile");
  await expect(page.getByRole("heading", { name: "Profile" })).toBeVisible();
  await page.getByLabel("Display name").fill("Ada Lovelace");
  await page.getByRole("button", { name: "Save profile" }).click();
  // ricarica: il nome persiste
  await page.reload();
  await expect(page.getByLabel("Display name")).toHaveValue("Ada Lovelace");
});
```

> **Nota:** la prima prova apre il dropdown e verifica che compaia; se il seed non crea notifiche demo (Task 11 lo fa), il dropdown mostra comunque l'header "Notifications" (asserzione robusta). Adeguare i selettori.

- [ ] **Step 2: Eseguire l'E2E**

Run: `cd frontend-next && npx playwright test e2e/users.spec.ts --reporter=line`
Expected: PASS.

- [ ] **Step 3: Suite completa (kill server residui prima)**

Run: `lsof -ti:8080 -ti:3000 | xargs kill 2>/dev/null; sleep 1; cd frontend-next && npx playwright test --reporter=list`
Expected: tutti verdi (login, projects, issues, collaboration, search, board, workflow, reports, users). Pulire `test-results/`/`playwright-report/`.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/e2e/users.spec.ts
git commit -m "test(e2e): notification bell and profile save"
```

---

### Task 11: Seed gruppo + notifica demo + gap report

**Files:**
- Modify: `cmd/seed/main.go`
- Modify: `docs/contracts/gap-report.md` (rigenerato)

- [ ] **Step 1: Seed idempotente**

In `cmd/seed/main.go`, dopo il seed della dashboard, aggiungere (VERIFICARE le firme reali `group.NewService`/`Create`/`AddUser` e `notification.Service.Create`):
- un gruppo demo `"developers"` con l'admin come membro (check per nome);
- una notifica demo per l'admin (check per user_id+title) così la campanella mostra qualcosa.

```go
	grpSvc := group.NewService(s.DB)
	var existingG group.Group
	if err := s.DB.Where("name = ?", "developers").First(&existingG).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		g, _ := grpSvc.Create("developers")
		grpSvc.AddUser(g.ID, admin.ID)
		fmt.Println("created demo group")
	}

	notifSvc := notification.NewService(s.DB)
	var existingN int64
	s.DB.Model(&notification.Notification{}).Where("user_id = ? AND title = ?", admin.ID, "Welcome to Open Jira").Count(&existingN)
	if existingN == 0 {
		notifSvc.Create(admin.ID, "welcome", "Welcome to Open Jira", "Your demo workspace is ready", "/jira/projects")
		fmt.Println("created demo notification")
	}
```

> **Nota implementatore:** VERIFICARE la firma di `notification.NewService` (potrebbe richiedere un broadcaster — passare `nil` o usare il costruttore reale) e di `Create` (args esatti: userID, type, title, body, link?). Leggere `internal/domain/notification/service.go`. `group.NewService(db)` da Task 2. Import `group` e `notification`.

- [ ] **Step 2: Verificare idempotenza**

Run: `rm -f /tmp/s8.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/s8.db go run ./cmd/seed && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/s8.db go run ./cmd/seed && rm -f /tmp/s8.db`
Expected: prima run stampa "created demo group"/"created demo notification"; seconda no; entrambe exit 0.

- [ ] **Step 3: Gap report**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md`
Expected: nuovi endpoint conformi (`/group`, `/group/member`, `/group/user`, `/groups/picker`, `/permissions`, `/mypermissions`, `/user/search`, `/user/assignable/search`). Riportare old→new count.

- [ ] **Step 4: Commit**

```bash
git add cmd/seed/main.go docs/contracts/gap-report.md
git commit -m "feat(seed): demo group and notification; regenerate gap report for Round 8"
```

---

### Task 12: Gate finale + STATE.md → Round 9

**Files:**
- Modify: `docs/superpowers/STATE.md`

- [ ] **Step 1: Gate a tre livelli**

Run:
```bash
go build ./... && echo BUILD_OK
go vet ./... && echo VET_OK
go test ./... 2>&1 | grep -vE '^ok|no test files'
cd frontend-next && npx tsc --noEmit && npm run build && npx playwright test --reporter=list; cd ..
```
Expected: BUILD_OK, VET_OK, nessun FAIL Go, frontend build OK, tutti gli E2E verdi.

- [ ] **Step 2: Gap report senza drift**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md`
Expected: nessun drift inatteso.

- [ ] **Step 3: Aggiornare STATE.md**

In `docs/superpowers/STATE.md`:
- aggiungere alla sezione "Round completati" la riga del **Round 8 — Utenti & permessi** (gruppi conformi `/group`+`/group/member`+`/group/user`+`/groups/picker`; `/permissions` + `/mypermissions` derivati da is_admin + ruolo project_members; user service + `/user/search`/`/user/assignable/search` conformi + `PUT /myself` profilo; UI: campanella notifiche collegata al backend esistente + pagina profilo/preferenze; migrazione 000014);
- cambiare "Prossimo" in **Round 9 — Integrazioni** (webhook in uscita, integrazione Git — `GitProvider` Forgejo/GitLab/GitHub, automation base);
- aggiornare data e conteggio gap;
- aggiungere ai follow-up: ruoli progetto configurabili + role actor; **permission scheme + grant + enforcement middleware** (ora `/mypermissions` è informativo, non applicato lato server → le rotte restano aperte a ogni utente loggato); notification scheme; **email SMTP reale** + coda Redis (worker oggi stub); global roles (`/role`).

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/STATE.md docs/contracts/gap-report.md
git commit -m "docs: mark Round 8 (Users & Permissions) complete, Round 9 (Integrations) next"
```

---

## Note di chiusura round

- **Follow-up:** ruoli progetto configurabili (`/project/{key}/role/{id}` con actor user/group) sostituendo l'enum fisso admin/member/viewer; **permission scheme + grant + enforcement**: middleware che applica i permessi sulle rotte (403) — oggi `/mypermissions` è solo informativo, la sicurezza reale è ancora "ogni utente loggato può tutto"; notification scheme per progetto; **email SMTP** reale nel worker (ora logga) + coda Redis; global roles `/role`; avatar upload (ora `avatarUrl` è un campo testo).
- **Rischi noti:** `/mypermissions` derivato NON è enforcement — un client malevolo può ancora chiamare le API mutanti; l'enforcement è il vero lavoro di sicurezza rinviato. I gruppi non sono ancora usati da nessun permesso (foundational per il permission scheme futuro). Il round chiude solo con i tre livelli verdi.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura spec (roadmap Round 8):**
- Gruppi → Task 2/3 (dominio + endpoint conformi). ✅
- Ruoli progetto → **parziale/rinviato**: l'enum `project_members.role` esistente alimenta `/mypermissions`; ruoli configurabili con actor sono follow-up. ⚠️ (dichiarato)
- Permission scheme → **rinviato** (enforcement); `/permissions`+`/mypermissions` derivati coprono il gating UI (Task 4). ⚠️ (dichiarato)
- Profilo utente → Task 5 (service + `PUT /myself`) + Task 9 (UI). ✅
- Notifiche in-app → Task 8 (campanella su backend esistente). ✅
- Notifiche email → **rinviato** (SMTP/worker stub). ⚠️ (dichiarato)
- Preferenze notifica → Task 9 (UI su backend esistente). ✅
- Gate: contract Task 6, E2E Task 10, gate Task 12. ✅

**2. Placeholder scan:** codice completo per il nuovo backend (group/permission/user service + handler) e frontend (bell, profilo). Le "Note implementatore" indicano le verifiche su firme reali (v3.User TimeZone/Locale, notification.Create args, apiFetch, helper harness, UpdateSettings shape, itoa→strconv) — non placeholder di logica.

**3. Consistenza tipi:** `group.Service` (Create/FindByName/Delete/AddUser/RemoveUser/MemberIDs/Search) usato in Task 3 + seed Task 11. `permission.All()/ForRole(role, isAdmin)` (Task 4) usato nell'handler permessi. `user.Service` (Get/Search/AssignableForProject/UpdateProfile) (Task 5) usato negli handler user. `v3.JiraGroup`/`FoundGroups`/`GroupRef` (Task 3); `v3.Page[v3.User]`/`WritePage`; `v3.User`+TimeZone/Locale (Task 5). Frontend `notifications`/`profile`/`permissions`/`groups` client (Task 7) usati in Task 8/9/10. La campanella usa `notifications.unreadCount/list/markRead/markAllRead`; il profilo usa `profile.me/update` + `notifications.settings/updateSettings`. Shape notifica (`is_read`, `created_at` int64) coerente col modello di dominio.
