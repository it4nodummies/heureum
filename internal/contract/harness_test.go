package contract

import (
	"net/http"
	"strings"
	"testing"
)

func TestValidator_ValidatesKnownGoodResponse(t *testing.T) {
	v, err := NewValidator("../../docs/contracts/jira-platform-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	// Risposta plausibile per GET /rest/api/3/myself secondo lo schema User.
	body := `{
	  "self": "http://localhost:8080/rest/api/3/user?accountId=u1",
	  "accountId": "u1",
	  "accountType": "atlassian",
	  "emailAddress": "alice@example.com",
	  "displayName": "Alice",
	  "active": true,
	  "avatarUrls": {"16x16": "http://x/a.png", "24x24": "http://x/a.png",
	                 "32x32": "http://x/a.png", "48x48": "http://x/a.png"}
	}`
	err = v.ValidateResponse("GET", "/rest/api/3/myself", 200,
		http.Header{"Content-Type": []string{"application/json"}},
		strings.NewReader(body))
	if err != nil {
		t.Errorf("valid myself body rejected: %v", err)
	}
}

func TestValidator_RejectsBadResponse(t *testing.T) {
	v, err := NewValidator("../../docs/contracts/jira-platform-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	// "active" con tipo sbagliato deve essere rifiutato.
	body := `{"accountId": "u1", "active": "yes"}`
	err = v.ValidateResponse("GET", "/rest/api/3/myself", 200,
		http.Header{"Content-Type": []string{"application/json"}},
		strings.NewReader(body))
	if err == nil {
		t.Error("invalid body accepted")
	}
}
