package contract

import (
	"net/http"
	"strings"
	"testing"
)

const specPath = "../../docs/contracts/jira-platform-v3.json"

func TestValidator_ValidatesKnownGoodResponse(t *testing.T) {
	v := MustLoad(t, specPath)
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
	err := v.ValidateResponse("GET", "/rest/api/3/myself", 200,
		http.Header{"Content-Type": []string{"application/json"}},
		strings.NewReader(body))
	if err != nil {
		t.Errorf("valid myself body rejected: %v", err)
	}
}

func TestValidator_RejectsBadResponse(t *testing.T) {
	v := MustLoad(t, specPath)
	// "active" con tipo sbagliato deve essere rifiutato.
	body := `{"accountId": "u1", "active": "yes"}`
	err := v.ValidateResponse("GET", "/rest/api/3/myself", 200,
		http.Header{"Content-Type": []string{"application/json"}},
		strings.NewReader(body))
	if err == nil {
		t.Fatal("invalid body accepted")
	}
	if !strings.Contains(err.Error(), "active") {
		t.Errorf("error does not mention offending field %q: %v", "active", err)
	}
}

func TestValidator_PathParamRoute(t *testing.T) {
	v := MustLoad(t, specPath)
	// GET /rest/api/3/priority/{id}: esercita l'estrazione dei path param
	// da parte di FindRoute. Risposta plausibile secondo lo schema Priority.
	body := `{
	  "self": "http://localhost:8080/rest/api/3/priority/3",
	  "id": "3",
	  "name": "Major",
	  "description": "Major loss of function.",
	  "iconUrl": "http://localhost:8080/images/icons/priorities/major.png",
	  "statusColor": "#009900",
	  "isDefault": false
	}`
	err := v.ValidateResponse("GET", "/rest/api/3/priority/3", 200,
		http.Header{"Content-Type": []string{"application/json"}},
		strings.NewReader(body))
	if err != nil {
		t.Errorf("valid priority body rejected: %v", err)
	}
}

func TestValidator_PathParamRouteRejectsBadResponse(t *testing.T) {
	v := MustLoad(t, specPath)
	// "isDefault" con tipo sbagliato (string invece di boolean) deve essere rifiutato.
	body := `{"id": "3", "name": "Major", "isDefault": "yes"}`
	err := v.ValidateResponse("GET", "/rest/api/3/priority/3", 200,
		http.Header{"Content-Type": []string{"application/json"}},
		strings.NewReader(body))
	if err == nil {
		t.Fatal("invalid priority body accepted")
	}
	if !strings.Contains(err.Error(), "isDefault") {
		t.Errorf("error does not mention offending field %q: %v", "isDefault", err)
	}
}
