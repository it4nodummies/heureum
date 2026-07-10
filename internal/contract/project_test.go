package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

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

func TestCreateProject_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	body := `{"key":"NEW","name":"New Project","projectTypeKey":"software","projectTemplateKey":"com.pyxis.greenhopper.jira:gh-scrum-template","assigneeType":"UNASSIGNED"}`
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
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}

// TestCreateProject_InvalidKey_400 verifica che una key non conforme al
// formato Jira (qui tutta numerica) venga rifiutata con 400 e un errore di
// campo su "key". Questo impedisce che una key numerica possa mascherare una
// lookup per seq_id numerico in GET.
func TestCreateProject_InvalidKey_400(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	body := `{"key":"99999","name":"X","projectTypeKey":"software","projectTemplateKey":"com.pyxis.greenhopper.jira:gh-scrum-template"}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/project", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	var errBody struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	if errBody.Errors["key"] == "" {
		t.Errorf("expected a 'key' field error, got %+v", errBody.Errors)
	}
}

func TestGetProject_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
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

// TestGetProjectByNumericID prova che create -> get-by-numeric-id fa
// round-trip: creiamo un progetto, leggiamo l'id numerico (seq_id) restituito
// da POST, e recuperiamo il progetto con GET /project/{id} numerico.
func TestGetProjectByNumericID(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)

	body := `{"key":"NUM","name":"Numeric Project","projectTypeKey":"software","projectTemplateKey":"com.pyxis.greenhopper.jira:gh-scrum-template"}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/project", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("create status = %d: %s", res.StatusCode, b)
	}
	var created struct {
		ID  int64  `json:"id"`
		Key string `json:"key"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		res.Body.Close()
		t.Fatal(err)
	}
	res.Body.Close()
	if created.ID < 10000 {
		t.Fatalf("numeric id = %d, want >= 10000", created.ID)
	}

	idStr := strconv.FormatInt(created.ID, 10)
	req2, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/project/"+idStr, nil)
	req2.Header.Set("Authorization", "Bearer "+jwt)
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != 200 {
		t.Fatalf("get-by-id status = %d", res2.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/project/"+idStr, res2.StatusCode, res2.Header, res2.Body); err != nil {
		t.Errorf("GET /project/{numericId} non conforme: %v", err)
	}
}
