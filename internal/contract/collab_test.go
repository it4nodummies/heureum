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
