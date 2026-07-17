package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// createVersionViaAPI crea una version su un progetto (per key) e ne restituisce
// il body decodificato.
func createVersionViaAPI(t *testing.T, srv, jwt, projectKey, name string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"name":        name,
		"description": "first release",
		"startDate":   "2026-01-01",
		"releaseDate": "2026-06-30",
		"project":     projectKey,
	})
	req, _ := http.NewRequest("POST", srv+"/rest/api/3/version", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create version status = %d: %s", res.StatusCode, b)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCreateVersion_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")

	body := `{"name":"v1.0","description":"initial","startDate":"2026-01-01","releaseDate":"2026-06-30","project":"DEMO"}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/version", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d: %s", res.StatusCode, b)
	}
	buf, _ := io.ReadAll(res.Body)

	// contract validation
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("POST", "/rest/api/3/version", res.StatusCode, res.Header, bytes.NewReader(buf)); err != nil {
		t.Errorf("POST /version non conforme: %v", err)
	}

	// field-level assertions: date-only + numeric projectId
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatal(err)
	}
	if m["startDate"] != "2026-01-01" {
		t.Errorf("startDate = %v, want 2026-01-01", m["startDate"])
	}
	if m["releaseDate"] != "2026-06-30" {
		t.Errorf("releaseDate = %v, want 2026-06-30", m["releaseDate"])
	}
	// projectId is a JSON number (seq_id >= 10000)
	pid, ok := m["projectId"].(float64)
	if !ok {
		t.Fatalf("projectId is not a number: %T %v", m["projectId"], m["projectId"])
	}
	if pid < 10000 {
		t.Errorf("projectId = %v, want >= 10000 (seq id)", pid)
	}
}

func TestGetVersion_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	created := createVersionViaAPI(t, srv.URL, jwt, "DEMO", "v2.0")
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("created version has no id")
	}

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/version/"+id, nil)
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
	if err := v.ValidateResponse("GET", "/rest/api/3/version/"+id, res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /version/{id} non conforme: %v", err)
	}
}

func TestListVersions_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	createVersionViaAPI(t, srv.URL, jwt, "DEMO", "v1")
	createVersionViaAPI(t, srv.URL, jwt, "DEMO", "v2")

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/project/DEMO/versions", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	buf, _ := io.ReadAll(res.Body)
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/project/DEMO/versions", res.StatusCode, res.Header, bytes.NewReader(buf)); err != nil {
		t.Errorf("GET /project/{key}/versions non conforme: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(buf, &arr); err != nil {
		t.Fatal(err)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(arr))
	}
	if _, ok := arr[0]["projectId"].(float64); !ok {
		t.Errorf("projectId not numeric in list: %v", arr[0]["projectId"])
	}
}

// TestVersion_NonAdmin_403 verifica che un utente non-admin del progetto non
// possa creare né aggiornare una version.
func TestVersion_NonAdmin_403(t *testing.T) {
	srv, authSvc := newTestServer(t)
	adminJWT := registerAndLogin(t, authSvc) // alice: creator -> project admin
	createProjectViaAPI(t, srv, adminJWT, "DEMO", "Demo Project")
	created := createVersionViaAPI(t, srv.URL, adminJWT, "DEMO", "v1")
	id, _ := created["id"].(string)

	bobJWT := registerUserAndLogin(t, authSvc, "bob@example.com", "bob")

	// bob is not a member of DEMO -> create must be forbidden
	body := `{"name":"nope","project":"DEMO"}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/version", bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+bobJWT)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Errorf("non-admin create status = %d, want 403", res.StatusCode)
	}

	// bob update must be forbidden too
	ureq, _ := http.NewRequest("PUT", srv.URL+"/rest/api/3/version/"+id, bytes.NewReader([]byte(`{"released":true}`)))
	ureq.Header.Set("Authorization", "Bearer "+bobJWT)
	ureq.Header.Set("Content-Type", "application/json")
	ures, err := http.DefaultClient.Do(ureq)
	if err != nil {
		t.Fatal(err)
	}
	ures.Body.Close()
	if ures.StatusCode != 403 {
		t.Errorf("non-admin update status = %d, want 403", ures.StatusCode)
	}
}

// TestVersionLifecycle_HandlerFlow: create -> get -> list -> update(released) ->
// delete, tutto via HTTP.
func TestVersionLifecycle_HandlerFlow(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	created := createVersionViaAPI(t, srv.URL, jwt, "DEMO", "v1")
	id, _ := created["id"].(string)
	if created["released"] != false {
		t.Errorf("new version released = %v, want false", created["released"])
	}

	// update released=true
	ureq, _ := http.NewRequest("PUT", srv.URL+"/rest/api/3/version/"+id, bytes.NewReader([]byte(`{"released":true}`)))
	ureq.Header.Set("Authorization", "Bearer "+jwt)
	ureq.Header.Set("Content-Type", "application/json")
	ures, err := http.DefaultClient.Do(ureq)
	if err != nil {
		t.Fatal(err)
	}
	var updated map[string]any
	json.NewDecoder(ures.Body).Decode(&updated)
	ures.Body.Close()
	if ures.StatusCode != 200 {
		t.Fatalf("update status = %d", ures.StatusCode)
	}
	if updated["released"] != true {
		t.Errorf("released = %v, want true", updated["released"])
	}

	// delete
	dreq, _ := http.NewRequest("DELETE", srv.URL+"/rest/api/3/version/"+id, nil)
	dreq.Header.Set("Authorization", "Bearer "+jwt)
	dres, err := http.DefaultClient.Do(dreq)
	if err != nil {
		t.Fatal(err)
	}
	dres.Body.Close()
	if dres.StatusCode != 204 {
		t.Errorf("delete status = %d, want 204", dres.StatusCode)
	}

	// get after delete -> 404
	greq, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/version/"+id, nil)
	greq.Header.Set("Authorization", "Bearer "+jwt)
	gres, err := http.DefaultClient.Do(greq)
	if err != nil {
		t.Fatal(err)
	}
	gres.Body.Close()
	if gres.StatusCode != 404 {
		t.Errorf("get after delete status = %d, want 404", gres.StatusCode)
	}
}
