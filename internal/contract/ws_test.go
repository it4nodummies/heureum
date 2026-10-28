package contract

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// dialBoardWS apre l'handshake WebSocket sulla rotta board del progetto key
// con il bearer jwt. Restituisce la connessione (nil se l'handshake è stato
// rifiutato) e lo status HTTP dell'handshake.
func dialBoardWS(t *testing.T, srvURL, jwt, key string) (*websocket.Conn, int) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srvURL, "http") + "/ws/v1/projects/" + key + "/board"
	hdr := http.Header{"Authorization": {"Bearer " + jwt}}
	conn, res, err := websocket.DefaultDialer.Dial(url, hdr)
	if err != nil {
		if res == nil {
			t.Fatalf("dial %s: %v (no response)", url, err)
		}
		return nil, res.StatusCode
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, res.StatusCode
}

// TestBoardWS_RequiresProjectBrowse (Round 27, Task 6): il socket board di un
// progetto è gated come le rotte HTTP sorelle — BROWSE_PROJECTS su {key}. Un
// utente senza ruolo nel progetto riceve 403 all'handshake; un membro riceve
// 101 Switching Protocols; un progetto inesistente risponde 404.
func TestBoardWS_RequiresProjectBrowse(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob@example.com", "bob")
	createProjectViaAPI(t, srv, alice, "AZ", "Alice Proj")

	if conn, status := dialBoardWS(t, srv.URL, bob, "AZ"); conn != nil || status != http.StatusForbidden {
		t.Fatalf("bob (no role) handshake: conn=%v status=%d, want refused with 403", conn != nil, status)
	}

	if conn, status := dialBoardWS(t, srv.URL, alice, "AZ"); conn == nil || status != http.StatusSwitchingProtocols {
		t.Fatalf("alice (admin) handshake: conn=%v status=%d, want 101", conn != nil, status)
	}

	if conn, status := dialBoardWS(t, srv.URL, alice, "NOPE"); conn != nil || status != http.StatusNotFound {
		t.Fatalf("unknown project handshake: conn=%v status=%d, want refused with 404", conn != nil, status)
	}
}

// TestBoardWS_Unauthenticated: senza bearer l'handshake è rifiutato con 401
// (già vero da R12, pinnato qui perché il socket ora porta l'identità utente).
func TestBoardWS_Unauthenticated(t *testing.T) {
	srv, _, _ := newTestServerDB(t)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/v1/projects/AZ/board"
	conn, res, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		conn.Close()
		t.Fatal("expected handshake to be refused without a bearer token")
	}
	if res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated handshake status = %v, want 401", res)
	}
}
