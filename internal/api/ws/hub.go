// Package ws è l'hub WebSocket del server: tiene le connessioni aperte
// indicizzate per utente (un utente può avere più socket, es. più tab) e per
// stanza-progetto (il socket board di /ws/v1/projects/{key}/board), e
// consegna i messaggi solo ai destinatari legittimi.
//
// Round 27, Task 6: prima dell'hardening l'hub aveva un solo fan-out
// globale (Broadcast) usato dalle notifiche → ogni client riceveva le
// notifiche di tutti gli utenti; CheckOrigin accettava qualunque Origin; non
// c'era read pump, quindi un peer morto restava registrato finché il buffer
// di invio non si riempiva.
package ws

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait è il tempo massimo concesso a una scrittura verso il peer.
	writeWait = 10 * time.Second
	// pongWait è il tempo massimo di attesa del pong (o di qualunque frame)
	// dal peer: superato, la connessione è considerata morta.
	pongWait = 60 * time.Second
	// pingPeriod è l'intervallo tra i ping; deve essere < pongWait.
	pingPeriod = (pongWait * 9) / 10
	// maxMessageSize limita i frame in ingresso: il client non invia dati
	// applicativi, solo frame di controllo.
	maxMessageSize = 512
	// sendBufferSize è la capacità del buffer per connessione: un client che
	// non svuota il buffer viene scartato invece di bloccare l'hub.
	sendBufferSize = 256
)

// Client è una singola connessione WebSocket registrata nell'hub.
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	userID  string
	project string
}

// Hub indicizza i client per utente e per stanza-progetto. Le mappe sono
// protette da mu: le consegne (SendToUser/SendToProject) prendono il read
// lock e fanno invii non bloccanti; register/unregister prendono il write
// lock. send viene chiuso solo in unregister sotto write lock, quindi nessun
// invio può mai avvenire su un canale chiuso.
type Hub struct {
	mu        sync.RWMutex
	byUser    map[string]map[*Client]struct{}
	byProject map[string]map[*Client]struct{}
	upgrader  websocket.Upgrader

	// allowedOrigin è "scheme://host" di APP_BASE_URL (vuoto se non
	// configurato/parsabile): l'unico Origin browser accettato oltre al
	// same-host della richiesta.
	allowedOrigin string
}

// NewHub costruisce l'hub. baseURL è APP_BASE_URL: ne viene usato solo
// scheme+host per il controllo Origin degli handshake browser.
func NewHub(baseURL string) *Hub {
	h := &Hub{
		byUser:        make(map[string]map[*Client]struct{}),
		byProject:     make(map[string]map[*Client]struct{}),
		allowedOrigin: originOf(baseURL),
	}
	h.upgrader = websocket.Upgrader{CheckOrigin: h.checkOrigin}
	return h
}

// originOf riduce un URL alla sua origin "scheme://host" (host include la
// porta se presente), in minuscolo. Stringa vuota se non parsabile.
func originOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Scheme + "://" + u.Host)
}

// checkOrigin accetta l'handshake se: non c'è header Origin (client non
// browser, già autenticato dal bearer); oppure l'Origin coincide con
// APP_BASE_URL (scheme+host); oppure l'host dell'Origin coincide con l'Host
// della richiesta (same-host, il default di gorilla). Tutto il resto è
// rifiutato (gorilla risponde 403).
func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	if h.allowedOrigin != "" && strings.ToLower(u.Scheme+"://"+u.Host) == h.allowedOrigin {
		return true
	}
	return strings.EqualFold(u.Host, r.Host)
}

// ServeWs esegue l'upgrade e registra la connessione per userID nella stanza
// del progetto projectKey. Autenticazione e autorizzazione (bearer + permesso
// sul progetto) sono responsabilità del router: qui l'identità è già
// verificata.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, userID, projectKey string) {
	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade ha già scritto la risposta HTTP (403 per Origin rifiutato,
		// 400 per handshake malformato).
		return
	}
	client := &Client{
		hub:     hub,
		conn:    conn,
		send:    make(chan []byte, sendBufferSize),
		userID:  userID,
		project: projectKey,
	}
	hub.register(client)
	go client.writePump()
	go client.readPump()
}

// SendToUser consegna msg a tutti i socket aperti di userID (e solo a quelli).
// I client con buffer pieno vengono rimossi e chiusi invece di bloccare.
func (h *Hub) SendToUser(userID string, msg []byte) {
	h.mu.RLock()
	dead := h.deliver(h.byUser[userID], msg)
	h.mu.RUnlock()
	for _, c := range dead {
		h.unregister(c)
	}
}

// SendToProject consegna msg a tutti i socket registrati nella stanza del
// progetto projectKey (board events), a prescindere dall'utente.
func (h *Hub) SendToProject(projectKey string, msg []byte) {
	h.mu.RLock()
	dead := h.deliver(h.byProject[projectKey], msg)
	h.mu.RUnlock()
	for _, c := range dead {
		h.unregister(c)
	}
}

// deliver fa un invio non bloccante a ogni client del set e restituisce
// quelli con buffer pieno. Va chiamata con mu in read lock.
func (h *Hub) deliver(set map[*Client]struct{}, msg []byte) []*Client {
	var dead []*Client
	for c := range set {
		select {
		case c.send <- msg:
		default:
			dead = append(dead, c)
		}
	}
	return dead
}

// ClientCount restituisce quanti socket ha aperti userID (test/diagnostica).
func (h *Hub) ClientCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.byUser[userID])
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	addTo(h.byUser, c.userID, c)
	addTo(h.byProject, c.project, c)
}

// unregister rimuove il client dagli indici e chiude il canale di invio
// (una sola volta: la chiusura avviene solo se il client era ancora
// registrato). Idempotente: read pump, write pump e deliver possono tutti
// chiamarla per lo stesso client.
func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.byUser[c.userID][c]; !ok {
		return
	}
	removeFrom(h.byUser, c.userID, c)
	removeFrom(h.byProject, c.project, c)
	close(c.send)
}

func addTo(idx map[string]map[*Client]struct{}, key string, c *Client) {
	set, ok := idx[key]
	if !ok {
		set = make(map[*Client]struct{})
		idx[key] = set
	}
	set[c] = struct{}{}
}

func removeFrom(idx map[string]map[*Client]struct{}, key string, c *Client) {
	set := idx[key]
	delete(set, c)
	if len(set) == 0 {
		delete(idx, key)
	}
}

// readPump consuma i frame in ingresso (il client non invia dati
// applicativi: servono solo per i frame di controllo close/pong) e mantiene
// la read deadline rinnovata dai pong. Al primo errore — close frame, peer
// sparito, pong mancato entro pongWait — la connessione viene rimossa
// dall'hub e chiusa, così un peer morto non resta registrato.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump scrive i messaggi dal buffer e invia un ping ogni pingPeriod.
// Un errore di scrittura rimuove la connessione dall'hub; la chiusura di
// send (unregister) fa inviare il close frame e termina il pump.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.hub.unregister(c)
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
