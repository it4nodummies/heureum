package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
)

// tinyPNG è un'immagine PNG minima: la firma di 8 byte è sufficiente perché
// http.DetectContentType riconosca "image/png".
var tinyPNG = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01, 0x02, 0x03}

// buildMultipart costruisce una richiesta multipart/form-data con un solo campo
// "file", impostando esplicitamente il Content-Type della parte.
func buildMultipart(t *testing.T, filename, contentType string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	hdr.Set("Content-Type", contentType)
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("part write: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("mw close: %v", err)
	}
	return &buf, mw.FormDataContentType()
}

func TestAvatarUploadAndServe(t *testing.T) {
	h, s := setupAuthHandler(t)
	defer s.Close()
	u, err := h.svc.Register("avatar@test.com", "avataruser", "Avatar User", "password123")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	dir := t.TempDir()
	avatarH := NewAvatarHandler(s.DB, dir, testBaseURL)

	// --- Upload ---
	body, ctype := buildMultipart(t, "me.png", "image/png", tinyPNG)
	req := httptest.NewRequest("POST", "/rest/api/3/myself/avatar", body)
	req.Header.Set("Content-Type", ctype)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
	rec := httptest.NewRecorder()
	avatarH.Upload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var got v3.User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, rec.Body.String())
	}
	wantURL := "/rest/api/3/user/avatar/" + u.ID
	if got.AvatarUrls["48x48"] != wantURL {
		t.Fatalf("avatarUrls[48x48] = %q, want %q", got.AvatarUrls["48x48"], wantURL)
	}

	// --- Serve (public) ---
	sreq := httptest.NewRequest("GET", "/rest/api/3/user/avatar/"+u.ID, nil)
	sreq.SetPathValue("userId", u.ID)
	srec := httptest.NewRecorder()
	avatarH.Serve(srec, sreq)
	if srec.Code != http.StatusOK {
		t.Fatalf("serve status = %d, want 200, body: %s", srec.Code, srec.Body.String())
	}
	if !bytes.Equal(srec.Body.Bytes(), tinyPNG) {
		t.Fatalf("served bytes do not match uploaded PNG")
	}
	if ct := srec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		t.Fatalf("served Content-Type = %q, want image/*", ct)
	}
}

// TestAvatarServeRejectsGlobChars verifica che un userId con metacaratteri di
// glob (es. "*") non serva l'avatar di un altro utente ma dia 404: senza la
// validazione charset, filepath.Glob("<dir>/*.*") matcherebbe un file altrui.
func TestAvatarServeRejectsGlobChars(t *testing.T) {
	h, s := setupAuthHandler(t)
	defer s.Close()
	u, err := h.svc.Register("victim@test.com", "victim", "Victim", "password123")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	dir := t.TempDir()
	avatarH := NewAvatarHandler(s.DB, dir, testBaseURL)

	// Carica un avatar reale per la vittima, così esiste un file sul disco.
	body, ctype := buildMultipart(t, "v.png", "image/png", tinyPNG)
	req := httptest.NewRequest("POST", "/rest/api/3/myself/avatar", body)
	req.Header.Set("Content-Type", ctype)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
	rec := httptest.NewRecorder()
	avatarH.Upload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup upload status = %d, want 200", rec.Code)
	}

	for _, bad := range []string{"*", "?", "[a-z]", "a/b", ".."} {
		sreq := httptest.NewRequest("GET", "/rest/api/3/user/avatar/"+bad, nil)
		sreq.SetPathValue("userId", bad)
		srec := httptest.NewRecorder()
		avatarH.Serve(srec, sreq)
		if srec.Code != http.StatusNotFound {
			t.Fatalf("serve userId=%q status = %d, want 404 (must not serve another user's avatar)", bad, srec.Code)
		}
	}
}

func TestAvatarUploadRejectsNonImage(t *testing.T) {
	h, s := setupAuthHandler(t)
	defer s.Close()
	u, err := h.svc.Register("txt@test.com", "txtuser", "Txt User", "password123")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	dir := t.TempDir()
	avatarH := NewAvatarHandler(s.DB, dir, testBaseURL)

	body, ctype := buildMultipart(t, "note.txt", "text/plain", []byte("this is not an image at all"))
	req := httptest.NewRequest("POST", "/rest/api/3/myself/avatar", body)
	req.Header.Set("Content-Type", ctype)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, u.ID))
	rec := httptest.NewRecorder()
	avatarH.Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("upload status = %d, want 400, body: %s", rec.Code, rec.Body.String())
	}
}
