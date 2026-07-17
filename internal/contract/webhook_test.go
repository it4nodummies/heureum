package contract

import (
	"net/http"
	"testing"
)

func TestWebhook_CRUD(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "WH", "Webhook Proj")

	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/project/WH/webhooks", map[string]any{
		"url": "https://example.com/hook", "events": []string{"issue_created"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook %d", resp.StatusCode)
	}
	created, _ := decodeBody(t, resp)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("webhook senza id")
	}

	resp = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/project/WH/webhooks", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list webhooks %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, srv, http.MethodDelete, tok, "/rest/api/3/project/WH/webhooks/"+id, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete webhook %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestWebhook_EnqueuesOnIssueCreate verifica il nuovo modello a coda persistente:
// creando una issue la dispatcher NON esegue HTTP nel percorso della request, ma
// accoda una webhook_delivery in stato 'pending' (la consegna effettiva la fa il
// worker con retry/backoff — affidabilità sulla immediatezza).
func TestWebhook_EnqueuesOnIssueCreate(t *testing.T) {
	srv, authSvc, db := newTestServerDB(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "WH", "Webhook Proj")

	// registra un webhook sull'evento issue_created
	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/project/WH/webhooks", map[string]any{
		"url": "https://example.com/hook", "events": []string{"issue_created"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook %d", resp.StatusCode)
	}
	resp.Body.Close()

	// crea una issue → deve accodare una consegna pending
	createIssueViaAPI(t, srv, tok, "WH", "Trigger me")

	var status, payload, eventType string
	row := db.Raw(`SELECT status, payload, event_type FROM webhook_deliveries WHERE event_type = ? LIMIT 1`, "issue_created").Row()
	if err := row.Scan(&status, &payload, &eventType); err != nil {
		t.Fatalf("nessuna consegna accodata dopo la creazione della issue: %v", err)
	}
	if status != "pending" {
		t.Errorf("status = %q, atteso pending", status)
	}
	if payload == "" {
		t.Errorf("payload della consegna vuoto")
	}
}
