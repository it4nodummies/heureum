# Round 3 — Collaborazione: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Portare le funzioni di collaborazione (commenti ADF con @menzioni, worklog/time tracking, voti, watchers, issue link, remote link, changelog/history) alla conformità drop-in con la Jira Cloud REST API v3, con la UI integrata nella vista issue.

**Architecture:** Si riusa il pattern dei round 1-2: mapping v3 dedicati (author/updateAuthor via `v3.JiraUser`, body/commento in ADF via `internal/adf`, timestamp via `v3.JiraTime`), un contract test per endpoint contro `docs/contracts/jira-platform-v3.json`. Gli schemi Comment/Worklog/PageOfComments sono `additionalProperties:true` (permissivi → bassa barra); Votes/Watchers/IssueLink/RemoteIssueLink/PageBeanChangelog sono stretti (`additionalProperties:false` → attenzione ai campi). Le @menzioni sono nodi ADF `type:"mention"` con `attrs.id = accountId`; alla creazione di un commento si estraggono per le notifiche (riuso di `notifSvc`).

**Tech Stack:** Go 1.25, GORM, golang-migrate, kin-openapi (contract), Next.js 16 + TanStack Query + Playwright.

**Contesto codebase:**
- Mapping v3 di riferimento: `internal/api/v3/user.go` (`JiraUser(u user.User, baseURL) v3.User`), `issue.go` (`JiraIssue`), `datetime.go` (`JiraTime(t) string`). Helper risposta: `v3.WriteJSON`, `v3.WriteError(w,status,[]string,map[string]string)`, `v3.WritePage[T]{StartAt,MaxResults,Total,Values}` (emette startAt/maxResults/total/isLast/values).
- ADF: `internal/adf` — `adf.Node{Type,Version,Text,Content,Attrs,Marks}`, `adf.FromText`, `adf.Validate`.
- Commenti: `internal/domain/issue/comment_service.go` — `CommentService`, `NewCommentService(db)`, `SetNotifier(CommentNotifier)`, `AddComment(issueID, authorID, bodyJSON string)(*Comment,error)`, `GetComments(issueID)([]Comment,error)`, `GetComment(id)(*Comment,error)`, `UpdateComment(id, bodyJSON)(*Comment,error)`, `GetCommentsByIDs([]string)`, `SoftDeleteComment(id)error`. `Comment{ID,IssueID,AuthorID *string,BodyJSON,IsDeleted,CreatedAt,UpdatedAt}`.
- Issue service (link/watchers/history): `AddLink(sourceID,targetID string, linkType LinkType)(*IssueLink,error)`, `ListLinks(issueID)([]IssueLink,error)`, `GetLink(id)`, `DeleteLink(id)`, `Watch(issueID,userID)error`, `Unwatch(issueID,userID)error`, `GetWatchers(issueID)([]IssueWatcher,error)`, `GetHistory(issueID)([]IssueHistory,error)`, `GetByKey`, `GetBySeqID`, `DB()`. `LinkType`: `blocks|is_blocked|duplicates|relates`. `IssueLink{...}` (leggere il modello per i campi: probabilmente SourceID/TargetID/Type). `IssueWatcher{IssueID,UserID}`. `IssueHistory` (leggere per campi: probabilmente IssueID, ActorID, FieldName, OldValue, NewValue, CreatedAt).
- Handler esistenti (forma NON conforme, da allineare): `comment_handler.go` (`CommentHandler`, `NewCommentHandler(svc *issue.CommentService, issueSvc *issue.Service)`, Get/List/Create/Update/Delete/ListByIDs), `issuelink_handler.go` (`IssueLinkHandler`, `NewIssueLinkHandler(issueSvc)`, Get/Create/Delete), `history_handler.go` (`HistoryHandler`, `NewHistoryHandler(db, issueSvc)`, GetHistory), e watchers su `issue_handler.go` (`GetWatchers/AddWatcher/RemoveWatcher`). Questi handler NON hanno ancora `baseURL` — va aggiunto ai costruttori (come fatto per project/user/issue nei round precedenti), passando `cfg.BaseURL` dal router.
- `IssueHandler` ha già `baseURL`, `resolveIssue(idOrKey)(*issue.Issue,error)`, `itoaInt64`, `buildIssueInput`. Riusare `resolveIssue` dove serve risolvere l'issue dal path.
- Notifiche: `notifSvc` in `router.go`; `commentSvc.SetNotifier(notifSvc)` già cablato. Il `CommentNotifier` esistente riceve la notifica sulla creazione — le @menzioni vanno estratte e notificate (vedi Task menzioni).
- Contract harness: `contract.MustLoad(tb,"../../docs/contracts/jira-platform-v3.json")`, `(*Validator).ValidateResponse(method,path,status,header,body)`; helper (package contract): `newTestServer(t)`, `registerAndLogin(t,authSvc)`, `createProjectViaAPI(t,srv,jwt,key,name)`, `createIssueViaAPI(t,srv,jwt,projectKey,summary) string`.
- Ultima migrazione: `000007`. Le nuove partono da `000008`. NON esistono tabelle worklog né votes.
- Commit: conventional. No push. index.lock: riprovare, mai cancellare. Handler che toccano lo STESSO file vanno serializzati: `comment_handler.go` (T3), `issue_handler.go` (watchers T6), `issuelink_handler.go` (T7), `history_handler.go` (T8). I nuovi file (worklog_handler.go, votes_handler.go, remotelink_handler.go) sono indipendenti.

**Schemi v3 (verificati):**
- `Comment` addlProps:TRUE — chiavi usate: `self,id,author,body(ADF),updateAuthor,created,updated`. `PageOfComments` addlProps:TRUE `{comments,maxResults,startAt,total}` (usa `comments`, NON `values`).
- `Worklog` addlProps:TRUE — `{self,id,issueId,author,updateAuthor,comment(ADF),created,updated,started,timeSpent,timeSpentSeconds}`. `PageOfWorklogs` `{maxResults,startAt,total,worklogs}`.
- `Votes` addlProps:FALSE `{self,votes,hasVoted,voters([]User)}`.
- `Watchers` addlProps:FALSE `{self,isWatching,watchCount,watchers([]User)}`.
- `IssueLink` (POST body) req `[inwardIssue,outwardIssue,type]` addlProps:FALSE `{id,inwardIssue,outwardIssue,self,type}`; `type` = `{id,name,inward,outward,self}`; inward/outwardIssue = `{id,key,self,fields:{summary,status,...}}`.
- `PageBeanChangelog` addlProps:FALSE `{startAt,maxResults,total,isLast,nextPage,self,values}`; `Changelog` = `{id,author(User),created,items([{field,fieldtype,from,fromString,to,toString}])}`.
- `RemoteIssueLink` addlProps:FALSE `{self,id,globalId,application,relationship,object}`; `object` = `{url,title,summary,icon,...}`.

---

## File Structure

- `migrations/000008_worklogs.up/down.sql` — tabella `issue_worklogs`.
- `migrations/000009_votes.up/down.sql` — tabella `issue_votes`.
- `migrations/000010_remote_links.up/down.sql` — tabella `issue_remote_links`.
- `internal/api/v3/comment.go` (+test) — `JiraComment`, `ExtractMentions`, `PageOfComments`.
- `internal/api/v3/worklog.go` (+test) — `JiraWorklog`, `PageOfWorklogs`.
- `internal/api/v3/collab.go` (+test) — `Votes`, `Watchers` builders, `JiraIssueLink`, `JiraChangelogPage`, `RemoteLink`.
- `internal/domain/issue/worklog_service.go` (+test), `vote_service.go` (+test), `remotelink_service.go` (+test) — CRUD.
- `internal/api/handlers/comment_handler.go` — riscrittura v3.
- `internal/api/handlers/worklog_handler.go`, `votes_handler.go`, `remotelink_handler.go` — nuovi.
- `internal/api/handlers/issuelink_handler.go`, `history_handler.go`, `issue_handler.go` (watchers) — riscrittura v3.
- `internal/api/router.go` — costruttori con baseURL + nuove rotte.
- `internal/contract/collab_test.go` — contract test.
- `cmd/seed/main.go` — commenti demo.
- Frontend: `frontend-next/lib/api.ts`, `frontend-next/components/issues/IssueView.tsx`, `frontend-next/components/issues/Comments.tsx`, `frontend-next/e2e/collaboration.spec.ts`.

---

### Task 1: Migrazioni — worklog, votes, remote links

**Files:** Create `migrations/000008_worklogs.up.sql`+`.down.sql`, `migrations/000009_votes.up.sql`+`.down.sql`, `migrations/000010_remote_links.up.sql`+`.down.sql`

- [ ] **Step 1: worklog migration**

```sql
-- migrations/000008_worklogs.up.sql
CREATE TABLE issue_worklogs (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_id TEXT,
    comment_json TEXT NOT NULL DEFAULT '{}',
    time_spent_seconds INTEGER NOT NULL DEFAULT 0,
    started TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_issue_worklogs_issue_id ON issue_worklogs(issue_id);
```
```sql
-- migrations/000008_worklogs.down.sql
DROP INDEX idx_issue_worklogs_issue_id;
DROP TABLE issue_worklogs;
```

- [ ] **Step 2: votes migration**

```sql
-- migrations/000009_votes.up.sql
CREATE TABLE issue_votes (
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (issue_id, user_id)
);
```
```sql
-- migrations/000009_votes.down.sql
DROP TABLE issue_votes;
```

- [ ] **Step 3: remote links migration**

```sql
-- migrations/000010_remote_links.up.sql
CREATE TABLE issue_remote_links (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    global_id TEXT DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    summary TEXT DEFAULT '',
    relationship TEXT DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_issue_remote_links_issue_id ON issue_remote_links(issue_id);
```
```sql
-- migrations/000010_remote_links.down.sql
DROP INDEX idx_issue_remote_links_issue_id;
DROP TABLE issue_remote_links;
```

Guarda `migrations/000004_api_tokens.up.sql` per lo stile (compatibile postgres/mysql/sqlite).

- [ ] **Step 4: verifica**

Run: `APP_SECRET=x DB_DRIVER=sqlite DB_DSN=/tmp/mig-r3.db go run ./cmd/seed && sqlite3 /tmp/mig-r3.db '.tables' | tr ' ' '\n' | grep -E "issue_worklogs|issue_votes|issue_remote_links" && rm -f /tmp/mig-r3.db`
Expected: le tre tabelle esistono; exit 0.

- [ ] **Step 5: commit**

```bash
git add migrations/000008_worklogs.* migrations/000009_votes.* migrations/000010_remote_links.*
git commit -m "feat(collab): migrations for worklogs, votes, remote links"
```

---

### Task 2: Mapping v3 commenti + estrazione @menzioni

**Files:** Create `internal/api/v3/comment.go`, `internal/api/v3/comment_test.go`

- [ ] **Step 1: test fallenti**

```go
// internal/api/v3/comment_test.go
package v3

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

func TestJiraComment(t *testing.T) {
	c := issue.Comment{ID: "c1", IssueID: "i1", BodyJSON: `{"content":"Hello"}`,
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	author := &user.User{ID: "u1", DisplayName: "Ada", IsActive: true}
	jc := JiraComment(c, author, author, "http://h")
	if jc.ID != "c1" {
		t.Errorf("id = %q", jc.ID)
	}
	if jc.Self != "http://h/rest/api/3/issue/i1/comment/c1" {
		t.Errorf("self = %q", jc.Self)
	}
	if jc.Author == nil || jc.Author.AccountID != "u1" {
		t.Errorf("author = %+v", jc.Author)
	}
	raw, _ := json.Marshal(jc.Body)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	if doc["type"] != "doc" {
		t.Errorf("body not ADF: %s", raw)
	}
	if jc.Created == "" {
		t.Error("created empty")
	}
}

func TestExtractMentions(t *testing.T) {
	body := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
		{"type":"text","text":"ciao "},
		{"type":"mention","attrs":{"id":"acc-42","text":"@Bob"}},
		{"type":"text","text":" e "},
		{"type":"mention","attrs":{"id":"acc-99"}}
	]}]}`
	ids := ExtractMentions(body)
	if len(ids) != 2 || ids[0] != "acc-42" || ids[1] != "acc-99" {
		t.Errorf("mentions = %v", ids)
	}
	// body non-ADF o senza menzioni → slice vuota
	if len(ExtractMentions(`{"content":"plain"}`)) != 0 {
		t.Error("expected no mentions")
	}
}
```

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/api/v3/ -run 'TestJiraComment|TestExtractMentions'`
Expected: FAIL — simboli non definiti.

- [ ] **Step 3: implementa**

```go
// internal/api/v3/comment.go
package v3

import (
	"encoding/json"
	"fmt"

	"github.com/open-jira/open-jira/internal/adf"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

// Comment è la rappresentazione Jira v3 di un commento (schema Comment).
type Comment struct {
	Self         string    `json:"self"`
	ID           string    `json:"id"`
	Author       *User     `json:"author,omitempty"`
	Body         *adf.Node `json:"body,omitempty"`
	UpdateAuthor *User     `json:"updateAuthor,omitempty"`
	Created      string    `json:"created"`
	Updated      string    `json:"updated"`
}

// PageOfComments è la pagina di commenti (schema PageOfComments): usa la chiave
// "comments" (non "values").
type PageOfComments struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Comments   []Comment `json:"comments"`
}

// bodyADF interpreta BodyJSON (doc ADF, {"content":"..."} legacy, o testo) come ADF.
func bodyADF(bodyJSON string) *adf.Node {
	if bodyJSON == "" || bodyJSON == "{}" {
		return nil
	}
	var node adf.Node
	if err := json.Unmarshal([]byte(bodyJSON), &node); err == nil && node.Type == "doc" {
		return &node
	}
	var legacy struct {
		Content string `json:"content"`
	}
	text := bodyJSON
	if err := json.Unmarshal([]byte(bodyJSON), &legacy); err == nil && legacy.Content != "" {
		text = legacy.Content
	}
	doc := adf.FromText(text)
	return &doc
}

func JiraComment(c issue.Comment, author, updateAuthor *user.User, baseURL string) Comment {
	jc := Comment{
		Self:    fmt.Sprintf("%s/rest/api/3/issue/%s/comment/%s", baseURL, c.IssueID, c.ID),
		ID:      c.ID,
		Body:    bodyADF(c.BodyJSON),
		Created: JiraTime(c.CreatedAt),
		Updated: JiraTime(c.UpdatedAt),
	}
	if author != nil {
		u := JiraUser(*author, baseURL)
		jc.Author = &u
	}
	if updateAuthor != nil {
		u := JiraUser(*updateAuthor, baseURL)
		jc.UpdateAuthor = &u
	}
	return jc
}

// ExtractMentions raccoglie gli accountId dai nodi ADF type:"mention" (attrs.id).
func ExtractMentions(bodyJSON string) []string {
	var node adf.Node
	if err := json.Unmarshal([]byte(bodyJSON), &node); err != nil || node.Type != "doc" {
		return nil
	}
	var out []string
	var walk func(n adf.Node)
	walk = func(n adf.Node) {
		if n.Type == "mention" {
			if id, ok := n.Attrs["id"].(string); ok && id != "" {
				out = append(out, id)
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(node)
	return out
}
```

Nota: `self` del commento usa `c.IssueID` (UUID interno). Jira usa la key/seqid; ma poiché lo schema Comment è `additionalProperties:true` e `self` è solo `format:uri`, un URL con l'UUID è schema-valido. Per fedeltà si potrebbe risolvere il seqid, ma è opzionale (follow-up).

- [ ] **Step 4: verifica PASS**

Run: `go test ./internal/api/v3/ -count=1`
Expected: PASS

- [ ] **Step 5: commit**

```bash
git add internal/api/v3/comment.go internal/api/v3/comment_test.go
git commit -m "feat(v3): comment mapping (ADF body) and @mention extraction"
```

---

### Task 3: Endpoint commenti conformi

**Files:** Modify `internal/api/handlers/comment_handler.go`, `internal/api/router.go`; Test `internal/contract/collab_test.go` (create)

Contesto: aggiungi `baseURL string` a `CommentHandler` e al costruttore `NewCommentHandler(svc, issueSvc, baseURL)`; aggiorna il router. Il body dei commenti in v3 è `{"body": <ADF>}`.

- [ ] **Step 1: contract test fallenti** (create `internal/contract/collab_test.go`)

```go
package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func addCommentViaAPI(t *testing.T, srv *httptest.Server, jwt, issueKey, text string) string {
	t.Helper()
	body := `{"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"` + text + `"}]}]}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue/"+issueKey+"/comment", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("add comment status = %d: %s", res.StatusCode, b)
	}
	var out struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&out)
	return out.ID
}

func TestAddComment_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Has comments")

	body := `{"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"First comment"}]}]}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue/"+key+"/comment", strings.NewReader(body))
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
	if err := v.ValidateResponse("POST", "/rest/api/3/issue/"+key+"/comment", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("POST comment non conforme: %v", err)
	}
}

func TestListComments_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Has comments")
	addCommentViaAPI(t, srv, jwt, key, "c1")

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key+"/comment", nil)
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
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key+"/comment", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET comments non conforme: %v", err)
	}
}
```

Verifica lo status di `POST /comment` nel contratto (Jira risponde 201): `python3 -c 'import json;d=json.load(open("docs/contracts/jira-platform-v3.json"));print(list(d["paths"]["/rest/api/3/issue/{issueIdOrKey}/comment"]["post"]["responses"].keys()))'` e allinea.

- [ ] **Step 2: verifica FAIL**

Run: `go test ./internal/contract/ -run 'TestAddComment|TestListComments' -count=1`
Expected: FAIL — forma interna non conforme.

- [ ] **Step 3: riscrivi il comment handler**

Leggi `comment_handler.go`. Riscrivi `Create`, `List`, `Get`, `Update`, `Delete` in forma v3. Aggiungi un helper per caricare l'autore:

```go
func (h *CommentHandler) author(c *issue.Comment) *user.User {
	if c.AuthorID == nil {
		return nil
	}
	var u user.User
	if h.issueSvc.DB().First(&u, "id = ?", *c.AuthorID).Error != nil {
		return nil
	}
	return &u
}
```

`Create`:
```go
func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	iss, err := h.issueSvc.GetByKey(r.PathValue("issueIdOrKey"))
	if err != nil || iss == nil {
		if n, perr := strconv.ParseInt(r.PathValue("issueIdOrKey"), 10, 64); perr == nil {
			iss, err = h.issueSvc.GetBySeqID(n)
		}
	}
	if err != nil || iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	var req struct {
		Body any `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	if req.Body == nil {
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"body": "The comment body is required."})
		return
	}
	bodyJSON, _ := json.Marshal(req.Body)
	userID := middleware.UserIDFromContext(r.Context())
	c, err := h.svc.AddComment(iss.ID, userID, string(bodyJSON))
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to add comment."}, nil)
		return
	}
	// @menzioni → notifiche (best-effort)
	for _, accountID := range v3.ExtractMentions(string(bodyJSON)) {
		h.notifyMention(iss, c, accountID)
	}
	author := h.author(c)
	v3.WriteJSON(w, http.StatusCreated, v3.JiraComment(*c, author, author, h.baseURL))
}
```

Per `notifyMention`: usa il notifier già iniettato nel `CommentService` OPPURE, se più semplice, aggiungi un metodo no-op nel handler che chiama il servizio notifiche se disponibile. Se cablare le notifiche di menzione richiede più del banale, implementa `notifyMention` come best-effort che logga e continua, e traccia la notifica-vera come follow-up (l'importante per il contratto è la risposta). Leggi `notification` service per la firma; se non c'è un metodo adatto, `notifyMention` resta un log.

`List` (PageOfComments):
```go
func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r) // helper condiviso che risolve issueIdOrKey → *issue.Issue o nil
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	comments, _ := h.svc.GetComments(iss.ID)
	out := make([]v3.Comment, 0, len(comments))
	for i := range comments {
		a := h.author(&comments[i])
		out = append(out, v3.JiraComment(comments[i], a, a, h.baseURL))
	}
	v3.WriteJSON(w, http.StatusOK, v3.PageOfComments{StartAt: 0, MaxResults: len(out), Total: len(out), Comments: out})
}
```

`Get`/`Update`/`Delete` per `/comment/{id}`: `Get`→200 Comment; `Update`→200 Comment (body ADF, aggiorna updated); `Delete`→204. Estrai un helper `resolve(r)` (issueIdOrKey→issue) e riusalo. Verifica lo status del DELETE nel contratto (204).

Aggiungi al handler i campi/argomenti necessari (`baseURL`, accesso a issueSvc già presente). Import `strconv`, `internal/api/v3`, `internal/domain/user`, `internal/api/middleware`.

- [ ] **Step 4: aggiorna il router**

`commentH := handlers.NewCommentHandler(commentSvc, issueSvc, cfg.BaseURL)`. Le rotte comment esistono già; verifica che i path param si chiamino `{issueIdOrKey}` e `{id}` coerentemente con gli handler.

- [ ] **Step 5: verifica PASS**

Run: `go test ./internal/contract/ -run 'TestAddComment|TestListComments' -count=1 && go build ./... && go vet ./...`
Expected: PASS + clean. Aggiorna eventuali test handler preesistenti sulla vecchia forma.

- [ ] **Step 6: commit**

```bash
git add internal/api/handlers/comment_handler.go internal/api/router.go internal/contract/collab_test.go
git commit -m "feat(v3): comments endpoints conform to contract (ADF body, PageOfComments)"
```

---

### Task 4: Worklog — dominio + mapping + endpoint

**Files:** Create `internal/domain/issue/worklog_service.go`+test, `internal/api/v3/worklog.go`+test, `internal/api/handlers/worklog_handler.go`; Modify `internal/api/router.go`; Test add to `internal/contract/collab_test.go`

- [ ] **Step 1: worklog service + model (TDD)**

Test `internal/domain/issue/worklog_service_test.go`:
```go
package issue

import "testing"

func TestWorklogCRUD(t *testing.T) {
	db := newIssueTestDB(t) // helper esistente; assicurati che AutoMigri &Worklog{}
	db.AutoMigrate(&Worklog{})
	svc := NewWorklogService(db)
	wl, err := svc.Add("i1", "u1", `{"content":"worked"}`, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if wl.TimeSpentSeconds != 3600 || wl.IssueID != "i1" {
		t.Errorf("unexpected: %+v", wl)
	}
	list, _ := svc.ListByIssue("i1")
	if len(list) != 1 {
		t.Fatalf("list = %d", len(list))
	}
	if err := svc.Delete(wl.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = svc.ListByIssue("i1")
	if len(list) != 0 {
		t.Errorf("after delete list = %d", len(list))
	}
}
```
Implementa in `internal/domain/issue/worklog_service.go`:
```go
package issue

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Worklog struct {
	ID               string     `gorm:"primaryKey;type:text" json:"id"`
	IssueID          string     `gorm:"type:text;not null;index" json:"issue_id"`
	AuthorID         *string    `gorm:"type:text" json:"author_id,omitempty"`
	CommentJSON      string     `gorm:"column:comment_json;type:text;default:'{}'" json:"comment_json"`
	TimeSpentSeconds int        `gorm:"column:time_spent_seconds;default:0" json:"time_spent_seconds"`
	Started          *time.Time `json:"started,omitempty"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Worklog) TableName() string { return "issue_worklogs" }

type WorklogService struct{ db *gorm.DB }

func NewWorklogService(db *gorm.DB) *WorklogService { return &WorklogService{db: db} }

func (s *WorklogService) Add(issueID, authorID, commentJSON string, seconds int) (*Worklog, error) {
	now := time.Now()
	wl := &Worklog{ID: uuid.NewString(), IssueID: issueID, CommentJSON: commentJSON, TimeSpentSeconds: seconds, Started: &now}
	if authorID != "" {
		wl.AuthorID = &authorID
	}
	if commentJSON == "" {
		wl.CommentJSON = "{}"
	}
	if err := s.db.Create(wl).Error; err != nil {
		return nil, err
	}
	return wl, nil
}

func (s *WorklogService) ListByIssue(issueID string) ([]Worklog, error) {
	var out []Worklog
	err := s.db.Where("issue_id = ?", issueID).Order("created_at asc").Find(&out).Error
	return out, err
}

func (s *WorklogService) Get(id string) (*Worklog, error) {
	var wl Worklog
	if err := s.db.First(&wl, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &wl, nil
}

func (s *WorklogService) Delete(id string) error {
	return s.db.Delete(&Worklog{}, "id = ?", id).Error
}
```
Verifica che `newIssueTestDB` esista nel package (dal Round 2) e migri le tabelle necessarie; aggiungi `&Worklog{}` alla sua AutoMigrate o migralo nel test.

Run: `go test ./internal/domain/issue/ -run TestWorklogCRUD` → dopo impl PASS.

- [ ] **Step 2: v3 mapping (TDD)** `internal/api/v3/worklog.go`+`worklog_test.go`

Test:
```go
package v3

import (
	"testing"
	"time"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

func TestJiraWorklog(t *testing.T) {
	started := time.Now()
	wl := issue.Worklog{ID: "w1", IssueID: "10001", CommentJSON: `{"content":"x"}`, TimeSpentSeconds: 3600, Started: &started, CreatedAt: started, UpdatedAt: started}
	author := &user.User{ID: "u1", DisplayName: "Ada", IsActive: true}
	jw := JiraWorklog(wl, author, "http://h")
	if jw.ID != "w1" || jw.IssueID != "10001" || jw.TimeSpentSeconds != 3600 {
		t.Errorf("unexpected: %+v", jw)
	}
	if jw.TimeSpent != "1h" {
		t.Errorf("timeSpent = %q, want 1h", jw.TimeSpent)
	}
	if jw.Author == nil || jw.Author.AccountID != "u1" {
		t.Errorf("author = %+v", jw.Author)
	}
	if jw.Self == "" || jw.Started == "" {
		t.Errorf("self/started empty: %+v", jw)
	}
}
```
Impl `internal/api/v3/worklog.go`:
```go
package v3

import (
	"fmt"

	"github.com/open-jira/open-jira/internal/adf"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

type Worklog struct {
	Self             string    `json:"self"`
	ID               string    `json:"id"`
	IssueID          string    `json:"issueId"`
	Author           *User     `json:"author,omitempty"`
	UpdateAuthor     *User     `json:"updateAuthor,omitempty"`
	Comment          *adf.Node `json:"comment,omitempty"`
	Created          string    `json:"created"`
	Updated          string    `json:"updated"`
	Started          string    `json:"started"`
	TimeSpent        string    `json:"timeSpent"`
	TimeSpentSeconds int       `json:"timeSpentSeconds"`
}

type PageOfWorklogs struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Worklogs   []Worklog `json:"worklogs"`
}

// formatSeconds rende i secondi in formato Jira ("1h", "30m", "1h 30m", "0m").
func formatSeconds(sec int) string {
	if sec <= 0 {
		return "0m"
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

func JiraWorklog(wl issue.Worklog, author *user.User, baseURL string) Worklog {
	jw := Worklog{
		Self:             fmt.Sprintf("%s/rest/api/3/issue/%s/worklog/%s", baseURL, wl.IssueID, wl.ID),
		ID:               wl.ID,
		IssueID:          wl.IssueID,
		Comment:          bodyADF(wl.CommentJSON),
		Created:          JiraTime(wl.CreatedAt),
		Updated:          JiraTime(wl.UpdatedAt),
		TimeSpent:        formatSeconds(wl.TimeSpentSeconds),
		TimeSpentSeconds: wl.TimeSpentSeconds,
	}
	if wl.Started != nil {
		jw.Started = JiraTime(*wl.Started)
	}
	if author != nil {
		u := JiraUser(*author, baseURL)
		jw.Author = &u
		jw.UpdateAuthor = &u
	}
	return jw
}
```
Nota: sostituisci `adfNodeAlias` con `*adf.Node` importando `internal/adf` (l'alias qui è solo un segnaposto per evitare confusione — usa `*adf.Node` come in `comment.go`). Riusa `bodyADF` da comment.go (stesso package).

Run: `go test ./internal/api/v3/ -run TestJiraWorklog` → PASS.

- [ ] **Step 3: handler + rotte + contract test**

`internal/api/handlers/worklog_handler.go`:
```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/api/middleware"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

type WorklogHandler struct {
	svc      *issue.WorklogService
	issueSvc *issue.Service
	baseURL  string
}

func NewWorklogHandler(svc *issue.WorklogService, issueSvc *issue.Service, baseURL string) *WorklogHandler {
	return &WorklogHandler{svc: svc, issueSvc: issueSvc, baseURL: baseURL}
}

func (h *WorklogHandler) resolve(r *http.Request) *issue.Issue {
	k := r.PathValue("issueIdOrKey")
	if n, err := strconv.ParseInt(k, 10, 64); err == nil {
		iss, _ := h.issueSvc.GetBySeqID(n)
		return iss
	}
	iss, _ := h.issueSvc.GetByKey(k)
	return iss
}

func (h *WorklogHandler) author(id *string) *user.User {
	if id == nil {
		return nil
	}
	var u user.User
	if h.issueSvc.DB().First(&u, "id = ?", *id).Error != nil {
		return nil
	}
	return &u
}

func (h *WorklogHandler) List(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	logs, _ := h.svc.ListByIssue(iss.ID)
	out := make([]v3.Worklog, 0, len(logs))
	for i := range logs {
		out = append(out, v3.JiraWorklog(logs[i], h.author(logs[i].AuthorID), h.baseURL))
	}
	v3.WriteJSON(w, http.StatusOK, v3.PageOfWorklogs{StartAt: 0, MaxResults: len(out), Total: len(out), Worklogs: out})
}

func (h *WorklogHandler) Create(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Issue does not exist or you do not have permission to see it."}, nil)
		return
	}
	var req struct {
		Comment          any `json:"comment"`
		TimeSpentSeconds int `json:"timeSpentSeconds"`
		TimeSpent        string `json:"timeSpent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	seconds := req.TimeSpentSeconds
	if seconds == 0 && req.TimeSpent != "" {
		seconds = parseJiraDuration(req.TimeSpent) // helper sotto
	}
	commentJSON := "{}"
	if req.Comment != nil {
		if b, err := json.Marshal(req.Comment); err == nil {
			commentJSON = string(b)
		}
	}
	userID := middleware.UserIDFromContext(r.Context())
	wl, err := h.svc.Add(iss.ID, userID, commentJSON, seconds)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to add worklog."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, v3.JiraWorklog(*wl, h.author(wl.AuthorID), h.baseURL))
}

func (h *WorklogHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.PathValue("id")); err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"Worklog does not exist."}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseJiraDuration converte "1h", "30m", "1h 30m", "2d" in secondi (1d=8h, 1w=5d).
func parseJiraDuration(s string) int {
	units := map[byte]int{'w': 5 * 8 * 3600, 'd': 8 * 3600, 'h': 3600, 'm': 60}
	total, num := 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			num = num*10 + int(c-'0')
		case units[c] != 0:
			total += num * units[c]
			num = 0
		}
	}
	return total
}
```
Rotte in `router.go` (costruisci `worklogSvc := issue.NewWorklogService(db)` e `worklogH := handlers.NewWorklogHandler(worklogSvc, issueSvc, cfg.BaseURL)`):
```go
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/worklog", authMw(http.HandlerFunc(worklogH.List)))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/worklog", authMw(http.HandlerFunc(worklogH.Create)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/worklog/{id}", authMw(http.HandlerFunc(worklogH.Delete)))
```
Contract test in `collab_test.go`:
```go
func TestWorklog_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Track time")
	// add
	body := `{"timeSpentSeconds":3600,"comment":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"worked"}]}]}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue/"+key+"/worklog", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt); req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatal(err) }
	defer res.Body.Close()
	if res.StatusCode != 201 { t.Fatalf("status = %d", res.StatusCode) }
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("POST", "/rest/api/3/issue/"+key+"/worklog", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("POST worklog non conforme: %v", err)
	}
	// list
	lreq, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key+"/worklog", nil)
	lreq.Header.Set("Authorization", "Bearer "+jwt)
	lres, err := http.DefaultClient.Do(lreq)
	if err != nil { t.Fatal(err) }
	defer lres.Body.Close()
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key+"/worklog", lres.StatusCode, lres.Header, lres.Body); err != nil {
		t.Errorf("GET worklog non conforme: %v", err)
	}
}
```
Verifica lo status del POST worklog nel contratto (201) e adatta.

- [ ] **Step 4: verifica** `go test ./internal/contract/ -run TestWorklog -count=1 && go build ./... && go vet ./...` → PASS.

- [ ] **Step 5: commit**

```bash
git add internal/domain/issue/worklog_service.go internal/domain/issue/worklog_service_test.go internal/api/v3/worklog.go internal/api/v3/worklog_test.go internal/api/handlers/worklog_handler.go internal/api/router.go internal/contract/collab_test.go
git commit -m "feat(v3): worklog domain, mapping and endpoints"
```

---

### Task 5: Votes — dominio + endpoint

**Files:** Create `internal/domain/issue/vote_service.go`+test, `internal/api/handlers/votes_handler.go`; Modify `internal/api/router.go`, `internal/api/v3/collab.go` (create con `Votes`); Test add to `collab_test.go`

- [ ] **Step 1: vote service (TDD)** `internal/domain/issue/vote_service.go`

Test `vote_service_test.go`: `Add(issueID,userID)` idempotente, `Remove`, `Count(issueID) int`, `HasVoted(issueID,userID) bool`, `Voters(issueID) []string`.
```go
package issue

import (
	"time"

	"gorm.io/gorm"
)

type Vote struct {
	IssueID   string    `gorm:"primaryKey;type:text" json:"issue_id"`
	UserID    string    `gorm:"primaryKey;type:text" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Vote) TableName() string { return "issue_votes" }

type VoteService struct{ db *gorm.DB }

func NewVoteService(db *gorm.DB) *VoteService { return &VoteService{db: db} }

func (s *VoteService) Add(issueID, userID string) error {
	return s.db.Where(Vote{IssueID: issueID, UserID: userID}).FirstOrCreate(&Vote{IssueID: issueID, UserID: userID}).Error
}
func (s *VoteService) Remove(issueID, userID string) error {
	return s.db.Delete(&Vote{}, "issue_id = ? AND user_id = ?", issueID, userID).Error
}
func (s *VoteService) Count(issueID string) int {
	var n int64
	s.db.Model(&Vote{}).Where("issue_id = ?", issueID).Count(&n)
	return int(n)
}
func (s *VoteService) HasVoted(issueID, userID string) bool {
	var n int64
	s.db.Model(&Vote{}).Where("issue_id = ? AND user_id = ?", issueID, userID).Count(&n)
	return n > 0
}
func (s *VoteService) Voters(issueID string) []string {
	var ids []string
	s.db.Model(&Vote{}).Where("issue_id = ?", issueID).Pluck("user_id", &ids)
	return ids
}
```

- [ ] **Step 2: v3 Votes mapping** — aggiungi a `internal/api/v3/collab.go` (create):
```go
package v3

import (
	"fmt"

	"github.com/open-jira/open-jira/internal/domain/user"
)

type Votes struct {
	Self     string `json:"self"`
	Votes    int    `json:"votes"`
	HasVoted bool   `json:"hasVoted"`
	Voters   []User `json:"voters"`
}

func JiraVotes(issueKey, baseURL string, count int, hasVoted bool, voters []user.User) Votes {
	vs := Votes{
		Self:     fmt.Sprintf("%s/rest/api/3/issue/%s/votes", baseURL, issueKey),
		Votes:    count,
		HasVoted: hasVoted,
		Voters:   make([]User, 0, len(voters)),
	}
	for _, u := range voters {
		vs.Voters = append(vs.Voters, JiraUser(u, baseURL))
	}
	return vs
}
```
Test in `collab_v3_test.go`: verifica campi (Votes addlProps:FALSE, voters mai nil → []).

- [ ] **Step 3: handler + rotte + contract test**

`votes_handler.go`: `List` (GET → Votes), `Add` (POST → 204), `Remove` (DELETE → 204). Risolve issue via seqid/key, carica i voter come `[]user.User` dagli id di `Voters`. Rotte:
```go
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/votes", authMw(http.HandlerFunc(votesH.List)))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/votes", authMw(http.HandlerFunc(votesH.Add)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/votes", authMw(http.HandlerFunc(votesH.Remove)))
```
Contract test `TestVotes_ConformsToContract`: POST votes (204), GET votes (200, valida contro schema Votes). Verifica gli status nel contratto e adatta.

- [ ] **Step 4: verifica** `go test ./internal/contract/ -run TestVotes -count=1 && go build ./...` → PASS.

- [ ] **Step 5: commit**

```bash
git add internal/domain/issue/vote_service.go internal/domain/issue/vote_service_test.go internal/api/v3/collab.go internal/api/v3/collab_v3_test.go internal/api/handlers/votes_handler.go internal/api/router.go internal/contract/collab_test.go
git commit -m "feat(v3): votes domain and endpoints"
```

---

### Task 6: Watchers conformi

**Files:** Modify `internal/api/handlers/issue_handler.go` (GetWatchers/AddWatcher/RemoveWatcher), `internal/api/v3/collab.go` (add `Watchers`), `internal/api/router.go`; Test add to `collab_test.go`

- [ ] **Step 1: v3 Watchers mapping** — add to `collab.go`:
```go
type Watchers struct {
	Self       string `json:"self"`
	IsWatching bool   `json:"isWatching"`
	WatchCount int    `json:"watchCount"`
	Watchers   []User `json:"watchers"`
}

func JiraWatchers(issueKey, baseURL string, isWatching bool, watchers []user.User) Watchers {
	ws := Watchers{
		Self:       fmt.Sprintf("%s/rest/api/3/issue/%s/watchers", baseURL, issueKey),
		IsWatching: isWatching,
		WatchCount: len(watchers),
		Watchers:   make([]User, 0, len(watchers)),
	}
	for _, u := range watchers {
		ws.Watchers = append(ws.Watchers, JiraUser(u, baseURL))
	}
	return ws
}
```

- [ ] **Step 2: contract test** `TestWatchers_ConformsToContract`: POST watchers (add current user — Jira POST body is a raw accountId string, but our impl can add the caller; status 204), GET watchers (200 → schema Watchers). Add to collab_test.go.

- [ ] **Step 3: riscrivi i tre handler watchers in issue_handler.go**
`GetWatchers`: risolvi issue, `svc.GetWatchers(iss.ID)` → carica gli utenti → `v3.JiraWatchers(iss.Key, h.baseURL, isWatching, users)` (isWatching = current user tra i watchers). `AddWatcher`: `svc.Watch(iss.ID, userID)` → 204. `RemoveWatcher`: `svc.Unwatch(iss.ID, userID)` → 204. IssueWatcher ha solo IssueID/UserID: carica gli utenti con una query `user.User` sugli userID.

- [ ] **Step 4: rotte** già presenti (GET/POST/DELETE watchers) — puntano già ai metodi. Nessun cambio se i nomi combaciano.

- [ ] **Step 5: verifica** `go test ./internal/contract/ -run TestWatchers -count=1 && go build ./...` → PASS.

- [ ] **Step 6: commit**

```bash
git add internal/api/handlers/issue_handler.go internal/api/v3/collab.go internal/api/router.go internal/contract/collab_test.go
git commit -m "feat(v3): watchers endpoints conform to contract"
```

---

### Task 7: Issue link conforme

**Files:** Modify `internal/api/handlers/issuelink_handler.go`, `internal/api/v3/collab.go` (add link mapping), `internal/api/router.go`; Test add to `collab_test.go`

- [ ] **Step 1: v3 issue-link mapping** — add to `collab.go`:
```go
type LinkTypeRef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
	Self    string `json:"self"`
}

type LinkedIssueRef struct {
	ID     string         `json:"id"`
	Key    string         `json:"key"`
	Self   string         `json:"self"`
	Fields map[string]any `json:"fields"`
}

type IssueLinkV3 struct {
	ID           string          `json:"id"`
	Self         string          `json:"self"`
	Type         LinkTypeRef     `json:"type"`
	InwardIssue  *LinkedIssueRef `json:"inwardIssue,omitempty"`
	OutwardIssue *LinkedIssueRef `json:"outwardIssue,omitempty"`
}
```
Un builder `JiraLinkType(name, baseURL)` che mappa il nostro enum interno (`blocks|is_blocked|duplicates|relates`) a name/inward/outward standard Jira (es. name "Blocks", inward "is blocked by", outward "blocks"). E un `LinkedIssue(iss issue.Issue, baseURL)` che produce `{id: seqid, key, self, fields:{summary, status}}`.

- [ ] **Step 2: contract test** — nota: `POST /issueLink` in Jira risponde **201 senza body** (verifica: `python3 -c 'import json;d=json.load(open("docs/contracts/jira-platform-v3.json"));print(list(d["paths"]["/rest/api/3/issueLink"]["post"]["responses"].keys()))'`). Il contract test verifica quindi lo status e che l'input sia accettato. Il body di richiesta è `{type:{name},inwardIssue:{key},outwardIssue:{key}}`:
```go
func TestCreateIssueLink_Status(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	a := createIssueViaAPI(t, srv, jwt, "DEMO", "Blocker")
	b := createIssueViaAPI(t, srv, jwt, "DEMO", "Blocked")
	body := `{"type":{"name":"Blocks"},"inwardIssue":{"key":"` + b + `"},"outwardIssue":{"key":"` + a + `"}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issueLink", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt); req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatal(err) }
	res.Body.Close()
	if res.StatusCode != 201 { t.Fatalf("status = %d, want 201", res.StatusCode) }
}
```
(Se il contratto documenta 200/204, adatta.)

- [ ] **Step 3: riscrivi il POST handler** per accettare `{type:{name},inwardIssue:{key},outwardIssue:{key}}`, mappare il name al `LinkType` interno, risolvere le due issue, chiamare `issueSvc.AddLink(outwardID, inwardID, linkType)`, rispondere 201. Mantieni `GET/DELETE /issueLink/{linkId}` come estensione, aggiornando la loro risposta a `IssueLinkV3` (non-standard path ma utile). Aggiungi `baseURL` al costruttore.

- [ ] **Step 4: verifica** `go test ./internal/contract/ -run TestCreateIssueLink -count=1 && go build ./...` → PASS.

- [ ] **Step 5: commit**

```bash
git add internal/api/handlers/issuelink_handler.go internal/api/v3/collab.go internal/api/router.go internal/contract/collab_test.go
git commit -m "feat(v3): issue link creation conforms to contract"
```

---

### Task 8: Changelog conforme (PageBeanChangelog)

**Files:** Modify `internal/api/handlers/history_handler.go`, `internal/api/v3/collab.go` (add changelog mapping), `internal/api/router.go`; Test add to `collab_test.go`

- [ ] **Step 1: v3 changelog mapping** — add to `collab.go`:
```go
type ChangeItem struct {
	Field      string `json:"field"`
	Fieldtype  string `json:"fieldtype"`
	From       string `json:"from,omitempty"`
	FromString string `json:"fromString,omitempty"`
	To         string `json:"to,omitempty"`
	ToString   string `json:"toString,omitempty"`
}

type Changelog struct {
	ID      string       `json:"id"`
	Author  *User        `json:"author,omitempty"`
	Created string       `json:"created"`
	Items   []ChangeItem `json:"items"`
}
```
Usa `v3.WritePage[Changelog]` per la risposta (PageBeanChangelog ha esattamente la forma di WritePage: startAt/maxResults/total/isLast/values). Ogni `IssueHistory` → un `Changelog` con un solo `ChangeItem` (field=FieldName, fromString=OldValue, toString=NewValue, fieldtype="jira"), author dall'ActorID, created da CreatedAt.

- [ ] **Step 2: contract test** `TestChangelog_ConformsToContract`: crea issue, GET changelog → 200, valida contro PageBeanChangelog. (Le issue create hanno già una voce history "created".)

- [ ] **Step 3: riscrivi GetHistory** → produce `[]v3.Changelog` da `issueSvc.GetHistory(iss.ID)`, risponde con `v3.WritePage`. Aggiungi baseURL. Leggi i campi reali di `IssueHistory` e adatta il mapping.

- [ ] **Step 4: verifica** `go test ./internal/contract/ -run TestChangelog -count=1 && go build ./...` → PASS.

- [ ] **Step 5: commit**

```bash
git add internal/api/handlers/history_handler.go internal/api/v3/collab.go internal/api/router.go internal/contract/collab_test.go
git commit -m "feat(v3): issue changelog conforms to PageBeanChangelog"
```

---

### Task 9: Remote links — dominio + endpoint

**Files:** Create `internal/domain/issue/remotelink_service.go`+test, `internal/api/handlers/remotelink_handler.go`; Modify `internal/api/v3/collab.go` (add RemoteLink), `internal/api/router.go`; Test add to `collab_test.go`

- [ ] **Step 1: service** `internal/domain/issue/remotelink_service.go` — `RemoteLink{ID,IssueID,GlobalID,URL,Title,Summary,Relationship,CreatedAt}` (TableName issue_remote_links), `NewRemoteLinkService(db)`, `Add(issueID, url, title, summary, relationship string)(*RemoteLink,error)`, `ListByIssue(issueID)`, `Delete(id)`. TDD.

- [ ] **Step 2: v3 mapping** — add to `collab.go`:
```go
type RemoteLinkObject struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
}

type RemoteLink struct {
	Self         string           `json:"self"`
	ID           int              `json:"id"`
	GlobalID     string           `json:"globalId,omitempty"`
	Relationship string           `json:"relationship,omitempty"`
	Object       RemoteLinkObject `json:"object"`
}
```
Nota: `RemoteIssueLink.id` è integer nel contratto — usa un hash stabile dell'UUID o un seq. Per semplicità puoi esporre un intero derivato (es. FNV dell'ID) O aggiungere una colonna seq; la via più pulita e coerente col resto del progetto è un seq_id, ma per un remote link un intero derivato deterministico è accettabile (documentalo). Verifica nel contratto se `id` è davvero integer e regola.

- [ ] **Step 3: handler + rotte + contract test** — GET (array RemoteIssueLink), POST (crea; risposta con {id,self} o 201), DELETE. Rotte:
```go
	mux.Handle("GET /rest/api/3/issue/{issueIdOrKey}/remotelink", authMw(http.HandlerFunc(remoteH.List)))
	mux.Handle("POST /rest/api/3/issue/{issueIdOrKey}/remotelink", authMw(http.HandlerFunc(remoteH.Create)))
	mux.Handle("DELETE /rest/api/3/issue/{issueIdOrKey}/remotelink/{id}", authMw(http.HandlerFunc(remoteH.Delete)))
```
Contract test valida GET (200 array) contro lo schema. Verifica il response schema/status del POST e adatta.

- [ ] **Step 4: verifica** `go test ./internal/contract/ -run TestRemoteLink -count=1 && go build ./...` → PASS.

- [ ] **Step 5: commit**

```bash
git add internal/domain/issue/remotelink_service.go internal/domain/issue/remotelink_service_test.go internal/api/handlers/remotelink_handler.go internal/api/v3/collab.go internal/api/router.go internal/contract/collab_test.go
git commit -m "feat(v3): remote issue links domain and endpoints"
```

---

### Task 10: Seed commenti demo + gap report + suite

**Files:** Modify `cmd/seed/main.go`, `docs/contracts/gap-report.md`

- [ ] **Step 1: seed commenti** — dopo la creazione delle issue DEMO, aggiungi (idempotente) 1-2 commenti ADF a DEMO-1 via `commentSvc.AddComment(issueID, admin.ID, adfJSON)`. Costruisci l'ADF con `adf.FromText("...")` marshalizzato, o una stringa ADF letterale. Guardia idempotente: crea i commenti solo se `commentSvc.GetComments(demo1.ID)` è vuoto. Serve accesso a `issue.NewCommentService(s.DB)` e all'ID della issue DEMO-1 (query per key "DEMO-1").

- [ ] **Step 2: verifica idempotenza** — due run del seed; la seconda non duplica i commenti; exit 0.

- [ ] **Step 3: suite + gap report**

Run: `go build ./... && go vet ./... && go test ./... -count=1 2>&1 | grep -c "^FAIL"` → 0. Poi `go run ./cmd/gapreport` → `matched` aumenta (comment, worklog, votes, watchers, issueLink, changelog, remotelink).

- [ ] **Step 4: commit**

```bash
git add cmd/seed/main.go docs/contracts/gap-report.md
git commit -m "feat(seed): demo comments; regenerate gap report"
```

---

### Task 11: Frontend — tipi collaborazione e chiamate API

**Files:** Modify `frontend-next/lib/api.ts`

- [ ] **Step 1: tipi + chiamate** — aggiungi:
```ts
export interface Comment {
  self: string;
  id: string;
  author: JiraUserRef | null;
  body: ADFNode | null;
  created: string;
  updated: string;
}
export interface PageOfComments { startAt: number; maxResults: number; total: number; comments: Comment[]; }

export const comments = {
  list: (issueKey: string) => apiFetch<PageOfComments>(`/rest/api/3/issue/${issueKey}/comment`),
  add: (issueKey: string, body: ADFNode) =>
    apiFetch<Comment>(`/rest/api/3/issue/${issueKey}/comment`, { method: "POST", body: JSON.stringify({ body }) }),
  del: (issueKey: string, id: string) =>
    apiFetch<void>(`/rest/api/3/issue/${issueKey}/comment/${id}`, { method: "DELETE" }),
};

export interface Watchers { self: string; isWatching: boolean; watchCount: number; watchers: JiraUserRef[]; }
export interface Votes { self: string; votes: number; hasVoted: boolean; voters: JiraUserRef[]; }

export const watchers = {
  get: (issueKey: string) => apiFetch<Watchers>(`/rest/api/3/issue/${issueKey}/watchers`),
  watch: (issueKey: string) => apiFetch<void>(`/rest/api/3/issue/${issueKey}/watchers`, { method: "POST" }),
  unwatch: (issueKey: string) => apiFetch<void>(`/rest/api/3/issue/${issueKey}/watchers`, { method: "DELETE" }),
};
export const votes = {
  get: (issueKey: string) => apiFetch<Votes>(`/rest/api/3/issue/${issueKey}/votes`),
  vote: (issueKey: string) => apiFetch<void>(`/rest/api/3/issue/${issueKey}/votes`, { method: "POST" }),
  unvote: (issueKey: string) => apiFetch<void>(`/rest/api/3/issue/${issueKey}/votes`, { method: "DELETE" }),
};

export function textToADF(text: string): ADFNode {
  return { type: "doc", version: 1, content: [{ type: "paragraph", content: [{ type: "text", text }] }] };
}
```
Riusa `JiraUserRef`, `ADFNode`, `apiFetch`. Non toccare gli altri export.

- [ ] **Step 2: verifica** `cd frontend-next && npm run build` → clean.

- [ ] **Step 3: commit**

```bash
git add frontend-next/lib/api.ts
git commit -m "feat(frontend): collaboration API types and calls (comments, watchers, votes)"
```

---

### Task 12: Frontend — sezione commenti nella vista issue

**Files:** Create `frontend-next/components/issues/Comments.tsx`; Modify `frontend-next/components/issues/IssueView.tsx`

- [ ] **Step 1: componente Comments** — lista commenti (autore + data + body renderizzato con `AdfRenderer`) e una textarea + bottone "Add" che chiama `comments.add(issueKey, textToADF(text))` via `useMutation`, invalidando `["comments", issueKey]`. Struttura:
```tsx
"use client";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { comments, textToADF } from "@/lib/api";
import { AdfRenderer } from "./adf";

export function Comments({ issueKey }: { issueKey: string }) {
  const qc = useQueryClient();
  const { data } = useQuery({ queryKey: ["comments", issueKey], queryFn: () => comments.list(issueKey) });
  const [text, setText] = useState("");
  const add = useMutation({
    mutationFn: () => comments.add(issueKey, textToADF(text)),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["comments", issueKey] }); setText(""); },
  });
  const list = data?.comments ?? [];
  return (
    <section className="mt-8">
      <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">Comments</h2>
      <div className="space-y-4">
        {list.map((c) => (
          <div key={c.id} className="rounded-lg border border-slate-200 p-3">
            <div className="mb-1 text-sm font-semibold text-[#1a1f36]">{c.author?.displayName ?? "Unknown"} <span className="ml-2 text-xs font-normal text-slate-400">{c.created?.slice(0, 10)}</span></div>
            <AdfRenderer doc={c.body} />
          </div>
        ))}
        {list.length === 0 && <p className="text-sm text-slate-400">No comments yet.</p>}
      </div>
      <div className="mt-4">
        <textarea value={text} onChange={(e) => setText(e.target.value)} placeholder="Add a comment…" rows={3}
          className="w-full rounded border border-slate-300 px-3 py-2" aria-label="Add a comment" />
        <button onClick={() => add.mutate()} disabled={!text.trim() || add.isPending}
          className="mt-2 rounded bg-[#0052cc] px-4 py-2 font-semibold text-white disabled:opacity-60">Add comment</button>
      </div>
    </section>
  );
}
```

- [ ] **Step 2: monta Comments in IssueView** — sotto la descrizione, `<Comments issueKey={issue.key} />`. Import in cima.

- [ ] **Step 3: verifica** `cd frontend-next && npm run build` → clean.

- [ ] **Step 4: commit**

```bash
git add frontend-next/components/issues/Comments.tsx frontend-next/components/issues/IssueView.tsx
git commit -m "feat(frontend): comments section on issue view"
```

---

### Task 13: Frontend — toggle watch/vote nella vista issue

**Files:** Modify `frontend-next/components/issues/IssueView.tsx`

- [ ] **Step 1: sidebar watch/vote** — nella sidebar della IssueView aggiungi due controlli:
```tsx
import { watchers, votes } from "@/lib/api";
// dentro il componente:
const { data: w } = useQuery({ queryKey: ["watchers", issueKey], queryFn: () => watchers.get(issueKey) });
const { data: v } = useQuery({ queryKey: ["votes", issueKey], queryFn: () => votes.get(issueKey) });
const toggleWatch = useMutation({
  mutationFn: () => (w?.isWatching ? watchers.unwatch(issueKey) : watchers.watch(issueKey)),
  onSuccess: () => qc.invalidateQueries({ queryKey: ["watchers", issueKey] }),
});
const toggleVote = useMutation({
  mutationFn: () => (v?.hasVoted ? votes.unvote(issueKey) : votes.vote(issueKey)),
  onSuccess: () => qc.invalidateQueries({ queryKey: ["votes", issueKey] }),
});
// nella sidebar:
<button onClick={() => toggleWatch.mutate()} className="w-full rounded border border-slate-300 px-3 py-2 text-sm">
  {w?.isWatching ? "Stop watching" : "Watch"} ({w?.watchCount ?? 0})
</button>
<button onClick={() => toggleVote.mutate()} className="mt-2 w-full rounded border border-slate-300 px-3 py-2 text-sm">
  {v?.hasVoted ? "Unvote" : "Vote"} ({v?.votes ?? 0})
</button>
```
`qc` è già disponibile dal Round 2 (useQueryClient in IssueView). Mantieni gli hook sopra gli early return (Rules of Hooks).

- [ ] **Step 2: verifica** `cd frontend-next && npm run build` → clean.

- [ ] **Step 3: commit**

```bash
git add frontend-next/components/issues/IssueView.tsx
git commit -m "feat(frontend): watch and vote toggles on issue view"
```

---

### Task 14: E2E collaborazione

**Files:** Create `frontend-next/e2e/collaboration.spec.ts`

- [ ] **Step 1: spec** — login, vai a `/jira/browse/DEMO-1`, verifica la sezione Comments (i commenti seedati sono visibili), aggiungi un commento e verifica che compaia; clicca Watch e verifica il conteggio cambia; clicca Vote e verifica.
```ts
import { test, expect } from "@playwright/test";

async function login(page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira\/projects/);
}

test("aggiunge un commento e lo vede", async ({ page }) => {
  await login(page);
  await page.goto("/jira/browse/DEMO-1");
  await expect(page.getByRole("heading", { name: /comments/i })).toBeVisible();
  await page.getByLabel(/add a comment/i).fill("Commento E2E");
  await page.getByRole("button", { name: /add comment/i }).click();
  await expect(page.getByText("Commento E2E")).toBeVisible();
});

test("watch e vote toggle", async ({ page }) => {
  await login(page);
  await page.goto("/jira/browse/DEMO-2");
  const watchBtn = page.getByRole("button", { name: /watch/i });
  await watchBtn.click();
  await expect(page.getByRole("button", { name: /stop watching/i })).toBeVisible();
  const voteBtn = page.getByRole("button", { name: /^vote/i });
  await voteBtn.click();
  await expect(page.getByRole("button", { name: /unvote/i })).toBeVisible();
});
```
Adatta i selettori dopo aver letto i componenti reali. Il seed deve avere commenti su DEMO-1 (Task 10).

- [ ] **Step 2: run** `cd frontend-next && npx playwright test e2e/collaboration.spec.ts` → 2 passed. Se la porta 8080 è occupata, verifica su porta alternativa e ripristina la config (come nei round precedenti).

- [ ] **Step 3: commit**

```bash
git add frontend-next/e2e/collaboration.spec.ts
git commit -m "test(e2e): comments, watch and vote flows"
```

---

### Task 15: Verifica finale del round

- [ ] **Step 1: suite completa** `go build ./... && go vet ./... && go test ./... -count=1 2>&1 | grep -c "^FAIL"` → 0.
- [ ] **Step 2: gap report pulito** `go run ./cmd/gapreport && git diff --exit-code docs/contracts/gap-report.md` → clean (già committato al Task 10; qui solo conferma).
- [ ] **Step 3: frontend** `cd frontend-next && npm run build` → clean; `npx playwright test` → tutti verdi.
- [ ] **Step 4: aggiorna STATE.md** — segna Round 3 completo e imposta Round 4 (Ricerca & JQL) come prossimo. Commit.

```bash
git add docs/superpowers/STATE.md
git commit -m "docs: mark Round 3 complete, Round 4 (Search & JQL) next"
```

---

## Definition of Done del Round 3

- `go build ./... && go vet ./... && go test ./...` verdi.
- Contract test verdi per: `POST/GET /issue/{k}/comment`, `POST/GET /issue/{k}/worklog`, `GET/POST/DELETE /issue/{k}/votes`, `GET /issue/{k}/watchers`, `POST /issueLink`, `GET /issue/{k}/changelog`, `GET /issue/{k}/remotelink`.
- Commenti con body ADF e @menzioni estratte per le notifiche; worklog con `timeSpent`/`timeSpentSeconds`; votes/watchers con conteggi e stato utente; changelog come PageBeanChangelog.
- `docs/contracts/gap-report.md` rigenerato e committato.
- Frontend: vista issue con sezione commenti, toggle watch/vote; build pulito; E2E verdi.

## Note e follow-up

- `self` di commenti/worklog usa l'UUID interno della issue anziché il seqid/key (schema-valido perché `format:uri`); fedeltà migliorabile.
- Notifiche di @menzione: best-effort in questo round; integrazione completa con il notification service se non banale è un follow-up.
- Editor ADF ricco (TipTap) con autocomplete @menzioni: rimandato (qui commento = testo semplice → ADF).
- `RemoteIssueLink.id` intero derivato/hash: valutare una colonna seq dedicata se serve stabilità garantita.
- Attachments (upload file) NON in questo round — sono nella spec 🟡 ma richiedono storage; pianificare separatamente se serve.
- Follow-up dei round precedenti ancora aperti: reporter alla creazione (#51), harness status non documentati (#49), dead code ListStatuses (#50), favourite/star (#32), workflow/stati default sui progetti seedati.
