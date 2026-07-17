package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestWorklog_UpdatesIssueTimeSpent verifica end-to-end che POST
// /issue/{key}/worklog aggiorni anche fields.timetracking.timeSpentSeconds
// letto da GET /issue/{key} (non solo la riga di worklog).
func TestWorklog_UpdatesIssueTimeSpent(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Log some work")

	wbody := `{"timeSpentSeconds":3600}`
	wreq, _ := http.NewRequest("POST", srv.URL+"/rest/api/3/issue/"+key+"/worklog", strings.NewReader(wbody))
	wreq.Header.Set("Authorization", "Bearer "+jwt)
	wreq.Header.Set("Content-Type", "application/json")
	wres, err := http.DefaultClient.Do(wreq)
	if err != nil {
		t.Fatal(err)
	}
	defer wres.Body.Close()
	if wres.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(wres.Body)
		t.Fatalf("create worklog status = %d: %s", wres.StatusCode, b)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key, nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get issue status = %d", res.StatusCode)
	}
	var got struct {
		Fields struct {
			TimeTracking struct {
				TimeSpentSeconds int `json:"timeSpentSeconds"`
			} `json:"timetracking"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Fields.TimeTracking.TimeSpentSeconds != 3600 {
		t.Errorf("timeSpentSeconds = %d, want 3600", got.Fields.TimeTracking.TimeSpentSeconds)
	}
}

// TestUpdateIssue_TimeTracking_SetsOriginalAndRemainingEstimate verifica che
// PUT /issue/{key} con fields.timetracking scriva
// originalEstimateSeconds/remainingEstimateSeconds e che GET li rilegga.
func TestUpdateIssue_TimeTracking_SetsOriginalAndRemainingEstimate(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "DEMO", "Demo Project")
	key := createIssueViaAPI(t, srv, jwt, "DEMO", "Estimate me")

	body := `{"fields":{"timetracking":{"originalEstimateSeconds":28800,"remainingEstimateSeconds":14400}}}`
	req, _ := http.NewRequest("PUT", srv.URL+"/rest/api/3/issue/"+key, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("update issue status = %d: %s", res.StatusCode, b)
	}

	greq, _ := http.NewRequest("GET", srv.URL+"/rest/api/3/issue/"+key, nil)
	greq.Header.Set("Authorization", "Bearer "+jwt)
	gres, err := http.DefaultClient.Do(greq)
	if err != nil {
		t.Fatal(err)
	}
	defer gres.Body.Close()
	if gres.StatusCode != http.StatusOK {
		t.Fatalf("get issue status = %d", gres.StatusCode)
	}
	v := MustLoad(t, "../../docs/contracts/jira-platform-v3.json")
	b, err := io.ReadAll(gres.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateResponse("GET", "/rest/api/3/issue/"+key, gres.StatusCode, gres.Header, strings.NewReader(string(b))); err != nil {
		t.Errorf("GET /issue/{key} non conforme: %v", err)
	}
	var got struct {
		Fields struct {
			TimeTracking struct {
				OriginalEstimateSeconds  int `json:"originalEstimateSeconds"`
				RemainingEstimateSeconds int `json:"remainingEstimateSeconds"`
			} `json:"timetracking"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Fields.TimeTracking.OriginalEstimateSeconds != 28800 {
		t.Errorf("originalEstimateSeconds = %d, want 28800", got.Fields.TimeTracking.OriginalEstimateSeconds)
	}
	if got.Fields.TimeTracking.RemainingEstimateSeconds != 14400 {
		t.Errorf("remainingEstimateSeconds = %d, want 14400", got.Fields.TimeTracking.RemainingEstimateSeconds)
	}
}
