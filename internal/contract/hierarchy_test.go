package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// decodeListBody reads a JSON array response body.
func decodeListBody(t *testing.T, res *http.Response) ([]any, []byte) {
	t.Helper()
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out []any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode list body: %v (body=%s)", err, b)
	}
	return out, b
}

// Round 28 — issue hierarchy over the Jira v3 surface: fields.parent carries
// the parent's fields (summary/status/issuetype/priority), issue types carry
// hierarchyLevel, and POST/PUT /issue accept fields.parent (key or id, or
// null on PUT) with the strict Epic → Story → Subtask rules enforced as 400
// errors.parent. Every response is validated against the official spec.

const v3Spec = "../../docs/contracts/jira-platform-v3.json"

// createTypedIssue posts an issue with the given type and optional extra
// fields (e.g. "parent") and returns the raw response for the caller to assert.
func createTypedIssue(t *testing.T, srv *httptest.Server, jwt, projectKey, summary, typeName string, extra map[string]any) *http.Response {
	t.Helper()
	fields := map[string]any{
		"project":   map[string]any{"key": projectKey},
		"summary":   summary,
		"issuetype": map[string]any{"name": typeName},
	}
	for k, v := range extra {
		fields[k] = v
	}
	return doJSON(t, srv, http.MethodPost, jwt, "/rest/api/3/issue", map[string]any{"fields": fields})
}

// mustCreateTypedIssue is createTypedIssue asserting 201 (validated against
// the spec) and returning the new issue's key and id.
func mustCreateTypedIssue(t *testing.T, srv *httptest.Server, jwt, projectKey, summary, typeName string, extra map[string]any) (key, id string) {
	t.Helper()
	res := createTypedIssue(t, srv, jwt, projectKey, summary, typeName, extra)
	if res.StatusCode != http.StatusCreated {
		_, b := decodeBody(t, res)
		t.Fatalf("create %s %q: status %d body %s", typeName, summary, res.StatusCode, b)
	}
	body, _ := decodeBody(t, res)
	key, _ = body["key"].(string)
	id, _ = body["id"].(string)
	if key == "" || id == "" {
		t.Fatalf("create %s %q: missing key/id in %v", typeName, summary, body)
	}
	return key, id
}

// getIssueFields GETs the issue (validated against the spec) and returns its
// "fields" object.
func getIssueFields(t *testing.T, srv *httptest.Server, jwt, key string) map[string]any {
	t.Helper()
	res := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/issue/"+key, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get %s: status %d", key, res.StatusCode)
	}
	body, raw := decodeBody(t, res)
	v := MustLoad(t, v3Spec)
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key, res.StatusCode, res.Header, bytesReader(raw)); err != nil {
		t.Errorf("GET /issue/%s non conforme: %v", key, err)
	}
	fields, _ := body["fields"].(map[string]any)
	if fields == nil {
		t.Fatalf("get %s: no fields in %s", key, raw)
	}
	return fields
}

// putParent PUTs {"fields":{"parent": parent}} on the issue; parent may be a
// map, or nil for an explicit JSON null (clear).
func putParent(t *testing.T, srv *httptest.Server, jwt, key string, parent any) *http.Response {
	t.Helper()
	return doJSON(t, srv, http.MethodPut, jwt, "/rest/api/3/issue/"+key, map[string]any{
		"fields": map[string]any{"parent": parent},
	})
}

// expectParentError asserts a 400 whose errors map has a "parent" entry.
func expectParentError(t *testing.T, res *http.Response, what string) {
	t.Helper()
	body, raw := decodeBody(t, res)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("%s: status %d, want 400 (body %s)", what, res.StatusCode, raw)
	}
	errs, _ := body["errors"].(map[string]any)
	if msg, _ := errs["parent"].(string); msg == "" {
		t.Errorf("%s: expected errors.parent in %s", what, raw)
	}
}

func TestIssueParent_CreateUpdateClear(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "EPC", "Epic Proj")
	v := MustLoad(t, v3Spec)

	epicA, _ := mustCreateTypedIssue(t, srv, jwt, "EPC", "Epic A", "Epic", nil)
	epicB, epicBID := mustCreateTypedIssue(t, srv, jwt, "EPC", "Epic B", "Epic", nil)

	// Create a Story under Epic A by key: 201 conformant to the spec.
	res := createTypedIssue(t, srv, jwt, "EPC", "Story one", "Story", map[string]any{"parent": map[string]any{"key": epicA}})
	body, raw := decodeBody(t, res)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create story under epic: status %d body %s", res.StatusCode, raw)
	}
	if err := v.ValidateResponse("POST", "/rest/api/3/issue", res.StatusCode, res.Header, bytesReader(raw)); err != nil {
		t.Errorf("POST /issue non conforme: %v", err)
	}
	story1, _ := body["key"].(string)

	fields := getIssueFields(t, srv, jwt, story1)
	parent, _ := fields["parent"].(map[string]any)
	if parent == nil {
		t.Fatalf("fields.parent missing after create with parent.key: %v", fields)
	}
	if got, _ := parent["key"].(string); got != epicA {
		t.Errorf("parent.key = %q, want %q", got, epicA)
	}
	pf, _ := parent["fields"].(map[string]any)
	if pf == nil {
		t.Fatalf("fields.parent.fields missing: %v", parent)
	}
	if got, _ := pf["summary"].(string); got != "Epic A" {
		t.Errorf("parent.fields.summary = %q, want %q", got, "Epic A")
	}
	pit, _ := pf["issuetype"].(map[string]any)
	if lvl, _ := pit["hierarchyLevel"].(float64); lvl != 1 {
		t.Errorf("parent.fields.issuetype.hierarchyLevel = %v, want 1 (%v)", pit["hierarchyLevel"], pit)
	}
	if _, ok := pf["status"].(map[string]any); !ok {
		t.Errorf("parent.fields.status missing: %v", pf)
	}
	if it, _ := fields["issuetype"].(map[string]any); it["hierarchyLevel"] != float64(0) {
		t.Errorf("story issuetype.hierarchyLevel = %v, want 0", it["hierarchyLevel"])
	}

	// Create a second Story under Epic B by numeric id.
	story2, _ := mustCreateTypedIssue(t, srv, jwt, "EPC", "Story two", "Story", map[string]any{"parent": map[string]any{"id": epicBID}})
	if p, _ := getIssueFields(t, srv, jwt, story2)["parent"].(map[string]any); p["key"] != epicB {
		t.Errorf("story created with parent.id: parent.key = %v, want %q", p["key"], epicB)
	}

	// Move story1 to Epic B.
	res = putParent(t, srv, jwt, story1, map[string]any{"key": epicB})
	if res.StatusCode != http.StatusNoContent {
		_, raw := decodeBody(t, res)
		t.Fatalf("PUT parent move: status %d body %s", res.StatusCode, raw)
	}
	res.Body.Close()
	if p, _ := getIssueFields(t, srv, jwt, story1)["parent"].(map[string]any); p["key"] != epicB {
		t.Errorf("after move parent.key = %v, want %q", p["key"], epicB)
	}

	// Explicit null clears; absent leaves it unchanged.
	res = doJSON(t, srv, http.MethodPut, jwt, "/rest/api/3/issue/"+story1, map[string]any{
		"fields": map[string]any{"summary": "Story one renamed"},
	})
	res.Body.Close()
	if p, _ := getIssueFields(t, srv, jwt, story1)["parent"].(map[string]any); p["key"] != epicB {
		t.Errorf("PUT without parent must not touch it: parent = %v", p)
	}
	res = putParent(t, srv, jwt, story1, nil)
	if res.StatusCode != http.StatusNoContent {
		_, raw := decodeBody(t, res)
		t.Fatalf("PUT parent null: status %d body %s", res.StatusCode, raw)
	}
	res.Body.Close()
	if p, ok := getIssueFields(t, srv, jwt, story1)["parent"]; ok && p != nil {
		t.Errorf("after parent:null the parent must be gone, got %v", p)
	}

	// Story under a Story → 400 errors.parent.
	expectParentError(t, putParent(t, srv, jwt, story1, map[string]any{"key": story2}), "PUT story under story")
	// Unknown parent key on create → 400 (was: 201 with the parent dropped).
	expectParentError(t, createTypedIssue(t, srv, jwt, "EPC", "Orphan", "Story", map[string]any{"parent": map[string]any{"key": "EPC-9999"}}), "create with unknown parent key")
	// Unknown parent id on update → 400.
	expectParentError(t, putParent(t, srv, jwt, story1, map[string]any{"id": "99999999"}), "PUT unknown parent id")
	// An Epic never has a parent.
	expectParentError(t, createTypedIssue(t, srv, jwt, "EPC", "Epic C", "Epic", map[string]any{"parent": map[string]any{"key": epicA}}), "create epic with parent")
	// A Subtask under an Epic → 400; under a Story → 201.
	expectParentError(t, createTypedIssue(t, srv, jwt, "EPC", "Sub under epic", "Subtask", map[string]any{"parent": map[string]any{"key": epicA}}), "create subtask under epic")
	sub, _ := mustCreateTypedIssue(t, srv, jwt, "EPC", "Sub under story", "Subtask", map[string]any{"parent": map[string]any{"key": story2}})
	if p, _ := getIssueFields(t, srv, jwt, sub)["parent"].(map[string]any); p["key"] != story2 {
		t.Errorf("subtask parent.key = %v, want %q", p["key"], story2)
	}

	// /subtasks lists the direct children of any level: Epic B → story2.
	res = doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/issue/"+epicB+"/subtasks", nil)
	children, raw := decodeBody(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("subtasks of epic: status %d body %s", res.StatusCode, raw)
	}
	vals, _ := children["values"].([]any)
	if len(vals) != 1 {
		t.Fatalf("epic B children = %d, want 1 (%s)", len(vals), raw)
	}
	if first, _ := vals[0].(map[string]any); first["key"] != story2 {
		t.Errorf("epic B child = %v, want %q", first["key"], story2)
	}
}

func TestProjectGet_IssueTypesWithHierarchyLevel(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "HIER", "Hierarchy Proj")

	res := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/project/HIER", nil)
	body, raw := decodeBody(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET project: status %d body %s", res.StatusCode, raw)
	}
	v := MustLoad(t, v3Spec)
	if err := v.ValidateResponse("GET", "/rest/api/3/project/HIER", res.StatusCode, res.Header, bytesReader(raw)); err != nil {
		t.Errorf("GET /project/{key} non conforme: %v", err)
	}
	types, _ := body["issueTypes"].([]any)
	if len(types) == 0 {
		t.Fatalf("issueTypes missing or empty in %s", raw)
	}
	levels := map[string]float64{}
	subtask := map[string]bool{}
	for _, raw := range types {
		it, _ := raw.(map[string]any)
		name, _ := it["name"].(string)
		lvl, ok := it["hierarchyLevel"].(float64)
		if !ok {
			t.Errorf("issue type %q has no hierarchyLevel: %v", name, it)
		}
		levels[name] = lvl
		subtask[name], _ = it["subtask"].(bool)
		if id, _ := it["id"].(string); id == "" {
			t.Errorf("issue type %q has no id", name)
		}
		if icon, _ := it["iconUrl"].(string); icon == "" {
			t.Errorf("issue type %q has no iconUrl", name)
		}
	}
	for name, want := range map[string]float64{"Epic": 1, "Story": 0, "Task": 0, "Bug": 0, "Subtask": -1} {
		if got, ok := levels[name]; !ok || got != want {
			t.Errorf("issueTypes[%s].hierarchyLevel = %v (present=%v), want %v", name, got, ok, want)
		}
	}
	if !subtask["Subtask"] || subtask["Epic"] {
		t.Errorf("subtask flags wrong: %v", subtask)
	}
}

func TestIssueTypes_HierarchyLevel(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "LVL", "Levels Proj")

	res := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/issuetype", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /issuetype: status %d", res.StatusCode)
	}
	list, raw := decodeListBody(t, res)
	v := MustLoad(t, v3Spec)
	if err := v.ValidateResponse("GET", "/rest/api/3/issuetype", res.StatusCode, res.Header, bytesReader(raw)); err != nil {
		t.Errorf("GET /issuetype non conforme: %v", err)
	}
	found := map[string]float64{}
	for _, item := range list {
		it, _ := item.(map[string]any)
		name, _ := it["name"].(string)
		lvl, ok := it["hierarchyLevel"].(float64)
		if !ok {
			t.Errorf("/issuetype %q has no hierarchyLevel: %v", name, it)
			continue
		}
		found[name] = lvl
	}
	if found["Epic"] != 1 || found["Subtask"] != -1 {
		t.Errorf("/issuetype levels = %v, want Epic=1 Subtask=-1", found)
	}
}
