package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestGroups_CRUDConformant esercita create/get/member/picker per i gruppi e
// valida ogni risposta contro lo schema Jira v3 (GroupRef, PageBeanUserDetails,
// FoundGroups).
func TestGroups_CRUDConformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	v := MustLoad(t, specPath)

	// create
	res := doJSON(t, srv, http.MethodPost, tok, "/rest/api/3/group", map[string]any{"name": "developers"})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST /group status = %d, want 201", res.StatusCode)
	}
	created, rawCreated := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodPost, "/rest/api/3/group", res.StatusCode, res.Header, strings.NewReader(string(rawCreated))); err != nil {
		t.Errorf("POST /group non conforme: %v", err)
	}
	if created["name"] != "developers" {
		t.Errorf("group creato name = %v, want developers", created["name"])
	}

	// get by groupname
	res = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/group?groupname=developers", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /group status = %d, want 200", res.StatusCode)
	}
	_, rawGet := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/group", res.StatusCode, res.Header, strings.NewReader(string(rawGet))); err != nil {
		t.Errorf("GET /group non conforme: %v", err)
	}

	// members (empty page, ma con shape PageBeanUserDetails)
	res = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/group/member?groupname=developers", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /group/member status = %d, want 200", res.StatusCode)
	}
	body, rawMembers := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/group/member", res.StatusCode, res.Header, strings.NewReader(string(rawMembers))); err != nil {
		t.Errorf("GET /group/member non conforme: %v", err)
	}
	if _, ok := body["values"]; !ok {
		t.Error("member page deve avere values")
	}

	// picker
	res = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/groups/picker?query=dev", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /groups/picker status = %d, want 200", res.StatusCode)
	}
	_, rawPicker := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/groups/picker", res.StatusCode, res.Header, strings.NewReader(string(rawPicker))); err != nil {
		t.Errorf("GET /groups/picker non conforme: %v", err)
	}
}

// TestPermissions_Conformant valida /permissions (catalogo completo) e
// /mypermissions (con havePermission derivato dal ruolo dell'utente corrente).
func TestPermissions_Conformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	v := MustLoad(t, specPath)

	res := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/permissions", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /permissions status = %d, want 200", res.StatusCode)
	}
	_, rawPerms := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/permissions", res.StatusCode, res.Header, strings.NewReader(string(rawPerms))); err != nil {
		t.Errorf("GET /permissions non conforme: %v", err)
	}

	res = doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/mypermissions", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /mypermissions status = %d, want 200", res.StatusCode)
	}
	body, rawMy := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/mypermissions", res.StatusCode, res.Header, strings.NewReader(string(rawMy))); err != nil {
		t.Errorf("GET /mypermissions non conforme: %v", err)
	}
	perms, ok := body["permissions"].(map[string]any)
	if !ok || len(perms) == 0 {
		t.Error("mypermissions deve avere una mappa permissions non vuota")
	}
}

// TestUserSearch_Conformant valida GET /user/search contro lo schema array di
// User del contratto v3. registerAndLogin crea un solo utente (Alice); la
// query "a" può quindi matchare quell'utente o restituire lista vuota a
// seconda del matching sql — l'importante è status 200 + shape conforme, non
// un conteggio specifico.
func TestUserSearch_Conformant(t *testing.T) {
	srv, authSvc := newTestServer(t)
	tok := registerAndLogin(t, authSvc)
	v := MustLoad(t, specPath)

	res := doJSON(t, srv, http.MethodGet, tok, "/rest/api/3/user/search?query=a", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /user/search status = %d, want 200", res.StatusCode)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/user/search", res.StatusCode, res.Header, strings.NewReader(string(raw))); err != nil {
		t.Errorf("GET /user/search non conforme: %v", err)
	}
	var users []map[string]any
	if err := json.Unmarshal(raw, &users); err != nil {
		t.Fatalf("decode user/search array: %v (body=%s)", err, raw)
	}
}
