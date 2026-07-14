# Round 9 — Integrazioni Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dare a open-jira i webhook in uscita reali (registrazione + consegna HTTP firmata + log), il wiring degli eventi issue verso webhook e regole di automation, l'auto-commento Git sui commit che referenziano una issue, e una UI Integrazioni (Git provider + webhook + automation) con pannello Development sulla issue.

**Architecture:** Il grosso è già scheletrato: dominio `git` (provider Forgejo/GitHub/GitLab, webhook inbound, link commit/branch/PR) e dominio `automation` (regole trigger→condizione→azione, `ProcessRules`) esistono e sono montati. Round 9 aggiunge (1) un dominio `webhook` (CRUD sulla tabella `webhooks` già presente + consegne su nuova tabella `webhook_deliveries`) con un **delivery client HTTP** che firma il payload in HMAC-SHA256; (2) un `EventSink` su `issue.Service` (stesso pattern del `Notifier` esistente) che a create/update/transition invoca un `integration.Dispatcher` — il quale consegna ai webhook del progetto e fa partire le regole di automation per il trigger giusto (`issue_created`/`issue_updated`/`issue_transitioned`); (3) l'auto-commento Git; (4) la UI. I webhook v3 (dynamic, Connect/OAuth, con scadenza) NON combaciano col nostro modello per-progetto: restiamo su `/project/{key}/webhooks` come estensione (dichiarato).

**Tech Stack:** Go 1.25 (net/http, GORM, golang-migrate, SQLite in test, `net/http/httptest` per i test di delivery, `crypto/hmac`+`crypto/sha256` per la firma), domini `internal/domain/{webhook,git,automation,issue,notification}`, harness `internal/contract`. Frontend Next.js 16 + React 19 + TanStack Query + Tailwind + Playwright.

---

## Contesto per l'implementatore (leggere una volta)

**Codice ESISTENTE (verificato):**
- Tabelle già presenti (`migrations/000001_init_schema.up.sql`): `webhooks` (`id PK, project_id FK→projects CASCADE, url NOT NULL, secret DEFAULT '', events_json DEFAULT '[]', is_active BOOLEAN DEFAULT TRUE, created_at`) — **nessun handler/service, solo un log-stub nel worker**. `git_providers`, `issue_commits`, `issue_branches`, `issue_pull_requests` (Git, completi). `automation_rules` (`id, project_id, name, is_active, trigger_type, conditions_json, actions_json, created_at`), `automation_runs`.
- `internal/domain/git`: `ConfigService` (Create/GetProvider/FindByWebhookToken/DeleteProvider/LinkCommit/LinkBranch/LinkPullRequest/GetIssueCommits|Branches|PullRequests/DB), provider interface + Forgejo/GitHub/GitLab, `ExtractIssueKeys(regex [A-Z]+-\d+)`. Handler `GitHandler` (ConfigureProvider/GetProvider/DeleteProvider/Webhook/GetIssueGitInfo). Rotte: `POST /rest/api/3/webhooks/git/{token}`, `GET /rest/api/3/issue/{issueKey}/git`, e le config provider sotto `/project/{key}/...`. `processPushEvent` fa `issueSvc.GetByKey` + `LinkCommit` per ogni issue key.
- `internal/domain/automation`: `NewService(db)`, `ProcessRules(triggerType, issueID string)` (esegue le regole attive del progetto per quel trigger), azioni `set_assignee/add_label/transition_issue/add_comment`, condizioni `priority/title_contains`, trigger `issue_created/issue_updated/issue_transitioned`. Handler `AutomationHandler` (ListRules/CreateRule/GetRule/UpdateRule/DeleteRule/ExecuteRule/ListRuns) montato su `/project/{projectID}/automation` e `/automation/{ruleID}[...]`. **NOTA bug**: `automation_rules.actions_json` default migrazione `'{}'` ma il servizio lo unmarshalla come array `[]` — usare sempre `'[]'` e tollerare valori vuoti.
- `internal/domain/issue/service.go`: `Service{db, notifier Notifier}`, `SetNotifier(n)`, `Notifier` interface. `Create(...)` e `Update(key, title, descriptionJSON *string, priority *Priority, assigneeID, statusID *string, storyPoints *int)`. **Il wiring eventi seguirà lo STESSO pattern del notifier** (campo opzionale + setter). `GetByKey(key)`, `DB()`. `Issue{ID, Key, Title, ProjectID, StatusID *string, ...}`.
- `internal/domain/issue/comment_service.go`: `CommentService`, `NewCommentService(db)`, `AddComment(issueID, authorID, bodyJSON string)(*Comment,error)` (scrive history + notifica). Per l'auto-commento Git.
- `internal/domain/notification`: `Service.Create(userID, notifType, title, body, link string) error`.
- Router `internal/api/router.go`: costruisce `gitConfigSvc := git.NewConfigService(db)` (:72), `autoSvc := automation.NewService(db)` (:81), `issueSvc`, `projectSvc`, `commentSvc`(?), `authMw`, `mux`, `cfg.BaseURL`. `middleware.UserIDFromContext`.
- **Nessun client HTTP outbound in produzione** — va creato `&http.Client{Timeout: 10*time.Second}`.

**Migrazioni:** ultima `000014`. Prossima **`000015`**.

**Scelte di scope (esplicite, follow-up):**
- Webhook v3 dynamic (`/rest/api/3/webhook` Connect/OAuth, con `refresh`/scadenza) → NON implementato; usiamo il modello per-progetto `/project/{key}/webhooks` (estensione). Consegna: fire-and-forget con goroutine + log su `webhook_deliveries`; NIENTE retry/backoff (follow-up).
- Git dev-info conforme (`/rest/devinfo/0.10/*`) → fuori scope (spec separata); si usa l'endpoint esistente `GET /issue/{key}/git`.
- L'enforcement dei permessi (dal R8) resta un follow-up del Round 10 — Round 9 non lo affronta.
- Il vecchio polling del worker (`processWebhookDeliveries`/`processAutomationRules`) resta ma diventa ridondante rispetto al firing event-driven → nota: si può disattivare/ridurre, ma non è obbligatorio per questo round.

**Harness contract / test:** `newTestServer`, `registerAndLogin`, `createProjectViaAPI`, `createIssueViaAPI`, `doJSON`, `decodeBody`, `MustLoad` (i webhook sono estensioni custom → i test verificano status+shape+comportamento, non `ValidateResponse`). Per il delivery HTTP usare `net/http/httptest.NewServer` per catturare la POST firmata.

---

## Struttura dei file

**Migrazioni:** `migrations/000015_webhook_deliveries.up.sql` / `.down.sql`.

**Backend:**
- `internal/domain/webhook/model.go` — `Webhook` (tabella `webhooks`), `Delivery` (tabella `webhook_deliveries`).
- `internal/domain/webhook/service.go` — CRUD + `ListActiveForEvent(projectID, eventType)` + `RecordDelivery`.
- `internal/domain/webhook/delivery.go` — `Sign(secret, body)`, `Deliver(client, hook, eventType, body)(Delivery)`.
- `internal/integration/dispatcher.go` — `Dispatcher` che implementa `issue.EventSink`: consegna webhook + `automation.ProcessRules`.
- `internal/domain/issue/service.go` — `EventSink` interface + `SetEventSink` + chiamate in Create/Update.
- `internal/api/handlers/webhook_handler.go` — CRUD `/project/{key}/webhooks`.
- `internal/api/handlers/git_handler.go` — auto-commento in `processPushEvent`.
- `internal/api/router.go` — rotte webhook + costruzione dispatcher + `issueSvc.SetEventSink`.

**Frontend:**
- `frontend-next/lib/api.ts` — client `integrations` (gitProvider get/configure/delete, webhooks list/create/delete, automation listRules) + `issueGit` (get commits/branches/prs).
- `frontend-next/components/projects/IntegrationsTab.tsx` + tab in `ProjectSettings`.
- `frontend-next/components/issues/DevelopmentPanel.tsx` + montaggio in `IssueView`.
- `frontend-next/e2e/integrations.spec.ts`.

**Seed:** `cmd/seed/main.go` — un webhook demo (idempotente).

---

### Task 1: Migrazione 000015 — webhook_deliveries

**Files:**
- Create: `migrations/000015_webhook_deliveries.up.sql`
- Create: `migrations/000015_webhook_deliveries.down.sql`

- [ ] **Step 1: up**

`migrations/000015_webhook_deliveries.up.sql`:

```sql
CREATE TABLE webhook_deliveries (
    id TEXT PRIMARY KEY,
    webhook_id TEXT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    url TEXT NOT NULL,
    status_code INTEGER DEFAULT 0,
    success BOOLEAN DEFAULT FALSE,
    error TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 2: down**

`migrations/000015_webhook_deliveries.down.sql`:

```sql
DROP TABLE IF EXISTS webhook_deliveries;
```

- [ ] **Step 3: Verificare**

Run: `rm -f /tmp/mig15.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/mig15.db go run ./cmd/seed && rm -f /tmp/mig15.db`
Expected: `seed complete`, exit 0.

- [ ] **Step 4: Commit**

```bash
git add migrations/000015_webhook_deliveries.up.sql migrations/000015_webhook_deliveries.down.sql
git commit -m "feat(migrations): webhook_deliveries table"
```

---

### Task 2: Dominio webhook — modello + service

**Files:**
- Create: `internal/domain/webhook/model.go`
- Create: `internal/domain/webhook/service.go`
- Test: `internal/domain/webhook/service_test.go`

- [ ] **Step 1: Modello**

`internal/domain/webhook/model.go`:

```go
package webhook

import "time"

// Webhook è una registrazione webhook per-progetto (estensione: non il modello
// dynamic-webhook Connect/OAuth di Jira). events è un JSON array di stringhe.
type Webhook struct {
	ID         string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID  string    `gorm:"column:project_id;type:text;not null;index" json:"project_id"`
	URL        string    `gorm:"column:url;type:text;not null" json:"url"`
	Secret     string    `gorm:"column:secret;type:text;default:''" json:"secret,omitempty"`
	EventsJSON string    `gorm:"column:events_json;type:text;default:'[]'" json:"-"`
	IsActive   bool      `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Webhook) TableName() string { return "webhooks" }

// Delivery è il log di un tentativo di consegna.
type Delivery struct {
	ID         string    `gorm:"primaryKey;type:text" json:"id"`
	WebhookID  string    `gorm:"column:webhook_id;type:text;not null;index" json:"webhook_id"`
	EventType  string    `gorm:"column:event_type;type:text;not null" json:"event_type"`
	URL        string    `gorm:"column:url;type:text;not null" json:"url"`
	StatusCode int       `gorm:"column:status_code" json:"status_code"`
	Success    bool      `gorm:"column:success" json:"success"`
	Error      string    `gorm:"column:error;type:text;default:''" json:"error,omitempty"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Delivery) TableName() string { return "webhook_deliveries" }
```

- [ ] **Step 2: Test**

`internal/domain/webhook/service_test.go`:

```go
package webhook

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
	if err := db.AutoMigrate(&Webhook{}, &Delivery{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateAndList(t *testing.T) {
	svc := NewService(newDB(t))
	h, err := svc.Create("proj-1", "https://example.com/hook", "s3cr3t", []string{"issue_created", "issue_updated"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if h.ID == "" || h.URL != "https://example.com/hook" {
		t.Errorf("webhook errato: %+v", h)
	}
	list, err := svc.ListByProject("proj-1")
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("attesa 1 webhook, %d", len(list))
	}
	if got := list[0].Events(); len(got) != 2 || got[0] != "issue_created" {
		t.Errorf("events non deserializzati: %v", got)
	}
}

func TestListActiveForEvent(t *testing.T) {
	svc := NewService(newDB(t))
	svc.Create("proj-1", "https://a", "", []string{"issue_created"})
	svc.Create("proj-1", "https://b", "", []string{"issue_updated"})
	svc.Create("proj-2", "https://c", "", []string{"issue_created"})
	hooks, err := svc.ListActiveForEvent("proj-1", "issue_created")
	if err != nil {
		t.Fatalf("ListActiveForEvent: %v", err)
	}
	if len(hooks) != 1 || hooks[0].URL != "https://a" {
		t.Errorf("filtro evento errato: %+v", hooks)
	}
}

func TestDeleteAndRecordDelivery(t *testing.T) {
	db := newDB(t)
	svc := NewService(db)
	h, _ := svc.Create("proj-1", "https://a", "", []string{"issue_created"})
	if err := svc.RecordDelivery(h.ID, "issue_created", h.URL, 200, true, ""); err != nil {
		t.Fatalf("RecordDelivery: %v", err)
	}
	var cnt int64
	db.Model(&Delivery{}).Where("webhook_id = ?", h.ID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("attesa 1 delivery, %d", cnt)
	}
	if err := svc.Delete(h.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if l, _ := svc.ListByProject("proj-1"); len(l) != 0 {
		t.Error("webhook dovrebbe essere eliminato")
	}
}
```

- [ ] **Step 3: Eseguire (falliscono)**

Run: `go test ./internal/domain/webhook/ -v`
Expected: FAIL con "undefined: NewService".

- [ ] **Step 4: Service**

`internal/domain/webhook/service.go`:

```go
package webhook

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Events deserializza EventsJSON in slice (vuoto se non valido).
func (w *Webhook) Events() []string {
	var out []string
	if w.EventsJSON == "" {
		return out
	}
	_ = json.Unmarshal([]byte(w.EventsJSON), &out)
	return out
}

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) Create(projectID, url, secret string, events []string) (*Webhook, error) {
	ev, _ := json.Marshal(events)
	h := &Webhook{ID: uuid.NewString(), ProjectID: projectID, URL: url, Secret: secret, EventsJSON: string(ev), IsActive: true}
	if err := s.db.Create(h).Error; err != nil {
		return nil, err
	}
	return h, nil
}

func (s *Service) ListByProject(projectID string) ([]Webhook, error) {
	var hooks []Webhook
	if err := s.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&hooks).Error; err != nil {
		return nil, err
	}
	return hooks, nil
}

func (s *Service) Delete(id string) error {
	return s.db.Where("id = ?", id).Delete(&Webhook{}).Error
}

// ListActiveForEvent restituisce i webhook attivi del progetto sottoscritti a eventType.
func (s *Service) ListActiveForEvent(projectID, eventType string) ([]Webhook, error) {
	var hooks []Webhook
	if err := s.db.Where("project_id = ? AND is_active = ?", projectID, true).Find(&hooks).Error; err != nil {
		return nil, err
	}
	out := make([]Webhook, 0, len(hooks))
	for _, h := range hooks {
		for _, e := range h.Events() {
			if e == eventType {
				out = append(out, h)
				break
			}
		}
	}
	return out, nil
}

func (s *Service) RecordDelivery(webhookID, eventType, url string, statusCode int, success bool, errMsg string) error {
	d := &Delivery{ID: uuid.NewString(), WebhookID: webhookID, EventType: eventType, URL: url, StatusCode: statusCode, Success: success, Error: errMsg}
	return s.db.Create(d).Error
}
```

- [ ] **Step 5: Eseguire (passano)**

Run: `go test ./internal/domain/webhook/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/webhook/
git commit -m "feat(webhook): outbound webhook domain (registration + delivery log)"
```

---

### Task 3: Webhook delivery — firma HMAC + POST HTTP

**Files:**
- Create: `internal/domain/webhook/delivery.go`
- Test: `internal/domain/webhook/delivery_test.go`

- [ ] **Step 1: Test (con httptest)**

`internal/domain/webhook/delivery_test.go`:

```go
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSign(t *testing.T) {
	got := Sign("secret", []byte("body"))
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("body"))
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got != want {
		t.Errorf("Sign = %q want %q", got, want)
	}
}

func TestDeliver_PostsSignedPayload(t *testing.T) {
	var gotSig, gotBody, gotEvent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-OpenJira-Signature")
		gotEvent = r.Header.Get("X-OpenJira-Event")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	hook := Webhook{ID: "h1", URL: srv.URL, Secret: "topsecret"}
	body := []byte(`{"event":"issue_created"}`)
	d := Deliver(client, hook, "issue_created", body)

	if !d.Success || d.StatusCode != 200 {
		t.Errorf("delivery non riuscita: %+v", d)
	}
	if gotBody != string(body) {
		t.Errorf("body errato: %q", gotBody)
	}
	if gotEvent != "issue_created" {
		t.Errorf("event header errato: %q", gotEvent)
	}
	if gotSig != Sign("topsecret", body) || !strings.HasPrefix(gotSig, "sha256=") {
		t.Errorf("firma errata: %q", gotSig)
	}
}

func TestDeliver_RecordsFailureOnBadURL(t *testing.T) {
	client := &http.Client{Timeout: 1 * time.Second}
	hook := Webhook{ID: "h1", URL: "http://127.0.0.1:0/nope", Secret: ""}
	d := Deliver(client, hook, "issue_created", []byte("{}"))
	if d.Success {
		t.Error("delivery verso URL non valida deve fallire")
	}
	if d.Error == "" {
		t.Error("errore atteso valorizzato")
	}
}
```

- [ ] **Step 2: Eseguire (falliscono)**

Run: `go test ./internal/domain/webhook/ -run 'TestSign|TestDeliver' -v`
Expected: FAIL con "undefined: Sign/Deliver".

- [ ] **Step 3: Implementare**

`internal/domain/webhook/delivery.go`:

```go
package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
)

// Sign calcola la firma HMAC-SHA256 del body col secret, come "sha256=<hex>".
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Deliver esegue la POST del payload al webhook, con header di firma ed evento,
// e restituisce una Delivery (non persistita: il chiamante la registra). Non
// solleva: cattura gli errori nel campo Error della Delivery.
func Deliver(client *http.Client, hook Webhook, eventType string, body []byte) Delivery {
	d := Delivery{WebhookID: hook.ID, EventType: eventType, URL: hook.URL}
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		d.Error = err.Error()
		return d
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenJira-Event", eventType)
	req.Header.Set("X-OpenJira-Signature", Sign(hook.Secret, body))
	resp, err := client.Do(req)
	if err != nil {
		d.Error = err.Error()
		return d
	}
	defer resp.Body.Close()
	d.StatusCode = resp.StatusCode
	d.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	if !d.Success {
		d.Error = fmt.Sprintf("non-2xx status: %d", resp.StatusCode)
	}
	return d
}
```

- [ ] **Step 4: Eseguire (passano)**

Run: `go test ./internal/domain/webhook/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/webhook/delivery.go internal/domain/webhook/delivery_test.go
git commit -m "feat(webhook): HTTP delivery with HMAC-SHA256 signature"
```

---

### Task 4: Handler webhook + rotte

**Files:**
- Create: `internal/api/handlers/webhook_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Handler**

`internal/api/handlers/webhook_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/webhook"
)

type WebhookHandler struct {
	svc        *webhook.Service
	projectSvc *project.Service
}

func NewWebhookHandler(svc *webhook.Service, projectSvc *project.Service) *WebhookHandler {
	return &WebhookHandler{svc: svc, projectSvc: projectSvc}
}

// webhookOut è la rappresentazione di risposta (events come array, secret nascosto).
type webhookOut struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"project_id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	IsActive  bool     `json:"is_active"`
}

func toOut(h webhook.Webhook) webhookOut {
	return webhookOut{ID: h.ID, ProjectID: h.ProjectID, URL: h.URL, Events: h.Events(), IsActive: h.IsActive}
}

// List: GET /rest/api/3/project/{key}/webhooks.
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	hooks, err := h.svc.ListByProject(p.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list webhooks"}, nil)
		return
	}
	out := make([]webhookOut, 0, len(hooks))
	for _, hook := range hooks {
		out = append(out, toOut(hook))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// Create: POST /rest/api/3/project/{key}/webhooks {url, secret?, events[]}.
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"project not found"}, nil)
		return
	}
	var req struct {
		URL    string   `json:"url"`
		Secret string   `json:"secret"`
		Events []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"url is required"}, nil)
		return
	}
	if len(req.Events) == 0 {
		req.Events = []string{"issue_created", "issue_updated", "issue_transitioned"}
	}
	hook, err := h.svc.Create(p.ID, req.URL, req.Secret, req.Events)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to create webhook"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, toOut(*hook))
}

// Delete: DELETE /rest/api/3/project/{key}/webhooks/{id}.
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.PathValue("id")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete webhook"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Rotte + costruzione service**

In `internal/api/router.go`:

```go
	webhookSvc := webhook.NewService(db)
	webhookH := handlers.NewWebhookHandler(webhookSvc, projectSvc)
```
```go
	mux.Handle("GET /rest/api/3/project/{key}/webhooks", authMw(http.HandlerFunc(webhookH.List)))
	mux.Handle("POST /rest/api/3/project/{key}/webhooks", authMw(http.HandlerFunc(webhookH.Create)))
	mux.Handle("DELETE /rest/api/3/project/{key}/webhooks/{id}", authMw(http.HandlerFunc(webhookH.Delete)))
```
Aggiungere import `"github.com/open-jira/open-jira/internal/domain/webhook"`. (Il `webhookSvc` sarà riusato dal dispatcher in Task 5 — dichiararlo prima del dispatcher.)

- [ ] **Step 3: Build + vet**

Run: `go build ./... && go vet ./internal/api/...`
Expected: compila.

- [ ] **Step 4: Commit**

```bash
git add internal/api/handlers/webhook_handler.go internal/api/router.go
git commit -m "feat(api): per-project webhook CRUD endpoints"
```

---

### Task 5: Event dispatcher + wiring issue.Service

**Files:**
- Create: `internal/integration/dispatcher.go`
- Modify: `internal/domain/issue/service.go` (EventSink + hook in Create/Update)
- Modify: `internal/api/router.go` (costruire dispatcher, `issueSvc.SetEventSink`)
- Test: `internal/integration/dispatcher_test.go`

- [ ] **Step 1: EventSink su issue.Service**

In `internal/domain/issue/service.go`:
- aggiungere al `Service` struct il campo `eventSink EventSink` (accanto a `notifier`);
- definire l'interfaccia e il setter:
```go
// EventSink riceve eventi di dominio sulle issue (per integrazioni: webhook, automation).
type EventSink interface {
	IssueEvent(eventType string, iss *Issue)
}

func (s *Service) SetEventSink(e EventSink) { s.eventSink = e }

func (s *Service) emit(eventType string, iss *Issue) {
	if s.eventSink != nil {
		s.eventSink.IssueEvent(eventType, iss)
	}
}
```
- in `Create(...)`, dopo che l'issue è creata con successo (prima del return): `s.emit("issue_created", issue)`.
- in `Update(...)`, dopo aver applicato gli update con successo: emettere `s.emit("issue_updated", issue)`; se lo `statusID` è cambiato (il blocco `if statusID != nil` con `*statusID != old`), emettere anche `s.emit("issue_transitioned", issue)`. Usare la variabile issue aggiornata (ricaricare o aggiornare i campi in memoria se necessario perché `emit` legga `iss.ProjectID`/`iss.Key`).

> **Nota implementatore:** leggere Create/Update per i nomi esatti delle variabili (`issue`/`iss`). L'`emit` va chiamato SOLO su successo (dopo che il DB update non ha errori). Per `issue_transitioned`, riusare la stessa condizione di cambio-status già presente per il notifier (`*statusID != old`).

- [ ] **Step 2: Test dispatcher**

`internal/integration/dispatcher_test.go`:

```go
package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/webhook"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&webhook.Webhook{}, &webhook.Delivery{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDispatcher_DeliversToMatchingWebhook(t *testing.T) {
	db := newDB(t)
	whSvc := webhook.NewService(db)
	received := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received <- string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	whSvc.Create("proj-1", srv.URL, "s", []string{"issue_created"})

	d := NewDispatcher(whSvc, nil, &http.Client{Timeout: 5 * time.Second})
	d.IssueEvent("issue_created", &issue.Issue{ID: "i1", Key: "P-1", Title: "Hello", ProjectID: "proj-1"})

	select {
	case body := <-received:
		if body == "" {
			t.Error("payload vuoto")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("webhook non consegnato entro il timeout")
	}
	// la delivery è registrata
	var cnt int64
	db.Model(&webhook.Delivery{}).Count(&cnt)
	// attende breve per la goroutine di record (se async): riprova
	for i := 0; i < 20 && cnt == 0; i++ {
		time.Sleep(50 * time.Millisecond)
		db.Model(&webhook.Delivery{}).Count(&cnt)
	}
	if cnt == 0 {
		t.Error("delivery non registrata")
	}
}

func TestDispatcher_SkipsNonMatchingEvent(t *testing.T) {
	db := newDB(t)
	whSvc := webhook.NewService(db)
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true; w.WriteHeader(200) }))
	defer srv.Close()
	whSvc.Create("proj-1", srv.URL, "", []string{"issue_updated"})

	d := NewDispatcher(whSvc, nil, &http.Client{Timeout: 2 * time.Second})
	d.IssueEvent("issue_created", &issue.Issue{ID: "i1", Key: "P-1", ProjectID: "proj-1"})
	time.Sleep(300 * time.Millisecond)
	if hit {
		t.Error("un evento non sottoscritto non deve consegnare")
	}
}
```

- [ ] **Step 3: Eseguire (falliscono)**

Run: `go test ./internal/integration/ -v`
Expected: FAIL con "undefined: NewDispatcher".

- [ ] **Step 4: Dispatcher**

`internal/integration/dispatcher.go`:

```go
// Package integration collega gli eventi di dominio (issue) alle integrazioni:
// consegna dei webhook in uscita e attivazione delle regole di automation.
package integration

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/webhook"
)

// AutomationRunner è l'astrazione minima dell'automation service (per non
// accoppiare rigidamente e per testare con nil).
type AutomationRunner interface {
	ProcessRules(triggerType, issueID string)
}

// Dispatcher implementa issue.EventSink.
type Dispatcher struct {
	webhookSvc *webhook.Service
	auto       AutomationRunner
	client     *http.Client
	wg         sync.WaitGroup // per attendere le consegne nei test se serve
}

func NewDispatcher(webhookSvc *webhook.Service, auto AutomationRunner, client *http.Client) *Dispatcher {
	return &Dispatcher{webhookSvc: webhookSvc, auto: auto, client: client}
}

// IssueEvent consegna l'evento ai webhook sottoscritti (async) e alle regole di
// automation (sincrono: solo DB, veloce).
func (d *Dispatcher) IssueEvent(eventType string, iss *issue.Issue) {
	// automation (sincrono)
	if d.auto != nil {
		d.auto.ProcessRules(eventType, iss.ID)
	}
	// webhook (async fire-and-forget)
	hooks, err := d.webhookSvc.ListActiveForEvent(iss.ProjectID, eventType)
	if err != nil || len(hooks) == 0 {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"event": eventType,
		"issue": map[string]any{"id": iss.ID, "key": iss.Key, "summary": iss.Title, "projectId": iss.ProjectID},
	})
	for _, h := range hooks {
		hook := h
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			del := webhook.Deliver(d.client, hook, eventType, payload)
			_ = d.webhookSvc.RecordDelivery(hook.ID, eventType, hook.URL, del.StatusCode, del.Success, del.Error)
		}()
	}
}

// Wait attende le consegne in volo (usato dai test).
func (d *Dispatcher) Wait() { d.wg.Wait() }
```

- [ ] **Step 5: Eseguire (passano)**

Run: `go test ./internal/integration/ ./internal/domain/webhook/ ./internal/domain/issue/ -v`
Expected: PASS (i test issue esistenti non devono rompersi con l'aggiunta di emit — l'eventSink è nil se non impostato).

- [ ] **Step 6: Wiring nel router**

In `internal/api/router.go`, dopo aver costruito `webhookSvc`, `autoSvc`, `issueSvc`:
```go
	dispatcher := integration.NewDispatcher(webhookSvc, autoSvc, &http.Client{Timeout: 10 * time.Second})
	issueSvc.SetEventSink(dispatcher)
```
Aggiungere import `"github.com/open-jira/open-jira/internal/integration"` e `"net/http"`/`"time"` (probabilmente già presenti). VERIFICARE che `autoSvc` soddisfi `integration.AutomationRunner` (ha `ProcessRules(string, string)`); se la firma differisce, adeguare l'interfaccia.

- [ ] **Step 7: Build + gate**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | grep -vE '^ok|no test'`
Expected: verde.

- [ ] **Step 8: Commit**

```bash
git add internal/integration/ internal/domain/issue/service.go internal/api/router.go
git commit -m "feat(integration): event dispatcher wiring issue events to webhooks and automation"
```

---

### Task 6: Git — auto-commento sul commit che referenzia una issue

**Files:**
- Modify: `internal/api/handlers/git_handler.go`

- [ ] **Step 1: Aggiungere l'auto-commento in processPushEvent**

Leggere `processPushEvent` (`git_handler.go:162`). Dopo `gitConfigSvc.LinkCommit(iss.ID, cfg.ID, sha, message, author)` andato a buon fine, aggiungere un commento ADF sulla issue tramite il `CommentService`. Il `GitHandler` non ha ancora il comment service: aggiungerlo al struct + costruttore.

- estendere lo struct e il costruttore:
```go
type GitHandler struct {
	gitConfigSvc *git.ConfigService
	issueSvc     *issue.Service
	projectSvc   *project.Service
	commentSvc   *issue.CommentService // nuovo
}

func NewGitHandler(gitConfigSvc *git.ConfigService, issueSvc *issue.Service, projectSvc *project.Service, commentSvc *issue.CommentService) *GitHandler {
	return &GitHandler{gitConfigSvc: gitConfigSvc, issueSvc: issueSvc, projectSvc: projectSvc, commentSvc: commentSvc}
}
```
- dopo il LinkCommit riuscito, un commento (autore vuoto = sistema; body ADF con lo sha breve e il messaggio):
```go
	if h.commentSvc != nil {
		short := sha
		if len(short) > 8 {
			short = short[:8]
		}
		body := fmt.Sprintf(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":%q}]}]}`,
			fmt.Sprintf("Commit %s referenced this issue: %s", short, message))
		_, _ = h.commentSvc.AddComment(iss.ID, "", body)
	}
```
(Import `fmt` se non presente. Il commento con `AddComment` scrive anche history + eventuale notifica.)

- aggiornare la costruzione in `internal/api/router.go`: passare il comment service (esiste già come `commentSvc` per le rotte commenti — verificare il nome; se assente, costruirlo con `issue.NewCommentService(db)`).

> **Nota implementatore:** VERIFICARE il nome reale del comment service nel router e la firma di `AddComment(issueID, authorID, bodyJSON)`. Evitare doppioni: se un commit compare in più push (stesso sha), l'auto-commento si ripeterebbe — accettabile per ora (nota follow-up: dedurre da `issue_commits` se lo sha era già linkato e saltare il commento). Mantenere l'ADF valido (una sola riga di testo).

- [ ] **Step 2: Build + vet**

Run: `go build ./... && go vet ./...`
Expected: verde.

- [ ] **Step 3: Commit**

```bash
git add internal/api/handlers/git_handler.go internal/api/router.go
git commit -m "feat(git): auto-comment on issue when a commit references it"
```

---

### Task 7: Test integrazione — delivery end-to-end + evento issue

**Files:**
- Create: `internal/contract/webhook_test.go`

- [ ] **Step 1: Test**

`internal/contract/webhook_test.go` (usare gli helper reali dell'harness; per il ricevitore usare `httptest.NewServer`):

```go
package contract

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhook_CRUD(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "WH", "Webhook Proj")

	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/project/WH/webhooks", map[string]any{
		"url": "https://example.com/hook", "events": []string{"issue_created"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook %d", resp.StatusCode)
	}
	created := decodeBody(t, resp)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("webhook senza id")
	}

	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/project/WH/webhooks", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list webhooks %d", resp.StatusCode)
	}

	resp = doJSON(t, srv, http.MethodDelete, tok, "/rest/api/3/project/WH/webhooks/"+id, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete webhook %d", resp.StatusCode)
	}
}

func TestWebhook_FiresOnIssueCreate(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "WH", "Webhook Proj")

	received := make(chan string, 4)
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- r.Header.Get("X-OpenJira-Event"):
		default:
		}
		w.WriteHeader(200)
	}))
	defer recv.Close()

	// registra un webhook sull'evento issue_created verso il ricevitore locale
	doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/project/WH/webhooks", map[string]any{
		"url": recv.URL, "events": []string{"issue_created"},
	})

	// crea una issue → deve scatenare la consegna
	createIssueViaAPI(t, srv, tok, "WH", "Trigger me")

	select {
	case ev := <-received:
		if ev != "issue_created" {
			t.Errorf("evento header errato: %q", ev)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("webhook non consegnato dopo la creazione della issue")
	}
}
```

- [ ] **Step 2: Eseguire**

Run: `go test ./internal/contract/ -run 'TestWebhook' -v`
Expected: PASS. (Il secondo test verifica il wiring end-to-end: createIssue → dispatcher → delivery.)

- [ ] **Step 3: Suite completa**

Run: `go test ./...`
Expected: verde.

- [ ] **Step 4: Commit**

```bash
git add internal/contract/webhook_test.go
git commit -m "test(integration): webhook CRUD and fire-on-issue-create end-to-end"
```

---

### Task 8: Frontend — client integrazioni

**Files:**
- Modify: `frontend-next/lib/api.ts`

- [ ] **Step 1: Client**

In `frontend-next/lib/api.ts`:

```ts
export interface Webhook { id: string; project_id: string; url: string; events: string[]; is_active: boolean }
export interface GitProviderConfig { id: string; provider_type: string; base_url: string }
export interface IssueGitInfo {
  commits: { commit_sha: string; message: string; author: string }[];
  branches: { branch_name: string; repo_url: string }[];
  pull_requests: { pr_number: number; title: string; url: string; state: string }[];
}
export interface AutomationRule { id: string; name: string; trigger_type: string; is_active: boolean }

export const integrations = {
  webhooks: (projectKey: string) => apiFetch<Webhook[]>(`/rest/api/3/project/${projectKey}/webhooks`),
  createWebhook: (projectKey: string, url: string, events: string[]) =>
    apiFetch<Webhook>(`/rest/api/3/project/${projectKey}/webhooks`, { method: "POST", body: JSON.stringify({ url, events }) }),
  deleteWebhook: (projectKey: string, id: string) =>
    apiFetch<void>(`/rest/api/3/project/${projectKey}/webhooks/${id}`, { method: "DELETE" }),
  gitProvider: (projectKey: string) => apiFetch<GitProviderConfig | null>(`/rest/api/3/project/${projectKey}/git-provider`),
  configureGit: (projectKey: string, body: { provider_type: string; base_url: string; token: string; webhook_secret: string }) =>
    apiFetch<GitProviderConfig>(`/rest/api/3/project/${projectKey}/git-provider`, { method: "POST", body: JSON.stringify(body) }),
  automationRules: (projectId: string) => apiFetch<AutomationRule[]>(`/rest/api/3/project/${projectId}/automation`),
};

export const issueGit = {
  info: (issueKey: string) => apiFetch<IssueGitInfo>(`/rest/api/3/issue/${issueKey}/git`),
};
```

> **Nota implementatore (CRITICO):** VERIFICARE le rotte REALI del git provider config in `internal/api/router.go` (lo scout indica `ConfigureProvider`/`GetProvider`/`DeleteProvider` sotto `/project/{key}/...` — trovare il path esatto, es. `/project/{key}/git-provider` o simile, e adeguare i path del client). Verificare lo shape di `GET /issue/{key}/git` (`GetIssueGitInfo` restituisce `{commits, branches, pull_requests}` — confermare le chiavi dei sottocampi leggendo l'handler). Verificare lo shape della lista automation (`ListRules`). Confermare `apiFetch`. Adeguare i tipi TS di conseguenza.

- [ ] **Step 2: Type-check**

Run: `cd frontend-next && npx tsc --noEmit`
Expected: nessun errore.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): integrations API client (webhooks, git provider, automation)"
```

---

### Task 9: Frontend — tab Integrations nelle impostazioni progetto

**Files:**
- Create: `frontend-next/components/projects/IntegrationsTab.tsx`
- Modify: `frontend-next/components/projects/ProjectSettings.tsx`

- [ ] **Step 1: Componente**

`frontend-next/components/projects/IntegrationsTab.tsx`:

```tsx
"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { integrations, type Webhook } from "@/lib/api";

export function IntegrationsTab({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const [url, setUrl] = useState("");

  const hooks = useQuery({ queryKey: ["webhooks", projectKey], queryFn: () => integrations.webhooks(projectKey) });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["webhooks", projectKey] });

  const create = useMutation({
    mutationFn: () => integrations.createWebhook(projectKey, url, ["issue_created", "issue_updated", "issue_transitioned"]),
    onSuccess: () => { setUrl(""); invalidate(); },
  });
  const del = useMutation({ mutationFn: (id: string) => integrations.deleteWebhook(projectKey, id), onSuccess: invalidate });

  return (
    <div className="space-y-6" data-testid="integrations-tab">
      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Outgoing webhooks</h3>
        <ul className="mb-2 space-y-1" data-testid="webhooks-list">
          {(hooks.data ?? []).map((h: Webhook) => (
            <li key={h.id} className="flex items-center justify-between border-b border-slate-100 py-1 text-sm">
              <span className="truncate text-[#1a1f36]">{h.url}</span>
              <span className="flex items-center gap-2">
                <span className="text-xs text-slate-400">{h.events.join(", ")}</span>
                <button onClick={() => del.mutate(h.id)} className="text-xs text-red-600 hover:underline" aria-label={`Delete webhook ${h.url}`}>Remove</button>
              </span>
            </li>
          ))}
          {hooks.data && hooks.data.length === 0 && <li className="py-2 text-sm text-slate-400">No webhooks</li>}
        </ul>
        <div className="flex gap-2">
          <input aria-label="Webhook URL" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/hook" className="flex-1 rounded border border-slate-300 px-3 py-1.5 text-sm" />
          <button onClick={() => url && create.mutate()} disabled={create.isPending} className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60">Add webhook</button>
        </div>
        <p className="mt-1 text-xs text-slate-400">Fires on issue created / updated / transitioned. Payload is signed with HMAC-SHA256 (X-OpenJira-Signature).</p>
      </section>
    </div>
  );
}
```

> **Nota:** la config Git provider e la lista automation sono utili ma opzionali per questo tab; il webhook CRUD è il cuore. Se il tempo lo consente, aggiungere una sezione "Git provider" (form provider_type/base_url/token/secret → `integrations.configureGit`) e una lista read-only delle regole automation. YAGNI: il webhook basta per l'E2E e il valore.

- [ ] **Step 2: Tab in ProjectSettings**

In `frontend-next/components/projects/ProjectSettings.tsx`, estendere l'union `tab` (attuale `"general" | "workflow" | "summary"` dal R6/R7) con `"integrations"`, aggiungere il bottone tab "Integrations" e renderizzare `<IntegrationsTab projectKey={projectKey} />` sotto `tab === "integrations"`. Mantenere i tab esistenti.

> **Nota:** mirrorare il pattern dei tab esistenti (già estesi 2 volte in R6/R7). Importare `IntegrationsTab`.

- [ ] **Step 3: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/projects/
git commit -m "feat(frontend): integrations tab with webhook management"
```

---

### Task 10: Frontend — pannello Development sulla issue

**Files:**
- Create: `frontend-next/components/issues/DevelopmentPanel.tsx`
- Modify: `frontend-next/components/issues/IssueView.tsx`

- [ ] **Step 1: Componente**

`frontend-next/components/issues/DevelopmentPanel.tsx`:

```tsx
"use client";

import { useQuery } from "@tanstack/react-query";
import { issueGit } from "@/lib/api";

export function DevelopmentPanel({ issueKey }: { issueKey: string }) {
  const info = useQuery({ queryKey: ["issue-git", issueKey], queryFn: () => issueGit.info(issueKey) });
  const d = info.data;
  const empty = d && d.commits.length === 0 && d.branches.length === 0 && d.pull_requests.length === 0;
  if (info.isLoading) return null;
  return (
    <section className="mt-4 rounded border border-slate-200 bg-white p-3" data-testid="development-panel">
      <h3 className="mb-2 text-sm font-semibold text-slate-700">Development</h3>
      {empty && <p className="text-sm text-slate-400">No linked commits, branches or pull requests</p>}
      {d && d.commits.length > 0 && (
        <div className="mb-2">
          <div className="text-xs font-semibold text-slate-500">Commits</div>
          <ul className="text-sm">
            {d.commits.map((c) => (
              <li key={c.commit_sha} className="text-[#1a1f36]">
                <span className="font-mono text-xs text-slate-500">{c.commit_sha.slice(0, 8)}</span> {c.message}
              </li>
            ))}
          </ul>
        </div>
      )}
      {d && d.pull_requests.length > 0 && (
        <div className="mb-2">
          <div className="text-xs font-semibold text-slate-500">Pull requests</div>
          <ul className="text-sm">
            {d.pull_requests.map((p) => (
              <li key={p.pr_number}><a href={p.url} className="text-[#0052cc] hover:underline">#{p.pr_number} {p.title}</a> <span className="text-xs text-slate-400">({p.state})</span></li>
            ))}
          </ul>
        </div>
      )}
      {d && d.branches.length > 0 && (
        <div>
          <div className="text-xs font-semibold text-slate-500">Branches</div>
          <ul className="text-sm text-[#1a1f36]">{d.branches.map((b) => <li key={b.branch_name}>{b.branch_name}</li>)}</ul>
        </div>
      )}
    </section>
  );
}
```

- [ ] **Step 2: Montare in IssueView**

In `frontend-next/components/issues/IssueView.tsx`, dopo la sezione commenti (o nella colonna laterale), montare `<DevelopmentPanel issueKey={issueKey} />`. VERIFICARE la prop reale usata da IssueView per la chiave della issue (probabilmente `issueKey` o dal path). Importare il componente.

> **Nota:** confermare lo shape di `GET /issue/{key}/git` (chiavi `commits`/`branches`/`pull_requests` e i sottocampi `commit_sha`/`message`, `pr_number`/`title`/`url`/`state`, `branch_name`) leggendo `GitHandler.GetIssueGitInfo`; adeguare i tipi client (Task 8) e il rendering se differiscono.

- [ ] **Step 3: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/issues/
git commit -m "feat(frontend): issue development panel (commits/branches/PRs)"
```

---

### Task 11: E2E — integrazioni

**Files:**
- Create: `frontend-next/e2e/integrations.spec.ts`

- [ ] **Step 1: E2E**

`frontend-next/e2e/integrations.spec.ts` (login helper reale da `board.spec.ts`):

```ts
import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira/);
}

test("integrations tab adds and lists a webhook", async ({ page }) => {
  await login(page);
  await page.goto("/jira/projects/DEMO/settings");
  await page.getByRole("button", { name: "Integrations" }).click();
  await expect(page.getByTestId("integrations-tab")).toBeVisible();

  await page.getByLabel("Webhook URL").fill("https://example.com/my-hook");
  await page.getByRole("button", { name: "Add webhook" }).click();
  await expect(page.getByText("https://example.com/my-hook")).toBeVisible();
});

test("issue development panel renders", async ({ page }) => {
  await login(page);
  await page.goto("/jira/browse/DEMO-1");
  await expect(page.getByTestId("development-panel")).toBeVisible();
});
```

> **Nota:** verificare il path della vista issue (`/jira/browse/DEMO-1` dai round precedenti). Se il pannello development non compare per assenza dati, l'asserzione sul testid regge comunque (il componente rende sempre l'header + empty state). Adeguare i selettori.

- [ ] **Step 2: Eseguire**

Run: `cd frontend-next && npx playwright test e2e/integrations.spec.ts --reporter=line`
Expected: 2 PASS.

- [ ] **Step 3: Suite completa (kill server residui)**

Run: `lsof -ti:8080 -ti:3000 | xargs kill 2>/dev/null; sleep 1; cd frontend-next && npx playwright test --reporter=list`
Expected: tutti verdi. Pulire `test-results/`/`playwright-report/`.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/e2e/integrations.spec.ts
git commit -m "test(e2e): integrations webhook add and development panel"
```

---

### Task 12: Seed webhook demo + gap report

**Files:**
- Modify: `cmd/seed/main.go`
- Modify: `docs/contracts/gap-report.md`

- [ ] **Step 1: Seed idempotente**

In `cmd/seed/main.go`, dopo il seed del gruppo/notifica, aggiungere un webhook demo idempotente sul progetto DEMO (check per project_id+url), usando `webhook.NewService(s.DB)`:

```go
	whSvc := webhook.NewService(s.DB)
	var existingW int64
	s.DB.Table("webhooks").Where("project_id = ? AND url = ?", demo.ID, "https://example.com/demo-hook").Count(&existingW)
	if existingW == 0 {
		if _, err := whSvc.Create(demo.ID, "https://example.com/demo-hook", "demo-secret", []string{"issue_created", "issue_updated"}); err != nil {
			log.Fatalf("seed webhook: %v", err)
		}
		fmt.Println("created demo webhook")
	}
```
(Import `webhook "github.com/open-jira/open-jira/internal/domain/webhook"`.)

- [ ] **Step 2: Verificare idempotenza**

Run: `rm -f /tmp/s9.db && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/s9.db go run ./cmd/seed && APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/s9.db go run ./cmd/seed && rm -f /tmp/s9.db`
Expected: prima run "created demo webhook"; seconda no; entrambe exit 0.

- [ ] **Step 3: Gap report**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md`
Expected: i webhook per-progetto sono estensioni → probabile "extra"; riportare comunque il conteggio. Rimuovere il binario `seed` se `go build ./cmd/seed` lo ha creato nella cwd (`rm -f seed`).

- [ ] **Step 4: Commit**

```bash
git add cmd/seed/main.go docs/contracts/gap-report.md
git commit -m "feat(seed): demo webhook; regenerate gap report for Round 9"
```

---

### Task 13: Gate finale + STATE.md → Round 10

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

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md && rm -f seed`
Expected: nessun drift inatteso; nessun binario `seed` orfano committato.

- [ ] **Step 3: Aggiornare STATE.md**

In `docs/superpowers/STATE.md`:
- aggiungere alla sezione "Round completati" la riga del **Round 9 — Integrazioni** (webhook in uscita: dominio `webhook` + CRUD `/project/{key}/webhooks` + delivery HTTP firmato HMAC-SHA256 + log `webhook_deliveries`; `integration.Dispatcher` che a issue created/updated/transitioned consegna ai webhook e fa partire l'automation via `EventSink` su `issue.Service`; auto-commento Git sul commit che referenzia una issue; migrazione 000015; UI tab Integrations + pannello Development sulla issue);
- cambiare "Prossimo" in **Round 10 — Release 1.0** (rimozione vecchio `frontend`, **security review incl. enforcement permessi**, performance/load test, rename senza trademark, LICENSE AGPL-3.0, README/CONTRIBUTING/CoC, Helm chart, docker-compose demo, CI/CD release, pubblicazione GitHub);
- aggiornare data e conteggio gap;
- aggiungere ai follow-up: retry/backoff sulla consegna webhook (ora fire-and-forget one-shot); webhook v3 dynamic (Connect/OAuth) conformi; dedup auto-commento Git su sha già linkato; disattivare il polling ridondante del worker (`processWebhookDeliveries`/`processAutomationRules`) ora che il firing è event-driven; git provider outbound API (fetch reale di branch/commit/PR, non solo da webhook inbound).

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/STATE.md docs/contracts/gap-report.md
git commit -m "docs: mark Round 9 (Integrations) complete, Round 10 (Release 1.0) next"
```

---

## Note di chiusura round

- **Follow-up:** retry/backoff + coda persistente per la consegna webhook (ora one-shot fire-and-forget); webhook v3 dynamic conformi (`/rest/api/3/webhook` con scadenza/refresh); dedup dell'auto-commento Git; ritiro del polling del worker ora ridondante; API outbound dei git provider (leggere branch/commit/PR dal provider, non solo riceverli via webhook inbound); firma configurabile e verifica lato ricevitore documentata.
- **Rischi noti:** la consegna webhook è asincrona (goroutine) e best-effort: in caso di crash del server tra evento e consegna il webhook si perde (accettabile per un modello base; la coda persistente è follow-up). Il dispatcher chiama `automation.ProcessRules` in modo sincrono nel path della richiesta di create/update issue: se una regola è lenta, rallenta la risposta — valutare l'async anche per l'automation in un round di hardening. Il round chiude solo con i tre livelli verdi.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura spec (roadmap Round 9):**
- Webhook in uscita → Task 2/3/4 (dominio + delivery firmato + CRUD) + Task 5 (firing su eventi). ✅
- Integrazione Git (`GitProvider` Forgejo/GitLab/GitHub, branch/commit/PR ↔ issue) → **già esistente** (inbound completo); Round 9 aggiunge l'auto-commento (Task 6) e la UI Development panel (Task 10). L'API outbound dei provider è follow-up. ⚠️ (dichiarato)
- Automation base (trigger→condizione→azione) → **già esistente**; Round 9 la rende event-driven (Task 5, `ProcessRules` su create/update/transition) invece del polling. ✅
- UI integrazioni → Task 9 (tab) + Task 10 (dev panel). ✅
- Gate: test Task 7, E2E Task 11, gate Task 13. ✅

**2. Placeholder scan:** codice completo per webhook (dominio/delivery/handler), dispatcher, wiring, auto-commento, frontend. Le "Note implementatore" indicano verifiche su firme reali (rotte git provider config, shape `GET /issue/{key}/git`, comment service nel router, `apiFetch`, IssueView prop, `AutomationRunner` firma) con i file da leggere — non placeholder di logica.

**3. Consistenza tipi:** `webhook.Service` (Create/ListByProject/Delete/ListActiveForEvent/RecordDelivery) + `Webhook.Events()` + `webhook.Sign`/`Deliver` (Task 2/3) usati in handler (Task 4) + dispatcher (Task 5). `integration.Dispatcher` implementa `issue.EventSink{IssueEvent(string,*Issue)}` (Task 5) e usa `AutomationRunner{ProcessRules(string,string)}` (soddisfatto da `automation.Service`). `issue.Service.SetEventSink` + `emit` (Task 5). `GitHandler` + `commentSvc` (Task 6). Frontend `integrations` + `issueGit` client (Task 8) usati in Task 9/10/11. Payload webhook `{event, issue{id,key,summary,projectId}}` + header `X-OpenJira-Event`/`X-OpenJira-Signature` coerenti tra Deliver (T3), dispatcher (T5) e test (T3/T7).
