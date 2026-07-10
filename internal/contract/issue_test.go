package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPriorities_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/priority", nil)
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
	if err := v.ValidateResponse("GET", "/rest/api/3/priority", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /priority non conforme: %v", err)
	}
}

func TestIssueTypes_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issuetype", nil)
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
	if err := v.ValidateResponse("GET", "/rest/api/3/issuetype", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /issuetype non conforme: %v", err)
	}
}

func TestResolutions_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/resolution", nil)
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
	if err := v.ValidateResponse("GET", "/rest/api/3/resolution", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /resolution non conforme: %v", err)
	}
}

// createIssueViaAPI crea un'issue via POST /rest/api/3/issue e restituisce la
// Key (es. DEMO-1) dal CreatedIssue restituito.
func createIssueViaAPI(t *testing.T, srv *httptest.Server, jwt, projectKey, summary string) string {
	t.Helper()
	body := `{"fields":{"project":{"key":"` + projectKey + `"},"summary":"` + summary + `","issuetype":{"name":"Task"}}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create issue status = %d: %s", res.StatusCode, b)
	}
	var out struct {
		Key string `json:"key"`
	}
	json.NewDecoder(res.Body).Decode(&out)
	return out.Key
}

func TestCreateIssue_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	body := `{"fields":{"project":{"key":"DEMO"},"summary":"New issue","issuetype":{"name":"Task"},"priority":{"id":"2"}}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue", strings.NewReader(body))
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
	if err := v.ValidateResponse("POST", "/rest/api/3/issue", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("POST /issue non conforme: %v", err)
	}
}

func TestCreateIssue_MissingSummary_400(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue", strings.NewReader(`{"fields":{"project":{"key":"DEMO"}}}`))
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

func TestGetIssue_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "First issue")
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key, nil)
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
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key, res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /issue/{key} non conforme: %v", err)
	}
}

func TestUpdateIssue_204(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Before")
	body := `{"fields":{"summary":"After"}}`
	req, _ := http.NewRequest("PUT", srv.URL+"/rest/api/3/issue/"+key, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("PUT status = %d, want 204", res.StatusCode)
	}
	greq, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key, nil)
	greq.Header.Set("Authorization", "Bearer "+jwt)
	gres, _ := http.DefaultClient.Do(greq)
	defer gres.Body.Close()
	var bean struct {
		Fields struct {
			Summary string `json:"summary"`
		} `json:"fields"`
	}
	json.NewDecoder(gres.Body).Decode(&bean)
	if bean.Fields.Summary != "After" {
		t.Errorf("summary not updated: %q", bean.Fields.Summary)
	}
}

func TestDeleteIssue_204(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Trash")
	req, _ := http.NewRequest("DELETE", srv.URL+"/rest/api/3/issue/"+key, nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, _ := http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != 204 {
		t.Fatalf("DELETE status = %d, want 204", res.StatusCode)
	}
	// verifica 404 dopo delete
	greq, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key, nil)
	greq.Header.Set("Authorization", "Bearer "+jwt)
	gres, _ := http.DefaultClient.Do(greq)
	gres.Body.Close()
	if gres.StatusCode != 404 {
		t.Fatalf("GET after delete = %d, want 404", gres.StatusCode)
	}
}
