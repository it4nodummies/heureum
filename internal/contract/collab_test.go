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

func TestWorklog_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Has worklogs")

	body := `{"timeSpentSeconds":3600,"comment":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Worked on it"}]}]}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue/"+key+"/worklog", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("POST worklog status = %d: %s", res.StatusCode, b)
	}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("POST", "/rest/api/3/issue/"+key+"/worklog", res.StatusCode, res.Header, strings.NewReader(string(bodyBytes))); err != nil {
		t.Errorf("POST worklog non conforme: %v", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		t.Fatal(err)
	}

	getReq, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key+"/worklog", nil)
	getReq.Header.Set("Authorization", "Bearer "+jwt)
	getRes, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer getRes.Body.Close()
	if getRes.StatusCode != 200 {
		b, _ := io.ReadAll(getRes.Body)
		t.Fatalf("GET worklog status = %d: %s", getRes.StatusCode, b)
	}
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key+"/worklog", getRes.StatusCode, getRes.Header, getRes.Body); err != nil {
		t.Errorf("GET worklog non conforme: %v", err)
	}

	delReq, _ := http.NewRequest("DELETE", srv.URL+"/rest/api/3/issue/"+key+"/worklog/"+created.ID, nil)
	delReq.Header.Set("Authorization", "Bearer "+jwt)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer delRes.Body.Close()
	if delRes.StatusCode != 204 {
		b, _ := io.ReadAll(delRes.Body)
		t.Fatalf("DELETE worklog status = %d: %s", delRes.StatusCode, b)
	}
}

func TestVotes_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Has votes")

	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")

	postReq, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue/"+key+"/votes", nil)
	postReq.Header.Set("Authorization", "Bearer "+jwt)
	postRes, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatal(err)
	}
	defer postRes.Body.Close()
	if postRes.StatusCode != 204 {
		b, _ := io.ReadAll(postRes.Body)
		t.Fatalf("POST votes status = %d: %s", postRes.StatusCode, b)
	}

	getReq, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key+"/votes", nil)
	getReq.Header.Set("Authorization", "Bearer "+jwt)
	getRes, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer getRes.Body.Close()
	if getRes.StatusCode != 200 {
		b, _ := io.ReadAll(getRes.Body)
		t.Fatalf("GET votes status = %d: %s", getRes.StatusCode, b)
	}
	bodyBytes, err := io.ReadAll(getRes.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key+"/votes", getRes.StatusCode, getRes.Header, strings.NewReader(string(bodyBytes))); err != nil {
		t.Errorf("GET votes non conforme: %v", err)
	}
	var votes struct {
		Votes    int    `json:"votes"`
		HasVoted bool   `json:"hasVoted"`
		Voters   []any  `json:"voters"`
		Self     string `json:"self"`
	}
	if err := json.Unmarshal(bodyBytes, &votes); err != nil {
		t.Fatal(err)
	}
	if votes.Votes != 1 {
		t.Errorf("votes = %d, want 1", votes.Votes)
	}
	if !votes.HasVoted {
		t.Error("hasVoted = false, want true")
	}
	if votes.Voters == nil {
		t.Error("voters = nil, want non-nil slice")
	}

	delReq, _ := http.NewRequest("DELETE", srv.URL+"/rest/api/3/issue/"+key+"/votes", nil)
	delReq.Header.Set("Authorization", "Bearer "+jwt)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer delRes.Body.Close()
	if delRes.StatusCode != 204 {
		b, _ := io.ReadAll(delRes.Body)
		t.Fatalf("DELETE votes status = %d: %s", delRes.StatusCode, b)
	}
}

func TestWatchers_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Watch me")
	// add current user as watcher
	areq, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue/"+key+"/watchers", nil)
	areq.Header.Set("Authorization", "Bearer "+jwt)
	ares, err := http.DefaultClient.Do(areq)
	if err != nil {
		t.Fatal(err)
	}
	ares.Body.Close()
	if ares.StatusCode != 204 {
		t.Fatalf("POST watchers = %d, want 204", ares.StatusCode)
	}
	// get
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key+"/watchers", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("GET watchers = %d", res.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key+"/watchers", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET watchers non conforme: %v", err)
	}
}

func TestChangelog_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "History")
	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key+"/changelog", nil)
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
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key+"/changelog", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET changelog non conforme: %v", err)
	}
}

func TestRemoteLink_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Has remote links")

	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")

	body := `{"object":{"url":"https://acme.com/ticket/1","title":"Ticket 1"},"globalId":"system=http://acme.com&id=1","relationship":"causes"}`
	postReq, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue/"+key+"/remotelink", strings.NewReader(body))
	postReq.Header.Set("Authorization", "Bearer "+jwt)
	postReq.Header.Set("Content-Type", "application/json")
	postRes, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatal(err)
	}
	defer postRes.Body.Close()
	if postRes.StatusCode != 201 {
		b, _ := io.ReadAll(postRes.Body)
		t.Fatalf("POST remotelink status = %d: %s", postRes.StatusCode, b)
	}
	postBody, err := io.ReadAll(postRes.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateResponse("POST", "/rest/api/3/issue/"+key+"/remotelink", postRes.StatusCode, postRes.Header, strings.NewReader(string(postBody))); err != nil {
		t.Errorf("POST remotelink non conforme: %v", err)
	}
	var created struct {
		Self string `json:"self"`
	}
	if err := json.Unmarshal(postBody, &created); err != nil {
		t.Fatal(err)
	}
	if created.Self == "" {
		t.Error("self = empty, want a URL")
	}

	getReq, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key+"/remotelink", nil)
	getReq.Header.Set("Authorization", "Bearer "+jwt)
	getRes, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer getRes.Body.Close()
	if getRes.StatusCode != 200 {
		b, _ := io.ReadAll(getRes.Body)
		t.Fatalf("GET remotelink status = %d: %s", getRes.StatusCode, b)
	}
	getBody, err := io.ReadAll(getRes.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key+"/remotelink", getRes.StatusCode, getRes.Header, strings.NewReader(string(getBody))); err != nil {
		t.Errorf("GET remotelink non conforme: %v", err)
	}
	var links []struct {
		Self         string `json:"self"`
		GlobalID     string `json:"globalId"`
		Relationship string `json:"relationship"`
		Object       struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"object"`
	}
	if err := json.Unmarshal(getBody, &links); err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("GET remotelink len = %d, want 1", len(links))
	}
	if links[0].Object.URL != "https://acme.com/ticket/1" {
		t.Errorf("Object.URL = %q", links[0].Object.URL)
	}
	if links[0].Relationship != "causes" {
		t.Errorf("Relationship = %q", links[0].Relationship)
	}

	// The self URL for the created link is of the form
	// .../remotelink/{id}; extract {id} to exercise DELETE.
	idx := strings.LastIndex(created.Self, "/")
	linkID := created.Self[idx+1:]
	delReq, _ := http.NewRequest("DELETE", srv.URL+"/rest/api/3/issue/"+key+"/remotelink/"+linkID, nil)
	delReq.Header.Set("Authorization", "Bearer "+jwt)
	delRes, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer delRes.Body.Close()
	if delRes.StatusCode != 204 {
		b, _ := io.ReadAll(delRes.Body)
		t.Fatalf("DELETE remotelink status = %d: %s", delRes.StatusCode, b)
	}

	getRes2, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer getRes2.Body.Close()
	var links2 []any
	if err := json.NewDecoder(getRes2.Body).Decode(&links2); err != nil {
		t.Fatal(err)
	}
	if len(links2) != 0 {
		t.Errorf("GET remotelink after DELETE len = %d, want 0", len(links2))
	}
}

func TestCreateIssueLink_Status(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	a := createIssueViaAPI(t, srv, jwt, "DEMO", "Blocker")
	b := createIssueViaAPI(t, srv, jwt, "DEMO", "Blocked")
	body := `{"type":{"name":"Blocks"},"inwardIssue":{"key":"` + b + `"},"outwardIssue":{"key":"` + a + `"}}`
	req, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issueLink", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", res.StatusCode)
	}
}
