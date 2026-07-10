package contract

import (
	"net/http"
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
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/resolution", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /resolution non conforme: %v", err)
	}
}
