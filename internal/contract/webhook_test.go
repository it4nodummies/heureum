package contract

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestWebhook_FiresOnIssueCreate(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, tok, "WH", "Webhook Proj")

	received := make(chan string, 4)
	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- r.Header.Get("X-OpenJira-Event"):
		default:
		}
		w.WriteHeader(200)
	}))
	defer recv.Close()

	// registra un webhook sull'evento issue_created verso il ricevitore locale
	resp := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/project/WH/webhooks", map[string]any{
		"url": recv.URL, "events": []string{"issue_created"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create webhook %d", resp.StatusCode)
	}
	resp.Body.Close()

	// crea una issue → deve scatenare la consegna
	createIssueViaAPI(t, srv, tok, "WH", "Trigger me")

	select {
	case ev := <-received:
		if ev != "issue_created" {
			t.Errorf("evento header errato: %q", ev)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("webhook non consegnato dopo la creazione della issue")
	}
}
