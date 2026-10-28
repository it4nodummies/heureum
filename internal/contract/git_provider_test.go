package contract

import (
	"net/http"
	"strings"
	"testing"

	"github.com/it4nodummies/heureum/internal/domain/user"
)

// TestGetProvider_ViewerForbidden: GET /project/{key}/git/providers è una
// lettura amministrativa (espone la configurazione dell'integrazione git),
// quindi deve richiedere AdministerProjects come i fratelli POST/DELETE — un
// viewer del progetto riceve 403, l'admin 200. Passa dal router reale perché
// il buco era nel wiring (BrowseProjects), non nell'handler.
func TestGetProvider_ViewerForbidden(t *testing.T) {
	srv, authSvc, db := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob-viewer@example.com", "bob")

	createProjectViaAPI(t, srv, alice, "RD", "R&D")

	var bobUser user.User
	if err := db.Where("email = ?", "bob-viewer@example.com").First(&bobUser).Error; err != nil {
		t.Fatal(err)
	}
	res := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/project/RD/members",
		map[string]any{"user_id": bobUser.ID, "role": "viewer"})
	if res.StatusCode/100 != 2 {
		_, raw := decodeBody(t, res)
		t.Fatalf("add viewer status = %d: %s", res.StatusCode, raw)
	}
	res.Body.Close()

	res = doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/project/RD/git/providers",
		map[string]any{"provider_type": "github", "base_url": "https://github.com/acme/rd", "token": "ghp_secret", "webhook_secret": "hmac-secret-0123456789"})
	if res.StatusCode != http.StatusCreated {
		_, raw := decodeBody(t, res)
		t.Fatalf("configure provider status = %d: %s", res.StatusCode, raw)
	}
	res.Body.Close()

	// Viewer: 403 (stile Enforce dei fratelli POST/DELETE).
	res = doJSON(t, srv, http.MethodGet, bob, "/rest/api/3/project/RD/git/providers", nil)
	if res.StatusCode != http.StatusForbidden {
		_, raw := decodeBody(t, res)
		t.Errorf("viewer GET git/providers status = %d, want 403: %s", res.StatusCode, raw)
	} else {
		res.Body.Close()
	}

	// Admin: 200 e nessun segreto nel body.
	res = doJSON(t, srv, http.MethodGet, alice, "/rest/api/3/project/RD/git/providers", nil)
	if res.StatusCode != http.StatusOK {
		_, raw := decodeBody(t, res)
		t.Fatalf("admin GET git/providers status = %d, want 200: %s", res.StatusCode, raw)
	}
	body, raw := decodeBody(t, res)
	for _, k := range []string{"token_encrypted", "webhook_secret"} {
		if _, ok := body[k]; ok {
			t.Errorf("admin body exposes %q: %s", k, raw)
		}
	}
	if strings.Contains(string(raw), "ghp_secret") || strings.Contains(string(raw), "hmac-secret-0123456789") {
		t.Errorf("admin body contains a secret value: %s", raw)
	}
	if body["has_token"] != true {
		t.Errorf("has_token = %v, want true: %s", body["has_token"], raw)
	}
	if u, _ := body["webhook_url"].(string); !strings.Contains(u, "/rest/api/3/webhooks/git/") {
		t.Errorf("webhook_url = %q, want inbound git webhook URL", u)
	}
}
