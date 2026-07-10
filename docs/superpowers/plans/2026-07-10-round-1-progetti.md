# Round 1 — Progetti: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Portare la risorsa Project alla conformità drop-in con la Jira Cloud REST API v3 (mapping camelCase certificato dal contratto ufficiale) — CRUD, search paginata, tipi, categorie, archivio/restore, avatar — con la UI di lista/creazione/impostazioni allineata a Jira.

**Architecture:** Si riusa il layer `internal/api/v3` (WriteJSON/WriteError/WritePage generico, ParsePagination/Expand/Fields) e il pattern di mapping `JiraUser`: si introduce `v3.JiraProject` che traduce il modello interno `project.Project` (snake_case) nello schema ufficiale `Project` (camelCase). Il modello interno resta invariato nella sostanza ma guadagna i campi che Jira espone (categoria, assignee type, private/simplified/style). Ogni endpoint conforme ottiene un contract test contro `docs/contracts/jira-platform-v3.json`, esattamente come `/myself`.

**Tech Stack:** Go 1.25, GORM, golang-migrate, kin-openapi (contract), Next.js 16 + TanStack Query + Playwright.

**Contesto codebase (per chi non lo conosce):**
- Mapping v3 di riferimento: `internal/api/v3/user.go` (`JiraUser(u user.User, baseURL string) v3.User`). Replicare lo stile per i progetti.
- Helper risposta: `v3.WriteJSON(w, status, v)`, `v3.WriteError(w, status, []string{msg}, fieldErrors)`, `v3.WritePage[T any](w, status, v3.Page[T]{StartAt,MaxResults,Total,Values})`; parsing: `v3.ParsePagination(r, def, cap)`, `v3.ParseExpand(r)`, `v3.ParseFields(r)`.
- Contract harness: `contract.MustLoad(tb, "../../docs/contracts/jira-platform-v3.json")` → `(*Validator).ValidateResponse(method, path string, status int, header http.Header, body io.Reader) error`. Helper `newTestServer(t)` in `internal/contract/myself_test.go` (package `contract`) avvia il router reale su SQLite temporaneo con migrazioni e ritorna `(*httptest.Server, *auth.Service)`.
- Modello progetto: `internal/domain/project/model.go` — `Project{ID, OrgID, Name, Key, Description, Type(scrum|kanban|business), LeadUserID, DefaultAssignee, IconURL, IsArchived, CreatedAt, UpdatedAt}`, `ProjectFavorite`, `LeadInfo`, `ProjectWithLead`, `ListFilter`.
- Service progetto: `internal/domain/project/service.go` — `NewService(db, lead *user.User)`, `Create(name,key,description string, pType Type)`, `GetByKey`, `GetByID`, `ListWithFilters(f ListFilter, userID) ([]ProjectWithLead, int64, error)`, `Update(key, name, description)`, `Archive(key)`, `Star/Unstar`, `DB()`.
- Handler: `internal/api/handlers/project_handler.go` — `NewProjectHandler(svc *project.Service, wfSvc *workflow.Service)`, metodi `Create/Get/List/Update/Delete/...`. **Non importa ancora `internal/api/v3`** — va aggiunto.
- Router: `internal/api/router.go` righe ~100-110 per le rotte project. `projectSvc := project.NewService(db, nil)`; `projectH := handlers.NewProjectHandler(projectSvc, wfSvc)`.
- Migrazioni: ultima è `000004`; le nuove partono da `000005`. Esiste già una tabella `project_categories(id, name, description)` (da `000002`) ma **senza modello Go né FK dai progetti**.
- Avatar di default: pattern `serveDefaultAvatar` + rotta `GET /static/default-avatar.svg` in `router.go` (dal Round 0). Per i progetti si aggiunge un avatar di default analogo.
- Utente per il lead: `internal/domain/user/model.go` (`user.User{ID,Email,Username,DisplayName,AvatarURL,IsActive,...}`), mapping `v3.JiraUser`.
- Commit: conventional commits. Non fare push. Working tree condiviso: se `git` dà "index.lock", riprovare, mai cancellare il lock.

**Decisione di mapping (chiave del round):**
- `projectTypeKey`: `scrum`→`software`, `kanban`→`software`, `business`→`business`. (`service_desk` non usato in questo round.)
- `style`: `classic` di default (company-managed). `simplified`: `false` di default (team-managed = true in futuro).
- Distinzione Scrum/Kanban → esposta via `projectTemplateKey` derivato: scrum→`com.pyxis.greenhopper.jira:gh-scrum-template`, kanban→`com.pyxis.greenhopper.jira:gh-kanban-template`, business→`com.atlassian.jira-core-project-templates:jira-core-simplified-process-control`. Il campo interno `Type` resta la fonte di verità e non cambia semantica.
- `assigneeType`: interno `default_assignee` `"unassigned"`→`"UNASSIGNED"`, `"project_lead"`→`"PROJECT_LEAD"`.
- In `CreateProjectDetails` (POST): `projectTypeKey`+`projectTemplateKey` mappano indietro al `Type` interno; `key`+`name` obbligatori; `leadAccountId`, `categoryId`, `assigneeType`, `description`, `url` opzionali.

---

## File Structure

- `migrations/000005_project_v3_fields.up.sql` / `.down.sql` — aggiunge a `projects`: `category_id TEXT NULL`, `assignee_type TEXT DEFAULT 'UNASSIGNED'`, `is_private BOOLEAN DEFAULT FALSE`, `simplified BOOLEAN DEFAULT FALSE`, `style TEXT DEFAULT 'classic'`, `url TEXT DEFAULT ''`.
- `internal/domain/project/model.go` — nuovi campi sul `Project`; nuovo `ProjectCategory{ID,Name,Description}` con `TableName()`.
- `internal/domain/project/category.go` + `category_test.go` — CRUD categorie.
- `internal/domain/project/service.go` — `Create` esteso (accetta struct input), `CreateProject(in CreateInput)`, `Restore(key)`, `GetCategory`.
- `internal/api/v3/project.go` + `project_test.go` — `JiraProject`, `JiraProjectType`, mapping type↔template, `PageOfProjects`.
- `internal/api/handlers/project_handler.go` — riscrittura Create/Get/List in forma v3; nuovi `Search`, `Archive`, `Restore`, `ProjectTypes`, `ProjectTypeByKey`.
- `internal/api/handlers/projectcategory_handler.go` + `projectcategory_handler_test.go` — GET/POST `/rest/api/3/projectCategory`.
- `internal/api/router.go` — nuove rotte + avatar progetto di default.
- `internal/contract/project_test.go` — contract test per ogni endpoint conforme.
- `frontend-next/lib/api.ts` — tipo `Project` v3 (camelCase) + chiamate.
- `frontend-next/components/projects/ProjectsPage.tsx`, `CreateProjectModal.tsx` — adattamento ai campi v3.
- `frontend-next/app/jira/projects/[key]/settings/page.tsx` + `frontend-next/components/projects/ProjectSettings.tsx` — pagina impostazioni.
- `frontend-next/e2e/projects.spec.ts` — E2E creazione/lista/settings.

---

### Task 1: Migrazione — campi v3 sui progetti

**Files:**
- Create: `migrations/000005_project_v3_fields.up.sql`, `migrations/000005_project_v3_fields.down.sql`

- [ ] **Step 1: Scrivi la up migration**

```sql
-- migrations/000005_project_v3_fields.up.sql
ALTER TABLE projects ADD COLUMN category_id TEXT;
ALTER TABLE projects ADD COLUMN assignee_type TEXT NOT NULL DEFAULT 'UNASSIGNED';
ALTER TABLE projects ADD COLUMN is_private BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE projects ADD COLUMN simplified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE projects ADD COLUMN style TEXT NOT NULL DEFAULT 'classic';
ALTER TABLE projects ADD COLUMN url TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 2: Scrivi la down migration**

```sql
-- migrations/000005_project_v3_fields.down.sql
ALTER TABLE projects DROP COLUMN category_id;
ALTER TABLE projects DROP COLUMN assignee_type;
ALTER TABLE projects DROP COLUMN is_private;
ALTER TABLE projects DROP COLUMN simplified;
ALTER TABLE projects DROP COLUMN style;
ALTER TABLE projects DROP COLUMN url;
```

Nota dialetti: SQLite supporta `ADD COLUMN` (una per statement, già così). Verifica che le migrazioni 000001-000004 usino lo stesso stile un-comando-per-statement e mantieni la coerenza. `DROP COLUMN` è supportato da SQLite ≥ 3.35 (il driver `mattn/go-sqlite3` incluso lo copre), da Postgres e da MariaDB 10.6+.

- [ ] **Step 3: Verifica che le migrazioni applichino su SQLite pulito**

Run:
```bash
APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/mig-test.db go run ./cmd/seed && sqlite3 /tmp/mig-test.db 'PRAGMA table_info(projects);' && rm -f /tmp/mig-test.db
```
Expected: la tabella `projects` elenca le nuove colonne `category_id, assignee_type, is_private, simplified, style, url`; exit 0.

- [ ] **Step 4: Commit**

```bash
git add migrations/000005_project_v3_fields.*
git commit -m "feat(project): migration adds v3 fields (category, assignee type, style, private)"
```

---

### Task 2: Modello — campi v3 e ProjectCategory

**Files:**
- Modify: `internal/domain/project/model.go`
- Test: `internal/domain/project/model_test.go` (create)

- [ ] **Step 1: Scrivi il test fallente del mapping template↔type**

```go
// internal/domain/project/model_test.go
package project

import "testing"

func TestTemplateKeyForType(t *testing.T) {
	cases := map[Type]string{
		TypeScrum:    "com.pyxis.greenhopper.jira:gh-scrum-template",
		TypeKanban:   "com.pyxis.greenhopper.jira:gh-kanban-template",
		TypeBusiness: "com.atlassian.jira-core-project-templates:jira-core-simplified-process-control",
	}
	for typ, want := range cases {
		if got := TemplateKeyForType(typ); got != want {
			t.Errorf("TemplateKeyForType(%q) = %q, want %q", typ, got, want)
		}
	}
}

func TestTypeForTemplateKey(t *testing.T) {
	if TypeForTemplateKey("com.pyxis.greenhopper.jira:gh-kanban-template") != TypeKanban {
		t.Error("kanban template must map to TypeKanban")
	}
	// template sconosciuto → default scrum
	if TypeForTemplateKey("unknown") != TypeScrum {
		t.Error("unknown template must default to TypeScrum")
	}
}

func TestProjectTypeKeyForType(t *testing.T) {
	if ProjectTypeKeyForType(TypeScrum) != "software" || ProjectTypeKeyForType(TypeKanban) != "software" {
		t.Error("scrum/kanban must map to software")
	}
	if ProjectTypeKeyForType(TypeBusiness) != "business" {
		t.Error("business must map to business")
	}
}
```

- [ ] **Step 2: Verifica che fallisca**

Run: `go test ./internal/domain/project/ -run 'TestTemplateKeyForType|TestTypeForTemplateKey|TestProjectTypeKeyForType'`
Expected: FAIL — funzioni non definite.

- [ ] **Step 3: Aggiungi campi e helper al modello**

In `internal/domain/project/model.go`, aggiungere i campi al `Project` (dopo `IconURL`):

```go
	CategoryID   *string `gorm:"type:text" json:"category_id,omitempty"`
	AssigneeType string  `gorm:"type:text;not null;default:'UNASSIGNED'" json:"assignee_type"`
	IsPrivate    bool    `gorm:"not null;default:false" json:"is_private"`
	Simplified   bool    `gorm:"not null;default:false" json:"simplified"`
	Style        string  `gorm:"type:text;not null;default:'classic'" json:"style"`
	URL          string  `gorm:"type:text;not null;default:''" json:"url"`
```

E in fondo al file:

```go
// ProjectCategory rispecchia la tabella project_categories (migrazione 000002).
type ProjectCategory struct {
	ID          string `gorm:"primaryKey;type:text" json:"id"`
	Name        string `gorm:"type:text;not null" json:"name"`
	Description string `gorm:"type:text;default:''" json:"description"`
}

func (ProjectCategory) TableName() string { return "project_categories" }

// Mapping fra il tipo interno (scrum/kanban/business) e i valori Jira v3.
func TemplateKeyForType(t Type) string {
	switch t {
	case TypeKanban:
		return "com.pyxis.greenhopper.jira:gh-kanban-template"
	case TypeBusiness:
		return "com.atlassian.jira-core-project-templates:jira-core-simplified-process-control"
	default:
		return "com.pyxis.greenhopper.jira:gh-scrum-template"
	}
}

func TypeForTemplateKey(k string) Type {
	switch k {
	case "com.pyxis.greenhopper.jira:gh-kanban-template":
		return TypeKanban
	case "com.atlassian.jira-core-project-templates:jira-core-simplified-process-control":
		return TypeBusiness
	default:
		return TypeScrum
	}
}

func ProjectTypeKeyForType(t Type) string {
	if t == TypeBusiness {
		return "business"
	}
	return "software"
}
```

- [ ] **Step 4: Verifica che passi**

Run: `go test ./internal/domain/project/ -run 'TestTemplateKeyForType|TestTypeForTemplateKey|TestProjectTypeKeyForType'`
Expected: PASS

- [ ] **Step 5: Assicura la retro-compatibilità dei test esistenti**

Run: `go test ./internal/domain/project/ -count=1`
Expected: PASS (i test esistenti non devono rompersi; i nuovi campi hanno default).

- [ ] **Step 6: Commit**

```bash
git add internal/domain/project/model.go internal/domain/project/model_test.go
git commit -m "feat(project): add v3 model fields and type/template mapping helpers"
```

---

### Task 3: Mapping v3 — JiraProject e JiraProjectType

**Files:**
- Create: `internal/api/v3/project.go`, `internal/api/v3/project_test.go`

- [ ] **Step 1: Scrivi i test fallenti**

```go
// internal/api/v3/project_test.go
package v3

import (
	"testing"

	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
)

func TestJiraProject_Basic(t *testing.T) {
	lead := &user.User{ID: "u1", DisplayName: "Ada", Email: "ada@x.io", IsActive: true}
	p := project.Project{
		ID: "p1", Key: "DEMO", Name: "Demo", Description: "d",
		Type: project.TypeScrum, AssigneeType: "PROJECT_LEAD", Style: "classic",
	}
	jp := JiraProject(p, lead, nil, "http://h")

	if jp.ID != "p1" || jp.Key != "DEMO" || jp.Name != "Demo" {
		t.Fatalf("basic fields wrong: %+v", jp)
	}
	if jp.Self != "http://h/rest/api/3/project/p1" {
		t.Errorf("self = %q", jp.Self)
	}
	if jp.ProjectTypeKey != "software" {
		t.Errorf("projectTypeKey = %q, want software", jp.ProjectTypeKey)
	}
	if jp.Style != "classic" {
		t.Errorf("style = %q", jp.Style)
	}
	if jp.AssigneeType != "PROJECT_LEAD" {
		t.Errorf("assigneeType = %q", jp.AssigneeType)
	}
	if jp.Lead == nil || jp.Lead.AccountID != "u1" {
		t.Errorf("lead not mapped: %+v", jp.Lead)
	}
	// avatarUrls sempre presenti con le 4 taglie
	for _, s := range []string{"16x16", "24x24", "32x32", "48x48"} {
		if jp.AvatarUrls[s] == "" {
			t.Errorf("missing avatar size %s", s)
		}
	}
}

func TestJiraProject_WithCategory(t *testing.T) {
	p := project.Project{ID: "p2", Key: "K", Name: "N", Type: project.TypeBusiness}
	cat := &project.ProjectCategory{ID: "c1", Name: "Ops", Description: "operations"}
	jp := JiraProject(p, nil, cat, "http://h")
	if jp.ProjectTypeKey != "business" {
		t.Errorf("projectTypeKey = %q, want business", jp.ProjectTypeKey)
	}
	if jp.ProjectCategory == nil || jp.ProjectCategory.ID != "c1" || jp.ProjectCategory.Name != "Ops" {
		t.Errorf("category not mapped: %+v", jp.ProjectCategory)
	}
	if jp.Lead != nil {
		t.Error("lead should be nil when not provided")
	}
}

func TestJiraProjectType(t *testing.T) {
	jt := JiraProjectType("software", "http://h")
	if jt.Key != "software" || jt.Self == "" {
		t.Errorf("unexpected project type: %+v", jt)
	}
}
```

- [ ] **Step 2: Verifica che falliscano**

Run: `go test ./internal/api/v3/ -run 'TestJiraProject'`
Expected: FAIL — `undefined: JiraProject`.

- [ ] **Step 3: Implementa il mapping**

```go
// internal/api/v3/project.go
package v3

import (
	"fmt"

	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
)

// ProjectCategory è la rappresentazione v3 di una categoria di progetto.
type ProjectCategory struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Project è la rappresentazione Jira v3 di un progetto (schema "Project").
type Project struct {
	Self            string            `json:"self"`
	ID              string            `json:"id"`
	Key             string            `json:"key"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	ProjectTypeKey  string            `json:"projectTypeKey"`
	Simplified      bool              `json:"simplified"`
	Style           string            `json:"style"`
	IsPrivate       bool              `json:"isPrivate"`
	Archived        bool              `json:"archived"`
	AssigneeType    string            `json:"assigneeType,omitempty"`
	URL             string            `json:"url,omitempty"`
	AvatarUrls      map[string]string `json:"avatarUrls"`
	Lead            *User             `json:"lead,omitempty"`
	ProjectCategory *ProjectCategory  `json:"projectCategory,omitempty"`
}

// JiraProject traduce il modello interno nello schema v3. lead e cat possono
// essere nil. baseURL è l'origine pubblica (cfg.BaseURL).
func JiraProject(p project.Project, lead *user.User, cat *project.ProjectCategory, baseURL string) Project {
	avatar := baseURL + "/static/default-project-avatar.svg"
	jp := Project{
		Self:           fmt.Sprintf("%s/rest/api/3/project/%s", baseURL, p.ID),
		ID:             p.ID,
		Key:            p.Key,
		Name:           p.Name,
		Description:    p.Description,
		ProjectTypeKey: project.ProjectTypeKeyForType(p.Type),
		Simplified:     p.Simplified,
		Style:          p.Style,
		IsPrivate:      p.IsPrivate,
		Archived:       p.IsArchived,
		AssigneeType:   p.AssigneeType,
		URL:            p.URL,
		AvatarUrls: map[string]string{
			"16x16": avatar, "24x24": avatar, "32x32": avatar, "48x48": avatar,
		},
	}
	if jp.Style == "" {
		jp.Style = "classic"
	}
	if lead != nil {
		lu := JiraUser(*lead, baseURL)
		jp.Lead = &lu
	}
	if cat != nil {
		jp.ProjectCategory = &ProjectCategory{
			Self:        fmt.Sprintf("%s/rest/api/3/projectCategory/%s", baseURL, cat.ID),
			ID:          cat.ID,
			Name:        cat.Name,
			Description: cat.Description,
		}
	}
	return jp
}

// ProjectType è la rappresentazione v3 di un tipo di progetto.
type ProjectType struct {
	Key                string `json:"key"`
	FormattedKey       string `json:"formattedKey"`
	DescriptionI18nKey string `json:"descriptionI18nKey"`
	Icon               string `json:"icon"`
	Color              string `json:"color"`
	Self               string `json:"self,omitempty"`
}

func JiraProjectType(key, baseURL string) ProjectType {
	labels := map[string]string{"software": "Software", "business": "Business", "service_desk": "Service Desk"}
	return ProjectType{
		Key:                key,
		FormattedKey:       labels[key],
		DescriptionI18nKey: "jira.project.type." + key + ".description",
		Icon:               "",
		Color:              "#0052CC",
		Self:               fmt.Sprintf("%s/rest/api/3/project/type/%s", baseURL, key),
	}
}
```

- [ ] **Step 4: Verifica che passino**

Run: `go test ./internal/api/v3/ -run 'TestJiraProject'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/v3/project.go internal/api/v3/project_test.go
git commit -m "feat(v3): JiraProject/JiraProjectType mapping to official schema"
```

---

### Task 4: Avatar di default per i progetti

**Files:**
- Modify: `internal/api/router.go`
- Test: `internal/api/handlers/... ` (verifica via contract server nel Task 8; qui basta il router)

- [ ] **Step 1: Aggiungi la rotta avatar progetto**

In `internal/api/router.go`, vicino alla rotta `GET /static/default-avatar.svg`, aggiungi:

```go
	mux.HandleFunc("GET /static/default-project-avatar.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 48 48"><rect width="48" height="48" rx="6" fill="#0052CC"/><path d="M14 14h20v20H14z" fill="#fff" opacity="0.85"/></svg>`))
	})
```

- [ ] **Step 2: Verifica build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Verifica che l'avatar risponda 200**

Run:
```bash
APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/av.db APP_PORT=18091 go run ./cmd/server & sleep 4
curl -s -o /dev/null -w "%{http_code} %{content_type}\n" http://localhost:18091/static/default-project-avatar.svg
kill %1 2>/dev/null; rm -f /tmp/av.db
```
Expected: `200 image/svg+xml`.

- [ ] **Step 4: Commit**

```bash
git add internal/api/router.go
git commit -m "feat(project): serve default project avatar for v3 avatarUrls"
```

---

### Task 5: Service — CreateInput, Restore, GetCategory

**Files:**
- Modify: `internal/domain/project/service.go`
- Test: `internal/domain/project/service_test.go` (aggiunte)

- [ ] **Step 1: Scrivi i test fallenti**

Aggiungi a `internal/domain/project/service_test.go` (riusa l'helper DB già presente nel file; se non c'è, crea una `*gorm.DB` in-memory con le tabelle `projects` e `project_categories` come fanno gli altri test del package):

```go
func TestCreateProjectWithInput(t *testing.T) {
	db := newProjectTestDB(t) // helper esistente nel package (o crealo come gli altri test)
	svc := NewService(db, nil)
	p, err := svc.CreateProject(CreateInput{
		Key: "NEW", Name: "New Project", Description: "d",
		Type: TypeKanban, AssigneeType: "PROJECT_LEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Key != "NEW" || p.Type != TypeKanban || p.AssigneeType != "PROJECT_LEAD" {
		t.Errorf("unexpected: %+v", p)
	}
}

func TestArchiveThenRestore(t *testing.T) {
	db := newProjectTestDB(t)
	svc := NewService(db, nil)
	if _, err := svc.CreateProject(CreateInput{Key: "ARC", Name: "Arc", Type: TypeScrum}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Archive("ARC"); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetByKey("ARC")
	if !got.IsArchived {
		t.Fatal("expected archived")
	}
	if err := svc.Restore("ARC"); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.GetByKey("ARC")
	if got.IsArchived {
		t.Error("expected restored (not archived)")
	}
}
```

Se nel package non esiste già un helper `newProjectTestDB`, crealo in un file `internal/domain/project/testhelpers_test.go`:

```go
package project

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProjectTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Project{}, &ProjectFavorite{}, &ProjectCategory{}); err != nil {
		t.Fatal(err)
	}
	return db
}
```

(Se un helper equivalente esiste già, riusalo e non crearne un secondo.)

- [ ] **Step 2: Verifica che falliscano**

Run: `go test ./internal/domain/project/ -run 'TestCreateProjectWithInput|TestArchiveThenRestore'`
Expected: FAIL — `undefined: CreateInput` / `Restore`.

- [ ] **Step 3: Implementa**

In `internal/domain/project/service.go` aggiungi:

```go
// CreateInput raccoglie i campi di creazione progetto in forma v3-friendly.
type CreateInput struct {
	Key          string
	Name         string
	Description  string
	Type         Type
	LeadUserID   *string
	CategoryID   *string
	AssigneeType string
	URL          string
}

func generateProjectID() string { return uuid.NewString() } // se il package già genera ID altrove, riusa quella funzione

// CreateProject crea un progetto dai campi v3. Key e Name obbligatori.
func (s *Service) CreateProject(in CreateInput) (*Project, error) {
	if in.Key == "" || in.Name == "" {
		return nil, errors.New("key and name are required")
	}
	if in.Type == "" {
		in.Type = TypeScrum
	}
	if in.AssigneeType == "" {
		in.AssigneeType = "UNASSIGNED"
	}
	p := &Project{
		ID:           generateProjectID(),
		Key:          in.Key,
		Name:         in.Name,
		Description:  in.Description,
		Type:         in.Type,
		LeadUserID:   in.LeadUserID,
		CategoryID:   in.CategoryID,
		AssigneeType: in.AssigneeType,
		Style:        "classic",
		URL:          in.URL,
	}
	if err := s.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// Restore annulla l'archiviazione di un progetto.
func (s *Service) Restore(key string) error {
	return s.db.Model(&Project{}).Where("key = ?", key).Update("is_archived", false).Error
}

// GetCategory ritorna una categoria per ID (nil,nil se assente).
func (s *Service) GetCategory(id string) (*ProjectCategory, error) {
	if id == "" {
		return nil, nil
	}
	var c ProjectCategory
	if err := s.db.First(&c, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}
```

Assicurati che gli import `errors`, `gorm.io/gorm` e il generatore di UUID siano presenti (il package `service.go` probabilmente già importa `uuid`; riusa la funzione ID esistente invece di aggiungerne una se c'è già — controlla `service.go`).

- [ ] **Step 4: Verifica che passino**

Run: `go test ./internal/domain/project/ -count=1`
Expected: PASS (tutti, inclusi i preesistenti).

- [ ] **Step 5: Commit**

```bash
git add internal/domain/project/service.go internal/domain/project/service_test.go internal/domain/project/testhelpers_test.go
git commit -m "feat(project): CreateProject(input), Restore, GetCategory in service"
```

---

### Task 6: Handler — GET /rest/api/3/project/{idOrKey} conforme

**Files:**
- Modify: `internal/api/handlers/project_handler.go`, `internal/api/router.go`
- Test: `internal/contract/project_test.go` (create)

Contesto: `NewProjectHandler` deve ricevere il `baseURL` (come `NewUserHandler` nel Round 0). Aggiorna il costruttore a `NewProjectHandler(svc *project.Service, wfSvc *workflow.Service, baseURL string)` e il call-site in `router.go` (`cfg.BaseURL`). Cerca eventuali altri call-site (`grep -rn NewProjectHandler`).

- [ ] **Step 1: Scrivi il contract test fallente**

```go
// internal/contract/project_test.go
package contract

import (
	"net/http"
	"testing"
)

func TestGetProject_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc) // helper: vedi sotto
	// crea un progetto via API
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/project/DEMO", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/project/DEMO", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /project/{key} non conforme: %v", err)
	}
}
```

Aggiungi in fondo al file gli helper riusabili dai test progetto (se `registerAndLogin` non esiste già nel package `contract`, crealo; se esiste in `myself_test.go`, riusalo e non duplicarlo):

```go
import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"

	"github.com/open-jira/open-jira/internal/domain/auth"
)

func registerAndLogin(t *testing.T, authSvc *auth.Service) string {
	t.Helper()
	if _, err := authSvc.Register("alice@example.com", "alice", "Alice", "password-123"); err != nil {
		t.Fatal(err)
	}
	jwt, err := authSvc.Login("alice@example.com", "password-123")
	if err != nil {
		t.Fatal(err)
	}
	return jwt
}

func createProjectViaAPI(t *testing.T, srv *httptest.Server, jwt, key, name string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"key": key, "name": name, "projectTypeKey": "software",
		"projectTemplateKey": "com.pyxis.greenhopper.jira:gh-scrum-template",
	})
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/project", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create project status = %d: %s", res.StatusCode, b)
	}
}
```

- [ ] **Step 2: Verifica che fallisca**

Run: `go test ./internal/contract/ -run TestGetProject -count=1`
Expected: FAIL — o il create non è ancora conforme (Task 7) oppure il body GET è il modello interno snake_case (non conforme allo schema `Project`).

Nota: questo test dipende dal POST conforme (Task 7). Se preferisci ordine stretto TDD, implementa prima il GET usando un progetto creato via service direttamente; ma dato che POST e GET vanno insieme, è accettabile far diventare verde questo test alla fine del Task 7. Marca il test con un commento che lo lega al Task 7.

- [ ] **Step 3: Riscrivi `Get` in forma v3**

In `internal/api/handlers/project_handler.go`, importa `v3 "github.com/open-jira/open-jira/internal/domain/..."`? No — importa `"github.com/open-jira/open-jira/internal/api/v3"` e `"github.com/open-jira/open-jira/internal/domain/user"`. Riscrivi `Get`:

```go
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	p, err := h.svc.GetByKey(key)
	if err != nil || p == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	lead := h.leadOf(p)          // vedi helper sotto
	cat := h.categoryOf(p)       // vedi helper sotto
	v3.WriteJSON(w, http.StatusOK, v3.JiraProject(*p, lead, cat, h.baseURL))
}

// leadOf carica l'utente lead del progetto (nil se assente).
func (h *ProjectHandler) leadOf(p *project.Project) *user.User {
	if p.LeadUserID == nil {
		return nil
	}
	var u user.User
	if err := h.svc.DB().First(&u, "id = ?", *p.LeadUserID).Error; err != nil {
		return nil
	}
	return &u
}

// categoryOf carica la categoria del progetto (nil se assente).
func (h *ProjectHandler) categoryOf(p *project.Project) *project.ProjectCategory {
	if p.CategoryID == nil {
		return nil
	}
	c, _ := h.svc.GetCategory(*p.CategoryID)
	return c
}
```

Aggiungi il campo `baseURL string` alla struct `ProjectHandler` e impostalo nel costruttore `NewProjectHandler(svc, wfSvc, baseURL)`. Aggiorna il call-site in `router.go`: `projectH := handlers.NewProjectHandler(projectSvc, wfSvc, cfg.BaseURL)`.

- [ ] **Step 4: Non rompere gli altri handler**

Run: `go build ./...`
Expected: exit 0 (aggiusta ogni call-site di `NewProjectHandler`).

- [ ] **Step 5: Commit (parziale, GET pronto; il contract test diventa verde col Task 7)**

```bash
git add internal/api/handlers/project_handler.go internal/api/router.go internal/contract/project_test.go
git commit -m "feat(v3): GET /project/{idOrKey} returns official Project shape"
```

---

### Task 7: Handler — POST /rest/api/3/project conforme

**Files:**
- Modify: `internal/api/handlers/project_handler.go`
- Test: `internal/contract/project_test.go` (aggiunta)

- [ ] **Step 1: Scrivi il contract test fallente della creazione**

Aggiungi a `internal/contract/project_test.go`:

```go
func TestCreateProject_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)

	body := `{"key":"NEW","name":"New Project","projectTypeKey":"software","projectTemplateKey":"com.pyxis.greenhopper.jira:gh-scrum-template","leadAccountId":"","assigneeType":"UNASSIGNED"}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/project", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("POST", "/rest/api/3/project", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("POST /project non conforme: %v", err)
	}
}

func TestCreateProject_MissingKey_400(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/project", strings.NewReader(`{"name":"No Key"}`))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}
```

Aggiungi `"strings"` agli import del test file.

Nota sullo schema di risposta POST: il contratto per `POST /rest/api/3/project` risponde con lo schema `ProjectIdentifiers` (`{self, id, key}`), NON con `Project` intero. Verifica quale schema il contratto associa al 201 e conforma la risposta di conseguenza (vedi Step 3).

- [ ] **Step 2: Verifica che falliscano**

Run: `go test ./internal/contract/ -run TestCreateProject -count=1`
Expected: FAIL — la Create attuale ritorna il modello interno snake_case.

- [ ] **Step 3: Riscrivi `Create` in forma v3**

Verifica prima lo schema di risposta 201 nel contratto:
```bash
python3 -c 'import json;d=json.load(open("docs/contracts/jira-platform-v3.json"));print(d["paths"]["/rest/api/3/project"]["post"]["responses"]["201"]["content"]["application/json"]["schema"])'
```
Conforma la risposta a quello schema (tipicamente `ProjectIdentifiers`: `{self, id, key}`). Poi:

```go
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key                string `json:"key"`
		Name               string `json:"name"`
		Description        string `json:"description"`
		ProjectTypeKey     string `json:"projectTypeKey"`
		ProjectTemplateKey string `json:"projectTemplateKey"`
		LeadAccountID      string `json:"leadAccountId"`
		CategoryID         int    `json:"categoryId"`
		AssigneeType       string `json:"assigneeType"`
		URL                string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	fieldErrs := map[string]string{}
	if req.Key == "" {
		fieldErrs["key"] = "You must specify a valid project key."
	}
	if req.Name == "" {
		fieldErrs["name"] = "You must specify a valid project name."
	}
	if len(fieldErrs) > 0 {
		v3.WriteError(w, http.StatusBadRequest, nil, fieldErrs)
		return
	}
	in := project.CreateInput{
		Key:          req.Key,
		Name:         req.Name,
		Description:  req.Description,
		Type:         project.TypeForTemplateKey(req.ProjectTemplateKey),
		AssigneeType: req.AssigneeType,
		URL:          req.URL,
	}
	if req.LeadAccountID != "" {
		in.LeadUserID = &req.LeadAccountID
	}
	p, err := h.svc.CreateProject(in)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{err.Error()}, nil)
		return
	}
	// Risposta ProjectIdentifiers ({self,id,key}) — adatta se lo schema differisce.
	v3.WriteJSON(w, http.StatusCreated, map[string]any{
		"self": h.baseURL + "/rest/api/3/project/" + p.ID,
		"id":   p.ID,
		"key":  p.Key,
	})
}
```

Se il contratto per il 201 usa lo schema `Project` intero anziché `ProjectIdentifiers`, ritorna invece `v3.JiraProject(*p, h.leadOf(p), h.categoryOf(p), h.baseURL)`. La verità è nel JSON del contratto (Step 3, comando python sopra) — segui quello.

- [ ] **Step 4: Verifica che passino (create + get)**

Run: `go test ./internal/contract/ -run TestCreateProject -count=1 && go test ./internal/contract/ -run TestGetProject -count=1`
Expected: PASS entrambi.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/project_handler.go internal/contract/project_test.go
git commit -m "feat(v3): POST /project accepts CreateProjectDetails, conforms to contract"
```

---

### Task 8: Handler — GET /rest/api/3/project/search (PageBeanProject)

**Files:**
- Modify: `internal/api/handlers/project_handler.go`, `internal/api/router.go`
- Test: `internal/contract/project_test.go` (aggiunta)

- [ ] **Step 1: Scrivi il contract test fallente**

```go
func TestProjectSearch_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/project/search?maxResults=10", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/project/search", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /project/search non conforme: %v", err)
	}
}
```

- [ ] **Step 2: Verifica che fallisca**

Run: `go test ./internal/contract/ -run TestProjectSearch -count=1`
Expected: FAIL — rotta inesistente / schema non conforme.

- [ ] **Step 3: Implementa `Search`**

`PageBeanProject` ha forma `{startAt, maxResults, total, isLast, values: []Project}` più `self`/`nextPage` opzionali. Il nostro `v3.WritePage[T]` produce `{startAt, maxResults, total, isLast, values}` — sufficiente per lo schema (i campi extra sono opzionali). Aggiungi:

```go
func (h *ProjectHandler) Search(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	f := project.ListFilter{
		Search:  r.URL.Query().Get("query"),
		SortKey: "name", SortDir: "asc",
		StartAt: startAt, MaxResults: maxResults,
	}
	rows, total, err := h.svc.ListWithFilters(f, userID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to search projects."}, nil)
		return
	}
	values := make([]v3.Project, 0, len(rows))
	for i := range rows {
		var lead *user.User
		if rows[i].Lead != nil {
			lead = &user.User{ID: rows[i].Lead.ID, DisplayName: rows[i].Lead.DisplayName, Email: rows[i].Lead.Email, IsActive: true}
		}
		cat := h.categoryOf(&rows[i].Project)
		values = append(values, v3.JiraProject(rows[i].Project, lead, cat, h.baseURL))
	}
	v3.WritePage(w, http.StatusOK, v3.Page[v3.Project]{
		StartAt: startAt, MaxResults: maxResults, Total: int(total), Values: values,
	})
}
```

Verifica che `ListFilter` abbia i campi `StartAt`/`MaxResults` (nel Round 0 il modello li dichiarava; se i nomi differiscono, adatta). Registra la rotta in `router.go` **prima** di `GET /rest/api/3/project/{key}` per evitare che `search` sia catturato come `{key}` — con ServeMux di Go 1.22+ i pattern più specifici hanno precedenza, ma `search` è un literal segment mentre `{key}` è wildcard, quindi `GET /rest/api/3/project/search` vince comunque; aggiungila comunque esplicitamente:

```go
	mux.Handle("GET /rest/api/3/project/search", authMw(http.HandlerFunc(projectH.Search)))
```

- [ ] **Step 4: Verifica**

Run: `go test ./internal/contract/ -run TestProjectSearch -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/project_handler.go internal/api/router.go internal/contract/project_test.go
git commit -m "feat(v3): GET /project/search returns PageBeanProject"
```

---

### Task 9: Handler — PUT /project/{idOrKey}, archive, restore

**Files:**
- Modify: `internal/api/handlers/project_handler.go`, `internal/api/router.go`
- Test: `internal/contract/project_test.go` (aggiunta)

- [ ] **Step 1: Scrivi i test fallenti**

```go
func TestUpdateProject_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "UPD", "Before")

	req, _ := http.NewRequest("PUT", srv.URL+"/rest/api/3/project/UPD", strings.NewReader(`{"name":"After","description":"changed"}`))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("PUT", "/rest/api/3/project/UPD", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("PUT /project/{key} non conforme: %v", err)
	}
}

func TestArchiveProject_204(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "ARC", "Arc")
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/project/ARC/archive", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("archive status = %d, want 204", res.StatusCode)
	}
	// restore
	req2, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/project/ARC/restore", nil)
	req2.Header.Set("Authorization", "Bearer "+jwt)
	res2, _ := http.DefaultClient.Do(req2)
	defer res2.Body.Close()
	if res2.StatusCode != 200 && res2.StatusCode != 204 {
		t.Fatalf("restore status = %d", res2.StatusCode)
	}
}
```

- [ ] **Step 2: Verifica che falliscano**

Run: `go test ./internal/contract/ -run 'TestUpdateProject|TestArchiveProject' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementa Update (v3), Archive, Restore**

```go
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var req struct {
		Name         *string `json:"name"`
		Description  *string `json:"description"`
		AssigneeType *string `json:"assigneeType"`
		URL          *string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	name, desc := "", ""
	if req.Name != nil {
		name = *req.Name
	}
	if req.Description != nil {
		desc = *req.Description
	}
	p, err := h.svc.Update(key, name, desc) // Update esistente aggiorna name+description
	if err != nil || p == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	// aggiorna i campi extra se presenti
	updates := map[string]any{}
	if req.AssigneeType != nil {
		updates["assignee_type"] = *req.AssigneeType
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if len(updates) > 0 {
		h.svc.DB().Model(&project.Project{}).Where("key = ?", key).Updates(updates)
		p, _ = h.svc.GetByKey(key)
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraProject(*p, h.leadOf(p), h.categoryOf(p), h.baseURL))
}

func (h *ProjectHandler) Archive(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if err := h.svc.Archive(key); err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) Restore(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if err := h.svc.Restore(key); err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"No project could be found with key '" + key + "'."}, nil)
		return
	}
	p, _ := h.svc.GetByKey(key)
	if p == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraProject(*p, h.leadOf(p), h.categoryOf(p), h.baseURL))
}
```

Rotte in `router.go`:

```go
	mux.Handle("POST /rest/api/3/project/{key}/archive", authMw(http.HandlerFunc(projectH.Archive)))
	mux.Handle("POST /rest/api/3/project/{key}/restore", authMw(http.HandlerFunc(projectH.Restore)))
```

(La rotta `PUT /rest/api/3/project/{key}` esiste già e ora punta al nuovo `Update`.)

- [ ] **Step 4: Verifica**

Run: `go test ./internal/contract/ -run 'TestUpdateProject|TestArchiveProject' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/project_handler.go internal/api/router.go internal/contract/project_test.go
git commit -m "feat(v3): PUT /project, POST /project/{key}/archive and /restore"
```

---

### Task 10: Handler — project types e project categories

**Files:**
- Create: `internal/api/handlers/projectcategory_handler.go`
- Modify: `internal/api/handlers/project_handler.go`, `internal/api/router.go`
- Test: `internal/contract/project_test.go` (aggiunta)

- [ ] **Step 1: Scrivi i contract test fallenti**

```go
func TestProjectTypes_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/project/type", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/project/type", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /project/type non conforme: %v", err)
	}
}

func TestCreateProjectCategory_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/projectCategory", strings.NewReader(`{"name":"Ops","description":"operations"}`))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 201 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("POST", "/rest/api/3/projectCategory", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("POST /projectCategory non conforme: %v", err)
	}
}
```

Verifica lo status atteso per `POST /rest/api/3/projectCategory` nel contratto (200 o 201) e allinea l'assert e l'handler:
```bash
python3 -c 'import json;d=json.load(open("docs/contracts/jira-platform-v3.json"));print(list(d["paths"]["/rest/api/3/projectCategory"]["post"]["responses"].keys()))'
```

- [ ] **Step 2: Verifica che falliscano**

Run: `go test ./internal/contract/ -run 'TestProjectTypes|TestCreateProjectCategory' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementa i project type handler**

In `project_handler.go`:

```go
func (h *ProjectHandler) ProjectTypes(w http.ResponseWriter, r *http.Request) {
	types := []v3.ProjectType{
		v3.JiraProjectType("software", h.baseURL),
		v3.JiraProjectType("business", h.baseURL),
	}
	v3.WriteJSON(w, http.StatusOK, types)
}

func (h *ProjectHandler) ProjectTypeByKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("projectTypeKey")
	if key != "software" && key != "business" {
		v3.WriteError(w, http.StatusNotFound, []string{"No project type '" + key + "'."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraProjectType(key, h.baseURL))
}
```

- [ ] **Step 4: Implementa il project category handler**

```go
// internal/api/handlers/projectcategory_handler.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectCategoryHandler struct {
	db      *gorm.DB
	baseURL string
}

func NewProjectCategoryHandler(db *gorm.DB, baseURL string) *ProjectCategoryHandler {
	return &ProjectCategoryHandler{db: db, baseURL: baseURL}
}

func (h *ProjectCategoryHandler) jira(c project.ProjectCategory) v3.ProjectCategory {
	return v3.ProjectCategory{
		Self:        h.baseURL + "/rest/api/3/projectCategory/" + c.ID,
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
	}
}

func (h *ProjectCategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	var cats []project.ProjectCategory
	h.db.Find(&cats)
	out := make([]v3.ProjectCategory, 0, len(cats))
	for _, c := range cats {
		out = append(out, h.jira(c))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

func (h *ProjectCategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	if req.Name == "" {
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"name": "The category name must not be empty."})
		return
	}
	c := project.ProjectCategory{ID: uuid.NewString(), Name: req.Name, Description: req.Description}
	if err := h.db.Create(&c).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to create category."}, nil)
		return
	}
	// Allinea lo status a quello del contratto (Step 1): usa http.StatusOK o http.StatusCreated.
	v3.WriteJSON(w, http.StatusOK, h.jira(c))
}
```

- [ ] **Step 5: Registra le rotte**

In `router.go` (dopo la costruzione dell'handler `pcH := handlers.NewProjectCategoryHandler(db, cfg.BaseURL)`):

```go
	mux.Handle("GET /rest/api/3/project/type", authMw(http.HandlerFunc(projectH.ProjectTypes)))
	mux.Handle("GET /rest/api/3/project/type/{projectTypeKey}", authMw(http.HandlerFunc(projectH.ProjectTypeByKey)))
	mux.Handle("GET /rest/api/3/projectCategory", authMw(http.HandlerFunc(pcH.List)))
	mux.Handle("POST /rest/api/3/projectCategory", authMw(http.HandlerFunc(pcH.Create)))
```

- [ ] **Step 6: Verifica**

Run: `go test ./internal/contract/ -run 'TestProjectTypes|TestCreateProjectCategory' -count=1 && go build ./...`
Expected: PASS + build OK.

- [ ] **Step 7: Commit**

```bash
git add internal/api/handlers/project_handler.go internal/api/handlers/projectcategory_handler.go internal/api/router.go internal/contract/project_test.go
git commit -m "feat(v3): project types and project categories endpoints"
```

---

### Task 11: Aggiorna il gap report + verifica suite backend

**Files:**
- Modify: `docs/contracts/gap-report.md` (rigenerato)

- [ ] **Step 1: Suite completa verde**

Run: `go build ./... && go vet ./... && go test ./... -count=1 2>&1 | grep -E "FAIL|ok" | tail -20`
Expected: nessun `FAIL`.

- [ ] **Step 2: Rigenera il gap report**

Run: `go run ./cmd/gapreport`
Expected: `matched` cresce (nuovi endpoint project conformi ora coperti dalla router-match: search, type, archive, restore, projectCategory).

- [ ] **Step 3: Commit**

```bash
git add docs/contracts/gap-report.md
git commit -m "chore(contracts): regenerate gap report after project v3 endpoints"
```

---

### Task 12: Frontend — tipo Project v3 e chiamate API

**Files:**
- Modify: `frontend-next/lib/api.ts`

- [ ] **Step 1: Aggiorna il tipo Project e le chiamate**

In `frontend-next/lib/api.ts` sostituisci l'interfaccia `Project` e la sezione `projects` per riflettere lo schema v3 (camelCase). Nuovo tipo:

```ts
export type ProjectTypeKey = "software" | "business";

export interface JiraUserRef {
  accountId: string;
  displayName: string;
  emailAddress?: string;
  avatarUrls: Record<string, string>;
}

export interface ProjectCategoryRef {
  id: string;
  name: string;
  description: string;
}

export interface Project {
  self: string;
  id: string;
  key: string;
  name: string;
  description?: string;
  projectTypeKey: ProjectTypeKey;
  style: string;
  simplified: boolean;
  isPrivate: boolean;
  archived: boolean;
  assigneeType?: string;
  url?: string;
  avatarUrls: Record<string, string>;
  lead?: JiraUserRef;
  projectCategory?: ProjectCategoryRef;
}
```

Aggiorna `projects`:

```ts
export const projects = {
  search: (params: { query?: string; startAt?: number; maxResults?: number } = {}) => {
    const qs = buildQuery({
      query: params.query,
      startAt: params.startAt,
      maxResults: params.maxResults,
    });
    return apiFetch<PagedResponse<Project>>(`/rest/api/3/project/search${qs}`);
  },

  get: (key: string) => apiFetch<Project>(`/rest/api/3/project/${key}`),

  create: (payload: {
    key: string;
    name: string;
    description?: string;
    projectTypeKey: ProjectTypeKey;
    projectTemplateKey: string;
    assigneeType?: string;
    categoryId?: number;
  }) =>
    apiFetch<{ self: string; id: string; key: string }>("/rest/api/3/project", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  update: (key: string, payload: { name?: string; description?: string; assigneeType?: string; url?: string }) =>
    apiFetch<Project>(`/rest/api/3/project/${key}`, { method: "PUT", body: JSON.stringify(payload) }),

  archive: (key: string) =>
    apiFetch<void>(`/rest/api/3/project/${key}/archive`, { method: "POST" }),

  restore: (key: string) =>
    apiFetch<void>(`/rest/api/3/project/${key}/restore`, { method: "POST" }),

  types: () => apiFetch<{ key: string; formattedKey: string }[]>("/rest/api/3/project/type"),

  categories: () => apiFetch<ProjectCategoryRef[]>("/rest/api/3/projectCategory"),
};
```

Rimuovi le vecchie `list/star/unstar` se non più usate; se altri componenti le importano, adatta quelli (grep `projects.list`, `projects.star`).

- [ ] **Step 2: Verifica build**

Run: `cd frontend-next && npm run build`
Expected: eventuali errori TypeScript nei componenti che usavano i vecchi campi (snake_case) — verranno risolti nei Task 13-14. Se il build fallisce solo per quei componenti, procedi (li sistemi subito dopo); NON lasciare il build rotto oltre il Task 14.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): v3 Project type (camelCase) and project API calls"
```

---

### Task 13: Frontend — lista progetti e modale creazione v3

**Files:**
- Modify: `frontend-next/components/projects/ProjectsPage.tsx`, `frontend-next/components/projects/CreateProjectModal.tsx`

- [ ] **Step 1: Adatta ProjectsPage ai campi v3**

In `ProjectsPage.tsx`: sostituisci l'uso di TanStack Query per caricare la lista via `projects.search()` invece della vecchia `projects.list()`; rendi le righe con i campi v3 (`p.key`, `p.name`, `p.projectTypeKey` mostrato come "Software"/"Business", `p.lead?.displayName`, `p.avatarUrls["24x24"]`). Esempio della query:

```tsx
import { useQuery } from "@tanstack/react-query";
import { projects, Project, PagedResponse } from "@/lib/api";

const { data, isLoading } = useQuery({
  queryKey: ["projects", query],
  queryFn: () => projects.search({ query, maxResults: 50 }),
});
const rows: Project[] = data?.values ?? [];
```

Mostra `projectTypeKey === "software" ? "Software" : "Business"` nella colonna TYPE. Per il lead usa `p.lead?.displayName` e l'avatar `p.lead?.avatarUrls?.["24x24"]` con fallback all'iniziale.

- [ ] **Step 2: Adatta CreateProjectModal al payload v3**

In `CreateProjectModal.tsx`: il form deve raccogliere Name, Key, e un selettore di template (Scrum / Kanban / Business) che si traduce in `projectTypeKey` + `projectTemplateKey`:

```tsx
const TEMPLATES = {
  scrum:    { projectTypeKey: "software" as const, projectTemplateKey: "com.pyxis.greenhopper.jira:gh-scrum-template",  label: "Scrum" },
  kanban:   { projectTypeKey: "software" as const, projectTemplateKey: "com.pyxis.greenhopper.jira:gh-kanban-template", label: "Kanban" },
  business: { projectTypeKey: "business" as const, projectTemplateKey: "com.atlassian.jira-core-project-templates:jira-core-simplified-process-control", label: "Business" },
};
// onSubmit:
await projects.create({
  key, name, description,
  projectTypeKey: TEMPLATES[tpl].projectTypeKey,
  projectTemplateKey: TEMPLATES[tpl].projectTemplateKey,
});
// invalida la query lista
queryClient.invalidateQueries({ queryKey: ["projects"] });
```

Usa `useMutation` di TanStack Query e mostra gli errori (il messaggio arriva già parsato da `apiFetch`).

- [ ] **Step 3: Verifica build**

Run: `cd frontend-next && npm run build`
Expected: build pulito (nessun errore TS).

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/projects/ProjectsPage.tsx frontend-next/components/projects/CreateProjectModal.tsx
git commit -m "feat(frontend): projects list + create modal on v3 project API"
```

---

### Task 14: Frontend — pagina impostazioni progetto

**Files:**
- Create: `frontend-next/app/jira/projects/[key]/settings/page.tsx`, `frontend-next/components/projects/ProjectSettings.tsx`

- [ ] **Step 1: Crea la route Next**

```tsx
// frontend-next/app/jira/projects/[key]/settings/page.tsx
import { ProjectSettings } from "@/components/projects/ProjectSettings";

export default async function Page({ params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  return <ProjectSettings projectKey={key} />;
}
```

- [ ] **Step 2: Crea il componente impostazioni**

```tsx
// frontend-next/components/projects/ProjectSettings.tsx
"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useEffect } from "react";
import { projects } from "@/lib/api";

export function ProjectSettings({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const { data: project, isLoading } = useQuery({
    queryKey: ["project", projectKey],
    queryFn: () => projects.get(projectKey),
  });
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  useEffect(() => {
    if (project) {
      setName(project.name);
      setDescription(project.description ?? "");
    }
  }, [project]);

  const save = useMutation({
    mutationFn: () => projects.update(projectKey, { name, description }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["project", projectKey] }),
  });
  const archive = useMutation({
    mutationFn: () => projects.archive(projectKey),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["projects"] }),
  });

  if (isLoading) return <div className="p-8">Loading…</div>;
  if (!project) return <div className="p-8">Project not found.</div>;

  return (
    <div className="max-w-2xl p-8">
      <h1 className="text-2xl font-semibold mb-6">{project.key} · Settings</h1>
      <label htmlFor="proj-name" className="block text-xs font-semibold uppercase tracking-wider text-[#64748b] mb-1.5">Name</label>
      <input id="proj-name" value={name} onChange={(e) => setName(e.target.value)}
        className="w-full rounded border px-3 py-2 mb-4" />
      <label htmlFor="proj-desc" className="block text-xs font-semibold uppercase tracking-wider text-[#64748b] mb-1.5">Description</label>
      <textarea id="proj-desc" value={description} onChange={(e) => setDescription(e.target.value)}
        className="w-full rounded border px-3 py-2 mb-4" rows={4} />
      <div className="flex gap-3">
        <button onClick={() => save.mutate()} disabled={save.isPending}
          className="rounded bg-[#0052CC] px-4 py-2 text-white font-semibold">Save</button>
        <button onClick={() => archive.mutate()} disabled={archive.isPending}
          className="rounded border px-4 py-2 text-[#64748b] font-semibold">Archive project</button>
      </div>
      {save.isSuccess && <p className="mt-3 text-green-600">Saved.</p>}
    </div>
  );
}
```

- [ ] **Step 3: Aggiungi un link "Settings" dalla lista progetti**

In `ProjectsPage.tsx`, rendi il nome/chiave del progetto un link a `/jira/projects/${p.key}/settings` (Next `Link`).

- [ ] **Step 4: Verifica build**

Run: `cd frontend-next && npm run build`
Expected: build pulito.

- [ ] **Step 5: Commit**

```bash
git add frontend-next/app/jira/projects/ frontend-next/components/projects/ProjectSettings.tsx frontend-next/components/projects/ProjectsPage.tsx
git commit -m "feat(frontend): project settings page (rename, edit description, archive)"
```

---

### Task 15: E2E — creazione progetto, lista, impostazioni

**Files:**
- Create: `frontend-next/e2e/projects.spec.ts`

- [ ] **Step 1: Scrivi lo spec E2E**

```ts
// frontend-next/e2e/projects.spec.ts
import { test, expect } from "@playwright/test";

async function login(page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira\/projects/);
}

test("crea un nuovo progetto e lo vede nella lista", async ({ page }) => {
  await login(page);
  await page.getByRole("button", { name: /create project/i }).click();
  await page.getByLabel(/name/i).first().fill("Marketing Site");
  await page.getByLabel(/key/i).first().fill("MKT");
  // seleziona template se presente (Scrum default)
  await page.locator('form button[type="submit"], button:has-text("Create")').last().click();
  await expect(page.getByText("Marketing Site")).toBeVisible();
});

test("apre le impostazioni del progetto demo e rinomina", async ({ page }) => {
  await login(page);
  await page.getByText("Demo Project").click();
  await page.waitForURL(/\/jira\/projects\/DEMO\/settings/);
  const nameInput = page.locator("#proj-name");
  await expect(nameInput).toHaveValue(/Demo Project/);
  await nameInput.fill("Demo Project Renamed");
  await page.getByRole("button", { name: /^save$/i }).click();
  await expect(page.getByText(/saved/i)).toBeVisible();
});
```

Adatta i selettori alla UI reale dopo i Task 13-14 (leggi i componenti; se i label del modale non combaciano con `getByLabel`, aggiungi `htmlFor`/`id` come fatto per il login). Il secondo test presuppone che il click sul nome del progetto porti a `/settings` (Task 14 Step 3).

- [ ] **Step 2: Esegui (o build+lettura se la porta 8080 è occupata)**

Run: `cd frontend-next && npx playwright test e2e/projects.spec.ts`
Expected: 2 passed. Se la porta 8080 è occupata in questo ambiente, verifica con `APP_PORT` alternativo come documentato nel Round 0, poi ripristina la config.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/e2e/projects.spec.ts
git commit -m "test(e2e): project create, list and settings flows"
```

---

### Task 16: Aggiorna il seed con categorie e lead

**Files:**
- Modify: `cmd/seed/main.go`

- [ ] **Step 1: Aggiungi una categoria e assegna lead/assignee al progetto DEMO**

Estendi `cmd/seed/main.go`: dopo la creazione del progetto DEMO, crea (idempotente) una `project.ProjectCategory{ID, Name:"Demo Apps"}` se assente e assegna `CategoryID`, `LeadUserID = admin.ID`, `AssigneeType="PROJECT_LEAD"` al progetto DEMO via `UPDATE`. Usa `errors.Is(gorm.ErrRecordNotFound)` per i check, come nel resto del seeder.

```go
	// categoria demo (idempotente)
	var cat project.ProjectCategory
	if err := s.DB.Where("name = ?", "Demo Apps").First(&cat).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		cat = project.ProjectCategory{ID: uuid.NewString(), Name: "Demo Apps", Description: "Progetti demo"}
		if err := s.DB.Create(&cat).Error; err != nil {
			log.Fatalf("create category: %v", err)
		}
		fmt.Println("created category Demo Apps")
	} else if err != nil {
		log.Fatalf("check category: %v", err)
	}
	s.DB.Model(&project.Project{}).Where("key = ?", "DEMO").Updates(map[string]any{
		"category_id": cat.ID, "lead_user_id": admin.ID, "assignee_type": "PROJECT_LEAD",
	})
```

Aggiungi gli import necessari (`github.com/google/uuid`, `github.com/open-jira/open-jira/internal/domain/project` già presente).

- [ ] **Step 2: Verifica idempotenza**

Run:
```bash
rm -f /tmp/seed-r1.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/seed-r1.db go run ./cmd/seed && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/seed-r1.db go run ./cmd/seed && sqlite3 /tmp/seed-r1.db 'select count(*) from project_categories; select key,category_id,lead_user_id,assignee_type from projects;' && rm -f /tmp/seed-r1.db
```
Expected: 1 categoria; DEMO con category_id/lead_user_id valorizzati e assignee_type=PROJECT_LEAD; seconda run non duplica.

- [ ] **Step 3: Commit**

```bash
git add cmd/seed/main.go
git commit -m "feat(seed): demo project category, lead and assignee type"
```

---

## Definition of Done del Round 1

- `go build ./... && go vet ./... && go test ./...` verdi.
- Contract test verdi per: `GET /project/{idOrKey}`, `POST /project`, `GET /project/search`, `PUT /project/{idOrKey}`, `POST /project/{key}/archive` + `/restore`, `GET /project/type`, `POST /projectCategory`.
- `docs/contracts/gap-report.md` rigenerato e committato.
- Frontend build pulito; UI lista/creazione/impostazioni sui campi v3.
- E2E Playwright verdi per creazione + impostazioni progetto.
- Confronto UI con Jira reale (quando l'estensione Chrome è disponibile): lista progetti e schermata "Create project".

## Note e follow-up ereditati

- Migrazione del handler di **login/register** al formato errore v3 `{errorMessages,errors}` (oggi `{"error":...}`) — ereditato dal Round 0, opportuno farlo qui o in un round auth dedicato.
- Endpoint `members`/`invites` restano in forma custom (non standard Jira) — decidere in un round successivo se mapparli su `role`/`permission scheme`.
- Fixture E2E condivise (credenziali seed) e isolamento DB per-worker Playwright — miglioramento tracciato dal Round 0, da introdurre quando gli spec E2E si moltiplicano.
