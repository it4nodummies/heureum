package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Sign calcola la firma HMAC-SHA256 del body col secret, come "sha256=<hex>".
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// deliveryTimeout bounds one delivery attempt end to end (connect + TLS +
// request + response), so a slow target cannot stall the worker tick.
const deliveryTimeout = 10 * time.Second

// NewDeliveryClient returns the http.Client used for outgoing webhooks:
//   - never follows redirects (a public host could otherwise bounce the
//     signed POST to a cluster-internal address): the 3xx is returned as-is
//     and recorded as a failed attempt;
//   - 10s overall timeout;
//   - the dialer resolves the host through LookupIP, applies the same SSRF
//     policy as ValidateTargetURL (with allow) and connects only to an
//     address that passed — so a DNS answer that changes between the
//     pre-send check and the connect still cannot reach a private target.
//
// TLS keeps verifying the URL's hostname: only the TCP destination is pinned.
func NewDeliveryClient(allow []string) *http.Client {
	al := parseAllowlist(allow)
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // the policy is on the destination; never hand it to an env proxy
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := resolveAllowed(ctx, host, al)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
	return &http.Client{
		Timeout:   deliveryTimeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Deliverer esegue le consegne dei webhook in uscita con il client indurito e
// l'allowlist dell'operatore (APP_WEBHOOK_ALLOW_PRIVATE_HOSTS). Un'istanza per
// processo (worker): il client è condiviso tra le consegne.
type Deliverer struct {
	client *http.Client
	allow  []string
}

// NewDeliverer builds a Deliverer over NewDeliveryClient(allow).
func NewDeliverer(allow []string) *Deliverer {
	return &Deliverer{client: NewDeliveryClient(allow), allow: allow}
}

// Deliver esegue la POST del payload al webhook, con header di firma ed evento,
// e restituisce una Delivery (non persistita: il chiamante la registra). Non
// solleva: cattura gli errori nel campo Error della Delivery.
//
// L'URL è rivalidata PRIMA di inviare (stessa policy della create): un hook
// il cui nome oggi risolve a un indirizzo privato (DNS rebinding dopo la
// create) è marcato fallito senza alcuna richiesta — StatusCode resta 0.
func (dl *Deliverer) Deliver(hook Webhook, eventType string, body []byte) Delivery {
	d := Delivery{WebhookID: hook.ID, EventType: eventType, URL: hook.URL}
	if err := ValidateTargetURL(hook.URL, dl.allow); err != nil {
		d.Error = "not sent: " + err.Error()
		return d
	}
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		d.Error = err.Error()
		return d
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Heureum-Event", eventType)
	req.Header.Set("X-Heureum-Signature", Sign(hook.Secret, body))
	resp, err := dl.client.Do(req)
	if err != nil {
		d.Error = err.Error()
		return d
	}
	defer resp.Body.Close()
	d.StatusCode = resp.StatusCode
	d.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	if !d.Success {
		d.Error = fmt.Sprintf("non-2xx status: %d", resp.StatusCode)
	}
	return d
}
