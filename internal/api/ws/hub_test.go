package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newWSServer espone ServeWs dietro un httptest.Server. L'identità utente e
// il progetto della stanza arrivano da due header di test, così lo stesso
// server serve più client con identità diverse.
func newWSServer(t *testing.T, hub *Hub) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r, r.Header.Get("X-Test-User"), r.Header.Get("X-Test-Project"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

// dial apre un socket come userID sul progetto project; hdr aggiunge/override
// header dell'handshake (es. Origin).
func dial(t *testing.T, srv *httptest.Server, userID, project string, hdr http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	h := http.Header{}
	for k, v := range hdr {
		h[k] = v
	}
	h.Set("X-Test-User", userID)
	h.Set("X-Test-Project", project)
	conn, res, err := websocket.DefaultDialer.Dial(wsURL(srv), h)
	if conn != nil {
		t.Cleanup(func() { _ = conn.Close() })
	}
	return conn, res, err
}

func mustDial(t *testing.T, srv *httptest.Server, userID, project string) *websocket.Conn {
	t.Helper()
	conn, _, err := dial(t, srv, userID, project, nil)
	if err != nil {
		t.Fatalf("dial as %s: %v", userID, err)
	}
	return conn
}

// waitCount attende che il numero di socket registrati per userID sia want
// (la registrazione avviene dopo l'handshake, quindi il client può vedere il
// 101 prima che l'hub conosca la connessione).
func waitCount(t *testing.T, hub *Hub, userID string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ClientCount(userID) == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("ClientCount(%q) = %d, want %d (timeout)", userID, hub.ClientCount(userID), want)
}

func readWithin(t *testing.T, conn *websocket.Conn, d time.Duration) ([]byte, error) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(d))
	_, msg, err := conn.ReadMessage()
	return msg, err
}

func expectMessage(t *testing.T, conn *websocket.Conn, want string) {
	t.Helper()
	msg, err := readWithin(t, conn, 2*time.Second)
	if err != nil {
		t.Fatalf("expected message %q, got error: %v", want, err)
	}
	if string(msg) != want {
		t.Fatalf("message = %q, want %q", msg, want)
	}
}

func expectSilence(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	msg, err := readWithin(t, conn, 300*time.Millisecond)
	if err == nil {
		t.Fatalf("expected no message, got %q", msg)
	}
}

// TestHub_NotificationOnlyReachesOwner (Round 27, Task 6): un messaggio
// inviato all'utente A arriva solo ai socket di A, mai a quelli di B.
func TestHub_NotificationOnlyReachesOwner(t *testing.T) {
	hub := NewHub("")
	srv := newWSServer(t, hub)

	a := mustDial(t, srv, "user-a", "P")
	b := mustDial(t, srv, "user-b", "P")
	waitCount(t, hub, "user-a", 1)
	waitCount(t, hub, "user-b", 1)

	hub.SendToUser("user-a", []byte(`{"for":"a"}`))

	expectMessage(t, a, `{"for":"a"}`)
	expectSilence(t, b)
}

// TestHub_SendToUserReachesAllSocketsOfThatUser: lo stesso utente con due
// socket (due tab) riceve il messaggio su entrambi.
func TestHub_SendToUserReachesAllSocketsOfThatUser(t *testing.T) {
	hub := NewHub("")
	srv := newWSServer(t, hub)

	a1 := mustDial(t, srv, "user-a", "P")
	a2 := mustDial(t, srv, "user-a", "Q")
	waitCount(t, hub, "user-a", 2)

	hub.SendToUser("user-a", []byte("hi"))
	expectMessage(t, a1, "hi")
	expectMessage(t, a2, "hi")
}

// TestHub_ProjectRoomIsScoped: un client sulla board di P1 non riceve gli
// eventi della stanza P2.
func TestHub_ProjectRoomIsScoped(t *testing.T) {
	hub := NewHub("")
	srv := newWSServer(t, hub)

	a := mustDial(t, srv, "user-a", "P1")
	b := mustDial(t, srv, "user-b", "P2")
	waitCount(t, hub, "user-a", 1)
	waitCount(t, hub, "user-b", 1)

	hub.SendToProject("P1", []byte("p1-event"))
	expectMessage(t, a, "p1-event")
	expectSilence(t, b)
}

// TestCheckOrigin_RejectsForeignOrigin: Origin estraneo → handshake rifiutato
// (403); Origin uguale a APP_BASE_URL (scheme+host, path ignorato) → ok;
// Origin same-host della richiesta → ok; nessun Origin (client non browser)
// → ok.
func TestCheckOrigin_RejectsForeignOrigin(t *testing.T) {
	hub := NewHub("http://app.example:3000/app")
	srv := newWSServer(t, hub)

	conn, res, err := dial(t, srv, "u", "P", http.Header{"Origin": {"https://evil.example"}})
	if err == nil {
		conn.Close()
		t.Fatal("foreign Origin: expected handshake to be refused")
	}
	if res == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign Origin: status = %v, want 403", res)
	}

	if _, _, err := dial(t, srv, "u", "P", http.Header{"Origin": {"http://app.example:3000"}}); err != nil {
		t.Fatalf("APP_BASE_URL Origin: expected handshake to succeed, got %v", err)
	}

	sameHost := "http://" + strings.TrimPrefix(srv.URL, "http://")
	if _, _, err := dial(t, srv, "u", "P", http.Header{"Origin": {sameHost}}); err != nil {
		t.Fatalf("same-host Origin %s: expected handshake to succeed, got %v", sameHost, err)
	}

	if _, _, err := dial(t, srv, "u", "P", nil); err != nil {
		t.Fatalf("no Origin: expected handshake to succeed, got %v", err)
	}
}

// TestHub_UnregistersOnClose: alla chiusura del socket lato client l'hub
// rileva il peer morto (read pump) e rimuove la connessione.
func TestHub_UnregistersOnClose(t *testing.T) {
	hub := NewHub("")
	srv := newWSServer(t, hub)

	conn := mustDial(t, srv, "user-a", "P")
	waitCount(t, hub, "user-a", 1)

	_ = conn.Close()
	waitCount(t, hub, "user-a", 0)

	// Inviare a un utente senza socket non deve bloccare né andare in panic.
	hub.SendToUser("user-a", []byte("late"))
}

// TestHub_UnregistersOnWriteError: se il peer sparisce senza close frame, il
// write pump vede l'errore di scrittura e rimuove la connessione.
func TestHub_UnregistersOnWriteError(t *testing.T) {
	hub := NewHub("")
	srv := newWSServer(t, hub)

	conn := mustDial(t, srv, "user-a", "P")
	waitCount(t, hub, "user-a", 1)

	// Chiude il TCP sottostante senza handshake di chiusura.
	_ = conn.NetConn().Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.ClientCount("user-a") != 0 {
		hub.SendToUser("user-a", []byte("ping-the-dead"))
		time.Sleep(10 * time.Millisecond)
	}
	if got := hub.ClientCount("user-a"); got != 0 {
		t.Fatalf("ClientCount after dead peer = %d, want 0", got)
	}
}
