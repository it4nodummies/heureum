package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// uploadAttachment esegue una POST multipart su /issue/{key}/attachments con
// un singolo file (campo "file"), come farebbe un browser da un form di
// upload. Restituisce la risposta grezza per farne le asserzioni nel test
// chiamante.
func uploadAttachment(t *testing.T, srv *httptest.Server, jwt, issueKey, filename string, content []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/rest/api/3/issue/"+issueKey+"/attachments", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestAttachments_UploadAndListForIssue copre il Task 5 di Round 13: upload
// multipart -> shape v3 (filename/size/mimeType/content), poi
// GET /issue/{key}/attachments -> lista con lo stesso shape. Il campo
// "content" deve puntare a /rest/api/3/attachment/content/{id} (URL di
// download), non al file_path grezzo sul filesystem.
func TestAttachments_UploadAndListForIssue(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "ATT", "Attachment Proj")
	key := createIssueViaAPI(t, srv, jwt, "ATT", "Issue with attachment")

	res := uploadAttachment(t, srv, jwt, key, "hello.txt", []byte("hello world"))
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("upload attachment status = %d: %s", res.StatusCode, b)
	}
	uploaded, _ := decodeBody(t, res)
	if uploaded["filename"] != "hello.txt" {
		t.Errorf("filename = %v, atteso hello.txt", uploaded["filename"])
	}
	size, _ := uploaded["size"].(float64)
	if size <= 0 {
		t.Errorf("size = %v, atteso > 0", uploaded["size"])
	}
	id, _ := uploaded["id"].(string)
	if id == "" {
		t.Fatalf("id assente nella risposta di upload")
	}
	wantContent := "/rest/api/3/attachment/content/" + id
	if uploaded["content"] != wantContent {
		t.Errorf("content = %v, atteso %v", uploaded["content"], wantContent)
	}
	mimeType, _ := uploaded["mimeType"].(string)
	if mimeType == "" {
		t.Errorf("mimeType assente/vuoto")
	}

	listRes := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/issue/"+key+"/attachments", nil)
	if listRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(listRes.Body)
		listRes.Body.Close()
		t.Fatalf("list attachments status = %d: %s", listRes.StatusCode, b)
	}
	defer listRes.Body.Close()
	var list []map[string]any
	if err := json.NewDecoder(listRes.Body).Decode(&list); err != nil {
		t.Fatalf("decode attachments list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("atteso 1 allegato, ottenuti %d", len(list))
	}
	got := list[0]
	if got["filename"] != "hello.txt" {
		t.Errorf("list[0].filename = %v, atteso hello.txt", got["filename"])
	}
	if got["content"] != wantContent {
		t.Errorf("list[0].content = %v, atteso %v", got["content"], wantContent)
	}
	if got["id"] != id {
		t.Errorf("list[0].id = %v, atteso %v", got["id"], id)
	}
}
