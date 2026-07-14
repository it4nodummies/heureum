package contract

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"
)

// TestAuthzRead_NonMemberGets404 verifica che un utente non membro del
// progetto riceva 404 (non 403) su singole risorse: progetto, issue, board.
// Il 404 (anziché 403) è intenzionale: non deve trapelare l'esistenza della
// risorsa a chi non ne fa parte. Alice (creator/membro implicito) resta un
// controllo positivo nello stesso test.
func TestAuthzRead_NonMemberGets404(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob-nonmember@example.com", "bob")

	createProjectViaAPI(t, srv, alice, "RD", "R&D")
	issueKey := createIssueViaAPI(t, srv, alice, "RD", "Segreto")

	// Positivo: alice vede il proprio progetto.
	res := doJSON(t, srv, http.MethodGet, alice, "/rest/api/3/project/RD", nil)
	if res.StatusCode != http.StatusOK {
		b, _ := decodeBody(t, res)
		t.Fatalf("alice GET /project/RD status = %d, want 200: %v", res.StatusCode, b)
	}
	res.Body.Close()

	// Negativo: bob non è membro, deve ricevere 404 sulle tre risorse.
	res = doJSON(t, srv, http.MethodGet, bob, "/rest/api/3/project/RD", nil)
	if res.StatusCode != http.StatusNotFound {
		b, raw := decodeBody(t, res)
		t.Errorf("bob GET /project/RD status = %d, want 404: %v (%s)", res.StatusCode, b, raw)
	} else {
		res.Body.Close()
	}

	res = doJSON(t, srv, http.MethodGet, bob, "/rest/api/3/issue/"+issueKey, nil)
	if res.StatusCode != http.StatusNotFound {
		b, raw := decodeBody(t, res)
		t.Errorf("bob GET /issue/%s status = %d, want 404: %v (%s)", issueKey, res.StatusCode, b, raw)
	} else {
		res.Body.Close()
	}

	res = doJSON(t, srv, http.MethodGet, bob, "/rest/api/3/project/RD/board", nil)
	if res.StatusCode != http.StatusNotFound {
		b, raw := decodeBody(t, res)
		t.Errorf("bob GET /project/RD/board status = %d, want 404: %v (%s)", res.StatusCode, b, raw)
	} else {
		res.Body.Close()
	}
}

// TestAuthzRead_ListsAreFiltered verifica che le liste (project/search,
// search/jql) non trapelino l'esistenza di progetti a cui l'utente non
// appartiene: né tramite un 404 puntuale (qui il rischio è l'omissione
// silenziosa dai risultati), né tramite conteggi diversi da zero.
func TestAuthzRead_ListsAreFiltered(t *testing.T) {
	srv, authSvc, _ := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob-lists@example.com", "bob")

	createProjectViaAPI(t, srv, alice, "RD", "R&D")
	createIssueViaAPI(t, srv, alice, "RD", "Segreto")

	// project/search: bob non deve vedere RD tra i values.
	res := doJSON(t, srv, http.MethodGet, bob, "/rest/api/3/project/search", nil)
	if res.StatusCode != http.StatusOK {
		b, raw := decodeBody(t, res)
		t.Fatalf("bob GET /project/search status = %d, want 200: %v (%s)", res.StatusCode, b, raw)
	}
	body, raw := decodeBody(t, res)
	values, _ := body["values"].([]any)
	for _, v := range values {
		m, ok := v.(map[string]any)
		if ok && m["key"] == "RD" {
			t.Errorf("bob GET /project/search values contiene RD, non dovrebbe: %s", raw)
		}
	}

	// search/jql project=RD: bob deve vedere 0 issue (non un errore, un
	// filtro silenzioso: JQL su un progetto invisibile non deve rivelarne
	// l'esistenza né i contenuti).
	q := url.Values{"jql": {"project = RD"}}
	res = doJSON(t, srv, http.MethodGet, bob, "/rest/api/3/search/jql?"+q.Encode(), nil)
	if res.StatusCode != http.StatusOK {
		b, raw := decodeBody(t, res)
		t.Fatalf("bob GET /search/jql status = %d, want 200: %v (%s)", res.StatusCode, b, raw)
	}
	body, raw = decodeBody(t, res)
	issues, _ := body["issues"].([]any)
	if len(issues) != 0 {
		t.Errorf("bob /search/jql project=RD issues = %d, want 0: %s", len(issues), raw)
	}
}

// TestAuthzRead_UserDirectoryHidesEmails verifica che la directory utenti
// nasconda l'email ai chiamanti non-admin, e che promuovendo il chiamante ad
// admin globale l'email torni visibile (prova che il branch admin esiste
// davvero e non è morto).
func TestAuthzRead_UserDirectoryHidesEmails(t *testing.T) {
	srv, authSvc, db := newTestServerDB(t)
	_ = registerAndLogin(t, authSvc) // alice@example.com
	bob := registerUserAndLogin(t, authSvc, "bob-directory@example.com", "bob")

	// user/search e users/search restituiscono un array JSON (non un
	// oggetto), quindi leggiamo il body raw invece di usare decodeBody
	// (che si aspetta una mappa e fallirebbe il decode).
	rawBodyOf := func(path string) []byte {
		t.Helper()
		res := doJSON(t, srv, http.MethodGet, bob, path, nil)
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("bob GET %s status = %d, want 200: %s", path, res.StatusCode, b)
		}
		raw, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}

	assertNoEmail := func(path string) {
		t.Helper()
		raw := rawBodyOf(path)
		if bytes.Contains(raw, []byte("alice@example.com")) {
			t.Errorf("bob GET %s espone alice@example.com senza essere admin: %s", path, raw)
		}
	}

	assertNoEmail("/rest/api/3/user/search?query=alice")
	assertNoEmail("/rest/api/3/users/search?query=alice")

	promoteAdmin(t, db, "bob-directory@example.com")

	raw := rawBodyOf("/rest/api/3/user/search?query=alice")
	if !bytes.Contains(raw, []byte("alice@example.com")) {
		t.Errorf("admin bob GET /user/search dovrebbe esporre alice@example.com, body=%s", raw)
	}
}

// TestAuthzRead_GlobalAdminSeesAll verifica che un admin globale, promosso
// dopo la creazione del progetto altrui, veda il progetto e le sue issue: la
// promozione ad admin deve avere effetto immediato senza richiedere un nuovo
// login (il Checker rilegge is_admin dal DB ad ogni richiesta).
func TestAuthzRead_GlobalAdminSeesAll(t *testing.T) {
	srv, authSvc, db := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob-admin@example.com", "bob")

	createProjectViaAPI(t, srv, alice, "RD", "R&D")
	createIssueViaAPI(t, srv, alice, "RD", "Segreto")

	promoteAdmin(t, db, "bob-admin@example.com")

	res := doJSON(t, srv, http.MethodGet, bob, "/rest/api/3/project/RD", nil)
	if res.StatusCode != http.StatusOK {
		b, raw := decodeBody(t, res)
		t.Fatalf("admin bob GET /project/RD status = %d, want 200: %v (%s)", res.StatusCode, b, raw)
	}

	qAdmin := url.Values{"jql": {"project = RD"}}
	res = doJSON(t, srv, http.MethodGet, bob, "/rest/api/3/search/jql?"+qAdmin.Encode(), nil)
	if res.StatusCode != http.StatusOK {
		b, raw := decodeBody(t, res)
		t.Fatalf("admin bob GET /search/jql status = %d, want 200: %v (%s)", res.StatusCode, b, raw)
	}
	body, raw := decodeBody(t, res)
	issues, _ := body["issues"].([]any)
	if len(issues) < 1 {
		t.Errorf("admin bob /search/jql project=RD issues = %d, want >= 1: %s", len(issues), raw)
	}
}
