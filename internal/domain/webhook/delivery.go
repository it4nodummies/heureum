package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
)

// Sign calcola la firma HMAC-SHA256 del body col secret, come "sha256=<hex>".
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Deliver esegue la POST del payload al webhook, con header di firma ed evento,
// e restituisce una Delivery (non persistita: il chiamante la registra). Non
// solleva: cattura gli errori nel campo Error della Delivery.
func Deliver(client *http.Client, hook Webhook, eventType string, body []byte) Delivery {
	d := Delivery{WebhookID: hook.ID, EventType: eventType, URL: hook.URL}
	req, err := http.NewRequest(http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		d.Error = err.Error()
		return d
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Heureum-Event", eventType)
	req.Header.Set("X-Heureum-Signature", Sign(hook.Secret, body))
	resp, err := client.Do(req)
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
