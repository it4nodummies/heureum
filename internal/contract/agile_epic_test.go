package contract

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/it4nodummies/heureum/internal/domain/issue"
)

// Round 28 — the agile Epic API (jira-agile-1.0): GET/POST /epic/{id},
// GET/POST /epic/{id}/issue, GET/POST /epic/none/issue, PUT /epic/{id}/rank,
// GET /board/{id}/epic (with done filter), GET /board/{id}/epic/{id}/issue and
// GET /board/{id}/epic/none/issue. The spec's epic 200 schemas are free-form
// ({}), so beyond ValidateResponse (route exists, status allowed) the tests pin
// the shape we promise: {id, key, self, name, summary, color{key}, done}.

// epicGet GETs an epic (validated against the spec) and returns its body.
func epicGet(t *testing.T, srv *httptest.Server, jwt, idOrKey string) map[string]any {
	t.Helper()
	res := doJSON(t, srv, http.MethodGet, jwt, "/rest/agile/1.0/epic/"+idOrKey, nil)
	body, raw := decodeBody(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /epic/%s: status %d body %s", idOrKey, res.StatusCode, raw)
	}
	v := MustLoad(t, agileSpecPath)
	if err := v.ValidateResponse(http.MethodGet, "/rest/agile/1.0/epic/{epicIdOrKey}", res.StatusCode, res.Header, bytesReader(raw)); err != nil {
		t.Errorf("GET /epic/{id} non conforme: %v", err)
	}
	return body
}

// epicPost POSTs a partial update to an epic and returns the raw response.
func epicPost(t *testing.T, srv *httptest.Server, jwt, idOrKey string, body map[string]any) *http.Response {
	t.Helper()
	return doJSON(t, srv, http.MethodPost, jwt, "/rest/agile/1.0/epic/"+idOrKey, body)
}

// mustEpicPost is epicPost asserting 200 (validated) and returning the body.
func mustEpicPost(t *testing.T, srv *httptest.Server, jwt, idOrKey string, body map[string]any) map[string]any {
	t.Helper()
	res := epicPost(t, srv, jwt, idOrKey, body)
	out, raw := decodeBody(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("POST /epic/%s %v: status %d body %s", idOrKey, body, res.StatusCode, raw)
	}
	v := MustLoad(t, agileSpecPath)
	if err := v.ValidateResponse(http.MethodPost, "/rest/agile/1.0/epic/{epicIdOrKey}", res.StatusCode, res.Header, bytesReader(raw)); err != nil {
		t.Errorf("POST /epic/{id} non conforme: %v", err)
	}
	return out
}

// expectStatus asserts the status code and drains the body (which may be
// empty: 204 responses).
func expectStatus(t *testing.T, what string, res *http.Response, want int) {
	t.Helper()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		t.Errorf("%s: status %d, want %d (body %s)", what, res.StatusCode, want, raw)
	}
}

// issuePage GETs a paginated issue list (validated against specPath) and
// returns the keys in order plus the reported total.
func issuePage(t *testing.T, srv *httptest.Server, jwt, path, specPath string) ([]string, int) {
	t.Helper()
	res := doJSON(t, srv, http.MethodGet, jwt, path, nil)
	body, raw := decodeBody(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d body %s", path, res.StatusCode, raw)
	}
	v := MustLoad(t, agileSpecPath)
	if err := v.ValidateResponse(http.MethodGet, specPath, res.StatusCode, res.Header, bytesReader(raw)); err != nil {
		t.Errorf("GET %s non conforme: %v", specPath, err)
	}
	items, _ := body["issues"].([]any)
	keys := make([]string, 0, len(items))
	for _, it := range items {
		m, _ := it.(map[string]any)
		k, _ := m["key"].(string)
		keys = append(keys, k)
	}
	total, _ := body["total"].(float64)
	return keys, int(total)
}

// parentKeyOf returns fields.parent.key of the issue, or "" when it has none.
func parentKeyOf(t *testing.T, srv *httptest.Server, jwt, key string) string {
	t.Helper()
	p, _ := getIssueFields(t, srv, jwt, key)["parent"].(map[string]any)
	if p == nil {
		return ""
	}
	k, _ := p["key"].(string)
	return k
}

// statusCategoryOf returns fields.status.statusCategory.key of the issue.
func statusCategoryOf(t *testing.T, srv *httptest.Server, jwt, key string) string {
	t.Helper()
	st, _ := getIssueFields(t, srv, jwt, key)["status"].(map[string]any)
	cat, _ := st["statusCategory"].(map[string]any)
	k, _ := cat["key"].(string)
	return k
}

// transitionTo moves the issue to the named status through the v3 transitions
// endpoint (the status must be reachable by one available transition).
func transitionTo(t *testing.T, srv *httptest.Server, jwt, key, statusName string) {
	t.Helper()
	res := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/issue/"+key+"/transitions", nil)
	body, raw := decodeBody(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET transitions %s: status %d body %s", key, res.StatusCode, raw)
	}
	trs, _ := body["transitions"].([]any)
	for _, tr := range trs {
		m, _ := tr.(map[string]any)
		to, _ := m["to"].(map[string]any)
		if strings.EqualFold(to["name"].(string), statusName) {
			res := doJSON(t, srv, http.MethodPost, jwt, "/rest/api/3/issue/"+key+"/transitions", map[string]any{
				"transition": map[string]any{"id": m["id"]},
			})
			expectStatus(t, "transition "+key+" to "+statusName, res, http.StatusNoContent)
			return
		}
	}
	t.Fatalf("no transition from %s to %q in %s", key, statusName, raw)
}

func contains(keys []string, k string) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}

func TestAgileEpic_GetAndUpdate(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "EPX", "Epic X")

	epic, epicID := mustCreateTypedIssue(t, srv, jwt, "EPX", "Epic one", "Epic", nil)
	story, _ := mustCreateTypedIssue(t, srv, jwt, "EPX", "Story", "Story", nil)

	// GET by key and by id: the promised shape, colour assigned at creation,
	// done computed from the status category (TO DO → false).
	for _, ref := range []string{epic, epicID} {
		e := epicGet(t, srv, jwt, ref)
		if id, _ := e["id"].(float64); strconv.FormatInt(int64(id), 10) != epicID {
			t.Errorf("GET /epic/%s id = %v, want %s", ref, e["id"], epicID)
		}
		if e["key"] != epic || e["name"] != "Epic one" || e["summary"] != "Epic one" {
			t.Errorf("GET /epic/%s key/name/summary = %v/%v/%v", ref, e["key"], e["name"], e["summary"])
		}
		if self, _ := e["self"].(string); !strings.HasSuffix(self, "/rest/agile/1.0/epic/"+epic) {
			t.Errorf("GET /epic/%s self = %q", ref, self)
		}
		color, _ := e["color"].(map[string]any)
		if k, _ := color["key"].(string); !issue.IsEpicColorKey(k) {
			t.Errorf("GET /epic/%s color = %v, want one of %v", ref, e["color"], issue.EpicColorKeys)
		}
		if e["done"] != false {
			t.Errorf("GET /epic/%s done = %v, want false", ref, e["done"])
		}
	}

	// A story is not an epic: 404, same as an unknown key.
	expectStatus(t, "GET /epic/{story}", doJSON(t, srv, http.MethodGet, jwt, "/rest/agile/1.0/epic/"+story, nil), http.StatusNotFound)
	expectStatus(t, "GET /epic/unknown", doJSON(t, srv, http.MethodGet, jwt, "/rest/agile/1.0/epic/EPX-999", nil), http.StatusNotFound)

	// Colour + name: reflected in the response and persisted.
	e := mustEpicPost(t, srv, jwt, epic, map[string]any{"color": map[string]any{"key": "color_5"}, "name": "Epic renamed"})
	if c, _ := e["color"].(map[string]any); c["key"] != "color_5" {
		t.Errorf("POST colour: color = %v, want color_5", e["color"])
	}
	if e["name"] != "Epic renamed" || e["summary"] != "Epic renamed" {
		t.Errorf("POST name: name/summary = %v/%v, want both 'Epic renamed' (one title)", e["name"], e["summary"])
	}
	again := epicGet(t, srv, jwt, epic)
	if c, _ := again["color"].(map[string]any); c["key"] != "color_5" || again["summary"] != "Epic renamed" {
		t.Errorf("POST not persisted: %v", again)
	}
	// summary alone updates the title too.
	e = mustEpicPost(t, srv, jwt, epic, map[string]any{"summary": "Epic summary"})
	if e["name"] != "Epic summary" {
		t.Errorf("POST summary: name = %v, want 'Epic summary'", e["name"])
	}
	if got := getIssueFields(t, srv, jwt, epic)["summary"]; got != "Epic summary" {
		t.Errorf("issue summary after POST /epic = %v", got)
	}

	// Invalid colour key / empty title → 400, nothing changed.
	expectStatus(t, "POST bad colour", epicPost(t, srv, jwt, epic, map[string]any{"color": map[string]any{"key": "color_99"}}), http.StatusBadRequest)
	expectStatus(t, "POST non-palette colour", epicPost(t, srv, jwt, epic, map[string]any{"color": map[string]any{"key": "blue"}}), http.StatusBadRequest)
	expectStatus(t, "POST empty summary", epicPost(t, srv, jwt, epic, map[string]any{"summary": ""}), http.StatusBadRequest)
	expectStatus(t, "POST on a story", epicPost(t, srv, jwt, story, map[string]any{"name": "x"}), http.StatusNotFound)
	if c, _ := epicGet(t, srv, jwt, epic)["color"].(map[string]any); c["key"] != "color_5" {
		t.Errorf("colour changed by a rejected POST: %v", c)
	}

	// done is idempotent: done:false on an epic already in a todo-category
	// status is a no-op 200 (nothing transitions, the other fields still
	// apply), exactly like done:true on one already done (asserted below).
	e = mustEpicPost(t, srv, jwt, epic, map[string]any{"done": false, "name": "Epic still todo"})
	if e["done"] != false || e["name"] != "Epic still todo" {
		t.Errorf("POST done:false on a TO DO epic: done/name = %v/%v, want false/'Epic still todo'", e["done"], e["name"])
	}
	if cat := statusCategoryOf(t, srv, jwt, epic); cat != "new" {
		t.Errorf("status category after no-op done:false = %q, want new", cat)
	}

	// done:true from TO DO: the default workflow only reaches IN PROGRESS in
	// one transition → 409, status untouched.
	expectStatus(t, "POST done:true unreachable", epicPost(t, srv, jwt, epic, map[string]any{"done": true}), http.StatusConflict)
	if cat := statusCategoryOf(t, srv, jwt, epic); cat != "new" {
		t.Errorf("status category after 409 = %q, want new", cat)
	}
	// From TO BE TESTED, DONE is one available transition away.
	transitionTo(t, srv, jwt, epic, "IN PROGRESS")
	transitionTo(t, srv, jwt, epic, "TO BE TESTED")
	e = mustEpicPost(t, srv, jwt, epic, map[string]any{"done": true})
	if e["done"] != true {
		t.Errorf("POST done:true: done = %v", e["done"])
	}
	if cat := statusCategoryOf(t, srv, jwt, epic); cat != "done" {
		t.Errorf("status category after done:true = %q, want done", cat)
	}
	// Already done: no-op 200, the status stays in the done category.
	if e := mustEpicPost(t, srv, jwt, epic, map[string]any{"done": true}); e["done"] != true {
		t.Errorf("POST done:true twice: done = %v", e["done"])
	}
	if cat := statusCategoryOf(t, srv, jwt, epic); cat != "done" {
		t.Errorf("status category after no-op done:true = %q, want done", cat)
	}
	// DONE has no outgoing transition: done:false → 409.
	expectStatus(t, "POST done:false from DONE", epicPost(t, srv, jwt, epic, map[string]any{"done": false}), http.StatusConflict)

	// done:false from IN PROGRESS reaches TO DO.
	epic2, _ := mustCreateTypedIssue(t, srv, jwt, "EPX", "Epic two", "Epic", nil)
	transitionTo(t, srv, jwt, epic2, "IN PROGRESS")
	if e := mustEpicPost(t, srv, jwt, epic2, map[string]any{"done": false}); e["done"] != false {
		t.Errorf("POST done:false: done = %v", e["done"])
	}
	if cat := statusCategoryOf(t, srv, jwt, epic2); cat != "new" {
		t.Errorf("status category after done:false = %q, want new", cat)
	}
}

func TestAgileEpic_IssuesMoveAndRemove(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "EPY", "Epic Y")
	createProjectViaAPI(t, srv, jwt, "OTH", "Other")

	epic1, _ := mustCreateTypedIssue(t, srv, jwt, "EPY", "Epic 1", "Epic", nil)
	epic2, _ := mustCreateTypedIssue(t, srv, jwt, "EPY", "Epic 2", "Epic", nil)
	s1, _ := mustCreateTypedIssue(t, srv, jwt, "EPY", "S1", "Story", nil)
	s2, s2ID := mustCreateTypedIssue(t, srv, jwt, "EPY", "S2", "Task", nil)
	s3, _ := mustCreateTypedIssue(t, srv, jwt, "EPY", "S3", "Bug", nil)
	sub, _ := mustCreateTypedIssue(t, srv, jwt, "EPY", "Sub of S3", "Subtask", map[string]any{"parent": map[string]any{"key": s3}})
	other, _ := mustCreateTypedIssue(t, srv, jwt, "OTH", "Other story", "Story", nil)

	const epicIssuesSpec = "/rest/agile/1.0/epic/{epicIdOrKey}/issue"
	if keys, total := issuePage(t, srv, jwt, "/rest/agile/1.0/epic/"+epic1+"/issue", epicIssuesSpec); total != 0 || len(keys) != 0 {
		t.Errorf("fresh epic children = %v (total %d), want none", keys, total)
	}
	expectStatus(t, "GET /epic/{story}/issue", doJSON(t, srv, http.MethodGet, jwt, "/rest/agile/1.0/epic/"+s1+"/issue", nil), http.StatusNotFound)

	// Move two level-0 issues (by key and by id) → 204, parents set.
	res := doJSON(t, srv, http.MethodPost, jwt, "/rest/agile/1.0/epic/"+epic1+"/issue", map[string]any{"issues": []string{s1, s2ID}})
	expectStatus(t, "POST /epic/{id}/issue", res, http.StatusNoContent)
	if p := parentKeyOf(t, srv, jwt, s1); p != epic1 {
		t.Errorf("S1 parent = %q, want %s", p, epic1)
	}
	if p := parentKeyOf(t, srv, jwt, s2); p != epic1 {
		t.Errorf("S2 parent = %q, want %s", p, epic1)
	}
	keys, total := issuePage(t, srv, jwt, "/rest/agile/1.0/epic/"+epic1+"/issue", epicIssuesSpec)
	if total != 2 || !contains(keys, s1) || !contains(keys, s2) {
		t.Errorf("epic children = %v (total %d), want S1,S2", keys, total)
	}
	// Pagination.
	if keys, total := issuePage(t, srv, jwt, "/rest/agile/1.0/epic/"+epic1+"/issue?maxResults=1", epicIssuesSpec); total != 2 || len(keys) != 1 {
		t.Errorf("page maxResults=1: %v (total %d)", keys, total)
	}
	if keys, total := issuePage(t, srv, jwt, "/rest/agile/1.0/epic/"+epic1+"/issue?startAt=1&maxResults=5", epicIssuesSpec); total != 2 || len(keys) != 1 {
		t.Errorf("page startAt=1: %v (total %d)", keys, total)
	}

	// One bad issue in the batch → 400 and NOTHING moved.
	for name, bad := range map[string]string{
		"subtask":       sub,
		"other project": other,
		"unknown":       "EPY-9999",
		"epic":          epic1,
	} {
		res := doJSON(t, srv, http.MethodPost, jwt, "/rest/agile/1.0/epic/"+epic2+"/issue", map[string]any{"issues": []string{s3, bad}})
		expectStatus(t, "POST /epic/{id}/issue with "+name, res, http.StatusBadRequest)
		if p := parentKeyOf(t, srv, jwt, s3); p != "" {
			t.Errorf("partial write (%s): S3 parent = %q", name, p)
		}
	}
	if _, total := issuePage(t, srv, jwt, "/rest/agile/1.0/epic/"+epic2+"/issue", epicIssuesSpec); total != 0 {
		t.Errorf("epic2 children after rejected batches = %d, want 0", total)
	}
	expectStatus(t, "POST /epic/{id}/issue empty", doJSON(t, srv, http.MethodPost, jwt, "/rest/agile/1.0/epic/"+epic2+"/issue", map[string]any{"issues": []string{}}), http.StatusBadRequest)

	// Orphans: level-0 issues with no parent, across the caller's projects;
	// never epics, subtasks or parented issues.
	const noneSpec = "/rest/agile/1.0/epic/none/issue"
	keys, _ = issuePage(t, srv, jwt, noneSpec, noneSpec)
	for _, want := range []string{s3, other} {
		if !contains(keys, want) {
			t.Errorf("/epic/none/issue missing orphan %s: %v", want, keys)
		}
	}
	for _, notWant := range []string{s1, s2, sub, epic1, epic2} {
		if contains(keys, notWant) {
			t.Errorf("/epic/none/issue must not list %s: %v", notWant, keys)
		}
	}
	// jql narrows it; invalid jql → 400.
	keys, total = issuePage(t, srv, jwt, noneSpec+"?jql="+"project%3DEPY", noneSpec)
	if total != 1 || !contains(keys, s3) {
		t.Errorf("/epic/none/issue?jql=project=EPY = %v (total %d), want [S3]", keys, total)
	}
	expectStatus(t, "GET /epic/none/issue bad jql", doJSON(t, srv, http.MethodGet, jwt, noneSpec+"?jql=project%3D%3D", nil), http.StatusBadRequest)

	// Remove from epic: parent cleared; subtask → 400; unknown → 404.
	res = doJSON(t, srv, http.MethodPost, jwt, noneSpec, map[string]any{"issues": []string{s1}})
	expectStatus(t, "POST /epic/none/issue", res, http.StatusNoContent)
	if p := parentKeyOf(t, srv, jwt, s1); p != "" {
		t.Errorf("S1 parent after remove = %q, want none", p)
	}
	if _, total := issuePage(t, srv, jwt, "/rest/agile/1.0/epic/"+epic1+"/issue", epicIssuesSpec); total != 1 {
		t.Errorf("epic1 children after remove = %d, want 1", total)
	}
	expectStatus(t, "POST /epic/none/issue subtask", doJSON(t, srv, http.MethodPost, jwt, noneSpec, map[string]any{"issues": []string{s2, sub}}), http.StatusBadRequest)
	if p := parentKeyOf(t, srv, jwt, s2); p != epic1 {
		t.Errorf("partial write: S2 parent = %q after rejected batch", p)
	}
	if p := parentKeyOf(t, srv, jwt, sub); p != s3 {
		t.Errorf("subtask parent touched: %q", p)
	}
	expectStatus(t, "POST /epic/none/issue unknown", doJSON(t, srv, http.MethodPost, jwt, noneSpec, map[string]any{"issues": []string{"EPY-9999"}}), http.StatusNotFound)
}

func TestAgileEpic_RankAndBoard(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "EPZ", "Epic Z")
	createProjectViaAPI(t, srv, jwt, "OTZ", "Other Z")
	v := MustLoad(t, agileSpecPath)

	e1, e1ID := mustCreateTypedIssue(t, srv, jwt, "EPZ", "E1", "Epic", nil)
	e2, _ := mustCreateTypedIssue(t, srv, jwt, "EPZ", "E2", "Epic", nil)
	e3, _ := mustCreateTypedIssue(t, srv, jwt, "EPZ", "E3", "Epic", nil)
	s1, _ := mustCreateTypedIssue(t, srv, jwt, "EPZ", "S1", "Story", map[string]any{"parent": map[string]any{"key": e1}})
	s2, _ := mustCreateTypedIssue(t, srv, jwt, "EPZ", "S2", "Story", nil)
	_, otherEpicID := mustCreateTypedIssue(t, srv, jwt, "OTZ", "Other epic", "Epic", nil)

	res := doJSON(t, srv, http.MethodPost, jwt, "/rest/agile/1.0/board", map[string]any{"name": "Z", "type": "scrum", "projectKeyOrId": "EPZ"})
	board, _ := decodeBody(t, res)
	boardID := int64(board["id"].(float64))
	boardPath := agilePath("/rest/agile/1.0/board/", boardID, "")

	// boardEpics returns the epic keys in order (validated), with optional query.
	boardEpics := func(query string) ([]string, []map[string]any) {
		t.Helper()
		res := doJSON(t, srv, http.MethodGet, jwt, boardPath+"/epic"+query, nil)
		body, raw := decodeBody(t, res)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET /board/{id}/epic%s: status %d body %s", query, res.StatusCode, raw)
		}
		if err := v.ValidateResponse(http.MethodGet, "/rest/agile/1.0/board/{boardId}/epic", res.StatusCode, res.Header, bytesReader(raw)); err != nil {
			t.Errorf("GET /board/{id}/epic non conforme: %v", err)
		}
		values, _ := body["values"].([]any)
		keys := make([]string, 0, len(values))
		maps := make([]map[string]any, 0, len(values))
		for _, it := range values {
			m, _ := it.(map[string]any)
			keys = append(keys, m["key"].(string))
			maps = append(maps, m)
		}
		return keys, maps
	}

	keys, maps := boardEpics("")
	if strings.Join(keys, ",") != e1+","+e2+","+e3 {
		t.Errorf("board epics = %v, want [E1 E2 E3]", keys)
	}
	for _, m := range maps {
		color, _ := m["color"].(map[string]any)
		if k, _ := color["key"].(string); !issue.IsEpicColorKey(k) {
			t.Errorf("board epic %v colour = %v", m["key"], m["color"])
		}
		if self, _ := m["self"].(string); !strings.HasSuffix(self, "/rest/agile/1.0/epic/"+m["key"].(string)) {
			t.Errorf("board epic %v self = %v", m["key"], m["self"])
		}
		if m["done"] != false {
			t.Errorf("board epic %v done = %v, want false", m["key"], m["done"])
		}
	}

	// Rank E3 before E1 → E3, E1, E2.
	res = doJSON(t, srv, http.MethodPut, jwt, "/rest/agile/1.0/epic/"+e3+"/rank", map[string]any{"rankBeforeEpic": e1})
	expectStatus(t, "PUT /epic/{id}/rank before", res, http.StatusNoContent)
	if keys, _ := boardEpics(""); strings.Join(keys, ",") != e3+","+e1+","+e2 {
		t.Errorf("after rank before: %v, want [E3 E1 E2]", keys)
	}
	// Rank E3 after E2 (by id) → E1, E2, E3.
	res = doJSON(t, srv, http.MethodPut, jwt, "/rest/agile/1.0/epic/"+e3+"/rank", map[string]any{"rankAfterEpic": e2, "rankCustomFieldId": 10019})
	expectStatus(t, "PUT /epic/{id}/rank after", res, http.StatusNoContent)
	if keys, _ := boardEpics(""); strings.Join(keys, ",") != e1+","+e2+","+e3 {
		t.Errorf("after rank after: %v, want [E1 E2 E3]", keys)
	}
	// Into the middle: E1 immediately before E3 lands between E2 and E3
	// (not on E2's position) → E2, E1, E3.
	res = doJSON(t, srv, http.MethodPut, jwt, "/rest/agile/1.0/epic/"+e1+"/rank", map[string]any{"rankBeforeEpic": e3})
	expectStatus(t, "PUT /epic/{id}/rank into the middle", res, http.StatusNoContent)
	if keys, _ := boardEpics(""); strings.Join(keys, ",") != e2+","+e1+","+e3 {
		t.Errorf("after rank into the middle: %v, want [E2 E1 E3]", keys)
	}
	// Anchor must be an epic of the same project; exactly one anchor.
	expectStatus(t, "rank anchor story", doJSON(t, srv, http.MethodPut, jwt, "/rest/agile/1.0/epic/"+e1+"/rank", map[string]any{"rankAfterEpic": s1}), http.StatusBadRequest)
	expectStatus(t, "rank anchor other project", doJSON(t, srv, http.MethodPut, jwt, "/rest/agile/1.0/epic/"+e1+"/rank", map[string]any{"rankAfterEpic": otherEpicID}), http.StatusBadRequest)
	expectStatus(t, "rank both anchors", doJSON(t, srv, http.MethodPut, jwt, "/rest/agile/1.0/epic/"+e1+"/rank", map[string]any{"rankAfterEpic": e2, "rankBeforeEpic": e3}), http.StatusBadRequest)
	expectStatus(t, "rank no anchor", doJSON(t, srv, http.MethodPut, jwt, "/rest/agile/1.0/epic/"+e1+"/rank", map[string]any{}), http.StatusBadRequest)
	expectStatus(t, "rank a story", doJSON(t, srv, http.MethodPut, jwt, "/rest/agile/1.0/epic/"+s1+"/rank", map[string]any{"rankAfterEpic": e2}), http.StatusNotFound)
	if keys, _ := boardEpics(""); strings.Join(keys, ",") != e2+","+e1+","+e3 {
		t.Errorf("order changed by a rejected rank: %v", keys)
	}

	// done filter: E2 → DONE, then ?done=false hides it, ?done=true shows only it.
	transitionTo(t, srv, jwt, e2, "IN PROGRESS")
	transitionTo(t, srv, jwt, e2, "TO BE TESTED")
	mustEpicPost(t, srv, jwt, e2, map[string]any{"done": true})
	if keys, _ := boardEpics("?done=false"); strings.Join(keys, ",") != e1+","+e3 {
		t.Errorf("?done=false = %v, want [E1 E3]", keys)
	}
	if keys, maps := boardEpics("?done=true"); strings.Join(keys, ",") != e2 || maps[0]["done"] != true {
		t.Errorf("?done=true = %v (%v), want [E2] done", keys, maps)
	}
	if keys, _ := boardEpics(""); len(keys) != 3 {
		t.Errorf("unfiltered = %v, want 3", keys)
	}
	expectStatus(t, "?done=maybe", doJSON(t, srv, http.MethodGet, jwt, boardPath+"/epic?done=maybe", nil), http.StatusBadRequest)

	// Board-scoped children / orphans.
	keys, total := issuePage(t, srv, jwt, boardPath+"/epic/"+e1ID+"/issue", "/rest/agile/1.0/board/{boardId}/epic/{epicId}/issue")
	if total != 1 || !contains(keys, s1) {
		t.Errorf("board epic issues = %v (total %d), want [S1]", keys, total)
	}
	keys, total = issuePage(t, srv, jwt, boardPath+"/epic/none/issue", "/rest/agile/1.0/board/{boardId}/epic/none/issue")
	if total != 1 || !contains(keys, s2) {
		t.Errorf("board no-epic issues = %v (total %d), want [S2]", keys, total)
	}
	expectStatus(t, "board epic of another project", doJSON(t, srv, http.MethodGet, jwt, boardPath+"/epic/"+otherEpicID+"/issue", nil), http.StatusNotFound)
	expectStatus(t, "board epic that is a story", doJSON(t, srv, http.MethodGet, jwt, boardPath+"/epic/"+s1+"/issue", nil), http.StatusNotFound)
}

// TestAgileEpicNone_ScopedToMemberProjects: GET /epic/none/issue has no path
// context, so its scope is the caller's project memberships (the same as
// /search/jql): a plain user sees only the orphan level-0 issues of projects
// they belong to — never another project's, never an epic, never a story that
// already has an epic — while a global admin sees every project.
func TestAgileEpicNone_ScopedToMemberProjects(t *testing.T) {
	srv, authSvc, db := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob@example.com", "bob")
	createProjectViaAPI(t, srv, alice, "EPA", "Alice epics")
	createProjectViaAPI(t, srv, bob, "EPB", "Bob epics")

	epicA, _ := mustCreateTypedIssue(t, srv, alice, "EPA", "Epic A", "Epic", nil)
	childA, _ := mustCreateTypedIssue(t, srv, alice, "EPA", "Child A", "Story", map[string]any{"parent": map[string]any{"key": epicA}})
	orphanA, _ := mustCreateTypedIssue(t, srv, alice, "EPA", "Orphan A", "Story", nil)
	orphanB, _ := mustCreateTypedIssue(t, srv, bob, "EPB", "Orphan B", "Task", nil)

	const path = "/rest/agile/1.0/epic/none/issue"
	const spec = "/rest/agile/1.0/epic/none/issue"

	keys, total := issuePage(t, srv, alice, path, spec)
	if !contains(keys, orphanA) || contains(keys, orphanB) || contains(keys, epicA) || contains(keys, childA) || total != 1 {
		t.Errorf("alice (member of EPA only) sees %v (total %d), want only %s", keys, total, orphanA)
	}
	keys, total = issuePage(t, srv, bob, path, spec)
	if !contains(keys, orphanB) || contains(keys, orphanA) || total != 1 {
		t.Errorf("bob (member of EPB only) sees %v (total %d), want only %s", keys, total, orphanB)
	}

	// A global admin is unscoped: both orphans, still no epic / parented story.
	promoteAdmin(t, db, "alice@example.com")
	keys, total = issuePage(t, srv, alice, path, spec)
	if !contains(keys, orphanA) || !contains(keys, orphanB) || contains(keys, epicA) || contains(keys, childA) || total != 2 {
		t.Errorf("global admin sees %v (total %d), want exactly %s and %s", keys, total, orphanA, orphanB)
	}
}
