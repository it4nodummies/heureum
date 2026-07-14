package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
		gotSig = r.Header.Get("X-OpenJira-Signature")
		gotEvent = r.Header.Get("X-OpenJira-Event")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	hook := Webhook{ID: "h1", URL: srv.URL, Secret: "topsecret"}
	body := []byte(`{"event":"issue_created"}`)
	d := Deliver(client, hook, "issue_created", body)

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
	client := &http.Client{Timeout: 1 * time.Second}
	hook := Webhook{ID: "h1", URL: "http://127.0.0.1:0/nope", Secret: ""}
	d := Deliver(client, hook, "issue_created", []byte("{}"))
	if d.Success {
		t.Error("delivery verso URL non valida deve fallire")
	}
	if d.Error == "" {
		t.Error("errore atteso valorizzato")
	}
}
