# Round 4 — Ricerca & JQL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dare a open-jira un vero motore JQL (lexer → parser → AST → SQL builder) e gli endpoint di ricerca e filtri salvati conformi a Jira Cloud REST API v3, con UI di ricerca globale, list view a colonne e pagina filtri.

**Architecture:** Nuovo package `internal/jql` con pipeline pura (token → AST → `Compile` che produce `WHERE`+args+`ORDER BY` per GORM). Un `Resolver` (interfaccia) traduce nomi Jira (progetto, stato, tipo, utente, `currentUser()`) in id interni, tenendo il package JQL disaccoppiato dal dominio. Il dominio `search.Service` esegue la query compilata su `issues`. Il layer `internal/api/v3` aggiunge i mapper delle risposte (`SearchResults` offset, `SearchAndReconcileResults` token-paginato via cursore base64) e la proiezione dei `fields`. I filtri salvati diventano conformi allo schema `Filter`/`PageBeanFilterDetails`. Frontend: client tipizzato + ricerca globale + pagina `/jira/filters` con list view a colonne configurabili.

**Tech Stack:** Go 1.25 (net/http ServeMux, GORM, golang-migrate, SQLite in test), package esistenti `internal/api/v3` (WriteJSON/WriteError/WritePage/JiraIssue/ParseFields/ParsePagination), harness `internal/contract` (kin-openapi), Next.js 16 + React 19 + TanStack Query + Tailwind + Playwright.

---

## Contesto per l'implementatore (leggere una volta)

**Contratto ufficiale** versionato in `docs/contracts/jira-platform-v3.json`. Schemi rilevanti (nomi di proprietà ESATTI):

- `SearchAndReconcileRequestBean` (POST `/search/jql`): `jql:string`, `maxResults:int32`, `nextPageToken:string`, `fields:[]string`, `expand:string` (stringa, NON array), `fieldsByKeys:bool`, `properties:[]string`, `reconcileIssues:[]int`.
- `SearchAndReconcileResults` (risposta di `/search/jql`): `issues:[]IssueBean`, `nextPageToken:string`, `isLast:bool`, `names:object`, `schema:object`, `warnings:[]SearchWarning`. **Token-paginato: NIENTE `startAt`/`maxResults`/`total`.**
- `SearchRequestBean` (POST legacy `/search`): `jql:string`, `startAt:int32`, `maxResults:int32`, `fields:[]string`, `expand:[]string` (array qui), `properties:[]string`, `fieldsByKeys:bool`, `validateQuery:string`.
- `SearchResults` (risposta legacy `/search`): `issues:[]IssueBean`, `startAt:int32`, `maxResults:int32`, `total:int32`, `expand:string`, `names:object`, `schema:object`, `warningMessages:[]string`. **Offset-paginato.**
- `JQLCountRequestBean`: `jql:string`. `JQLCountResultsBean`: `count:int64`.
- `Filter`: `id:string`, `self:uri`, `name:string` (**required**), `description:string`, `jql:string`, `owner:User`, `favourite:bool`, `favouritedCount:int64`, `searchUrl:uri`, `viewUrl:uri`, `sharePermissions:[]SharePermission`, `editPermissions:[]SharePermission`.
- `PageBeanFilterDetails` (risposta `/filter/search`): offset-paginato `startAt:int64`, `maxResults:int32`, `total:int64`, `isLast:bool`, `self:uri`, `nextPage:uri`, `values:[]FilterDetails`. `FilterDetails` = come `Filter` + `expand:string`.
- `JQLReferenceData` (risposta `/jql/autocompletedata`): `visibleFieldNames:[]FieldReferenceData`, `visibleFunctionNames:[]FunctionReferenceData`, `jqlReservedWords:[]string`. `FieldReferenceData`: `value:string`, `displayName:string`, `orderable:"true"|"false"`, `searchable:"true"|"false"`, `operators:[]string`, `types:[]string`. `FunctionReferenceData`: `value:string`, `displayName:string`, `isList:"true"|"false"`, `types:[]string`.

**Primitive v3 già esistenti (riusare, non reinventare):**
- `internal/api/v3/respond.go`: `WriteJSON(w, status, v)`, `WriteError(w, status, messages []string, fieldErrors map[string]string)`, `WritePage[T](w, status, Page[T]{StartAt,MaxResults,Total,Values})` (aggiunge `isLast` da solo; chiavi json `startAt/maxResults/total/isLast/values`).
- `internal/api/v3/params.go`: `ParsePagination(r, defaultMax, capMax) (startAt, maxResults int)`, `ParseFields(r) Fields` con `f.Include(name) bool` (gestisce `*all`, `*navigable`, `-field`), `ParseExpand(r) Expand`.
- `internal/api/v3/issue.go`: `JiraIssue(in IssueInput) IssueBean` costruisce il bean ufficiale; `IssueInput` (issue.go:66) porta l'issue di dominio + entità risolte (Assignee/Reporter/IssueType/Status/Resolution/Project/Parent/Labels + BaseURL). `IssueBean{Self,ID,Key,Fields}`, `IssueFields` con chiavi `summary/description/issuetype/status/priority/assignee/reporter/resolution/project/parent/labels/created/updated/duedate/customfield_10016/timetracking`. `ID`/`Self` usano `iss.SeqID`.
- `internal/api/handlers/issue_handler.go`: `IssueHandler.buildIssueInput(iss *issue.Issue) v3.IssueInput` (issue_handler.go:53) fa i lookup per costruire l'`IssueInput`. Riusarlo nel search handler.

**Dominio issue (`internal/domain/issue/`):**
- `issue.Issue` (model.go:27): campi rilevanti `ID`, `ProjectID`, `Key`, `Title` (**il summary si chiama `Title`, colonna `title`**), `DescriptionJSON` (`description_json`), `TypeID *string`, `StatusID *string`, `Priority Priority` (enum stringa `highest/high/medium/low/lowest`, colonna testo `priority`), `AssigneeID *string`, `ReporterID *string`, `ResolutionID *string`, `ParentID *string`, `SprintID *string`, `IsArchived bool`, `SeqID int64` (id pubblico numerico), `CreatedAt`, `UpdatedAt`.
- `issue.Service.DB() *gorm.DB` (service.go:228) espone il gorm.DB.
- Labels: tabelle separate `labels(id, project_id, name, color)` e join `issue_labels(issue_id, label_id)` (migrations/000001). Nomi via `issue.Service.GetLabels(issueID) ([]string, error)`.

**Router (`internal/api/router.go`):** `mux := http.NewServeMux()`; middleware `authMw := middleware.Auth(...)` (router.go:107). Pattern rotta autenticata: `mux.Handle("GET /rest/api/3/<path>", authMw(http.HandlerFunc(<handler>.<Method>)))`. Path param via `r.PathValue("id")`. Le rotte search/filter attuali sono a router.go:225-242 e vanno **riscritte** in T13.

**Codice ESISTENTE da RIMPIAZZARE (non conforme):**
- `internal/domain/search/parser.go` (finto JQL `key=value`), `internal/domain/search/service.go` (ritorna `[]issue.Issue` grezzi).
- `internal/api/handlers/search_handler.go` (shape bespoke; rotte plurali `/filters`).
- `internal/domain/search/saved_filter.go` (`FilterService`) — si MANTIENE ma si estende (T11): la tabella `saved_filters` (migrations/000001:239) ha `id, project_id, owner_id, name, jql, is_shared, created_at` ma **manca `is_favourite`** (già usata dal modello e da `ToggleFavourite`) e `description`. La migrazione 000011 (T1) le aggiunge.

**Migrazioni:** la più alta è `000010_remote_links`. La prossima è **`000011`**.

**Harness contract (`internal/contract/`):** `MustLoad(tb, path)`, `newTestServer(t)`, `registerAndLogin`, `createProjectViaAPI`, `createIssueViaAPI`. I test contract validano la risposta contro l'OpenAPI. Attenzione: i campi `format:date-time` vanno in RFC3339 con offset `:` (già gestito da `v3.JiraTime`); usare `omitempty` per non emettere `null` su campi non `nullable`.

**Convenzioni di test Go:** `go test ./...` dalla root. Test contract: `go test ./internal/contract/ -run <Name> -v`.

**Scope escluso da questo round (follow-up):** `POST /jql/parse` (validazione client-side non necessaria alla UI), `sharePermissions`/`editPermissions` reali (emettiamo array vuoti conformi), sottoscrizioni filtro, colonne salvate lato server.

---

## Struttura dei file

**Nuovo package `internal/jql`** (pipeline pura, nessuna dipendenza dal dominio):
- `internal/jql/token.go` — tipi token + lexer `Lex(input) ([]Token, error)`.
- `internal/jql/ast.go` — nodi AST (`Query`, `And`, `Or`, `Not`, `Clause`, `OrderBy`).
- `internal/jql/parser.go` — `Parse(input) (*Query, error)` (discesa ricorsiva).
- `internal/jql/compile.go` — `Resolver` interface + `Compile(q *Query, r Resolver) (*Compiled, error)` → `Compiled{Where string, Args []any, Order string}`.
- Test affiancati: `token_test.go`, `parser_test.go`, `compile_test.go`.

**Dominio search (riscrittura):**
- `internal/domain/search/service.go` — `Service.Search(jql string, r jql.Resolver, offset, limit int) (SearchResult, error)` con `SearchResult{Issues []issue.Issue, Total int}`.
- `internal/domain/search/resolver.go` — `DBResolver` che implementa `jql.Resolver` sul gorm.DB (progetto/stato/tipo/utente) + `currentUserID`.
- `internal/domain/search/saved_filter.go` — esteso con `Description` e metodo `SetFavourite`/`Update` aggiornato (T11).

**Layer v3:**
- `internal/api/v3/search.go` — `SearchResults`, `SearchAndReconcileResults`, `EncodeCursor(offset int) string` / `DecodeCursor(token string) (int, error)`, `Filter`, `PageBeanFilterDetails`, `JiraFilter(...)`, `JQLReferenceData` + `AutocompleteData()`.
- `internal/api/v3/fields.go` — `ProjectFields(bean IssueBean, f Fields) (map[string]any, error)` (proiezione `fields`).

**Handler:**
- `internal/api/handlers/search_handler.go` — riscritto: `SearchJQL` (GET+POST `/search/jql`), `SearchLegacy` (GET+POST `/search`), `ApproximateCount`, `Autocomplete`.
- `internal/api/handlers/filter_handler.go` — nuovo: `Create/Get/Update/Delete/Search/My/Favourite/AddFavourite/RemoveFavourite`.

**Migrazioni:** `migrations/000011_filter_fields.up.sql` / `.down.sql`.

**Frontend:**
- `frontend-next/lib/api.ts` — aggiungere tipi + `search.jql(...)`, `filters.*`.
- `frontend-next/app/jira/filters/page.tsx` — pagina filtri + list view.
- `frontend-next/components/search/SearchResults.tsx` — list view a colonne.
- `frontend-next/components/search/GlobalSearch.tsx` — ricerca globale in top nav.
- `frontend-next/e2e/search.spec.ts` — E2E.

**Seed:** `cmd/seed/main.go` — un filtro salvato demo.

---

### Task 1: Migrazione 000011 — colonne filtro

**Files:**
- Create: `migrations/000011_filter_fields.up.sql`
- Create: `migrations/000011_filter_fields.down.sql`

- [ ] **Step 1: Scrivere la migrazione up**

`migrations/000011_filter_fields.up.sql`:

```sql
ALTER TABLE saved_filters ADD COLUMN description TEXT DEFAULT '';
ALTER TABLE saved_filters ADD COLUMN is_favourite BOOLEAN DEFAULT FALSE;
```

- [ ] **Step 2: Scrivere la migrazione down**

`migrations/000011_filter_fields.down.sql`:

```sql
ALTER TABLE saved_filters DROP COLUMN is_favourite;
ALTER TABLE saved_filters DROP COLUMN description;
```

- [ ] **Step 3: Verificare che le migrazioni si applichino a pulito**

Run: `DB_DRIVER=sqlite DB_DSN=/tmp/mig-test.db go run ./cmd/seed && rm -f /tmp/mig-test.db`
Expected: exit 0, nessun errore di migrazione (il seed applica tutte le migrazioni all'avvio).

- [ ] **Step 4: Commit**

```bash
git add migrations/000011_filter_fields.up.sql migrations/000011_filter_fields.down.sql
git commit -m "feat(migrations): add description and is_favourite to saved_filters"
```

---

### Task 2: JQL lexer

**Files:**
- Create: `internal/jql/token.go`
- Test: `internal/jql/token_test.go`

- [ ] **Step 1: Scrivere i test del lexer**

`internal/jql/token_test.go`:

```go
package jql

import "testing"

func TestLex_SimpleClause(t *testing.T) {
	toks, err := Lex(`project = DEMO`)
	if err != nil {
		t.Fatalf("Lex: %v", err)
	}
	want := []Token{
		{Kind: TokIdent, Val: "project"},
		{Kind: TokOp, Val: "="},
		{Kind: TokIdent, Val: "DEMO"},
		{Kind: TokEOF},
	}
	assertTokens(t, toks, want)
}

func TestLex_QuotedString(t *testing.T) {
	toks, _ := Lex(`summary ~ "login page"`)
	if toks[2].Kind != TokString || toks[2].Val != "login page" {
		t.Fatalf("quoted string not lexed: %+v", toks[2])
	}
}

func TestLex_OperatorsAndKeywords(t *testing.T) {
	toks, _ := Lex(`status IN (Done, "In Progress") AND assignee != currentUser() ORDER BY created DESC`)
	kinds := []TokKind{}
	for _, tk := range toks {
		kinds = append(kinds, tk.Kind)
	}
	// ident IN ( ident , string ) AND ident != ident ( ) ORDER BY ident DESC EOF
	// verifichiamo che parentesi, virgole e keyword multi-parola siano token distinti
	if toks[1].Kind != TokKeyword || toks[1].Val != "IN" {
		t.Errorf("IN non riconosciuto come keyword: %+v", toks[1])
	}
	if toks[2].Kind != TokLParen {
		t.Errorf("( atteso, %+v", toks[2])
	}
	if toks[6].Kind != TokRParen {
		t.Errorf(") atteso, %+v", toks[6])
	}
}

func TestLex_NotEqualAndComparators(t *testing.T) {
	toks, _ := Lex(`a != b >= c <= d > e < f !~ g ~ h`)
	ops := []string{}
	for _, tk := range toks {
		if tk.Kind == TokOp {
			ops = append(ops, tk.Val)
		}
	}
	got := len(ops)
	if got != 7 {
		t.Fatalf("attesi 7 operatori, trovati %d: %v", got, ops)
	}
	if ops[0] != "!=" || ops[1] != ">=" || ops[2] != "<=" {
		t.Errorf("operatori multi-char errati: %v", ops)
	}
}

func TestLex_UnterminatedString(t *testing.T) {
	if _, err := Lex(`summary ~ "oops`); err == nil {
		t.Fatal("attesa err per stringa non terminata")
	}
}

func assertTokens(t *testing.T, got, want []Token) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len token: got %d want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Val != want[i].Val {
			t.Errorf("token[%d]: got %+v want %+v", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 2: Eseguire i test (devono fallire)**

Run: `go test ./internal/jql/ -run TestLex -v`
Expected: FAIL con "undefined: Lex" / "undefined: Token".

- [ ] **Step 3: Implementare il lexer**

`internal/jql/token.go`:

```go
// Package jql implementa un lexer, parser e compilatore per un sottoinsieme
// realistico di JQL (Jira Query Language) verso SQL/GORM.
package jql

import (
	"fmt"
	"strings"
	"unicode"
)

type TokKind int

const (
	TokEOF TokKind = iota
	TokIdent
	TokString
	TokNumber
	TokOp      // = != > >= < <= ~ !~
	TokKeyword // AND OR NOT IN IS EMPTY NULL ASC DESC ORDER BY
	TokLParen
	TokRParen
	TokComma
)

type Token struct {
	Kind TokKind
	Val  string
}

// keywords riconosciute (case-insensitive). Il valore canonico è maiuscolo.
var keywords = map[string]bool{
	"AND": true, "OR": true, "NOT": true, "IN": true, "IS": true,
	"EMPTY": true, "NULL": true, "ORDER": true, "BY": true, "ASC": true, "DESC": true,
}

// Lex trasforma la stringa JQL in una lista di token terminata da TokEOF.
func Lex(input string) ([]Token, error) {
	var toks []Token
	rs := []rune(input)
	i := 0
	for i < len(rs) {
		c := rs[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case c == '(':
			toks = append(toks, Token{Kind: TokLParen, Val: "("})
			i++
		case c == ')':
			toks = append(toks, Token{Kind: TokRParen, Val: ")"})
			i++
		case c == ',':
			toks = append(toks, Token{Kind: TokComma, Val: ","})
			i++
		case c == '"' || c == '\'':
			quote := c
			i++
			start := i
			for i < len(rs) && rs[i] != quote {
				i++
			}
			if i >= len(rs) {
				return nil, fmt.Errorf("stringa non terminata")
			}
			toks = append(toks, Token{Kind: TokString, Val: string(rs[start:i])})
			i++ // salta quote di chiusura
		case c == '=' || c == '~':
			toks = append(toks, Token{Kind: TokOp, Val: string(c)})
			i++
		case c == '!':
			if i+1 < len(rs) && (rs[i+1] == '=' || rs[i+1] == '~') {
				toks = append(toks, Token{Kind: TokOp, Val: string(rs[i : i+2])})
				i += 2
			} else {
				return nil, fmt.Errorf("carattere inatteso '!'")
			}
		case c == '>' || c == '<':
			if i+1 < len(rs) && rs[i+1] == '=' {
				toks = append(toks, Token{Kind: TokOp, Val: string(rs[i : i+2])})
				i += 2
			} else {
				toks = append(toks, Token{Kind: TokOp, Val: string(c)})
				i++
			}
		default:
			// identificatore/numero/keyword: sequenza di caratteri non separatori
			start := i
			for i < len(rs) && !isSep(rs[i]) {
				i++
			}
			word := string(rs[start:i])
			up := strings.ToUpper(word)
			if keywords[up] {
				toks = append(toks, Token{Kind: TokKeyword, Val: up})
			} else if isNumber(word) {
				toks = append(toks, Token{Kind: TokNumber, Val: word})
			} else {
				toks = append(toks, Token{Kind: TokIdent, Val: word})
			}
		}
	}
	toks = append(toks, Token{Kind: TokEOF})
	return toks, nil
}

func isSep(r rune) bool {
	return unicode.IsSpace(r) || r == '(' || r == ')' || r == ',' ||
		r == '=' || r == '!' || r == '~' || r == '>' || r == '<' ||
		r == '"' || r == '\''
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	dot := false
	for i, r := range s {
		if r == '-' && i == 0 {
			continue
		}
		if r == '.' && !dot {
			dot = true
			continue
		}
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Eseguire i test (devono passare)**

Run: `go test ./internal/jql/ -run TestLex -v`
Expected: PASS (tutti i test lexer).

- [ ] **Step 5: Commit**

```bash
git add internal/jql/token.go internal/jql/token_test.go
git commit -m "feat(jql): lexer for JQL tokens"
```

---

### Task 3: JQL AST + parser

**Files:**
- Create: `internal/jql/ast.go`
- Create: `internal/jql/parser.go`
- Test: `internal/jql/parser_test.go`

- [ ] **Step 1: Scrivere l'AST**

`internal/jql/ast.go`:

```go
package jql

// Node è l'interfaccia comune dei nodi dell'albero di condizioni.
type Node interface{ isNode() }

// Query è la radice: una condizione opzionale + ordinamento opzionale.
type Query struct {
	Where Node       // nil se assente (query "tutte le issue")
	Order []OrderKey // vuoto se assente
}

// And / Or / Not compongono le condizioni.
type And struct{ Left, Right Node }
type Or struct{ Left, Right Node }
type Not struct{ Inner Node }

// Clause è una condizione atomica: field OP value.
type Clause struct {
	Field    string
	Op       string   // = != > >= < <= ~ !~ IN "NOT IN" IS "IS NOT"
	Value    string   // per operatori scalari
	Values   []string // per IN / NOT IN
	Func     string   // nome funzione se il valore è una funzione, es. "currentUser"
	IsEmpty  bool     // true se il valore è EMPTY/NULL (con IS / IS NOT)
}

// OrderKey è un campo di ordinamento con direzione.
type OrderKey struct {
	Field string
	Desc  bool
}

func (And) isNode()    {}
func (Or) isNode()     {}
func (Not) isNode()    {}
func (*Clause) isNode() {}
```

- [ ] **Step 2: Scrivere i test del parser**

`internal/jql/parser_test.go`:

```go
package jql

import "testing"

func TestParse_SingleClause(t *testing.T) {
	q, err := Parse(`project = DEMO`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	c, ok := q.Where.(*Clause)
	if !ok {
		t.Fatalf("atteso *Clause, got %T", q.Where)
	}
	if c.Field != "project" || c.Op != "=" || c.Value != "DEMO" {
		t.Errorf("clause errata: %+v", c)
	}
}

func TestParse_AndOrPrecedence(t *testing.T) {
	// AND lega più forte di OR: a=1 OR b=2 AND c=3  =>  Or(a=1, And(b=2, c=3))
	q, err := Parse(`a = 1 OR b = 2 AND c = 3`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	or, ok := q.Where.(Or)
	if !ok {
		t.Fatalf("atteso Or alla radice, got %T", q.Where)
	}
	if _, ok := or.Right.(And); !ok {
		t.Errorf("atteso And come figlio destro di Or, got %T", or.Right)
	}
}

func TestParse_Parens(t *testing.T) {
	q, _ := Parse(`(a = 1 OR b = 2) AND c = 3`)
	and, ok := q.Where.(And)
	if !ok {
		t.Fatalf("atteso And alla radice, got %T", q.Where)
	}
	if _, ok := and.Left.(Or); !ok {
		t.Errorf("atteso Or a sinistra, got %T", and.Left)
	}
}

func TestParse_InList(t *testing.T) {
	q, _ := Parse(`status IN (Done, "In Progress", Blocked)`)
	c := q.Where.(*Clause)
	if c.Op != "IN" {
		t.Fatalf("op atteso IN, got %q", c.Op)
	}
	if len(c.Values) != 3 || c.Values[1] != "In Progress" {
		t.Errorf("values errati: %v", c.Values)
	}
}

func TestParse_NotIn(t *testing.T) {
	q, _ := Parse(`status NOT IN (Done)`)
	c := q.Where.(*Clause)
	if c.Op != "NOT IN" || len(c.Values) != 1 {
		t.Errorf("NOT IN errato: %+v", c)
	}
}

func TestParse_IsEmpty(t *testing.T) {
	q, _ := Parse(`assignee IS EMPTY`)
	c := q.Where.(*Clause)
	if c.Op != "IS" || !c.IsEmpty {
		t.Errorf("IS EMPTY errato: %+v", c)
	}
	q2, _ := Parse(`resolution IS NOT EMPTY`)
	c2 := q2.Where.(*Clause)
	if c2.Op != "IS NOT" || !c2.IsEmpty {
		t.Errorf("IS NOT EMPTY errato: %+v", c2)
	}
}

func TestParse_Function(t *testing.T) {
	q, _ := Parse(`assignee = currentUser()`)
	c := q.Where.(*Clause)
	if c.Func != "currentUser" {
		t.Errorf("funzione non riconosciuta: %+v", c)
	}
}

func TestParse_OrderBy(t *testing.T) {
	q, _ := Parse(`project = DEMO ORDER BY priority DESC, created ASC`)
	if len(q.Order) != 2 {
		t.Fatalf("attese 2 chiavi order, %d", len(q.Order))
	}
	if q.Order[0].Field != "priority" || !q.Order[0].Desc {
		t.Errorf("order[0] errata: %+v", q.Order[0])
	}
	if q.Order[1].Field != "created" || q.Order[1].Desc {
		t.Errorf("order[1] errata: %+v", q.Order[1])
	}
}

func TestParse_OrderByOnly(t *testing.T) {
	q, err := Parse(`ORDER BY created DESC`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if q.Where != nil {
		t.Errorf("Where deve essere nil, got %T", q.Where)
	}
	if len(q.Order) != 1 {
		t.Errorf("attesa 1 chiave order")
	}
}

func TestParse_EmptyQuery(t *testing.T) {
	q, err := Parse(``)
	if err != nil {
		t.Fatalf("Parse vuota: %v", err)
	}
	if q.Where != nil || len(q.Order) != 0 {
		t.Errorf("query vuota deve dare Where nil e nessun order")
	}
}

func TestParse_Errors(t *testing.T) {
	for _, bad := range []string{`project =`, `= DEMO`, `project == DEMO`, `status IN Done`, `(a = 1`} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("attesa err per %q", bad)
		}
	}
}
```

- [ ] **Step 3: Eseguire i test (devono fallire)**

Run: `go test ./internal/jql/ -run TestParse -v`
Expected: FAIL con "undefined: Parse".

- [ ] **Step 4: Implementare il parser**

`internal/jql/parser.go`:

```go
package jql

import (
	"fmt"
	"strings"
)

// Parse trasforma una stringa JQL in un *Query. Stringa vuota => Query senza
// condizioni né ordinamento (equivale a "tutte le issue").
func Parse(input string) (*Query, error) {
	toks, err := Lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	q := &Query{}

	// condizione opzionale: presente se non iniziamo con ORDER o EOF
	if !p.at(TokEOF) && !p.atKeyword("ORDER") {
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		q.Where = node
	}

	// ORDER BY opzionale
	if p.atKeyword("ORDER") {
		p.next()
		if !p.atKeyword("BY") {
			return nil, fmt.Errorf("atteso BY dopo ORDER")
		}
		p.next()
		order, err := p.parseOrder()
		if err != nil {
			return nil, err
		}
		q.Order = order
	}

	if !p.at(TokEOF) {
		return nil, fmt.Errorf("token inatteso in coda: %q", p.cur().Val)
	}
	return q, nil
}

type parser struct {
	toks []Token
	pos  int
}

func (p *parser) cur() Token  { return p.toks[p.pos] }
func (p *parser) next() Token { t := p.toks[p.pos]; p.pos++; return t }
func (p *parser) at(k TokKind) bool { return p.cur().Kind == k }
func (p *parser) atKeyword(k string) bool {
	return p.cur().Kind == TokKeyword && p.cur().Val == k
}

func (p *parser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.atKeyword("OR") {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = Or{Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Node, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.atKeyword("AND") {
		p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = And{Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseNot() (Node, error) {
	if p.atKeyword("NOT") {
		p.next()
		inner, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return Not{Inner: inner}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Node, error) {
	if p.at(TokLParen) {
		p.next()
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.at(TokRParen) {
			return nil, fmt.Errorf("attesa ) di chiusura")
		}
		p.next()
		return node, nil
	}
	return p.parseClause()
}

func (p *parser) parseClause() (Node, error) {
	if !p.at(TokIdent) {
		return nil, fmt.Errorf("atteso nome campo, got %q", p.cur().Val)
	}
	field := strings.ToLower(p.next().Val)
	c := &Clause{Field: field}

	switch {
	case p.at(TokOp):
		c.Op = p.next().Val
		return p.parseScalarValue(c)
	case p.atKeyword("IN"):
		p.next()
		c.Op = "IN"
		return p.parseList(c)
	case p.atKeyword("NOT"):
		p.next()
		if !p.atKeyword("IN") {
			return nil, fmt.Errorf("atteso IN dopo NOT")
		}
		p.next()
		c.Op = "NOT IN"
		return p.parseList(c)
	case p.atKeyword("IS"):
		p.next()
		c.Op = "IS"
		if p.atKeyword("NOT") {
			p.next()
			c.Op = "IS NOT"
		}
		if !p.atKeyword("EMPTY") && !p.atKeyword("NULL") {
			return nil, fmt.Errorf("atteso EMPTY/NULL dopo IS")
		}
		p.next()
		c.IsEmpty = true
		return c, nil
	default:
		return nil, fmt.Errorf("atteso operatore dopo campo %q, got %q", field, p.cur().Val)
	}
}

func (p *parser) parseScalarValue(c *Clause) (Node, error) {
	switch p.cur().Kind {
	case TokIdent:
		val := p.next().Val
		// funzione? es. currentUser()
		if p.at(TokLParen) {
			p.next()
			if !p.at(TokRParen) {
				return nil, fmt.Errorf("solo funzioni senza argomenti supportate: %s", val)
			}
			p.next()
			c.Func = val
			return c, nil
		}
		c.Value = val
		return c, nil
	case TokString, TokNumber:
		c.Value = p.next().Val
		return c, nil
	case TokKeyword:
		if p.atKeyword("EMPTY") || p.atKeyword("NULL") {
			p.next()
			c.IsEmpty = true
			return c, nil
		}
		return nil, fmt.Errorf("valore inatteso %q", p.cur().Val)
	default:
		return nil, fmt.Errorf("atteso valore dopo operatore, got %q", p.cur().Val)
	}
}

func (p *parser) parseList(c *Clause) (Node, error) {
	if !p.at(TokLParen) {
		return nil, fmt.Errorf("attesa ( dopo IN")
	}
	p.next()
	for {
		switch p.cur().Kind {
		case TokIdent, TokString, TokNumber:
			c.Values = append(c.Values, p.next().Val)
		default:
			return nil, fmt.Errorf("valore di lista inatteso %q", p.cur().Val)
		}
		if p.at(TokComma) {
			p.next()
			continue
		}
		break
	}
	if !p.at(TokRParen) {
		return nil, fmt.Errorf("attesa ) di chiusura lista")
	}
	p.next()
	if len(c.Values) == 0 {
		return nil, fmt.Errorf("lista IN vuota")
	}
	return c, nil
}

func (p *parser) parseOrder() ([]OrderKey, error) {
	var keys []OrderKey
	for {
		if !p.at(TokIdent) {
			return nil, fmt.Errorf("atteso campo in ORDER BY, got %q", p.cur().Val)
		}
		k := OrderKey{Field: strings.ToLower(p.next().Val)}
		if p.atKeyword("ASC") {
			p.next()
		} else if p.atKeyword("DESC") {
			p.next()
			k.Desc = true
		}
		keys = append(keys, k)
		if p.at(TokComma) {
			p.next()
			continue
		}
		break
	}
	return keys, nil
}
```

- [ ] **Step 5: Eseguire i test (devono passare)**

Run: `go test ./internal/jql/ -run TestParse -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/jql/ast.go internal/jql/parser.go internal/jql/parser_test.go
git commit -m "feat(jql): recursive-descent parser to AST"
```

---

### Task 4: JQL compiler (AST → SQL WHERE + ORDER BY)

**Files:**
- Create: `internal/jql/compile.go`
- Test: `internal/jql/compile_test.go`

- [ ] **Step 1: Scrivere i test del compilatore**

`internal/jql/compile_test.go`:

```go
package jql

import (
	"strings"
	"testing"
)

// fakeResolver mappa nomi → id in modo deterministico per i test.
type fakeResolver struct{}

func (fakeResolver) ProjectID(k string) (string, bool) {
	if k == "DEMO" {
		return "proj-1", true
	}
	return "", false
}
func (fakeResolver) StatusID(n string) (string, bool) {
	m := map[string]string{"To Do": "st-todo", "Done": "st-done", "todo": "st-todo"}
	id, ok := m[n]
	return id, ok
}
func (fakeResolver) TypeID(n string) (string, bool) {
	if strings.EqualFold(n, "Bug") {
		return "type-bug", true
	}
	return "", false
}
func (fakeResolver) UserID(login string) (string, bool) {
	if login == "dev" {
		return "user-dev", true
	}
	return "", false
}
func (fakeResolver) CurrentUserID() string { return "user-me" }

func compile(t *testing.T, jql string) *Compiled {
	t.Helper()
	q, err := Parse(jql)
	if err != nil {
		t.Fatalf("Parse(%q): %v", jql, err)
	}
	c, err := Compile(q, fakeResolver{})
	if err != nil {
		t.Fatalf("Compile(%q): %v", jql, err)
	}
	return c
}

func TestCompile_ProjectEq(t *testing.T) {
	c := compile(t, `project = DEMO`)
	if c.Where != "project_id = ?" {
		t.Errorf("where: %q", c.Where)
	}
	if len(c.Args) != 1 || c.Args[0] != "proj-1" {
		t.Errorf("args: %v", c.Args)
	}
}

func TestCompile_StatusInList(t *testing.T) {
	c := compile(t, `status IN ("To Do", Done)`)
	if !strings.Contains(c.Where, "status_id IN (?,?)") {
		t.Errorf("where: %q", c.Where)
	}
	if len(c.Args) != 2 {
		t.Errorf("args: %v", c.Args)
	}
}

func TestCompile_CurrentUser(t *testing.T) {
	c := compile(t, `assignee = currentUser()`)
	if c.Where != "assignee_id = ?" || c.Args[0] != "user-me" {
		t.Errorf("currentUser errato: %q %v", c.Where, c.Args)
	}
}

func TestCompile_AssigneeEmpty(t *testing.T) {
	c := compile(t, `assignee IS EMPTY`)
	if c.Where != "assignee_id IS NULL" {
		t.Errorf("where: %q", c.Where)
	}
}

func TestCompile_SummaryContains(t *testing.T) {
	c := compile(t, `summary ~ login`)
	if !strings.Contains(c.Where, "title LIKE ?") {
		t.Errorf("where: %q", c.Where)
	}
	if c.Args[0] != "%login%" {
		t.Errorf("arg like: %v", c.Args)
	}
}

func TestCompile_PriorityName(t *testing.T) {
	// priority in JQL usa i nomi capitalizzati; li normalizziamo all'enum interno.
	c := compile(t, `priority = High`)
	if c.Where != "priority = ?" || c.Args[0] != "high" {
		t.Errorf("priority errata: %q %v", c.Where, c.Args)
	}
}

func TestCompile_Labels(t *testing.T) {
	c := compile(t, `labels = backend`)
	if !strings.Contains(c.Where, "EXISTS") || !strings.Contains(c.Where, "issue_labels") {
		t.Errorf("labels deve usare EXISTS: %q", c.Where)
	}
	if c.Args[0] != "backend" {
		t.Errorf("arg label: %v", c.Args)
	}
}

func TestCompile_AndOr(t *testing.T) {
	c := compile(t, `project = DEMO AND (status = Done OR assignee = currentUser())`)
	if !strings.Contains(c.Where, " AND ") || !strings.Contains(c.Where, " OR ") {
		t.Errorf("where composta errata: %q", c.Where)
	}
	if len(c.Args) != 3 {
		t.Errorf("args: %v", c.Args)
	}
}

func TestCompile_Not(t *testing.T) {
	c := compile(t, `NOT status = Done`)
	if !strings.HasPrefix(c.Where, "NOT (") {
		t.Errorf("NOT errato: %q", c.Where)
	}
}

func TestCompile_Order(t *testing.T) {
	c := compile(t, `project = DEMO ORDER BY priority DESC, created ASC`)
	if c.Order != "priority DESC, created_at ASC" {
		t.Errorf("order: %q", c.Order)
	}
}

func TestCompile_OrderOnly(t *testing.T) {
	c := compile(t, `ORDER BY created DESC`)
	if c.Where != "" || c.Order != "created_at DESC" {
		t.Errorf("order-only errato: where=%q order=%q", c.Where, c.Order)
	}
}

func TestCompile_UnknownField(t *testing.T) {
	q, _ := Parse(`bogus = 1`)
	if _, err := Compile(q, fakeResolver{}); err == nil {
		t.Error("atteso errore per campo sconosciuto")
	}
}

func TestCompile_UnknownProject(t *testing.T) {
	q, _ := Parse(`project = NOPE`)
	if _, err := Compile(q, fakeResolver{}); err == nil {
		t.Error("atteso errore per progetto inesistente")
	}
}
```

- [ ] **Step 2: Eseguire i test (devono fallire)**

Run: `go test ./internal/jql/ -run TestCompile -v`
Expected: FAIL con "undefined: Compile" / "undefined: Compiled".

- [ ] **Step 3: Implementare il compilatore**

`internal/jql/compile.go`:

```go
package jql

import (
	"fmt"
	"strings"
)

// Resolver traduce i nomi Jira in id interni. Implementato dal dominio
// (search.DBResolver) per tenere questo package disaccoppiato dal DB.
type Resolver interface {
	ProjectID(keyOrID string) (string, bool)
	StatusID(name string) (string, bool)
	TypeID(name string) (string, bool)
	UserID(login string) (string, bool)
	CurrentUserID() string
}

// Compiled è il risultato: clausola WHERE con placeholder ? e relativi
// argomenti, più la clausola ORDER BY (senza "ORDER BY").
type Compiled struct {
	Where string
	Args  []any
	Order string
}

// priorityNames normalizza i nomi di priorità JQL all'enum interno.
var priorityNames = map[string]string{
	"highest": "highest", "high": "high", "medium": "medium",
	"low": "low", "lowest": "lowest",
}

// orderColumns mappa i campi JQL ordinabili alle colonne SQL.
var orderColumns = map[string]string{
	"priority": "priority", "created": "created_at", "updated": "updated_at",
	"summary": "title", "status": "status_id", "key": "seq_id",
	"assignee": "assignee_id", "duedate": "due_date",
}

// Compile trasforma un *Query in una Compiled. Restituisce errore se un campo
// o un valore non è risolvibile.
func Compile(q *Query, r Resolver) (*Compiled, error) {
	out := &Compiled{}
	if q.Where != nil {
		where, args, err := compileNode(q.Where, r)
		if err != nil {
			return nil, err
		}
		out.Where = where
		out.Args = args
	}
	if len(q.Order) > 0 {
		parts := make([]string, 0, len(q.Order))
		for _, k := range q.Order {
			col, ok := orderColumns[k.Field]
			if !ok {
				return nil, fmt.Errorf("campo di ordinamento non supportato: %s", k.Field)
			}
			dir := "ASC"
			if k.Desc {
				dir = "DESC"
			}
			parts = append(parts, col+" "+dir)
		}
		out.Order = strings.Join(parts, ", ")
	}
	return out, nil
}

func compileNode(n Node, r Resolver) (string, []any, error) {
	switch t := n.(type) {
	case And:
		return compileBinary(t.Left, t.Right, "AND", r)
	case Or:
		return compileBinary(t.Left, t.Right, "OR", r)
	case Not:
		w, a, err := compileNode(t.Inner, r)
		if err != nil {
			return "", nil, err
		}
		return "NOT (" + w + ")", a, nil
	case *Clause:
		return compileClause(t, r)
	default:
		return "", nil, fmt.Errorf("nodo sconosciuto %T", n)
	}
}

func compileBinary(l, rt Node, op string, r Resolver) (string, []any, error) {
	lw, la, err := compileNode(l, r)
	if err != nil {
		return "", nil, err
	}
	rw, ra, err := compileNode(rt, r)
	if err != nil {
		return "", nil, err
	}
	return "(" + lw + " " + op + " " + rw + ")", append(la, ra...), nil
}

func compileClause(c *Clause, r Resolver) (string, []any, error) {
	switch c.Field {
	case "project":
		return resolvedEq("project_id", c, r.ProjectID)
	case "status":
		return resolvedEq("status_id", c, r.StatusID)
	case "type", "issuetype":
		return resolvedEq("type_id", c, r.TypeID)
	case "assignee":
		return userClause("assignee_id", c, r)
	case "reporter":
		return userClause("reporter_id", c, r)
	case "priority":
		return priorityClause(c)
	case "resolution":
		// solo IS EMPTY / IS NOT EMPTY (risolte vs non risolte)
		return nullableClause("resolution_id", c)
	case "labels":
		return labelsClause(c)
	case "summary":
		return textClause("title", c)
	case "text":
		return freeTextClause(c)
	case "key":
		return keyClause(c)
	case "created", "updated":
		return dateClause(c)
	default:
		return "", nil, fmt.Errorf("campo non supportato: %s", c.Field)
	}
}

// resolvedEq gestisce =, !=, IN, NOT IN, IS EMPTY per un campo id risolto per nome.
func resolvedEq(col string, c *Clause, resolve func(string) (string, bool)) (string, []any, error) {
	if c.IsEmpty {
		return nullableClause(col, c)
	}
	switch c.Op {
	case "=", "!=":
		id, ok := resolve(c.Value)
		if !ok {
			return "", nil, fmt.Errorf("valore non trovato per %s: %q", col, c.Value)
		}
		sqlOp := "="
		if c.Op == "!=" {
			sqlOp = "!="
		}
		return col + " " + sqlOp + " ?", []any{id}, nil
	case "IN", "NOT IN":
		ids := make([]any, 0, len(c.Values))
		for _, v := range c.Values {
			id, ok := resolve(v)
			if !ok {
				return "", nil, fmt.Errorf("valore non trovato per %s: %q", col, v)
			}
			ids = append(ids, id)
		}
		return inClause(col, c.Op, ids), ids, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per %s", c.Op, col)
	}
}

func userClause(col string, c *Clause, r Resolver) (string, []any, error) {
	if c.IsEmpty {
		return nullableClause(col, c)
	}
	if c.Func == "currentUser" {
		op := "="
		if c.Op == "!=" {
			op = "!="
		}
		return col + " " + op + " ?", []any{r.CurrentUserID()}, nil
	}
	return resolvedEq(col, c, r.UserID)
}

func priorityClause(c *Clause) (string, []any, error) {
	norm := func(v string) (string, bool) {
		n, ok := priorityNames[strings.ToLower(v)]
		return n, ok
	}
	switch c.Op {
	case "=", "!=":
		p, ok := norm(c.Value)
		if !ok {
			return "", nil, fmt.Errorf("priorità sconosciuta: %q", c.Value)
		}
		return "priority " + c.Op + " ?", []any{p}, nil
	case "IN", "NOT IN":
		args := make([]any, 0, len(c.Values))
		for _, v := range c.Values {
			p, ok := norm(v)
			if !ok {
				return "", nil, fmt.Errorf("priorità sconosciuta: %q", v)
			}
			args = append(args, p)
		}
		return inClause("priority", c.Op, args), args, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per priority", c.Op)
	}
}

func nullableClause(col string, c *Clause) (string, []any, error) {
	// IS EMPTY => IS NULL; IS NOT EMPTY => IS NOT NULL
	if c.Op == "IS NOT" || (c.Op == "!=" && c.IsEmpty) {
		return col + " IS NOT NULL", nil, nil
	}
	return col + " IS NULL", nil, nil
}

func labelsClause(c *Clause) (string, []any, error) {
	sub := "EXISTS (SELECT 1 FROM issue_labels il JOIN labels l ON l.id = il.label_id " +
		"WHERE il.issue_id = issues.id AND l.name = ?)"
	switch c.Op {
	case "=":
		return sub, []any{c.Value}, nil
	case "!=":
		return "NOT " + sub, []any{c.Value}, nil
	case "IN":
		clauses := make([]string, 0, len(c.Values))
		args := make([]any, 0, len(c.Values))
		for _, v := range c.Values {
			clauses = append(clauses, sub)
			args = append(args, v)
		}
		return "(" + strings.Join(clauses, " OR ") + ")", args, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per labels", c.Op)
	}
}

func textClause(col string, c *Clause) (string, []any, error) {
	if c.Op != "~" && c.Op != "!~" {
		return "", nil, fmt.Errorf("summary supporta solo ~ e !~")
	}
	neg := ""
	if c.Op == "!~" {
		neg = "NOT "
	}
	return neg + col + " LIKE ?", []any{"%" + c.Value + "%"}, nil
}

func freeTextClause(c *Clause) (string, []any, error) {
	if c.Op != "~" && c.Op != "!~" {
		return "", nil, fmt.Errorf("text supporta solo ~ e !~")
	}
	like := "%" + c.Value + "%"
	base := "(title LIKE ? OR description_json LIKE ?)"
	if c.Op == "!~" {
		return "NOT " + base, []any{like, like}, nil
	}
	return base, []any{like, like}, nil
}

func keyClause(c *Clause) (string, []any, error) {
	switch c.Op {
	case "=", "!=":
		return "key " + c.Op + " ?", []any{c.Value}, nil
	case "IN", "NOT IN":
		args := make([]any, len(c.Values))
		for i, v := range c.Values {
			args[i] = v
		}
		return inClause("key", c.Op, args), args, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per key", c.Op)
	}
}

func dateClause(c *Clause) (string, []any, error) {
	col := "created_at"
	if c.Field == "updated" {
		col = "updated_at"
	}
	switch c.Op {
	case "=", "!=", ">", ">=", "<", "<=":
		return col + " " + c.Op + " ?", []any{c.Value}, nil
	default:
		return "", nil, fmt.Errorf("operatore %q non valido per %s", c.Op, c.Field)
	}
}

func inClause(col, op string, args []any) string {
	ph := strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")
	if op == "NOT IN" {
		return col + " NOT IN (" + ph + ")"
	}
	return col + " IN (" + ph + ")"
}
```

- [ ] **Step 4: Eseguire i test (devono passare)**

Run: `go test ./internal/jql/ -v`
Expected: PASS (lexer + parser + compiler).

- [ ] **Step 5: Commit**

```bash
git add internal/jql/compile.go internal/jql/compile_test.go
git commit -m "feat(jql): compile AST to SQL WHERE and ORDER BY"
```

---

### Task 5: Dominio search — service + resolver

**Files:**
- Modify (riscrivere): `internal/domain/search/service.go`
- Create: `internal/domain/search/resolver.go`
- Delete: `internal/domain/search/parser.go` (sostituito dal package jql)
- Test: `internal/domain/search/service_test.go`

- [ ] **Step 1: Scrivere i test del service**

`internal/domain/search/service_test.go`:

```go
package search

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"gorm.io/gorm"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&issue.Issue{}, &issue.Label{}, &issue.IssueLabel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedIssue(t *testing.T, db *gorm.DB, projectID, title, statusID string, seq int64) *issue.Issue {
	t.Helper()
	iss := &issue.Issue{ID: uuid.NewString(), ProjectID: projectID, Key: "DEMO-" + title[:1], Title: title, StatusID: &statusID, SeqID: seq}
	if err := db.Create(iss).Error; err != nil {
		t.Fatalf("create issue: %v", err)
	}
	return iss
}

func TestSearch_ByProject(t *testing.T) {
	db := newDB(t)
	seedIssue(t, db, "proj-1", "Alpha", "st-todo", 1)
	seedIssue(t, db, "proj-2", "Beta", "st-todo", 2)

	svc := NewService(db)
	r := &staticResolver{project: map[string]string{"DEMO": "proj-1"}}
	res, err := svc.Search(`project = DEMO`, r, 0, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 1 || len(res.Issues) != 1 || res.Issues[0].Title != "Alpha" {
		t.Errorf("risultato errato: total=%d issues=%d", res.Total, len(res.Issues))
	}
}

func TestSearch_EmptyJQLReturnsAll(t *testing.T) {
	db := newDB(t)
	seedIssue(t, db, "proj-1", "Alpha", "st-todo", 1)
	seedIssue(t, db, "proj-1", "Beta", "st-todo", 2)
	svc := NewService(db)
	res, err := svc.Search(``, &staticResolver{}, 0, 50)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("attese 2 issue, %d", res.Total)
	}
}

func TestSearch_Pagination(t *testing.T) {
	db := newDB(t)
	for i := int64(1); i <= 5; i++ {
		seedIssue(t, db, "proj-1", string(rune('A'+i)), "st-todo", i)
	}
	svc := NewService(db)
	res, err := svc.Search(``, &staticResolver{}, 2, 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 5 {
		t.Errorf("total atteso 5, %d", res.Total)
	}
	if len(res.Issues) != 2 {
		t.Errorf("page size atteso 2, %d", len(res.Issues))
	}
}

func TestSearch_InvalidJQL(t *testing.T) {
	svc := NewService(newDB(t))
	if _, err := svc.Search(`project =`, &staticResolver{}, 0, 50); err == nil {
		t.Error("attesa err per JQL invalida")
	}
}

// staticResolver implementa jql.Resolver per i test.
type staticResolver struct {
	project map[string]string
}

func (s *staticResolver) ProjectID(k string) (string, bool) { id, ok := s.project[k]; return id, ok }
func (s *staticResolver) StatusID(string) (string, bool)    { return "", false }
func (s *staticResolver) TypeID(string) (string, bool)      { return "", false }
func (s *staticResolver) UserID(string) (string, bool)      { return "", false }
func (s *staticResolver) CurrentUserID() string             { return "user-me" }
```

- [ ] **Step 2: Eseguire i test (devono fallire)**

Run: `go test ./internal/domain/search/ -run TestSearch -v`
Expected: FAIL (firma `Search` cambiata / `SearchResult` non definito).

- [ ] **Step 3: Riscrivere il service**

Sostituire il contenuto di `internal/domain/search/service.go`:

```go
package search

import (
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/jql"
	"gorm.io/gorm"
)

// Service esegue ricerche JQL sulle issue.
type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// SearchResult porta la pagina di issue e il totale complessivo (per la
// paginazione offset del legacy /search).
type SearchResult struct {
	Issues []issue.Issue
	Total  int
}

// Search compila la JQL e la esegue. jql vuota => tutte le issue non archiviate.
// offset/limit implementano la paginazione. Escludiamo sempre le archiviate.
func (s *Service) Search(jqlStr string, r jql.Resolver, offset, limit int) (SearchResult, error) {
	q, err := jql.Parse(jqlStr)
	if err != nil {
		return SearchResult{}, err
	}
	compiled, err := jql.Compile(q, r)
	if err != nil {
		return SearchResult{}, err
	}

	base := s.db.Model(&issue.Issue{}).Where("is_archived = ?", false)
	if compiled.Where != "" {
		base = base.Where(compiled.Where, compiled.Args...)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return SearchResult{}, err
	}

	q2 := base
	if compiled.Order != "" {
		q2 = q2.Order(compiled.Order)
	} else {
		q2 = q2.Order("seq_id ASC")
	}
	if limit > 0 {
		q2 = q2.Limit(limit)
	}
	if offset > 0 {
		q2 = q2.Offset(offset)
	}

	var issues []issue.Issue
	if err := q2.Find(&issues).Error; err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Issues: issues, Total: int(total)}, nil
}
```

- [ ] **Step 4: Creare il resolver di dominio**

`internal/domain/search/resolver.go`:

```go
package search

import (
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/project"
	"github.com/open-jira/open-jira/internal/domain/user"
	"gorm.io/gorm"
)

// DBResolver implementa jql.Resolver risolvendo i nomi Jira su DB.
type DBResolver struct {
	db            *gorm.DB
	currentUserID string
}

func NewDBResolver(db *gorm.DB, currentUserID string) *DBResolver {
	return &DBResolver{db: db, currentUserID: currentUserID}
}

func (r *DBResolver) ProjectID(keyOrID string) (string, bool) {
	var p project.Project
	if err := r.db.Where("key = ? OR id = ?", keyOrID, keyOrID).First(&p).Error; err != nil {
		return "", false
	}
	return p.ID, true
}

func (r *DBResolver) StatusID(name string) (string, bool) {
	var st issue.Status
	if err := r.db.Where("name = ?", name).First(&st).Error; err != nil {
		return "", false
	}
	return st.ID, true
}

func (r *DBResolver) TypeID(name string) (string, bool) {
	var it issue.IssueType
	if err := r.db.Where("name = ?", name).First(&it).Error; err != nil {
		return "", false
	}
	return it.ID, true
}

func (r *DBResolver) UserID(login string) (string, bool) {
	var u user.User
	if err := r.db.Where("username = ? OR email = ? OR id = ?", login, login, login).First(&u).Error; err != nil {
		return "", false
	}
	return u.ID, true
}

func (r *DBResolver) CurrentUserID() string { return r.currentUserID }
```

> **Nota implementatore:** verificare i nomi dei tipi/tabelle prima di scrivere: `issue.Status` (tabella `statuses`, campi `ID`/`Name`), `issue.IssueType` (`ID`/`Name`), `project.Project` (`ID`/`Key`), `user.User` (`ID`/`Username`/`Email`). Se un tipo `Status` non esiste nel package `issue`, cercarlo con `grep -rn "type Status" internal/domain/` e adeguare l'import/campo. Adeguare i nomi colonna reali (es. `username` vs `user_name`).

- [ ] **Step 5: Eliminare il vecchio parser**

Run: `git rm internal/domain/search/parser.go`
Poi cercare riferimenti residui: `grep -rn "search.Parse\|search.Query\|\.Apply(" internal/ cmd/` ed eliminarli/adeguarli (il vecchio `Service.Search(query string)` e `search_handler.go` verranno riscritti in T7-T9; se il build rompe qui, è atteso e sarà verde dopo T9 — ma il package `internal/domain/search` e `internal/jql` devono compilare da soli).

- [ ] **Step 6: Eseguire i test del package (devono passare)**

Run: `go test ./internal/domain/search/ ./internal/jql/ -v`
Expected: PASS. (Il resto del progetto può non compilare finché non si riscrive l'handler in T7-T9.)

- [ ] **Step 7: Commit**

```bash
git add internal/domain/search/service.go internal/domain/search/resolver.go internal/domain/search/service_test.go
git rm internal/domain/search/parser.go 2>/dev/null; git add -A internal/domain/search/
git commit -m "feat(search): rewrite domain service on real JQL engine + DB resolver"
```

---

### Task 6: v3 — proiezione dei fields

**Files:**
- Create: `internal/api/v3/fields.go`
- Test: `internal/api/v3/fields_test.go`

- [ ] **Step 1: Scrivere i test**

`internal/api/v3/fields_test.go`:

```go
package v3

import "testing"

func TestProjectFields_All(t *testing.T) {
	bean := IssueBean{ID: "1", Key: "DEMO-1", Self: "http://x/1", Fields: IssueFields{Summary: "Hello"}}
	f := Fields{} // vuoto => default *all
	m, err := ProjectIssue(bean, f)
	if err != nil {
		t.Fatalf("ProjectIssue: %v", err)
	}
	fields, _ := m["fields"].(map[string]any)
	if fields["summary"] != "Hello" {
		t.Errorf("summary mancante in *all: %v", m)
	}
	if m["key"] != "DEMO-1" {
		t.Errorf("key mancante")
	}
}

func TestProjectFields_Subset(t *testing.T) {
	bean := IssueBean{ID: "1", Key: "DEMO-1", Self: "http://x/1", Fields: IssueFields{Summary: "Hello"}}
	f := ParseFieldsFromList([]string{"summary"})
	m, err := ProjectIssue(bean, f)
	if err != nil {
		t.Fatalf("ProjectIssue: %v", err)
	}
	fields := m["fields"].(map[string]any)
	if _, ok := fields["summary"]; !ok {
		t.Error("summary deve esserci")
	}
	if _, ok := fields["status"]; ok {
		t.Error("status NON deve esserci quando si chiede solo summary")
	}
	// id/key/self restano sempre a top-level
	if m["id"] == nil || m["key"] == nil {
		t.Error("id/key devono restare sempre presenti")
	}
}
```

- [ ] **Step 2: Eseguire i test (devono fallire)**

Run: `go test ./internal/api/v3/ -run TestProjectFields -v`
Expected: FAIL con "undefined: ProjectIssue" / "undefined: ParseFieldsFromList".

- [ ] **Step 3: Implementare la proiezione**

`internal/api/v3/fields.go`:

```go
package v3

import "encoding/json"

// ProjectIssue serializza un IssueBean in map[string]any applicando la
// selezione dei fields. id/key/self restano sempre a top-level; la selezione
// agisce solo sul sotto-oggetto "fields". Fields vuoto o con "*all" => tutti.
func ProjectIssue(bean IssueBean, f Fields) (map[string]any, error) {
	raw, err := json.Marshal(bean)
	if err != nil {
		return nil, err
	}
	var full map[string]any
	if err := json.Unmarshal(raw, &full); err != nil {
		return nil, err
	}
	if f.includeAll() {
		return full, nil
	}
	fieldsMap, _ := full["fields"].(map[string]any)
	if fieldsMap == nil {
		return full, nil
	}
	pruned := make(map[string]any, len(fieldsMap))
	for k, v := range fieldsMap {
		if f.Include(k) {
			pruned[k] = v
		}
	}
	full["fields"] = pruned
	return full, nil
}

// ParseFieldsFromList costruisce un Fields da una lista esplicita (per i body
// POST /search che passano fields come []string).
func ParseFieldsFromList(list []string) Fields {
	return newFields(list)
}
```

> **Nota implementatore:** questa task dipende dai dettagli interni del tipo `Fields` in `internal/api/v3/params.go`. Prima di scrivere: leggere `params.go` e verificare/aggiungere i seguenti helper NON esportati sul tipo `Fields`: `includeAll() bool` (true se vuoto o contiene `*all`/`*navigable`), e un costruttore `newFields(list []string) Fields`. Se `ParseFields` già costruisce internamente da una lista, estrarre quel costruttore in `newFields`. `Include(name string) bool` esiste già. Adeguare i nomi se il tipo interno differisce (es. campo `set map[string]bool`).

- [ ] **Step 4: Eseguire i test (devono passare)**

Run: `go test ./internal/api/v3/ -run TestProjectFields -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/v3/fields.go internal/api/v3/fields_test.go internal/api/v3/params.go
git commit -m "feat(v3): issue field projection for search responses"
```

---

### Task 7: v3 — mapper risposte search + cursore

**Files:**
- Create: `internal/api/v3/search.go`
- Test: `internal/api/v3/search_test.go`

- [ ] **Step 1: Scrivere i test**

`internal/api/v3/search_test.go`:

```go
package v3

import "testing"

func TestCursor_RoundTrip(t *testing.T) {
	tok := EncodeCursor(40)
	off, err := DecodeCursor(tok)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if off != 40 {
		t.Errorf("offset roundtrip: got %d", off)
	}
}

func TestCursor_EmptyIsZero(t *testing.T) {
	off, err := DecodeCursor("")
	if err != nil || off != 0 {
		t.Errorf("token vuoto deve dare offset 0, got %d err %v", off, err)
	}
}

func TestCursor_Invalid(t *testing.T) {
	if _, err := DecodeCursor("!!!not-base64!!!"); err == nil {
		t.Error("atteso errore per token non valido")
	}
}
```

- [ ] **Step 2: Eseguire i test (devono fallire)**

Run: `go test ./internal/api/v3/ -run TestCursor -v`
Expected: FAIL con "undefined: EncodeCursor".

- [ ] **Step 3: Implementare i mapper**

`internal/api/v3/search.go`:

```go
package v3

import (
	"encoding/base64"
	"strconv"
)

// SearchResults è la risposta del legacy POST/GET /rest/api/3/search (offset).
type SearchResults struct {
	Issues          []map[string]any `json:"issues"`
	StartAt         int              `json:"startAt"`
	MaxResults      int              `json:"maxResults"`
	Total           int              `json:"total"`
	WarningMessages []string         `json:"warningMessages,omitempty"`
}

// SearchAndReconcileResults è la risposta di GET/POST /rest/api/3/search/jql
// (token-paginato: nessun total/startAt).
type SearchAndReconcileResults struct {
	Issues        []map[string]any `json:"issues"`
	NextPageToken string           `json:"nextPageToken,omitempty"`
	IsLast        bool             `json:"isLast"`
}

// EncodeCursor codifica un offset in un token opaco base64url.
func EncodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("o:" + strconv.Itoa(offset)))
}

// DecodeCursor decodifica un token prodotto da EncodeCursor. Token vuoto => 0.
func DecodeCursor(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	s := string(b)
	if len(s) < 3 || s[:2] != "o:" {
		return 0, strconv.ErrSyntax
	}
	return strconv.Atoi(s[2:])
}
```

- [ ] **Step 4: Eseguire i test (devono passare)**

Run: `go test ./internal/api/v3/ -run TestCursor -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/v3/search.go internal/api/v3/search_test.go
git commit -m "feat(v3): search response mappers and opaque cursor"
```

---

### Task 8: Handler search — /search/jql, /search, approximate-count

**Files:**
- Modify (riscrivere): `internal/api/handlers/search_handler.go`
- Test: (coperto dai contract test in T12)

- [ ] **Step 1: Riscrivere l'handler search**

Sostituire il contenuto di `internal/api/handlers/search_handler.go` (mantenendo il package `handlers`):

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/open-jira/open-jira/internal/api/middleware"
	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/search"
)

type SearchHandler struct {
	svc      *search.Service
	issueH   *IssueHandler // riusa buildIssueInput per costruire gli IssueBean
	baseURL  string
}

func NewSearchHandler(svc *search.Service, issueH *IssueHandler, baseURL string) *SearchHandler {
	return &SearchHandler{svc: svc, issueH: issueH, baseURL: baseURL}
}

// jqlParams estrae jql/fields dal GET (query string) o POST (body).
type jqlBody struct {
	JQL           string   `json:"jql"`
	MaxResults    int      `json:"maxResults"`
	NextPageToken string   `json:"nextPageToken"`
	StartAt       int      `json:"startAt"`
	Fields        []string `json:"fields"`
}

func (h *SearchHandler) readParams(r *http.Request) (jqlBody, error) {
	var b jqlBody
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			return b, err
		}
		return b, nil
	}
	q := r.URL.Query()
	b.JQL = q.Get("jql")
	b.NextPageToken = q.Get("nextPageToken")
	if v := q.Get("fields"); v != "" {
		b.Fields = splitCSV(v)
	}
	if v := q.Get("maxResults"); v != "" {
		if n, err := parseIntSafe(v); err == nil {
			b.MaxResults = n
		}
	}
	if v := q.Get("startAt"); v != "" {
		if n, err := parseIntSafe(v); err == nil {
			b.StartAt = n
		}
	}
	return b, nil
}

// renderIssues costruisce gli IssueBean con proiezione dei fields.
func (h *SearchHandler) renderIssues(issues []issueRow, fields []string) ([]map[string]any, error) {
	f := v3.ParseFieldsFromList(fields)
	out := make([]map[string]any, 0, len(issues))
	for i := range issues {
		bean := v3.JiraIssue(h.issueH.buildIssueInput(&issues[i]))
		m, err := v3.ProjectIssue(bean, f)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// SearchJQL gestisce GET/POST /rest/api/3/search/jql (token-paginato).
func (h *SearchHandler) SearchJQL(w http.ResponseWriter, r *http.Request) {
	b, err := h.readParams(r)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	offset, err := v3.DecodeCursor(b.NextPageToken)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid nextPageToken"}, nil)
		return
	}
	limit := clampMax(b.MaxResults, 50, 100)
	uid := middleware.UserIDFromContext(r.Context())
	res, err := h.svc.Search(b.JQL, search.NewDBResolver(h.svc.DB(), uid), offset, limit)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid JQL: " + err.Error()}, nil)
		return
	}
	items, err := h.renderIssues(res.Issues, b.Fields)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	isLast := offset+len(res.Issues) >= res.Total
	out := v3.SearchAndReconcileResults{Issues: items, IsLast: isLast}
	if !isLast {
		out.NextPageToken = v3.EncodeCursor(offset + limit)
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// SearchLegacy gestisce GET/POST /rest/api/3/search (offset-paginato).
func (h *SearchHandler) SearchLegacy(w http.ResponseWriter, r *http.Request) {
	b, err := h.readParams(r)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	limit := clampMax(b.MaxResults, 50, 100)
	uid := middleware.UserIDFromContext(r.Context())
	res, err := h.svc.Search(b.JQL, search.NewDBResolver(h.svc.DB(), uid), b.StartAt, limit)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid JQL: " + err.Error()}, nil)
		return
	}
	items, err := h.renderIssues(res.Issues, b.Fields)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"render error"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, v3.SearchResults{
		Issues: items, StartAt: b.StartAt, MaxResults: limit, Total: res.Total,
	})
}

// ApproximateCount gestisce POST /rest/api/3/search/approximate-count.
func (h *SearchHandler) ApproximateCount(w http.ResponseWriter, r *http.Request) {
	var b jqlBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	res, err := h.svc.Search(b.JQL, search.NewDBResolver(h.svc.DB(), uid), 0, 0)
	if err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid JQL: " + err.Error()}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, map[string]any{"count": res.Total})
}
```

> **Note implementatore (dipendenze da verificare/aggiungere):**
> - `IssueHandler.buildIssueInput` accetta `*issue.Issue`; il tipo `issueRow` sopra è un segnaposto: usare `issue.Issue` reale (aggiungere l'import `"github.com/open-jira/open-jira/internal/domain/issue"` e cambiare la firma di `renderIssues([]issue.Issue, ...)`). `search.SearchResult.Issues` è `[]issue.Issue`.
> - Aggiungere a `search.Service` un metodo `DB() *gorm.DB` (getter del `db`) se non esiste, per costruire il resolver dall'handler.
> - `middleware.UserIDFromContext(r.Context()) string` esiste già (usato in comment_handler.go).
> - Helper locali `splitCSV`, `parseIntSafe`, `clampMax(v, def, cap)` — se non esistono già in package handlers, aggiungerli in cima al file (implementazioni banali: `strings.Split` filtrando vuoti; `strconv.Atoi`; `if v<=0 {return def}; if v>cap {return cap}; return v`). Controllare prima con `grep -rn "func splitCSV\|func clampMax" internal/api/handlers/`.
> - `buildIssueInput` fa lookup per-issue: per pagine da 50 va bene; ottimizzazioni batch sono un follow-up.

- [ ] **Step 2: Verificare che il package compili**

Run: `go build ./internal/api/handlers/`
Expected: build OK (dopo aver adeguato i tipi come da note). Se restano riferimenti al vecchio codice filtri in questo file, spostarli/eliminarli — i filtri vanno nel nuovo `filter_handler.go` (T10).

- [ ] **Step 3: Commit**

```bash
git add internal/api/handlers/search_handler.go internal/domain/search/service.go
git commit -m "feat(api): conformant /search/jql, /search, approximate-count handlers"
```

---

### Task 9: Handler autocomplete — /jql/autocompletedata

**Files:**
- Modify: `internal/api/v3/search.go` (aggiungere `AutocompleteData()`)
- Modify: `internal/api/handlers/search_handler.go` (aggiungere `Autocomplete`)
- Test: `internal/api/v3/autocomplete_test.go`

- [ ] **Step 1: Scrivere il test dei dati di autocomplete**

`internal/api/v3/autocomplete_test.go`:

```go
package v3

import "testing"

func TestAutocompleteData_HasCoreFields(t *testing.T) {
	d := AutocompleteData()
	names := map[string]bool{}
	for _, f := range d.VisibleFieldNames {
		names[f.Value] = true
	}
	for _, want := range []string{"project", "status", "assignee", "priority", "summary", "labels"} {
		if !names[want] {
			t.Errorf("campo %q mancante in autocomplete", want)
		}
	}
	// orderable/searchable devono essere stringhe "true"/"false" come da contratto
	for _, f := range d.VisibleFieldNames {
		if f.Orderable != "true" && f.Orderable != "false" {
			t.Errorf("orderable non stringa-booleana per %s: %q", f.Value, f.Orderable)
		}
	}
	found := false
	for _, fn := range d.VisibleFunctionNames {
		if fn.Value == "currentUser()" {
			found = true
		}
	}
	if !found {
		t.Error("funzione currentUser() mancante")
	}
}
```

- [ ] **Step 2: Eseguire il test (deve fallire)**

Run: `go test ./internal/api/v3/ -run TestAutocompleteData -v`
Expected: FAIL con "undefined: AutocompleteData".

- [ ] **Step 3: Implementare i dati di autocomplete**

Aggiungere in fondo a `internal/api/v3/search.go`:

```go
// FieldReferenceData e FunctionReferenceData: shape del contratto JQLReferenceData.
// Nota: orderable/searchable/isList sono STRINGHE "true"/"false" nello schema Jira.
type FieldReferenceData struct {
	Value       string   `json:"value"`
	DisplayName string   `json:"displayName"`
	Orderable   string   `json:"orderable"`
	Searchable  string   `json:"searchable"`
	Operators   []string `json:"operators"`
	Types       []string `json:"types"`
}

type FunctionReferenceData struct {
	Value       string   `json:"value"`
	DisplayName string   `json:"displayName"`
	IsList      string   `json:"isList"`
	Types       []string `json:"types"`
}

type JQLReferenceData struct {
	VisibleFieldNames    []FieldReferenceData    `json:"visibleFieldNames"`
	VisibleFunctionNames []FunctionReferenceData `json:"visibleFunctionNames"`
	JQLReservedWords     []string                `json:"jqlReservedWords"`
}

// AutocompleteData descrive i campi/funzioni JQL supportati dal nostro engine.
func AutocompleteData() JQLReferenceData {
	field := func(v, dn string, orderable bool, ops []string) FieldReferenceData {
		o := "false"
		if orderable {
			o = "true"
		}
		return FieldReferenceData{Value: v, DisplayName: dn, Orderable: o, Searchable: "true", Operators: ops, Types: []string{"java.lang.String"}}
	}
	eq := []string{"=", "!=", "in", "not in"}
	txt := []string{"~", "!~"}
	return JQLReferenceData{
		VisibleFieldNames: []FieldReferenceData{
			field("project", "Project", false, eq),
			field("status", "Status", true, eq),
			field("assignee", "Assignee", true, append([]string{"is", "is not"}, eq...)),
			field("reporter", "Reporter", false, eq),
			field("priority", "Priority", true, eq),
			field("type", "Type", false, eq),
			field("labels", "Labels", false, eq),
			field("summary", "Summary", true, txt),
			field("text", "Text", false, txt),
			field("resolution", "Resolution", false, []string{"is", "is not"}),
			field("created", "Created", true, []string{"=", "!=", ">", ">=", "<", "<="}),
			field("updated", "Updated", true, []string{"=", "!=", ">", ">=", "<", "<="}),
			field("key", "Key", true, eq),
		},
		VisibleFunctionNames: []FunctionReferenceData{
			{Value: "currentUser()", DisplayName: "currentUser()", IsList: "false", Types: []string{"com.atlassian.jira.user.ApplicationUser"}},
		},
		JQLReservedWords: []string{"and", "or", "not", "in", "is", "empty", "null", "order", "by", "asc", "desc"},
	}
}
```

- [ ] **Step 4: Aggiungere l'handler**

Aggiungere in `internal/api/handlers/search_handler.go`:

```go
// Autocomplete gestisce GET /rest/api/3/jql/autocompletedata.
func (h *SearchHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	v3.WriteJSON(w, http.StatusOK, v3.AutocompleteData())
}
```

- [ ] **Step 5: Eseguire i test (devono passare)**

Run: `go test ./internal/api/v3/ -run TestAutocompleteData -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/v3/search.go internal/api/v3/autocomplete_test.go internal/api/handlers/search_handler.go
git commit -m "feat(api): /jql/autocompletedata reference data"
```

---

### Task 10: Filtri salvati conformi — service + mapper + handler

**Files:**
- Modify: `internal/domain/search/saved_filter.go` (aggiungere `Description`, `SetFavourite`, `Search` paginato)
- Create: `internal/api/v3/filter.go` (mapper `Filter`/`FilterDetails`/`PageBeanFilterDetails`)
- Create: `internal/api/handlers/filter_handler.go`
- Test: `internal/api/v3/filter_test.go`

- [ ] **Step 1: Estendere il modello e il service filtri**

In `internal/domain/search/saved_filter.go`:
- aggiungere al modello `SavedFilter` il campo:
```go
	Description string `gorm:"type:text;default:''" json:"description"`
```
(posizionarlo dopo `Name`).
- aggiungere il parametro `description` a `Create` e `Update`:
```go
func (s *FilterService) Create(ownerID string, projectID *string, name, description, jql string, isShared bool) (*SavedFilter, error) {
	f := &SavedFilter{
		ID:          uuid.New().String(),
		OwnerID:     ownerID,
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		JQL:         jql,
		IsShared:    isShared,
	}
	if err := s.db.Create(f).Error; err != nil {
		return nil, err
	}
	return f, nil
}
```
- aggiungere un metodo `Search` paginato:
```go
// Search restituisce i filtri visibili all'utente (propri o condivisi) con
// paginazione offset, più il totale.
func (s *FilterService) Search(userID string, offset, limit int) ([]SavedFilter, int, error) {
	q := s.db.Model(&SavedFilter{}).Where("owner_id = ? OR is_shared = ?", userID, true)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var filters []SavedFilter
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&filters).Error; err != nil {
		return nil, 0, err
	}
	return filters, int(total), nil
}
```
(In `Update`, aggiungere la gestione di `description` nel map `updates` se non vuota.)

- [ ] **Step 2: Scrivere i test del mapper Filter**

`internal/api/v3/filter_test.go`:

```go
package v3

import "testing"

func TestJiraFilter_Shape(t *testing.T) {
	owner := &JiraUserRef{AccountID: "u1", DisplayName: "Ada"}
	f := JiraFilter(FilterInput{
		ID: "f1", Name: "My open", Description: "desc", JQL: "project = DEMO",
		Favourite: true, Owner: owner, BaseURL: "http://x",
	})
	if f.ID != "f1" || f.Name != "My open" || f.JQL != "project = DEMO" {
		t.Errorf("campi base errati: %+v", f)
	}
	if !f.Favourite {
		t.Error("favourite deve essere true")
	}
	if f.Self == "" || f.SearchURL == "" || f.ViewURL == "" {
		t.Error("self/searchUrl/viewUrl devono essere valorizzati")
	}
	if f.SharePermissions == nil || f.EditPermissions == nil {
		t.Error("sharePermissions/editPermissions devono essere array non-nil (anche vuoti)")
	}
}
```

- [ ] **Step 3: Eseguire i test (devono fallire)**

Run: `go test ./internal/api/v3/ -run TestJiraFilter -v`
Expected: FAIL con "undefined: JiraFilter".

- [ ] **Step 4: Implementare il mapper**

`internal/api/v3/filter.go`:

```go
package v3

import "fmt"

// Filter è lo schema del contratto Jira per un filtro salvato.
type Filter struct {
	Self             string        `json:"self"`
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Description      string        `json:"description,omitempty"`
	Owner            *JiraUserRef  `json:"owner,omitempty"`
	JQL              string        `json:"jql,omitempty"`
	ViewURL          string        `json:"viewUrl,omitempty"`
	SearchURL        string        `json:"searchUrl,omitempty"`
	Favourite        bool          `json:"favourite"`
	FavouritedCount  int64         `json:"favouritedCount"`
	SharePermissions []any         `json:"sharePermissions"`
	EditPermissions  []any         `json:"editPermissions"`
}

// PageBeanFilterDetails è la risposta paginata di /filter/search.
type PageBeanFilterDetails struct {
	Self       string   `json:"self,omitempty"`
	MaxResults int      `json:"maxResults"`
	StartAt    int64    `json:"startAt"`
	Total      int64    `json:"total"`
	IsLast     bool     `json:"isLast"`
	Values     []Filter `json:"values"`
}

// FilterInput porta i dati necessari a costruire un Filter di risposta.
type FilterInput struct {
	ID          string
	Name        string
	Description string
	JQL         string
	Favourite   bool
	Owner       *JiraUserRef
	BaseURL     string
}

// JiraFilter costruisce il Filter di risposta conforme.
func JiraFilter(in FilterInput) Filter {
	return Filter{
		Self:             fmt.Sprintf("%s/rest/api/3/filter/%s", in.BaseURL, in.ID),
		ID:               in.ID,
		Name:             in.Name,
		Description:      in.Description,
		Owner:            in.Owner,
		JQL:              in.JQL,
		ViewURL:          fmt.Sprintf("%s/issues/?filter=%s", in.BaseURL, in.ID),
		SearchURL:        fmt.Sprintf("%s/rest/api/3/search?jql=%s", in.BaseURL, in.ID),
		Favourite:        in.Favourite,
		SharePermissions: []any{},
		EditPermissions:  []any{},
	}
}
```

> **Nota implementatore:** verificare il nome esatto del tipo utente v3 già esistente. La summary del progetto lo chiama `JiraUser`; nel mapper commenti è usato come `v3.JiraUser`/`JiraUserRef`. Cercare con `grep -rn "type JiraUser" internal/api/v3/` e usare quel tipo per il campo `Owner` (adeguare `JiraUserRef` → nome reale). Costruire l'owner dall'utente proprietario del filtro come si fa in comment_handler (helper `author`/`JiraUser`).

- [ ] **Step 5: Creare l'handler filtri**

`internal/api/handlers/filter_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/open-jira/open-jira/internal/api/middleware"
	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/search"
	"github.com/open-jira/open-jira/internal/domain/user"
	"gorm.io/gorm"
)

type FilterHandler struct {
	svc     *search.FilterService
	db      *gorm.DB
	baseURL string
}

func NewFilterHandler(svc *search.FilterService, db *gorm.DB, baseURL string) *FilterHandler {
	return &FilterHandler{svc: svc, db: db, baseURL: baseURL}
}

// ownerRef costruisce il JiraUser del proprietario (nil se non trovato).
func (h *FilterHandler) ownerRef(ownerID string) *v3.JiraUserRef {
	var u user.User
	if h.db.First(&u, "id = ?", ownerID).Error != nil {
		return nil
	}
	return v3.JiraUser(&u, h.baseURL) // adeguare alla firma reale di v3.JiraUser
}

func (h *FilterHandler) toFilter(f *search.SavedFilter) v3.Filter {
	return v3.JiraFilter(v3.FilterInput{
		ID: f.ID, Name: f.Name, Description: f.Description, JQL: f.JQL,
		Favourite: f.IsFavourite, Owner: h.ownerRef(f.OwnerID), BaseURL: h.baseURL,
	})
}

type filterBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	JQL         string `json:"jql"`
}

func (h *FilterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var b filterBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if b.Name == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"name is required"}, nil)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	f, err := h.svc.Create(uid, nil, b.Name, b.Description, b.JQL, false)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to create filter"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, h.toFilter(f))
}

func (h *FilterHandler) Get(w http.ResponseWriter, r *http.Request) {
	f, err := h.svc.Get(r.PathValue("id"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, h.toFilter(f))
}

func (h *FilterHandler) Update(w http.ResponseWriter, r *http.Request) {
	var b filterBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	f, err := h.svc.Update(r.PathValue("id"), b.Name, b.JQL, nil) // estendere Update per description
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, h.toFilter(f))
}

func (h *FilterHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.PathValue("id")); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete filter"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FilterHandler) Search(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	filters, total, err := h.svc.Search(uid, startAt, maxResults)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to search filters"}, nil)
		return
	}
	values := make([]v3.Filter, 0, len(filters))
	for i := range filters {
		values = append(values, h.toFilter(&filters[i]))
	}
	v3.WriteJSON(w, http.StatusOK, v3.PageBeanFilterDetails{
		MaxResults: maxResults, StartAt: int64(startAt), Total: int64(total),
		IsLast: startAt+len(values) >= total, Values: values,
	})
}

func (h *FilterHandler) My(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	filters, err := h.svc.List(uid) // List esistente: owner o shared
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list filters"}, nil)
		return
	}
	h.writeArray(w, filters)
}

func (h *FilterHandler) Favourite(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	filters, err := h.svc.ListFavourites(uid)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list favourites"}, nil)
		return
	}
	h.writeArray(w, filters)
}

func (h *FilterHandler) AddFavourite(w http.ResponseWriter, r *http.Request) {
	h.setFav(w, r, true)
}
func (h *FilterHandler) RemoveFavourite(w http.ResponseWriter, r *http.Request) {
	h.setFav(w, r, false)
}
func (h *FilterHandler) setFav(w http.ResponseWriter, r *http.Request, fav bool) {
	id := r.PathValue("id")
	if err := h.svc.ToggleFavourite(id, fav); err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return
	}
	f, err := h.svc.Get(id)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, h.toFilter(f))
}

func (h *FilterHandler) writeArray(w http.ResponseWriter, filters []search.SavedFilter) {
	out := make([]v3.Filter, 0, len(filters))
	for i := range filters {
		out = append(out, h.toFilter(&filters[i]))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}
```

> **Nota implementatore:** adeguare `v3.JiraUser(&u, baseURL)` alla firma reale (verificare come comment_handler costruisce il `JiraUser` — potrebbe essere `v3.JiraUser(u, u, baseURL)` o simile). Estendere `FilterService.Update` per accettare/aggiornare anche `description` (aggiungere parametro o un metodo dedicato) — mantenere le firme coerenti con le chiamate qui.

- [ ] **Step 6: Eseguire i test (devono passare)**

Run: `go test ./internal/api/v3/ -run TestJiraFilter -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/search/saved_filter.go internal/api/v3/filter.go internal/api/v3/filter_test.go internal/api/handlers/filter_handler.go
git commit -m "feat(api): conformant saved-filter endpoints and Filter mapper"
```

---

### Task 11: Router — rimuovere rotte non conformi e cablare le nuove

**Files:**
- Modify: `internal/api/router.go:225-242` (blocco search/filter)

- [ ] **Step 1: Sostituire il blocco rotte search/filter**

Individuare in `internal/api/router.go` la costruzione degli handler search/filter (intorno a router.go:57-59) e il blocco rotte (router.go:225-242). Sostituire con:

```go
	// costruzione handler (nella sezione handler, vicino agli altri)
	searchSvc := search.NewService(db)
	searchH := handlers.NewSearchHandler(searchSvc, issueH, cfg.BaseURL)
	filterSvc := search.NewFilterService(db)
	filterH := handlers.NewFilterHandler(filterSvc, db, cfg.BaseURL)
```

```go
	// --- Ricerca (Round 4) ---
	mux.Handle("GET /rest/api/3/search/jql", authMw(http.HandlerFunc(searchH.SearchJQL)))
	mux.Handle("POST /rest/api/3/search/jql", authMw(http.HandlerFunc(searchH.SearchJQL)))
	mux.Handle("GET /rest/api/3/search", authMw(http.HandlerFunc(searchH.SearchLegacy)))
	mux.Handle("POST /rest/api/3/search", authMw(http.HandlerFunc(searchH.SearchLegacy)))
	mux.Handle("POST /rest/api/3/search/approximate-count", authMw(http.HandlerFunc(searchH.ApproximateCount)))
	mux.Handle("GET /rest/api/3/jql/autocompletedata", authMw(http.HandlerFunc(searchH.Autocomplete)))

	// --- Filtri salvati (Round 4) ---
	mux.Handle("POST /rest/api/3/filter", authMw(http.HandlerFunc(filterH.Create)))
	mux.Handle("GET /rest/api/3/filter/search", authMw(http.HandlerFunc(filterH.Search)))
	mux.Handle("GET /rest/api/3/filter/my", authMw(http.HandlerFunc(filterH.My)))
	mux.Handle("GET /rest/api/3/filter/favourite", authMw(http.HandlerFunc(filterH.Favourite)))
	mux.Handle("GET /rest/api/3/filter/{id}", authMw(http.HandlerFunc(filterH.Get)))
	mux.Handle("PUT /rest/api/3/filter/{id}", authMw(http.HandlerFunc(filterH.Update)))
	mux.Handle("DELETE /rest/api/3/filter/{id}", authMw(http.HandlerFunc(filterH.Delete)))
	mux.Handle("PUT /rest/api/3/filter/{id}/favourite", authMw(http.HandlerFunc(filterH.AddFavourite)))
	mux.Handle("DELETE /rest/api/3/filter/{id}/favourite", authMw(http.HandlerFunc(filterH.RemoveFavourite)))
```

**Rimuovere** le vecchie rotte plurali `/rest/api/3/filters` e `/rest/api/3/filters/{id}` e la vecchia registrazione di `GET /search` → handler `?q=`. Rimuovere gli handler orfani non più referenziati in `search_handler.go` (i vecchi filtri) se non già eliminati in T8.

> **Nota:** l'ordine di registrazione con ServeMux Go 1.22+ gestisce la specificità dei pattern automaticamente (`/filter/search` e `/filter/{id}` coesistono: il segmento letterale vince sul wildcard). Verificare comunque che `GET /filter/search` NON venga catturato da `GET /filter/{id}`.

- [ ] **Step 2: Build completa del server**

Run: `go build ./...`
Expected: build OK su tutto il progetto. Risolvere eventuali riferimenti residui al vecchio `search.Service.Search(string)` o handler eliminati.

- [ ] **Step 3: Go vet**

Run: `go vet ./...`
Expected: nessun errore.

- [ ] **Step 4: Commit**

```bash
git add internal/api/router.go internal/api/handlers/search_handler.go
git commit -m "feat(api): wire Round 4 search/filter routes, drop non-conformant routes"
```

---

### Task 12: Contract test — search, filtri, autocomplete

**Files:**
- Create: `internal/contract/search_test.go`

- [ ] **Step 1: Scrivere i contract test**

`internal/contract/search_test.go` (seguire il pattern degli altri file in `internal/contract/`, es. `collab_test.go`, per `newTestServer`/`registerAndLogin`/`createProjectViaAPI`/`createIssueViaAPI` e la validazione OpenAPI):

```go
package contract

import (
	"net/http"
	"testing"
)

func TestSearchJQL_Conformant(t *testing.T) {
	srv := newTestServer(t)
	tok := registerAndLogin(t, srv)
	createProjectViaAPI(t, srv, tok, "SR", "Search Proj")
	createIssueViaAPI(t, srv, tok, "SR", "Trovami")

	// GET /search/jql
	resp := doAuthGET(t, srv, tok, "/rest/api/3/search/jql?jql="+urlq(`project = SR`))
	assertStatus(t, resp, http.StatusOK)
	validateResponse(t, "/rest/api/3/search/jql", http.MethodGet, resp)
	body := decodeJSON(t, resp)
	if _, ok := body["isLast"]; !ok {
		t.Error("risposta /search/jql deve avere isLast")
	}
	issues, _ := body["issues"].([]any)
	if len(issues) != 1 {
		t.Errorf("attesa 1 issue, %d", len(issues))
	}

	// POST /search/jql con fields
	resp2 := doAuthPOST(t, srv, tok, "/rest/api/3/search/jql", map[string]any{
		"jql": "project = SR", "fields": []string{"summary", "status"}, "maxResults": 10,
	})
	assertStatus(t, resp2, http.StatusOK)
	validateResponse(t, "/rest/api/3/search/jql", http.MethodPost, resp2)

	// legacy POST /search offset-paginato
	resp3 := doAuthPOST(t, srv, tok, "/rest/api/3/search", map[string]any{"jql": "project = SR"})
	assertStatus(t, resp3, http.StatusOK)
	validateResponse(t, "/rest/api/3/search", http.MethodPost, resp3)
	b3 := decodeJSON(t, resp3)
	if b3["total"] == nil || b3["startAt"] == nil {
		t.Error("risposta legacy /search deve avere total/startAt")
	}

	// approximate-count
	resp4 := doAuthPOST(t, srv, tok, "/rest/api/3/search/approximate-count", map[string]any{"jql": "project = SR"})
	assertStatus(t, resp4, http.StatusOK)
	validateResponse(t, "/rest/api/3/search/approximate-count", http.MethodPost, resp4)
}

func TestSearchJQL_InvalidReturns400(t *testing.T) {
	srv := newTestServer(t)
	tok := registerAndLogin(t, srv)
	resp := doAuthGET(t, srv, tok, "/rest/api/3/search/jql?jql="+urlq(`project =`))
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestAutocomplete_Conformant(t *testing.T) {
	srv := newTestServer(t)
	tok := registerAndLogin(t, srv)
	resp := doAuthGET(t, srv, tok, "/rest/api/3/jql/autocompletedata")
	assertStatus(t, resp, http.StatusOK)
	validateResponse(t, "/rest/api/3/jql/autocompletedata", http.MethodGet, resp)
}

func TestFilters_CRUDConformant(t *testing.T) {
	srv := newTestServer(t)
	tok := registerAndLogin(t, srv)

	// create
	resp := doAuthPOST(t, srv, tok, "/rest/api/3/filter", map[string]any{
		"name": "My open", "jql": "assignee = currentUser()", "description": "mine",
	})
	assertStatus(t, resp, http.StatusCreated)
	validateResponse(t, "/rest/api/3/filter", http.MethodPost, resp)
	created := decodeJSON(t, resp)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("filtro creato senza id")
	}

	// get
	respG := doAuthGET(t, srv, tok, "/rest/api/3/filter/"+id)
	assertStatus(t, respG, http.StatusOK)
	validateResponse(t, "/rest/api/3/filter/{id}", http.MethodGet, respG)

	// favourite PUT
	respF := doAuthPUT(t, srv, tok, "/rest/api/3/filter/"+id+"/favourite", nil)
	assertStatus(t, respF, http.StatusOK)
	fav := decodeJSON(t, respF)
	if fav["favourite"] != true {
		t.Error("dopo PUT favourite, favourite deve essere true")
	}

	// filter/search paginato
	respS := doAuthGET(t, srv, tok, "/rest/api/3/filter/search")
	assertStatus(t, respS, http.StatusOK)
	validateResponse(t, "/rest/api/3/filter/search", http.MethodGet, respS)

	// filter/my e favourite (array)
	respMy := doAuthGET(t, srv, tok, "/rest/api/3/filter/my")
	assertStatus(t, respMy, http.StatusOK)

	// delete
	respD := doAuthDELETE(t, srv, tok, "/rest/api/3/filter/"+id)
	assertStatus(t, respD, http.StatusNoContent)
}
```

> **Nota implementatore:** i nomi degli helper (`doAuthGET`/`doAuthPOST`/`doAuthPUT`/`doAuthDELETE`/`assertStatus`/`decodeJSON`/`validateResponse`/`urlq`) devono corrispondere a quelli realmente presenti in `internal/contract/`. Leggere `internal/contract/collab_test.go` e il file harness per gli helper esatti e adeguare le chiamate (es. potrebbe essere `mustGET`, o `validateResponse` con firma diversa). Se un helper manca (es. `doAuthPUT`), aggiungerlo all'harness.

- [ ] **Step 2: Eseguire i contract test**

Run: `go test ./internal/contract/ -run 'TestSearch|TestAutocomplete|TestFilters' -v`
Expected: PASS. Se la validazione OpenAPI fallisce su un campo, correggere il mapper (di solito `omitempty` mancante che emette `null`, o un campo assente richiesto dallo schema).

- [ ] **Step 3: Suite backend completa**

Run: `go test ./...`
Expected: PASS su tutto.

- [ ] **Step 4: Commit**

```bash
git add internal/contract/search_test.go
git commit -m "test(contract): search, filters, autocomplete conformance"
```

---

### Task 13: Frontend — client API search + filtri

**Files:**
- Modify: `frontend-next/lib/api.ts`

- [ ] **Step 1: Aggiungere tipi e funzioni client**

In `frontend-next/lib/api.ts`, aggiungere (seguendo lo stile dell'oggetto `projects`/`issues` esistente e il fetch wrapper già presente):

```ts
export interface SearchIssue {
  id: string;
  key: string;
  self: string;
  fields: {
    summary?: string;
    status?: { name: string; statusCategory?: { key: string; colorName: string } };
    priority?: { name: string };
    assignee?: { displayName: string } | null;
    updated?: string;
  };
}

export interface SearchJqlResponse {
  issues: SearchIssue[];
  nextPageToken?: string;
  isLast: boolean;
}

export interface Filter {
  id: string;
  name: string;
  description?: string;
  jql: string;
  favourite: boolean;
  owner?: { displayName: string };
}

export const search = {
  jql: (jql: string, opts?: { fields?: string[]; nextPageToken?: string; maxResults?: number }) =>
    apiFetch<SearchJqlResponse>("/rest/api/3/search/jql", {
      method: "POST",
      body: JSON.stringify({
        jql,
        fields: opts?.fields ?? ["summary", "status", "priority", "assignee", "updated"],
        nextPageToken: opts?.nextPageToken,
        maxResults: opts?.maxResults ?? 50,
      }),
    }),
};

export const filters = {
  list: () => apiFetch<Filter[]>("/rest/api/3/filter/my"),
  favourites: () => apiFetch<Filter[]>("/rest/api/3/filter/favourite"),
  get: (id: string) => apiFetch<Filter>(`/rest/api/3/filter/${id}`),
  create: (name: string, jql: string, description?: string) =>
    apiFetch<Filter>("/rest/api/3/filter", {
      method: "POST",
      body: JSON.stringify({ name, jql, description }),
    }),
  del: (id: string) => apiFetch<void>(`/rest/api/3/filter/${id}`, { method: "DELETE" }),
  setFavourite: (id: string, fav: boolean) =>
    apiFetch<Filter>(`/rest/api/3/filter/${id}/favourite`, { method: fav ? "PUT" : "DELETE" }),
};
```

> **Nota implementatore:** adeguare `apiFetch` al nome reale del wrapper fetch in `lib/api.ts` (verificare come `projects.search` chiama il backend). Rispettare la gestione errori/headers/token già in uso.

- [ ] **Step 2: Type-check**

Run: `cd frontend-next && npx tsc --noEmit`
Expected: nessun errore di tipo.

- [ ] **Step 3: Commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): typed client for search and saved filters"
```

---

### Task 14: Frontend — list view risultati + pagina filtri

**Files:**
- Create: `frontend-next/components/search/SearchResults.tsx`
- Create: `frontend-next/app/jira/filters/page.tsx`

- [ ] **Step 1: Componente list view a colonne configurabili**

`frontend-next/components/search/SearchResults.tsx`:

```tsx
"use client";

import { useState } from "react";
import type { SearchIssue } from "@/lib/api";

const ALL_COLUMNS = [
  { key: "key", label: "Key" },
  { key: "summary", label: "Summary" },
  { key: "status", label: "Status" },
  { key: "priority", label: "Priority" },
  { key: "assignee", label: "Assignee" },
  { key: "updated", label: "Updated" },
] as const;

type ColKey = (typeof ALL_COLUMNS)[number]["key"];

export function SearchResults({ issues }: { issues: SearchIssue[] }) {
  const [cols, setCols] = useState<Set<ColKey>>(
    new Set(ALL_COLUMNS.map((c) => c.key)),
  );

  const toggle = (k: ColKey) =>
    setCols((prev) => {
      const next = new Set(prev);
      next.has(k) ? next.delete(k) : next.add(k);
      return next;
    });

  const cell = (iss: SearchIssue, k: ColKey) => {
    switch (k) {
      case "key":
        return <a href={`/jira/browse/${iss.key}`} className="text-blue-600 hover:underline">{iss.key}</a>;
      case "summary":
        return iss.fields.summary ?? "";
      case "status":
        return iss.fields.status?.name ?? "";
      case "priority":
        return iss.fields.priority?.name ?? "";
      case "assignee":
        return iss.fields.assignee?.displayName ?? "Unassigned";
      case "updated":
        return iss.fields.updated?.slice(0, 10) ?? "";
    }
  };

  return (
    <div>
      <div className="mb-3 flex flex-wrap gap-3 text-sm" aria-label="Columns">
        {ALL_COLUMNS.map((c) => (
          <label key={c.key} className="flex items-center gap-1">
            <input type="checkbox" checked={cols.has(c.key)} onChange={() => toggle(c.key)} />
            {c.label}
          </label>
        ))}
      </div>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b text-left text-gray-500">
            {ALL_COLUMNS.filter((c) => cols.has(c.key)).map((c) => (
              <th key={c.key} className="py-2 pr-4">{c.label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {issues.map((iss) => (
            <tr key={iss.id} className="border-b hover:bg-gray-50">
              {ALL_COLUMNS.filter((c) => cols.has(c.key)).map((c) => (
                <td key={c.key} className="py-2 pr-4">{cell(iss, c.key)}</td>
              ))}
            </tr>
          ))}
          {issues.length === 0 && (
            <tr><td className="py-4 text-gray-500" colSpan={cols.size}>No results</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 2: Pagina filtri con ricerca JQL**

`frontend-next/app/jira/filters/page.tsx`:

```tsx
"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { search, filters, type SearchIssue } from "@/lib/api";
import { SearchResults } from "@/components/search/SearchResults";

export default function FiltersPage() {
  const qc = useQueryClient();
  const [jql, setJql] = useState("");
  const [results, setResults] = useState<SearchIssue[]>([]);
  const [ran, setRan] = useState(false);

  const myFilters = useQuery({ queryKey: ["filters", "my"], queryFn: filters.list });

  const run = useMutation({
    mutationFn: (q: string) => search.jql(q),
    onSuccess: (data) => {
      setResults(data.issues);
      setRan(true);
    },
  });

  const save = useMutation({
    mutationFn: (name: string) => filters.create(name, jql),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["filters", "my"] }),
  });

  return (
    <div className="mx-auto max-w-5xl p-6">
      <h1 className="mb-4 text-xl font-semibold">Filters</h1>

      <div className="mb-4 flex gap-2">
        <input
          aria-label="JQL"
          value={jql}
          onChange={(e) => setJql(e.target.value)}
          placeholder='project = DEMO ORDER BY updated DESC'
          className="flex-1 rounded border px-3 py-2 font-mono text-sm"
        />
        <button
          onClick={() => run.mutate(jql)}
          className="rounded bg-blue-600 px-4 py-2 text-sm text-white"
        >
          Search
        </button>
        <button
          onClick={() => {
            const name = prompt("Filter name");
            if (name) save.mutate(name);
          }}
          className="rounded border px-4 py-2 text-sm"
        >
          Save filter
        </button>
      </div>

      {run.isError && <p className="mb-3 text-sm text-red-600">Invalid JQL</p>}

      <div className="grid grid-cols-[220px_1fr] gap-6">
        <aside>
          <h2 className="mb-2 text-sm font-semibold text-gray-500">Saved filters</h2>
          <ul className="space-y-1 text-sm">
            {myFilters.data?.map((f) => (
              <li key={f.id}>
                <button
                  className="text-left text-blue-600 hover:underline"
                  onClick={() => {
                    setJql(f.jql);
                    run.mutate(f.jql);
                  }}
                >
                  {f.name}
                </button>
              </li>
            ))}
            {myFilters.data?.length === 0 && <li className="text-gray-400">None yet</li>}
          </ul>
        </aside>
        <section>{ran && <SearchResults issues={results} />}</section>
      </div>
    </div>
  );
}
```

> **Nota implementatore:** verificare l'alias di import (`@/lib/api`, `@/components/...`) rispetto a `tsconfig.json`/`components.json` del progetto. Uniformare stile Tailwind alle altre pagine (`app/jira/projects/page.tsx`). La sidebar linka già `/jira/filters` (router: `filters` in read_page) — assicurarsi che la voce punti a questa pagina.

- [ ] **Step 3: Type-check + build**

Run: `cd frontend-next && npx tsc --noEmit && npm run build`
Expected: build OK.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/search/SearchResults.tsx frontend-next/app/jira/filters/page.tsx
git commit -m "feat(frontend): filters page with JQL search and configurable list view"
```

---

### Task 15: Frontend — ricerca globale in top nav

**Files:**
- Create: `frontend-next/components/search/GlobalSearch.tsx`
- Modify: il componente della top nav (cercare con `grep -rn "Search" frontend-next/components/ frontend-next/app/` il campo di ricerca esistente nella barra superiore)

- [ ] **Step 1: Componente ricerca globale**

`frontend-next/components/search/GlobalSearch.tsx`:

```tsx
"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

// GlobalSearch: la barra in top nav. Invio => vai alla pagina filtri con la
// query come JQL testuale (text ~ "...") oppure JQL grezza se contiene un operatore.
export function GlobalSearch() {
  const router = useRouter();
  const [q, setQ] = useState("");

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = q.trim();
    if (!trimmed) return;
    const isJql = /[=~<>]|\b(AND|OR|ORDER BY)\b/i.test(trimmed);
    const jql = isJql ? trimmed : `text ~ "${trimmed.replace(/"/g, "")}"`;
    router.push(`/jira/filters?jql=${encodeURIComponent(jql)}`);
  };

  return (
    <form onSubmit={submit}>
      <input
        aria-label="Search"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="Search"
        className="w-64 rounded border px-3 py-1.5 text-sm"
      />
    </form>
  );
}
```

- [ ] **Step 2: Montare nella top nav + leggere `?jql=` nella pagina filtri**

- Sostituire l'input di ricerca statico nella top nav con `<GlobalSearch />`.
- In `frontend-next/app/jira/filters/page.tsx`, leggere il parametro iniziale `?jql=` con `useSearchParams()` e, se presente, precompilare `jql` ed eseguire `run.mutate` una volta al mount:

```tsx
import { useSearchParams } from "next/navigation";
import { useEffect } from "react";
// ...dentro il componente:
const params = useSearchParams();
useEffect(() => {
  const initial = params.get("jql");
  if (initial) {
    setJql(initial);
    run.mutate(initial);
  }
  // eslint-disable-next-line react-hooks/exhaustive-deps
}, []);
```

> **Nota implementatore:** `useSearchParams` richiede un `<Suspense>` boundary in alcune configurazioni Next.js 16 in fase di build — se `npm run build` segnala l'errore, avvolgere il contenuto della pagina in `<Suspense>`.

- [ ] **Step 3: Build**

Run: `cd frontend-next && npm run build`
Expected: build OK.

- [ ] **Step 4: Commit**

```bash
git add frontend-next/components/search/GlobalSearch.tsx frontend-next/app/jira/filters/page.tsx
git commit -m "feat(frontend): global search wired to JQL filters page"
```

---

### Task 16: E2E — ricerca e filtri

**Files:**
- Create: `frontend-next/e2e/search.spec.ts`

- [ ] **Step 1: Scrivere il test E2E**

`frontend-next/e2e/search.spec.ts` (riusare l'helper `login()` come in `e2e/collaboration.spec.ts`):

```ts
import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByPlaceholder("you@example.com").fill("admin@example.com");
  await page.getByPlaceholder("••••••••").fill("admin-demo-123");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL(/\/jira/);
}

test("JQL search on filters page returns seeded issues", async ({ page }) => {
  await login(page);
  await page.goto("/jira/filters");
  await page.getByLabel("JQL").fill("project = DEMO");
  await page.getByRole("button", { name: "Search" }).click();

  // la list view mostra almeno una issue del progetto DEMO
  await expect(page.getByRole("cell", { name: /DEMO-1/ })).toBeVisible();
});

test("column toggle hides a column", async ({ page }) => {
  await login(page);
  await page.goto("/jira/filters");
  await page.getByLabel("JQL").fill("project = DEMO");
  await page.getByRole("button", { name: "Search" }).click();
  await expect(page.getByRole("columnheader", { name: "Priority" })).toBeVisible();

  // deseleziona la colonna Priority
  await page.getByLabel("Priority", { exact: true }).uncheck();
  await expect(page.getByRole("columnheader", { name: "Priority" })).toHaveCount(0);
});

test("save filter then run it from the sidebar", async ({ page }) => {
  await login(page);
  await page.goto("/jira/filters");
  await page.getByLabel("JQL").fill("project = DEMO ORDER BY created DESC");

  page.once("dialog", (d) => d.accept("E2E filter")); // prompt del nome
  await page.getByRole("button", { name: "Save filter" }).click();

  await expect(page.getByRole("button", { name: "E2E filter" })).toBeVisible();
  await page.getByRole("button", { name: "E2E filter" }).click();
  await expect(page.getByRole("cell", { name: /DEMO-1/ })).toBeVisible();
});
```

> **Nota implementatore:** adeguare i selettori del login a quelli reali (in `collaboration.spec.ts`). Se il campo "Priority" nel toggle colonne collide con altri testi "Priority" in pagina, usare uno scope più stretto (es. dentro il container `aria-label="Columns"`).

- [ ] **Step 2: Eseguire la suite E2E**

Run: `cd frontend-next && npx playwright test e2e/search.spec.ts --reporter=line`
Expected: PASS (il `webServer` di Playwright avvia backend seedato + Next).

- [ ] **Step 3: Suite E2E completa (nessuna regressione)**

Run: `cd frontend-next && npx playwright test --reporter=line`
Expected: tutti i test verdi (collaboration, issues, projects, login, search).

- [ ] **Step 4: Commit**

```bash
git add frontend-next/e2e/search.spec.ts
git commit -m "test(e2e): JQL search, column toggle, save-and-run filter"
```

---

### Task 17: Seed filtro demo + rigenerazione gap report

**Files:**
- Modify: `cmd/seed/main.go`
- Modify: `docs/contracts/gap-report.md` (rigenerato)

- [ ] **Step 1: Seedare un filtro demo idempotente**

In `cmd/seed/main.go`, dopo il seed dei commenti, aggiungere (seguendo lo stile idempotente già usato — controllare esistenza per nome+owner prima di creare):

```go
	// Filtro salvato demo (idempotente)
	filterSvc := search.NewFilterService(s.DB)
	var existingF search.SavedFilter
	err = s.DB.Where("owner_id = ? AND name = ?", admin.ID, "My open issues").First(&existingF).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if _, err := filterSvc.Create(admin.ID, nil, "My open issues", "Issue assegnate a me", "assignee = currentUser() ORDER BY updated DESC", false); err != nil {
			log.Fatalf("seed filter: %v", err)
		}
		fmt.Println("created demo filter")
	} else if err != nil {
		log.Fatalf("check demo filter: %v", err)
	}
```

(Aggiungere l'import `"github.com/open-jira/open-jira/internal/domain/search"` se assente.)

- [ ] **Step 2: Verificare il seed**

Run: `DB_DRIVER=sqlite DB_DSN=/tmp/seed-test.db go run ./cmd/seed && rm -f /tmp/seed-test.db`
Expected: stampa "created demo filter", exit 0. Eseguito due volte NON deve duplicare.

- [ ] **Step 3: Rigenerare il gap report**

Run: `go run ./cmd/gapreport`
Expected: `docs/contracts/gap-report.md` aggiornato; il conteggio di endpoint conformi aumenta (nuovi: `/search/jql` GET+POST, `/search` GET+POST, `/search/approximate-count`, `/jql/autocompletedata`, `/filter` POST, `/filter/{id}` GET/PUT/DELETE, `/filter/search`, `/filter/my`, `/filter/favourite`, `/filter/{id}/favourite` PUT/DELETE).

- [ ] **Step 4: Commit**

```bash
git add cmd/seed/main.go docs/contracts/gap-report.md
git commit -m "feat(seed): demo saved filter; regenerate gap report for Round 4"
```

---

### Task 18: Gate finale + STATE.md → Round 5

**Files:**
- Modify: `docs/superpowers/STATE.md`

- [ ] **Step 1: Gate a tre livelli**

Run (uno per riga, tutti devono essere verdi):
```bash
go build ./... && echo BUILD_OK
go vet ./... && echo VET_OK
go test ./... 2>&1 | tail -20
cd frontend-next && npx tsc --noEmit && npm run build && npx playwright test --reporter=line; cd ..
```
Expected: `BUILD_OK`, `VET_OK`, `go test` senza FAIL, frontend build OK, tutti gli E2E verdi.

- [ ] **Step 2: Confronto gap report**

Run: `go run ./cmd/gapreport && git diff --stat docs/contracts/gap-report.md`
Expected: nessun drift inatteso; il numero di endpoint conformi riflette i nuovi endpoint del Round 4.

- [ ] **Step 3: Aggiornare STATE.md**

In `docs/superpowers/STATE.md`:
- aggiungere alla sezione "Round completati" la riga del **Round 4 — Ricerca & JQL** (endpoint coperti, package `internal/jql`, filtri conformi, UI ricerca globale + pagina filtri);
- cambiare la sezione "Prossimo" in **Round 5 — Board, Backlog, Sprint** (Agile API 1.0, ranking LexoRank), citando `docs/contracts/jira-agile-1.0.json`;
- aggiornare il numero di endpoint conformi del gap report;
- aggiornare la data e la riga "Aggiornato".

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/STATE.md docs/contracts/gap-report.md
git commit -m "docs: mark Round 4 (Search & JQL) complete, Round 5 (Boards) next"
```

---

## Note di chiusura round

- **Follow-up creati/da tracciare:** `POST /jql/parse` (validazione JQL client-side); `sharePermissions`/`editPermissions` reali sui filtri (ora array vuoti); colonne list-view persistite lato server; ottimizzazione batch dei lookup in `renderIssues` (evitare N query per pagina); supporto funzioni JQL aggiuntive (`now()`, `startOfDay()`).
- **Rischi noti:** la validazione date in `dateClause` passa la stringa grezza a SQLite/Postgres — accettare solo `YYYY-MM-DD`/RFC3339 e documentare; l'ordinamento per `key` usa `seq_id` (numerico) che è l'ordine naturale delle issue.
- Il round è chiuso solo con i tre livelli verdi (contract + unit/integration + E2E) come da metodo del progetto.

---

## Self-Review (svolta in fase di scrittura)

**1. Copertura spec (roadmap Round 4):**
- "Parser JQL completo (campi, operatori, funzioni tipo `currentUser()`, ORDER BY)" → Task 2-4 (lexer, parser, compiler) con AND/OR/NOT/IN/IS EMPTY/`~`/comparatori/`currentUser()`/ORDER BY. ✅
- "`/rest/api/3/search/jql`" → Task 8 (GET+POST, token-paginato). ✅ (+ legacy `/search` e `approximate-count`, `autocompletedata`).
- "filtri salvati e condivisi" → Task 1 (migrazione), 10 (service+mapper+handler), 11 (rotte). ✅
- "UI: ricerca globale, list view con colonne configurabili, filtri" → Task 14 (list view + pagina filtri), 15 (ricerca globale). ✅
- Gate a tre livelli → Task 12 (contract), 16 (E2E), 18 (gate). ✅

**2. Placeholder scan:** nessun "TBD"/"handle edge cases" senza codice. Le "Note implementatore" indicano verifiche puntuali su identificatori reali (firme di `JiraUser`, `Fields`, helper contract/frontend) — legittime perché il piano non può conoscere ogni firma esistente senza rischio; ogni nota dà il comando `grep` per risolverla. Il codice dei task core (jql lexer/parser/compiler, mapper, handler) è completo.

**3. Consistenza tipi:** `search.Service.Search(jql, resolver, offset, limit) SearchResult{Issues,Total}` usato coerentemente in T5/T8/T9. `jql.Resolver` (5 metodi) implementato da `staticResolver` (test T5), `fakeResolver` (test T4), `DBResolver` (T5). `v3.ProjectIssue(bean, Fields) map` usato in T6/T8. `v3.SearchAndReconcileResults`/`SearchResults`/`EncodeCursor`/`DecodeCursor` definiti in T7, usati in T8. `v3.Filter`/`FilterInput`/`JiraFilter`/`PageBeanFilterDetails` definiti in T10, usati in T10 handler. `filters`/`search` client (T13) usati in T14/T15. Nomi colonna (`title`, `status_id`, `assignee_id`, `priority`, `created_at`) coerenti col modello reale verificato.
