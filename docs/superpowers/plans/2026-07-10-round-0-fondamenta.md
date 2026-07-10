# Round 0 — Audit & Fondamenta: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mettere in piedi le fondamenta della parità con Jira Cloud: contratti OpenAPI ufficiali versionati, gap analysis automatica, piattaforma v3 (errori, paginazione, expand), package ADF, auth Basic+token in stile Jira, harness di contract test, seed demo, CI, Playwright e primo endpoint certificato conforme (`/rest/api/3/myself`).

**Architecture:** Il backend Go (net/http ServeMux, GORM, migrazioni golang-migrate) acquisisce un package trasversale `internal/api/v3` (formato risposte/errori Jira) e `internal/adf` (Atlassian Document Format). Un package `internal/contract` valida le risposte HTTP contro l'OpenAPI ufficiale Atlassian con kin-openapi. Il frontend `frontend-next` (Next 16, Tailwind 4) riceve TanStack Query e Playwright.

**Tech Stack:** Go 1.25, GORM, golang-migrate, github.com/getkin/kin-openapi, SQLite (test), PostgreSQL (prod), Next.js 16, TypeScript, Tailwind 4, @tanstack/react-query, Playwright, GitHub Actions.

**Contesto per chi non conosce il repo:**
- Router: `internal/api/router.go`, funzione `api.NewRouter(cfg *config.Config, db *gorm.DB) http.Handler`, pattern `mux.Handle("GET /rest/api/3/...", authMw(...))`.
- Config: `internal/config/config.go` — env richieste: `APP_SECRET`, `DB_DSN`; `DB_DRIVER` = `postgres|mysql|sqlite`.
- Store: `internal/store/store.go` (`store.New`) e `internal/store/migrate.go` (`store.RunMigrations`) — SQLite supportato.
- Modello utente: `internal/domain/user/model.go` (`user.User`, campi `ID, Email, Username, DisplayName, AvatarURL, PasswordHash, IsAdmin, IsActive`).
- Auth esistente: JWT Bearer (`internal/domain/auth/jwt.go`, `internal/api/middleware/auth.go`).
- Test esistenti: `go test ./...` deve restare verde; stile testing standard library.
- Convenzione commit: conventional commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).

---

### Task 1: Contratti OpenAPI ufficiali versionati

**Files:**
- Create: `scripts/update-contracts.sh`
- Create: `docs/contracts/README.md`
- Create (scaricati): `docs/contracts/jira-platform-v3.json`, `docs/contracts/jira-agile-1.0.json`

- [ ] **Step 1: Scrivi lo script di download**

```bash
#!/usr/bin/env bash
# scripts/update-contracts.sh — scarica gli OpenAPI ufficiali Atlassian.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p docs/contracts

curl -fsSL "https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json" \
  -o docs/contracts/jira-platform-v3.json
curl -fsSL "https://developer.atlassian.com/cloud/jira/software/swagger.v3.json" \
  -o docs/contracts/jira-agile-1.0.json

echo "Platform paths: $(jq '.paths | length' docs/contracts/jira-platform-v3.json)"
echo "Agile paths:    $(jq '.paths | length' docs/contracts/jira-agile-1.0.json)"
```

- [ ] **Step 2: Esegui e verifica**

Run: `chmod +x scripts/update-contracts.sh && ./scripts/update-contracts.sh`
Expected: stampa il numero di path (platform > 300, agile > 40). Se un URL risponde 404, cercare l'URL corrente su https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/ (sezione "OpenAPI spec") e aggiornare lo script.

- [ ] **Step 3: Scrivi il README dei contratti**

```markdown
# Contratti API (docs/contracts)

- `jira-platform-v3.json` — OpenAPI ufficiale Jira Cloud REST API v3 (fonte Atlassian).
- `jira-agile-1.0.json` — OpenAPI ufficiale Jira Software (Agile) 1.0.

Questi file sono la **fonte di verità** per la compatibilità drop-in.
Aggiornali con `scripts/update-contracts.sh` e committa il diff.
I contract test in `internal/contract` validano le risposte del nostro server contro questi schemi.
```

- [ ] **Step 4: Commit**

```bash
git add scripts/update-contracts.sh docs/contracts/
git commit -m "feat(contracts): version official Jira v3 + Agile OpenAPI specs"
```

---

### Task 2: Gap report — endpoint implementati vs contratto

**Files:**
- Create: `cmd/gapreport/main.go`
- Create: `cmd/gapreport/main_test.go`
- Output generato: `docs/contracts/gap-report.md`

- [ ] **Step 1: Scrivi il test fallente**

```go
// cmd/gapreport/main_test.go
package main

import "testing"

func TestExtractRoutes(t *testing.T) {
	src := `
		mux.HandleFunc("POST /rest/api/3/auth/login", authH.Login)
		mux.Handle("GET /rest/api/3/project/{key}", authMw(http.HandlerFunc(projectH.Get)))
	`
	routes := extractRoutes(src)
	want := []Route{
		{Method: "POST", Path: "/rest/api/3/auth/login"},
		{Method: "GET", Path: "/rest/api/3/project/{key}"},
	}
	if len(routes) != len(want) {
		t.Fatalf("got %d routes, want %d", len(routes), len(want))
	}
	for i := range want {
		if routes[i] != want[i] {
			t.Errorf("route %d: got %+v, want %+v", i, routes[i], want[i])
		}
	}
}

func TestNormalizePath(t *testing.T) {
	// I nomi dei parametri non contano nel confronto: {key} e {projectIdOrKey} sono equivalenti.
	if normalizePath("/rest/api/3/project/{key}") != normalizePath("/rest/api/3/project/{projectIdOrKey}") {
		t.Error("normalized paths should match regardless of param names")
	}
}
```

- [ ] **Step 2: Verifica che fallisca**

Run: `go test ./cmd/gapreport/`
Expected: FAIL — `undefined: extractRoutes`

- [ ] **Step 3: Implementa il tool**

```go
// cmd/gapreport/main.go
// gapreport confronta le route registrate in internal/api/router.go
// con i path dell'OpenAPI ufficiale e genera docs/contracts/gap-report.md.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

type Route struct {
	Method string
	Path   string
}

var routeRe = regexp.MustCompile(`mux\.Handle(?:Func)?\(\s*"(GET|POST|PUT|PATCH|DELETE) ([^"]+)"`)
var paramRe = regexp.MustCompile(`\{[^}]+\}`)

func extractRoutes(src string) []Route {
	var out []Route
	for _, m := range routeRe.FindAllStringSubmatch(src, -1) {
		out = append(out, Route{Method: m[1], Path: m[2]})
	}
	return out
}

func normalizePath(p string) string {
	return paramRe.ReplaceAllString(strings.TrimSuffix(p, "/"), "{}")
}

type spec struct {
	Paths map[string]map[string]struct {
		Summary string   `json:"summary"`
		Tags    []string `json:"tags"`
	} `json:"paths"`
}

func loadSpecRoutes(path, prefix string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s spec
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	out := map[string]string{} // "METHOD /path" -> summary
	for p, ops := range s.Paths {
		for method, op := range ops {
			m := strings.ToUpper(method)
			if m == "PARAMETERS" {
				continue
			}
			out[m+" "+prefix+p] = op.Summary
		}
	}
	return out, nil
}

func main() {
	routerSrc, err := os.ReadFile("internal/api/router.go")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	implemented := map[string]bool{}
	for _, r := range extractRoutes(string(routerSrc)) {
		implemented[r.Method+" "+normalizePath(r.Path)] = true
	}

	specs := map[string]string{}
	for _, s := range []struct{ file, prefix string }{
		{"docs/contracts/jira-platform-v3.json", ""},
		{"docs/contracts/jira-agile-1.0.json", ""},
	} {
		m, err := loadSpecRoutes(s.file, s.prefix)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for k, v := range m {
			specs[k] = v
		}
	}

	var matched, missing, extra []string
	for k := range specs {
		if implemented[strings.SplitN(k, " ", 2)[0]+" "+normalizePath(strings.SplitN(k, " ", 2)[1])] {
			matched = append(matched, k)
		} else {
			missing = append(missing, k)
		}
	}
	specNorm := map[string]bool{}
	for k := range specs {
		parts := strings.SplitN(k, " ", 2)
		specNorm[parts[0]+" "+normalizePath(parts[1])] = true
	}
	for k := range implemented {
		if !specNorm[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(matched)
	sort.Strings(missing)
	sort.Strings(extra)

	var b strings.Builder
	b.WriteString("# Gap report — endpoint vs OpenAPI ufficiale\n\n")
	b.WriteString("> Generato da `go run ./cmd/gapreport`. Non modificare a mano.\n\n")
	fmt.Fprintf(&b, "- Nel contratto e implementati (path match): **%d**\n", len(matched))
	fmt.Fprintf(&b, "- Nel contratto ma mancanti: **%d**\n", len(missing))
	fmt.Fprintf(&b, "- Implementati ma fuori contratto (estensioni): **%d**\n\n", len(extra))
	b.WriteString("## Mancanti (dal contratto)\n\n")
	for _, k := range missing {
		fmt.Fprintf(&b, "- `%s` — %s\n", k, specs[k])
	}
	b.WriteString("\n## Estensioni fuori contratto\n\n")
	for _, k := range extra {
		fmt.Fprintf(&b, "- `%s`\n", k)
	}
	if err := os.WriteFile("docs/contracts/gap-report.md", []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("matched=%d missing=%d extra=%d → docs/contracts/gap-report.md\n",
		len(matched), len(missing), len(extra))
}
```

Nota: "path match" verifica solo metodo+path. La conformità dei payload la verificano i contract test (Task 7); il report serve a pianificare i round.

- [ ] **Step 4: Verifica che i test passino**

Run: `go test ./cmd/gapreport/`
Expected: PASS

- [ ] **Step 5: Genera il report reale**

Run: `go run ./cmd/gapreport && head -30 docs/contracts/gap-report.md`
Expected: report con conteggi; `matched` > 0.

- [ ] **Step 6: Commit**

```bash
git add cmd/gapreport docs/contracts/gap-report.md
git commit -m "feat(contracts): add gap report tool comparing router vs official OpenAPI"
```

---

### Task 3: Piattaforma v3 — formato errori e risposte Jira

**Files:**
- Create: `internal/api/v3/respond.go`
- Create: `internal/api/v3/respond_test.go`

- [ ] **Step 1: Scrivi i test fallenti**

```go
// internal/api/v3/respond_test.go
package v3

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteError_JiraShape(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, 404, []string{"Issue does not exist or you do not have permission to see it."}, nil)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	var body struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.ErrorMessages) != 1 {
		t.Fatalf("errorMessages = %v", body.ErrorMessages)
	}
	// Jira serializza sempre entrambe le chiavi, anche vuote.
	if body.Errors == nil {
		t.Error("errors must be {} not null")
	}
}

func TestWriteError_FieldErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, 400, nil, map[string]string{"summary": "Summary is required."})
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if string(body["errorMessages"]) != "[]" {
		t.Errorf("errorMessages = %s, want []", body["errorMessages"])
	}
}

func TestWritePage(t *testing.T) {
	rec := httptest.NewRecorder()
	WritePage(rec, 200, Page{StartAt: 0, MaxResults: 50, Total: 2, Values: []string{"a", "b"}})
	var body struct {
		StartAt    int      `json:"startAt"`
		MaxResults int      `json:"maxResults"`
		Total      int      `json:"total"`
		IsLast     bool     `json:"isLast"`
		Values     []string `json:"values"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.IsLast || body.Total != 2 || len(body.Values) != 2 {
		t.Errorf("unexpected page: %+v", body)
	}
}
```

- [ ] **Step 2: Verifica che falliscano**

Run: `go test ./internal/api/v3/`
Expected: FAIL — package non esiste.

- [ ] **Step 3: Implementa**

```go
// internal/api/v3/respond.go
// Package v3 implementa le convenzioni di risposta della Jira Cloud REST API v3:
// formato errori {errorMessages, errors}, paginazione {startAt, maxResults, total, isLast, values}.
package v3

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

// WriteError scrive un errore nel formato Jira v3. messages e fieldErrors
// possono essere nil: le chiavi vengono comunque serializzate vuote, come fa Jira.
func WriteError(w http.ResponseWriter, status int, messages []string, fieldErrors map[string]string) {
	if messages == nil {
		messages = []string{}
	}
	if fieldErrors == nil {
		fieldErrors = map[string]string{}
	}
	WriteJSON(w, status, errorBody{ErrorMessages: messages, Errors: fieldErrors})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type Page struct {
	StartAt    int `json:"startAt"`
	MaxResults int `json:"maxResults"`
	Total      int `json:"total"`
	Values     any `json:"values"`
}

type pageBody struct {
	Page
	IsLast bool `json:"isLast"`
}

func WritePage(w http.ResponseWriter, status int, p Page) {
	n := 0
	if vs, ok := p.Values.([]string); ok {
		n = len(vs)
	} else if raw, err := json.Marshal(p.Values); err == nil {
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil {
			n = len(arr)
		}
	}
	WriteJSON(w, status, pageBody{Page: p, IsLast: p.StartAt+n >= p.Total})
}
```

- [ ] **Step 4: Verifica che passino**

Run: `go test ./internal/api/v3/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/v3/
git commit -m "feat(v3): add Jira-shaped error and pagination response helpers"
```

---

### Task 4: Piattaforma v3 — parsing di paginazione, expand e fields

**Files:**
- Create: `internal/api/v3/params.go`
- Create: `internal/api/v3/params_test.go`

- [ ] **Step 1: Scrivi i test fallenti**

```go
// internal/api/v3/params_test.go
package v3

import (
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/rest/api/3/project/search", nil)
	startAt, maxResults := ParsePagination(r, 50, 100)
	if startAt != 0 || maxResults != 50 {
		t.Errorf("got %d,%d want 0,50", startAt, maxResults)
	}
}

func TestParsePagination_CapAndNegatives(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?startAt=-5&maxResults=9999", nil)
	startAt, maxResults := ParsePagination(r, 50, 100)
	if startAt != 0 || maxResults != 100 {
		t.Errorf("got %d,%d want 0,100", startAt, maxResults)
	}
}

func TestParseExpand(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?expand=description,lead,issueTypes", nil)
	e := ParseExpand(r)
	if !e.Has("lead") || e.Has("url") {
		t.Errorf("unexpected expand: %v", e)
	}
	// Il valore expand va rieccheggiato nella risposta come stringa.
	if e.String() != "description,lead,issueTypes" {
		t.Errorf("String() = %q", e.String())
	}
}

func TestParseFields(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?fields=summary,status,-comment", nil)
	f := ParseFields(r)
	if !f.Include("summary") || !f.Include("status") {
		t.Error("summary/status should be included")
	}
	if f.Include("comment") {
		t.Error("-comment should be excluded")
	}
	r2 := httptest.NewRequest("GET", "/x", nil)
	if !ParseFields(r2).Include("anything") {
		t.Error("no fields param means all fields (Jira default *navigable)")
	}
}
```

- [ ] **Step 2: Verifica che falliscano**

Run: `go test ./internal/api/v3/`
Expected: FAIL — `undefined: ParsePagination` ecc.

- [ ] **Step 3: Implementa**

```go
// internal/api/v3/params.go
package v3

import (
	"net/http"
	"strconv"
	"strings"
)

// ParsePagination legge startAt/maxResults con default e cap in stile Jira.
func ParsePagination(r *http.Request, defaultMax, capMax int) (startAt, maxResults int) {
	q := r.URL.Query()
	startAt, _ = strconv.Atoi(q.Get("startAt"))
	if startAt < 0 {
		startAt = 0
	}
	maxResults, err := strconv.Atoi(q.Get("maxResults"))
	if err != nil || maxResults <= 0 {
		maxResults = defaultMax
	}
	if maxResults > capMax {
		maxResults = capMax
	}
	return startAt, maxResults
}

// Expand è l'insieme dei valori richiesti nel query param expand.
type Expand struct {
	raw   string
	items map[string]bool
}

func ParseExpand(r *http.Request) Expand {
	raw := r.URL.Query().Get("expand")
	items := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			items[p] = true
		}
	}
	return Expand{raw: raw, items: items}
}

func (e Expand) Has(name string) bool { return e.items[name] }
func (e Expand) String() string       { return e.raw }

// Fields modella il query param fields: lista di campi da includere,
// con prefisso "-" per escludere. Assenza del parametro = tutti i campi.
type Fields struct {
	all      bool
	include  map[string]bool
	excluded map[string]bool
}

func ParseFields(r *http.Request) Fields {
	raw := r.URL.Query().Get("fields")
	if raw == "" {
		return Fields{all: true}
	}
	f := Fields{include: map[string]bool{}, excluded: map[string]bool{}}
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		switch {
		case p == "":
		case p == "*all" || p == "*navigable":
			f.all = true
		case strings.HasPrefix(p, "-"):
			f.excluded[p[1:]] = true
		default:
			f.include[p] = true
		}
	}
	return f
}

func (f Fields) Include(name string) bool {
	if f.excluded[name] {
		return false
	}
	return f.all || f.include[name]
}
```

- [ ] **Step 4: Verifica che passino**

Run: `go test ./internal/api/v3/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/v3/
git commit -m "feat(v3): parse startAt/maxResults, expand and fields params"
```

---

### Task 5: Package ADF (Atlassian Document Format)

**Files:**
- Create: `internal/adf/adf.go`
- Create: `internal/adf/adf_test.go`

Contesto: in Jira v3 descrizioni e commenti sono documenti ADF (JSON: `{"type":"doc","version":1,"content":[...]}`). Il modello issue attuale ha `DescriptionJSON string` — nei round successivi verrà interpretato come ADF; qui creiamo il package canonico.

- [ ] **Step 1: Scrivi i test fallenti**

```go
// internal/adf/adf_test.go
package adf

import (
	"encoding/json"
	"testing"
)

const sample = `{
  "type": "doc", "version": 1,
  "content": [
    {"type": "paragraph", "content": [
      {"type": "text", "text": "Hello "},
      {"type": "text", "text": "world", "marks": [{"type": "strong"}]}
    ]},
    {"type": "paragraph", "content": [{"type": "text", "text": "Second line"}]}
  ]
}`

func TestRoundTrip(t *testing.T) {
	var doc Node
	if err := json.Unmarshal([]byte(sample), &doc); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var again Node
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if again.Type != "doc" || again.Version != 1 || len(again.Content) != 2 {
		t.Errorf("round trip lost data: %+v", again)
	}
}

func TestPlainText(t *testing.T) {
	var doc Node
	_ = json.Unmarshal([]byte(sample), &doc)
	if got := PlainText(doc); got != "Hello world\nSecond line" {
		t.Errorf("PlainText = %q", got)
	}
}

func TestFromText(t *testing.T) {
	doc := FromText("Just a note")
	if err := Validate(doc); err != nil {
		t.Fatal(err)
	}
	if PlainText(doc) != "Just a note" {
		t.Errorf("PlainText = %q", PlainText(doc))
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(Node{Type: "paragraph"}); err == nil {
		t.Error("root must be doc")
	}
	if err := Validate(Node{Type: "doc", Version: 2}); err == nil {
		t.Error("version must be 1")
	}
}
```

- [ ] **Step 2: Verifica che falliscano**

Run: `go test ./internal/adf/`
Expected: FAIL — package non esiste.

- [ ] **Step 3: Implementa**

```go
// internal/adf/adf.go
// Package adf modella l'Atlassian Document Format usato da Jira Cloud v3
// per descrizioni e commenti. https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
package adf

import (
	"errors"
	"strings"
)

type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

type Node struct {
	Type    string         `json:"type"`
	Version int            `json:"version,omitempty"` // solo sul nodo radice "doc"
	Text    string         `json:"text,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

// FromText costruisce un documento ADF minimale da testo semplice
// (un paragrafo per riga).
func FromText(text string) Node {
	var paras []Node
	for _, line := range strings.Split(text, "\n") {
		p := Node{Type: "paragraph"}
		if line != "" {
			p.Content = []Node{{Type: "text", Text: line}}
		}
		paras = append(paras, p)
	}
	return Node{Type: "doc", Version: 1, Content: paras}
}

// PlainText estrae il testo del documento; i nodi block sono separati da newline.
func PlainText(n Node) string {
	var blocks []string
	for _, child := range n.Content {
		blocks = append(blocks, inlineText(child))
	}
	return strings.Join(blocks, "\n")
}

func inlineText(n Node) string {
	if n.Type == "text" {
		return n.Text
	}
	var b strings.Builder
	for _, child := range n.Content {
		b.WriteString(inlineText(child))
	}
	return b.String()
}

// Validate verifica i vincoli strutturali di base di un documento ADF.
func Validate(n Node) error {
	if n.Type != "doc" {
		return errors.New("adf: root node must have type \"doc\"")
	}
	if n.Version != 1 {
		return errors.New("adf: version must be 1")
	}
	return validateChildren(n.Content)
}

func validateChildren(nodes []Node) error {
	for _, n := range nodes {
		if n.Type == "" {
			return errors.New("adf: node without type")
		}
		if n.Type == "text" && n.Text == "" {
			return errors.New("adf: text node without text")
		}
		if err := validateChildren(n.Content); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Verifica che passino**

Run: `go test ./internal/adf/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/adf/
git commit -m "feat(adf): add Atlassian Document Format types, validation, plain-text extraction"
```

---

### Task 6: Auth in stile Jira — Basic auth con API token + errori 401 conformi

**Files:**
- Create: `migrations/000004_api_tokens.up.sql`, `migrations/000004_api_tokens.down.sql`
- Create: `internal/domain/auth/apitoken.go`, `internal/domain/auth/apitoken_test.go`
- Modify: `internal/api/middleware/auth.go` (riscrittura), `internal/api/middleware/auth_test.go` (aggiunte)
- Modify: `internal/api/router.go` (nuova route + nuova firma middleware)

Contesto: Jira Cloud usa `Authorization: Basic base64(email:api_token)`. Manteniamo anche il Bearer JWT per la sessione del frontend. Gli errori del middleware oggi sono `{"error":...}`: vanno portati al formato Jira.

- [ ] **Step 1: Scrivi la migrazione**

```sql
-- migrations/000004_api_tokens.up.sql
CREATE TABLE api_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT NOT NULL DEFAULT '',
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP
);
CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
```

```sql
-- migrations/000004_api_tokens.down.sql
DROP TABLE api_tokens;
```

- [ ] **Step 2: Scrivi il test fallente del servizio token**

```go
// internal/domain/auth/apitoken_test.go
package auth

import (
	"testing"

	"github.com/glebarez/sqlite" // se il repo usa gorm.io/driver/sqlite, usare quello
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT, username TEXT,
		display_name TEXT DEFAULT '', avatar_url TEXT DEFAULT '', password_hash TEXT DEFAULT '',
		is_admin BOOLEAN DEFAULT FALSE, is_active BOOLEAN DEFAULT TRUE)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE api_tokens (id TEXT PRIMARY KEY, user_id TEXT NOT NULL,
		label TEXT NOT NULL DEFAULT '', token_hash TEXT NOT NULL UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, last_used_at TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateAndVerifyAPIToken(t *testing.T) {
	db := newTestDB(t)
	db.Exec(`INSERT INTO users (id, email, username) VALUES ('u1', 'a@b.c', 'ab')`)
	svc := NewService(db, "test-secret")

	plaintext, err := svc.CreateAPIToken("u1", "ci")
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) < 24 {
		t.Fatalf("token too short: %q", plaintext)
	}

	userID, err := svc.VerifyAPIToken("a@b.c", plaintext)
	if err != nil || userID != "u1" {
		t.Fatalf("verify: userID=%q err=%v", userID, err)
	}

	if _, err := svc.VerifyAPIToken("a@b.c", "wrong-token"); err == nil {
		t.Error("wrong token must fail")
	}
	if _, err := svc.VerifyAPIToken("other@b.c", plaintext); err == nil {
		t.Error("wrong email must fail")
	}
}
```

Nota per l'esecutore: usare lo stesso driver sqlite già usato nel repo (`gorm.io/driver/sqlite`, vedi `internal/store/store.go`) — l'import nel blocco sopra va adattato di conseguenza.

- [ ] **Step 3: Verifica che fallisca**

Run: `go test ./internal/domain/auth/`
Expected: FAIL — `undefined: (*Service).CreateAPIToken`

- [ ] **Step 4: Implementa il servizio token**

```go
// internal/domain/auth/apitoken.go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/open-jira/open-jira/internal/domain/user"
)

type APIToken struct {
	ID         string     `gorm:"primaryKey;type:text" json:"id"`
	UserID     string     `gorm:"type:text;not null;index" json:"user_id"`
	Label      string     `gorm:"type:text;not null;default:''" json:"label"`
	TokenHash  string     `gorm:"type:text;not null;uniqueIndex" json:"-"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func (APIToken) TableName() string { return "api_tokens" }

func hashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// CreateAPIToken genera un token, ne salva l'hash e restituisce il plaintext
// (mostrato all'utente una sola volta, come fa Atlassian).
func (s *Service) CreateAPIToken(userID, label string) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plaintext := "ojt_" + base64.RawURLEncoding.EncodeToString(raw)
	tok := APIToken{ID: generateID(), UserID: userID, Label: label, TokenHash: hashToken(plaintext)}
	if err := s.db.Create(&tok).Error; err != nil {
		return "", err
	}
	return plaintext, nil
}

var ErrInvalidToken = errors.New("invalid email or api token")

// VerifyAPIToken implementa la Basic auth di Jira: email + api token.
func (s *Service) VerifyAPIToken(email, plaintext string) (string, error) {
	var u user.User
	if err := s.db.Where("email = ? AND is_active = ?", email, true).First(&u).Error; err != nil {
		return "", ErrInvalidToken
	}
	var tok APIToken
	if err := s.db.Where("user_id = ? AND token_hash = ?", u.ID, hashToken(plaintext)).
		First(&tok).Error; err != nil {
		return "", ErrInvalidToken
	}
	now := time.Now()
	s.db.Model(&tok).Update("last_used_at", &now)
	return u.ID, nil
}
```

Nota: se `Service` in `service.go` non espone il campo `db`, verificare il nome del campo esistente e riusarlo.

- [ ] **Step 5: Verifica che passi**

Run: `go test ./internal/domain/auth/`
Expected: PASS

- [ ] **Step 6: Scrivi i test fallenti del middleware**

Aggiungere a `internal/api/middleware/auth_test.go`:

```go
func TestAuth_BasicToken(t *testing.T) {
	// verifier fake: accetta solo alice@example.com + "good-token"
	verify := func(email, token string) (string, error) {
		if email == "alice@example.com" && token == "good-token" {
			return "user-1", nil
		}
		return "", errors.New("nope")
	}
	mw := Auth("secret", verify)
	var gotUserID string
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
	}))

	req := httptest.NewRequest("GET", "/rest/api/3/myself", nil)
	req.SetBasicAuth("alice@example.com", "good-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || gotUserID != "user-1" {
		t.Fatalf("code=%d userID=%q", rec.Code, gotUserID)
	}
}

func TestAuth_UnauthorizedJiraFormat(t *testing.T) {
	mw := Auth("secret", func(string, string) (string, error) { return "", errors.New("nope") })
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/rest/api/3/myself", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != 401 {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
	var body struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("401 body is not Jira-shaped JSON: %v — body: %s", err, rec.Body.String())
	}
	if len(body.ErrorMessages) == 0 {
		t.Error("errorMessages must not be empty")
	}
}
```

(Aggiungere gli import mancanti: `encoding/json`, `errors`, `net/http`, `net/http/httptest`.)

- [ ] **Step 7: Verifica che falliscano**

Run: `go test ./internal/api/middleware/`
Expected: FAIL — la firma attuale è `Auth(secret string)`, senza verifier.

- [ ] **Step 8: Riscrivi il middleware**

```go
// internal/api/middleware/auth.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// BasicVerifier valida email+api token e restituisce lo userID.
type BasicVerifier func(email, token string) (string, error)

// Auth accetta sia "Bearer <jwt>" (sessione frontend) sia
// "Basic base64(email:api_token)" (client API, come Jira Cloud).
func Auth(secret string, verifyBasic BasicVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				unauthorized(w, "Authentication required.")
				return
			}

			if email, token, ok := r.BasicAuth(); ok {
				userID, err := verifyBasic(email, token)
				if err != nil {
					unauthorized(w, "Basic authentication with an invalid email or API token.")
					return
				}
				next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), userID)))
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				unauthorized(w, "Unsupported Authorization scheme.")
				return
			}
			claims, err := auth.ValidateToken(secret, parts[1])
			if err != nil {
				unauthorized(w, "The access token is invalid or expired.")
				return
			}
			next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), claims.UserID)))
		})
	}
}

func withUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, UserIDKey, id)
}

func unauthorized(w http.ResponseWriter, msg string) {
	v3.WriteError(w, http.StatusUnauthorized, []string{msg}, nil)
}

func UserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}
```

- [ ] **Step 9: Aggiorna il router**

In `internal/api/router.go`: dove viene costruito `authMw` (cercare `middleware.Auth(`), passare il verifier del servizio auth e aggiungere la route dei token:

```go
authSvc := auth.NewService(db, cfg.Secret) // riusare l'istanza se già presente
authMw := middleware.Auth(cfg.Secret, authSvc.VerifyAPIToken)

// Gestione API token (estensione: Atlassian li gestisce su id.atlassian.com)
mux.Handle("POST /rest/api/3/auth/api-tokens", authMw(http.HandlerFunc(authH.CreateAPIToken)))
```

E in `internal/api/handlers/auth_handler.go` aggiungere:

```go
func (h *AuthHandler) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	token, err := h.svc.CreateAPIToken(userID, req.Label)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to create API token."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, map[string]string{"token": token, "label": req.Label})
}
```

(Adattare il nome del campo del service nel handler — verificare come `AuthHandler` referenzia il service, es. `h.svc` o `h.auth`.)

- [ ] **Step 10: Verifica tutta la suite**

Run: `go build ./... && go test ./...`
Expected: PASS ovunque (correggere gli altri punti di `router.go` che usano la vecchia firma).

- [ ] **Step 11: Commit**

```bash
git add migrations/000004_api_tokens.* internal/domain/auth/ internal/api/middleware/ internal/api/router.go internal/api/handlers/auth_handler.go
git commit -m "feat(auth): Jira-style Basic auth with API tokens and v3-shaped 401 errors"
```

---

### Task 7: Harness di contract test

**Files:**
- Create: `internal/contract/harness.go`
- Create: `internal/contract/harness_test.go`
- Modify: `go.mod` (nuova dipendenza kin-openapi)

- [ ] **Step 1: Aggiungi la dipendenza**

Run: `go get github.com/getkin/kin-openapi@latest`
Expected: `go.mod` aggiornato senza errori.

- [ ] **Step 2: Scrivi il test del harness (usa lo spec reale, valida una risposta finta)**

```go
// internal/contract/harness_test.go
package contract

import (
	"net/http"
	"strings"
	"testing"
)

func TestValidator_ValidatesKnownGoodResponse(t *testing.T) {
	v, err := NewValidator("../../docs/contracts/jira-platform-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	// Risposta plausibile per GET /rest/api/3/myself secondo lo schema User.
	body := `{
	  "self": "http://localhost:8080/rest/api/3/user?accountId=u1",
	  "accountId": "u1",
	  "accountType": "atlassian",
	  "emailAddress": "alice@example.com",
	  "displayName": "Alice",
	  "active": true,
	  "avatarUrls": {"16x16": "http://x/a.png", "24x24": "http://x/a.png",
	                 "32x32": "http://x/a.png", "48x48": "http://x/a.png"}
	}`
	err = v.ValidateResponse("GET", "/rest/api/3/myself", 200,
		http.Header{"Content-Type": []string{"application/json"}},
		strings.NewReader(body))
	if err != nil {
		t.Errorf("valid myself body rejected: %v", err)
	}
}

func TestValidator_RejectsBadResponse(t *testing.T) {
	v, err := NewValidator("../../docs/contracts/jira-platform-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	// "active" con tipo sbagliato deve essere rifiutato.
	body := `{"accountId": "u1", "active": "yes"}`
	err = v.ValidateResponse("GET", "/rest/api/3/myself", 200,
		http.Header{"Content-Type": []string{"application/json"}},
		strings.NewReader(body))
	if err == nil {
		t.Error("invalid body accepted")
	}
}
```

- [ ] **Step 3: Verifica che fallisca**

Run: `go test ./internal/contract/`
Expected: FAIL — `undefined: NewValidator`

- [ ] **Step 4: Implementa il harness**

```go
// internal/contract/harness.go
// Package contract valida le risposte del nostro server contro
// l'OpenAPI ufficiale di Jira Cloud (docs/contracts/).
package contract

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacy "github.com/getkin/kin-openapi/routers/legacy"
)

type Validator struct {
	doc    *openapi3.T
	router routers.Router
}

func NewValidator(specPath string) (*Validator, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}
	// Lo spec Atlassian ha servers con host cloud: azzeriamo per far matchare i path relativi.
	doc.Servers = openapi3.Servers{&openapi3.Server{URL: "/"}}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("build router: %w", err)
	}
	return &Validator{doc: doc, router: router}, nil
}

// ValidateResponse verifica che (method, path) esista nel contratto e che
// status/header/body rispettino lo schema della risposta.
func (v *Validator) ValidateResponse(method, path string, status int, header http.Header, body io.Reader) error {
	u, err := url.Parse(path)
	if err != nil {
		return err
	}
	req := &http.Request{Method: method, URL: u, Header: http.Header{}}
	route, pathParams, err := v.router.FindRoute(req)
	if err != nil {
		return fmt.Errorf("route %s %s not in contract: %w", method, path, err)
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
			Options: &openapi3filter.Options{
				AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			},
		},
		Status: status,
		Header: header,
	}
	input.SetBodyBytes(mustRead(body))
	return openapi3filter.ValidateResponse(context.Background(), input)
}

func mustRead(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}
```

Nota per l'esecutore: le API di kin-openapi cambiano tra versioni minori — se `SetBodyBytes` o `NoopAuthenticationFunc` non esistono nella versione risolta, consultare `go doc github.com/getkin/kin-openapi/openapi3filter` e adattare mantenendo il comportamento dei test.

- [ ] **Step 5: Verifica che passino**

Run: `go test ./internal/contract/`
Expected: PASS (il caricamento dello spec da ~10MB può richiedere qualche secondo).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/contract/
git commit -m "feat(contract): response validator against official Jira OpenAPI"
```

---

### Task 8: Primo endpoint certificato — GET /rest/api/3/myself

**Files:**
- Create: `internal/api/v3/user.go`, `internal/api/v3/user_test.go`
- Modify: `internal/api/handlers/user_handler.go` (GetMe → GetMyself per la route /myself)
- Modify: `internal/api/router.go`
- Create: `internal/contract/myself_test.go`

- [ ] **Step 1: Scrivi il test fallente del mapping utente → schema Jira**

```go
// internal/api/v3/user_test.go
package v3

import (
	"testing"

	"github.com/open-jira/open-jira/internal/domain/user"
)

func TestJiraUser(t *testing.T) {
	u := user.User{ID: "u1", Email: "alice@example.com", DisplayName: "Alice",
		AvatarURL: "http://x/a.png", IsActive: true}
	ju := JiraUser(u, "http://localhost:8080")

	if ju.AccountID != "u1" || ju.DisplayName != "Alice" || !ju.Active {
		t.Errorf("unexpected: %+v", ju)
	}
	if ju.Self != "http://localhost:8080/rest/api/3/user?accountId=u1" {
		t.Errorf("self = %q", ju.Self)
	}
	if ju.AccountType != "atlassian" {
		t.Errorf("accountType = %q", ju.AccountType)
	}
	if ju.AvatarUrls["48x48"] != "http://x/a.png" {
		t.Errorf("avatarUrls = %v", ju.AvatarUrls)
	}
}

func TestJiraUser_EmptyAvatar(t *testing.T) {
	ju := JiraUser(user.User{ID: "u2", IsActive: true}, "http://h")
	// Jira serializza sempre avatarUrls con le 4 taglie.
	for _, size := range []string{"16x16", "24x24", "32x32", "48x48"} {
		if _, ok := ju.AvatarUrls[size]; !ok {
			t.Errorf("missing avatar size %s", size)
		}
	}
}
```

- [ ] **Step 2: Verifica che fallisca**

Run: `go test ./internal/api/v3/`
Expected: FAIL — `undefined: JiraUser`

- [ ] **Step 3: Implementa il mapping**

```go
// internal/api/v3/user.go
package v3

import (
	"fmt"

	"github.com/open-jira/open-jira/internal/domain/user"
)

// User è la rappresentazione Jira v3 di un utente (schema "User" nel contratto).
type User struct {
	Self         string            `json:"self"`
	AccountID    string            `json:"accountId"`
	AccountType  string            `json:"accountType"`
	EmailAddress string            `json:"emailAddress,omitempty"`
	DisplayName  string            `json:"displayName"`
	Active       bool              `json:"active"`
	TimeZone     string            `json:"timeZone,omitempty"`
	Locale       string            `json:"locale,omitempty"`
	AvatarUrls   map[string]string `json:"avatarUrls"`
}

func JiraUser(u user.User, baseURL string) User {
	avatar := u.AvatarURL
	if avatar == "" {
		avatar = baseURL + "/static/default-avatar.png"
	}
	return User{
		Self:         fmt.Sprintf("%s/rest/api/3/user?accountId=%s", baseURL, u.ID),
		AccountID:    u.ID,
		AccountType:  "atlassian",
		EmailAddress: u.Email,
		DisplayName:  u.DisplayName,
		Active:       u.IsActive,
		AvatarUrls: map[string]string{
			"16x16": avatar, "24x24": avatar, "32x32": avatar, "48x48": avatar,
		},
	}
}
```

- [ ] **Step 4: Verifica che passi**

Run: `go test ./internal/api/v3/`
Expected: PASS

- [ ] **Step 5: Scrivi il contract test end-to-end fallente**

```go
// internal/contract/myself_test.go
package contract

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-jira/open-jira/internal/api"
	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/store"
)

// newTestServer avvia il router reale su SQLite temporaneo con migrazioni.
func newTestServer(t *testing.T) (*httptest.Server, *auth.Service) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	cfg := &config.Config{
		Port: 0, Env: "test", Secret: "contract-test-secret",
		BaseURL: "http://localhost:8080",
		DB:      config.DBConfig{Driver: "sqlite", DSN: dsn},
	}
	s, err := store.New(cfg.DB, cfg.Env)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := store.RunMigrations(cfg.DB); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.NewRouter(cfg, s.DB))
	t.Cleanup(srv.Close)
	return srv, auth.NewService(s.DB, cfg.Secret)
}

func TestMyself_ConformsToContract(t *testing.T) {
	if os.Getenv("SKIP_CONTRACT") != "" {
		t.Skip("SKIP_CONTRACT set")
	}
	srv, authSvc := newTestServer(t)

	if _, err := authSvc.Register("alice@example.com", "alice", "Alice", "password-123"); err != nil {
		t.Fatal(err)
	}
	jwt, err := authSvc.Login("alice@example.com", "password-123")
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/myself", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}

	v, err := NewValidator("../../docs/contracts/jira-platform-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateResponse("GET", "/rest/api/3/myself", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /rest/api/3/myself NON conforme al contratto: %v", err)
	}
}
```

Note per l'esecutore:
- verificare la firma reale di `auth.Service.Register` in `internal/domain/auth/service.go` (`Register(email, username, displayName, password string)`) e di `Login` (restituisce `(string, error)`), adattando se diverse;
- se `store.RunMigrations` risolve il path `migrations/` relativo alla working dir, eseguire i test dalla root o rendere il path assoluto.

- [ ] **Step 6: Verifica che fallisca (rosso: la risposta attuale è il modello interno, non lo schema Jira)**

Run: `go test ./internal/contract/ -run TestMyself -v`
Expected: FAIL — la validazione segnala campi mancanti (`accountId`, ecc.).

- [ ] **Step 7: Implementa la risposta conforme**

In `internal/api/handlers/user_handler.go`, aggiungere un handler dedicato:

```go
// GetMyself risponde a GET /rest/api/3/myself nel formato Jira v3.
func (h *UserHandler) GetMyself(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var u user.User
	if err := h.db.First(&u, "id = ?", userID).Error; err != nil {
		v3.WriteError(w, http.StatusUnauthorized, []string{"The user does not exist."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.JiraUser(u, h.baseURL))
}
```

Adattamenti: usare il modo in cui `UserHandler` accede al DB/service (leggere il file prima); se il handler non ha `baseURL`, passarlo dal router (`cfg.BaseURL`) nel costruttore del handler.

In `internal/api/router.go`:

```go
mux.Handle("GET /rest/api/3/myself", authMw(http.HandlerFunc(userH.GetMyself)))
```

(lasciando `GET /rest/api/3/users/me` sul vecchio `GetMe` per non rompere il frontend attuale).

- [ ] **Step 8: Verifica che passi tutto**

Run: `go test ./internal/contract/ ./internal/api/... && go build ./...`
Expected: PASS — primo endpoint certificato conforme.

- [ ] **Step 9: Commit**

```bash
git add internal/api/v3/user.go internal/api/v3/user_test.go internal/api/handlers/user_handler.go internal/api/router.go internal/contract/myself_test.go
git commit -m "feat(v3): GET /rest/api/3/myself conforms to official Jira contract"
```

---

### Task 9: Seed dati demo

**Files:**
- Create: `cmd/seed/main.go`

- [ ] **Step 1: Implementa il seeder (idempotente)**

```go
// cmd/seed/main.go
// seed popola il database con dati demo: utenti, un progetto Scrum e issue di esempio.
// Uso: APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=./dev.db go run ./cmd/seed
package main

import (
	"fmt"
	"log"

	"github.com/open-jira/open-jira/internal/config"
	"github.com/open-jira/open-jira/internal/domain/auth"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
	"github.com/open-jira/open-jira/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	s, err := store.New(cfg.DB, cfg.Env)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer s.Close()
	if err := store.RunMigrations(cfg.DB); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	authSvc := auth.NewService(s.DB, cfg.Secret)

	demoUsers := []struct{ email, username, name, password string }{
		{"admin@example.com", "admin", "Ada Admin", "admin-demo-123"},
		{"dev@example.com", "dev", "Devi Developer", "dev-demo-123"},
		{"pm@example.com", "pm", "Paolo PM", "pm-demo-123"},
	}
	for _, du := range demoUsers {
		var existing user.User
		if err := s.DB.Where("email = ?", du.email).First(&existing).Error; err == nil {
			continue // già presente: seed idempotente
		}
		if _, err := authSvc.Register(du.email, du.username, du.name, du.password); err != nil {
			log.Fatalf("register %s: %v", du.email, err)
		}
		fmt.Printf("created user %s (password: %s)\n", du.email, du.password)
	}

	var admin user.User
	if err := s.DB.Where("email = ?", "admin@example.com").First(&admin).Error; err != nil {
		log.Fatal(err)
	}

	projSvc := project.NewService(s.DB, &admin)
	var demo *project.Project
	if p, err := projSvc.GetByKey("DEMO"); err == nil {
		demo = p
	} else {
		demo, err = projSvc.Create("Demo Project", "DEMO", "Progetto demo con dati di esempio", project.Type("scrum"))
		if err != nil {
			log.Fatalf("create project: %v", err)
		}
		fmt.Println("created project DEMO")
	}

	issueSvc := issue.NewService(s.DB)
	var count int64
	s.DB.Model(&issue.Issue{}).Where("project_id = ?", demo.ID).Count(&count)
	if count == 0 {
		samples := []struct {
			title, desc string
			prio        issue.Priority
		}{
			{"Set up project skeleton", "Bootstrap iniziale del progetto.", issue.Priority("high")},
			{"Design login page", "Login con email e password.", issue.Priority("medium")},
			{"Fix flaky board test", "Il test della board fallisce a intermittenza.", issue.Priority("highest")},
			{"Write onboarding docs", "Guida per i nuovi contributor.", issue.Priority("low")},
			{"Implement dark mode", "Tema scuro per la UI.", issue.Priority("medium")},
		}
		for _, it := range samples {
			if _, err := issueSvc.Create(demo.Key, demo.ID, it.title, it.desc, it.prio, nil, nil); err != nil {
				log.Fatalf("create issue %q: %v", it.title, err)
			}
		}
		fmt.Printf("created %d issues in DEMO\n", len(samples))
	}
	fmt.Println("seed complete")
}
```

Nota: i valori di `issue.Priority` validi sono nel modello (`internal/domain/issue/model.go`, default `medium`) — verificare le costanti esistenti prima di usare stringhe.

- [ ] **Step 2: Prova il seeder su un DB pulito**

Run: `APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=/tmp/seed-test.db go run ./cmd/seed && APP_SECRET=dev DB_DRIVER=sqlite DB_DSN=/tmp/seed-test.db go run ./cmd/seed`
Expected: prima esecuzione crea utenti/progetto/issue; la seconda non duplica nulla (idempotente); exit 0 entrambe.

- [ ] **Step 3: Commit**

```bash
git add cmd/seed/
git commit -m "feat(seed): idempotent demo data seeder (users, DEMO project, issues)"
```

---

### Task 10: CI con GitHub Actions

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Scrivi il workflow**

```yaml
# .github/workflows/ci.yml
name: CI
on:
  push:
    branches: [master, main]
  pull_request:

jobs:
  backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Build
        run: go build ./...
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test ./... -count=1

  gap-report:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Gap report is up to date
        run: |
          go run ./cmd/gapreport
          git diff --exit-code docs/contracts/gap-report.md \
            || (echo "::error::gap-report.md non aggiornato: eseguire 'go run ./cmd/gapreport' e committare" && exit 1)

  frontend:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend-next
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: frontend-next/package-lock.json
      - run: npm ci
      - run: npm run build

  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: frontend-next/package-lock.json
      - name: Install frontend deps
        working-directory: frontend-next
        run: npm ci && npx playwright install --with-deps chromium
      - name: Run E2E
        working-directory: frontend-next
        run: npx playwright test
        env:
          APP_SECRET: ci-secret
```

Nota: il job `e2e` funziona dopo il Task 12 (config Playwright con `webServer` che avvia backend seedato + frontend). Se il Task 12 non è ancora fatto quando questo task viene eseguito, committare il workflow senza il job `e2e` e aggiungerlo nel Task 12.

- [ ] **Step 2: Verifica sintassi ed esegui localmente l'equivalente**

Run: `go build ./... && go vet ./... && go test ./... -count=1 && (cd frontend-next && npm ci && npm run build)`
Expected: tutto verde in locale.

- [ ] **Step 3: Commit e push**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add backend, frontend, gap-report and e2e workflows"
git push -u origin feat/frontend-next
```

Poi verificare su GitHub che il workflow parta e sia verde (`gh run watch` o `gh run list --limit 1`).

---

### Task 11: Fondamenta frontend-next — TanStack Query + errori Jira

**Files:**
- Modify: `frontend-next/package.json` (dipendenza)
- Create: `frontend-next/app/providers.tsx`
- Modify: `frontend-next/app/layout.tsx`
- Modify: `frontend-next/lib/api.ts` (parsing errori Jira)

- [ ] **Step 1: Installa TanStack Query**

Run: `cd frontend-next && npm install @tanstack/react-query`
Expected: dipendenza aggiunta.

- [ ] **Step 2: Crea il provider**

```tsx
// frontend-next/app/providers.tsx
"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { staleTime: 30_000, retry: 1 },
        },
      })
  );
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
```

- [ ] **Step 3: Monta il provider nel layout root**

In `frontend-next/app/layout.tsx`, avvolgere `{children}` con `<Providers>`:

```tsx
import { Providers } from "./providers";
// ... nel JSX del body:
<body className={...}>
  <Providers>{children}</Providers>
</body>
```

(mantenere className e struttura esistenti; aggiungere solo il wrapper).

- [ ] **Step 4: Adegua il parsing errori in lib/api.ts**

In `frontend-next/lib/api.ts`, sostituire il blocco `if (!res.ok) { ... }` di `apiFetch` con:

```ts
if (!res.ok) {
    const text = await res.text();
    let msg = `HTTP ${res.status}`;
    try {
      const json = JSON.parse(text);
      // Formato Jira v3: { errorMessages: string[], errors: Record<string,string> }
      if (Array.isArray(json.errorMessages) && json.errorMessages.length > 0) {
        msg = json.errorMessages.join(" ");
      } else if (json.errors && Object.keys(json.errors).length > 0) {
        msg = Object.entries(json.errors)
          .map(([field, err]) => `${field}: ${err}`)
          .join("; ");
      } else if (json.error) {
        msg = json.error; // retrocompatibilità con endpoint non ancora migrati
      }
    } catch {
      /* ignore */
    }
    throw new Error(msg);
}
```

- [ ] **Step 5: Verifica build**

Run: `cd frontend-next && npm run build`
Expected: build OK, nessun errore TypeScript.

- [ ] **Step 6: Commit**

```bash
git add frontend-next/package.json frontend-next/package-lock.json frontend-next/app/providers.tsx frontend-next/app/layout.tsx frontend-next/lib/api.ts
git commit -m "feat(frontend): add TanStack Query provider and Jira v3 error parsing"
```

---

### Task 12: Playwright E2E

**Files:**
- Create: `frontend-next/playwright.config.ts`
- Create: `frontend-next/e2e/login.spec.ts`
- Create: `scripts/e2e-backend.sh`
- Modify: `frontend-next/package.json` (script + devDependency)

- [ ] **Step 1: Installa Playwright**

Run: `cd frontend-next && npm install -D @playwright/test && npx playwright install chromium`
Expected: installato senza errori.

- [ ] **Step 2: Script di avvio backend per E2E**

```bash
#!/usr/bin/env bash
# scripts/e2e-backend.sh — avvia il backend su SQLite effimero seedato, porta 8080.
set -euo pipefail
cd "$(dirname "$0")/.."
export APP_SECRET="${APP_SECRET:-e2e-secret}"
export DB_DRIVER=sqlite
export DB_DSN="${DB_DSN:-/tmp/openjira-e2e.db}"
rm -f "$DB_DSN"
go run ./cmd/seed
exec go run ./cmd/server
```

Run: `chmod +x scripts/e2e-backend.sh`

- [ ] **Step 3: Config Playwright**

```ts
// frontend-next/playwright.config.ts
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  retries: process.env.CI ? 2 : 0,
  use: {
    baseURL: "http://localhost:3000",
    trace: "on-first-retry",
  },
  webServer: [
    {
      command: "bash ../scripts/e2e-backend.sh",
      url: "http://localhost:8080/rest/api/3/auth/login",
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      // l'endpoint risponde 405 a GET: qualsiasi risposta HTTP = server su
      ignoreHTTPSErrors: true,
    },
    {
      command: "npm run dev",
      url: "http://localhost:3000/login",
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: { NEXT_PUBLIC_API_URL: "http://localhost:8080" },
    },
  ],
});
```

Nota: se il check `url` sul backend non passa (Playwright richiede status 2xx), aggiungere nel backend una route `GET /healthz` che risponde `200 ok` (3 righe in `router.go`) e usarla come `url`.

- [ ] **Step 4: Primo test E2E — login e lista progetti**

```ts
// frontend-next/e2e/login.spec.ts
import { test, expect } from "@playwright/test";

test("login con utente demo e arrivo sui progetti", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.getByRole("button", { name: /log ?in|accedi/i }).click();

  await page.waitForURL(/\/jira\/projects/);
  await expect(page.getByText("Demo Project")).toBeVisible();
});

test("login con credenziali errate mostra errore", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("wrong-password");
  await page.getByRole("button", { name: /log ?in|accedi/i }).click();

  await expect(page.getByText(/invalid|errat|incorrect/i)).toBeVisible();
});
```

Nota: adattare i selettori alla pagina reale `frontend-next/app/login/page.tsx` (leggerla prima: se gli input non hanno `<label>`, usare `page.getByPlaceholder(...)` o aggiungere le label — preferibile aggiungerle, servono anche per l'accessibilità).

- [ ] **Step 5: Aggiungi lo script npm**

In `frontend-next/package.json`, sezione scripts: `"e2e": "playwright test"`.

- [ ] **Step 6: Esegui**

Run: `cd frontend-next && npx playwright test`
Expected: 2 passed.

- [ ] **Step 7: Commit**

```bash
git add frontend-next/playwright.config.ts frontend-next/e2e/ frontend-next/package.json frontend-next/package-lock.json scripts/e2e-backend.sh
git commit -m "test(e2e): Playwright setup with seeded backend and login specs"
```

---

### Task 13: Riferimento UI — cattura schermate di Jira reale

**Files:**
- Create: `docs/ui-reference/README.md`
- Create: `docs/ui-reference/*.png` (catturate via browser)

Questo task richiede la sessione Chrome dell'utente (istanza `harpaitalia.atlassian.net`) e va eseguito dall'agente principale con gli strumenti browser, non da un subagente sandboxed.

- [ ] **Step 1: Scrivi la checklist delle schermate**

```markdown
# UI Reference — Jira Cloud (harpaitalia.atlassian.net)

Screenshot di riferimento per la parità visiva. Catturati il 2026-07-10.
NON ridistribuire: uso interno di sviluppo, contengono layout Atlassian e dati aziendali.
Questa cartella è in .gitignore: i file restano solo in locale.

| File | Schermata | Note |
|---|---|---|
| 01-projects-list.png | Elenco progetti | filtri, tabella, star |
| 02-board.png | Board Scrum | colonne, card, header sprint |
| 03-backlog.png | Backlog | sprint collassabili, inline create |
| 04-issue-detail.png | Vista issue | pannello dx, commenti, ADF |
| 05-issue-create.png | Modal creazione issue | |
| 06-search.png | Ricerca / list view | JQL bar, colonne |
| 07-timeline.png | Timeline | barre epic, zoom |
| 08-project-settings.png | Impostazioni progetto | menu laterale |
| 09-navigation.png | Top nav + sidebar | menu Projects aperto |
| 10-dashboard.png | Dashboard | gadget |
```

- [ ] **Step 2: Escludi le immagini dal repo**

Aggiungere a `.gitignore` (root):

```
docs/ui-reference/*.png
```

Motivo: gli screenshot contengono dati aziendali reali e UI proprietaria Atlassian — servono solo come riferimento locale di sviluppo.

- [ ] **Step 3: Cattura le 10 schermate**

Con la sessione Chrome dell'utente: navigare su ciascuna schermata della checklist e salvare lo screenshot in `docs/ui-reference/` con il nome indicato. Verificare con `ls docs/ui-reference/*.png | wc -l` → 10.

- [ ] **Step 4: Commit (solo README e .gitignore)**

```bash
git add docs/ui-reference/README.md .gitignore
git commit -m "docs(ui): add UI reference checklist for visual parity (screenshots local-only)"
```

---

## Nota sulla shell UI

La shell (TopBar + Sidebar) esiste già in `frontend-next/components/layout/`. Il Round 0 la lascia in piedi e cattura i riferimenti visivi (Task 13); l'adeguamento visivo fine della shell al layout Jira reale è il **primo task del Round 1**, quando i riferimenti sono disponibili e si lavora comunque sulla UI progetti.

## Ordine di esecuzione e dipendenze

```
Task 1 (contratti) ──> Task 2 (gap report) 
        │
        └──> Task 7 (harness) ──> Task 8 (/myself conforme)
Task 3 (v3 errori) ──> Task 4 (v3 params)     [Task 3 richiesto da Task 6 e 8]
Task 5 (ADF)                                   [indipendente]
Task 6 (auth Basic) ──> Task 8
Task 9 (seed) ──> Task 12 (Playwright)
Task 10 (CI)                                   [dopo Task 2; job e2e dopo Task 12]
Task 11 (frontend foundation) ──> Task 12
Task 13 (UI reference)                         [indipendente, richiede browser]
```

Parallelizzabili in prima battuta: Task 1+3+5+9+11 (nessuna dipendenza reciproca).

## Definition of Done del Round 0

- `go build ./... && go vet ./... && go test ./...` verdi.
- `docs/contracts/gap-report.md` generato e committato.
- `GET /rest/api/3/myself` passa il contract test contro l'OpenAPI ufficiale.
- Login E2E Playwright verde con backend seedato.
- CI verde su GitHub.
- 10 screenshot di riferimento in `docs/ui-reference/` (locali).
