package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// loopbackAllow lets tests deliver to httptest servers (127.0.0.1), which the
// SSRF policy would otherwise reject — exactly what a self-hosted operator does
// with APP_WEBHOOK_ALLOW_PRIVATE_HOSTS.
var loopbackAllow = []string{"127.0.0.1"}

func TestSign(t *testing.T) {
	got := Sign("secret", []byte("body"))
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("body"))
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got != want {
		t.Errorf("Sign = %q want %q", got, want)
	}
}

func TestDeliver_PostsSignedPayload(t *testing.T) {
	var gotSig, gotBody, gotEvent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Heureum-Signature")
		gotEvent = r.Header.Get("X-Heureum-Event")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	hook := Webhook{ID: "h1", URL: srv.URL, Secret: "topsecret"}
	body := []byte(`{"event":"issue_created"}`)
	d := NewDeliverer(loopbackAllow).Deliver(hook, "issue_created", body)

	if !d.Success || d.StatusCode != 200 {
		t.Errorf("delivery non riuscita: %+v", d)
	}
	if gotBody != string(body) {
		t.Errorf("body errato: %q", gotBody)
	}
	if gotEvent != "issue_created" {
		t.Errorf("event header errato: %q", gotEvent)
	}
	if gotSig != Sign("topsecret", body) || !strings.HasPrefix(gotSig, "sha256=") {
		t.Errorf("firma errata: %q", gotSig)
	}
}

func TestDeliver_RecordsFailureOnBadURL(t *testing.T) {
	hook := Webhook{ID: "h1", URL: "http://127.0.0.1:0/nope", Secret: ""}
	d := NewDeliverer(loopbackAllow).Deliver(hook, "issue_created", []byte("{}"))
	if d.Success {
		t.Error("delivery verso URL non valida deve fallire")
	}
	if d.Error == "" {
		t.Error("errore atteso valorizzato")
	}
}

// TestDeliver_NoRedirectFollow: a 302 from the target must NOT be followed
// (a public host could otherwise bounce the signed POST to a cluster-internal
// address). The recorded status is the 302 itself; the redirect target is
// never requested.
func TestDeliver_NoRedirectFollow(t *testing.T) {
	targetHit := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit = true
		w.WriteHeader(200)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/internal", http.StatusFound)
	}))
	defer redirector.Close()

	d := NewDeliverer(loopbackAllow).Deliver(Webhook{ID: "h1", URL: redirector.URL}, "issue_created", []byte("{}"))
	if targetHit {
		t.Errorf("redirect target was requested: redirects must not be followed")
	}
	if d.Success || d.StatusCode != http.StatusFound {
		t.Errorf("delivery = %+v, want failed with status 302 (redirect not followed)", d)
	}
}

// TestDeliver_RevalidatesAtSendTime: the hook URL passed validation at create
// (public address) but at delivery time the name resolves to a private
// address (DNS rebinding). The delivery must be marked failed WITHOUT any
// request being sent.
func TestDeliver_RevalidatesAtSendTime(t *testing.T) {
	// A real listener on loopback stands in for the "internal" target: if the
	// deliverer ever connects to it, the test fails.
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("internal target received a request: %s %s", r.Method, r.URL)
		w.WriteHeader(200)
	}))
	defer internal.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(internal.URL, "http://"))

	hookURL := "http://hooks.example.com:" + port + "/heureum"
	// create-time: public → accepted
	withResolver(t, map[string][]string{"hooks.example.com": {"93.184.216.34"}})
	if err := ValidateTargetURL(hookURL, nil); err != nil {
		t.Fatalf("create-time validation should accept the public address: %v", err)
	}
	// delivery-time: the same name now points at loopback
	withResolver(t, map[string][]string{"hooks.example.com": {"127.0.0.1"}})

	d := NewDeliverer(nil).Deliver(Webhook{ID: "h1", URL: hookURL}, "issue_created", []byte("{}"))
	if d.Success {
		t.Errorf("delivery to a rebound private address must fail: %+v", d)
	}
	if d.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 (nothing was sent)", d.StatusCode)
	}
	if !strings.Contains(d.Error, ErrPrivateTarget.Error()) {
		t.Errorf("Error = %q, want it to carry %q", d.Error, ErrPrivateTarget.Error())
	}
}

// TestDeliveryClient_DialPinsValidatedAddress: even below Deliver, the client's
// dialer refuses to connect to an address the policy rejects, so a resolver
// answer that changes between the pre-send check and the dial still cannot
// reach a private target.
func TestDeliveryClient_DialPinsValidatedAddress(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("internal target received a request")
		w.WriteHeader(200)
	}))
	defer internal.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(internal.URL, "http://"))
	withResolver(t, map[string][]string{"hooks.example.com": {"127.0.0.1"}})

	client := NewDeliveryClient(nil)
	_, err := client.Post("http://hooks.example.com:"+port+"/x", "application/json", strings.NewReader("{}"))
	if err == nil || !errors.Is(err, ErrPrivateTarget) {
		t.Fatalf("dial to a private resolved address: err = %v, want ErrPrivateTarget", err)
	}
	if client.Timeout == 0 {
		t.Errorf("delivery client must have a timeout")
	}
	if client.CheckRedirect == nil {
		t.Errorf("delivery client must not follow redirects")
	}
	// sanity: the seam really is the only resolver the dialer consults
	ctx := context.Background()
	if _, err := LookupIP(ctx, "nope.invalid"); err == nil {
		t.Errorf("stub resolver should reject unknown hosts")
	}
}
