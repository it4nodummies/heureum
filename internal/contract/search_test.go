package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// doJSON esegue una richiesta HTTP autenticata con corpo JSON opzionale e
// restituisce la risposta. body == nil => nessun corpo (GET/DELETE/favourite
// toggle senza payload).
func doJSON(t *testing.T, srv *httptest.Server, method, jwt, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(method, srv.URL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// decodeBody legge e decodifica il body come mappa, restituendo anche i byte
// grezzi (per poterli rileggere per la validazione OpenAPI).
func decodeBody(t *testing.T, res *http.Response) (map[string]any, []byte) {
	t.Helper()
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, b)
	}
	return out, b
}

func TestSearchJQL_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	createProjectViaAPI(t, srv, jwt, "SR", "Search Proj")
	createIssueViaAPI(t, srv, jwt, "SR", "Trovami")

	v := MustLoad(t, specPath)

	// GET /search/jql
	q := url.Values{"jql": {"project = SR"}}
	res := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/search/jql?"+q.Encode(), nil)
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("GET /search/jql status = %d: %s", res.StatusCode, b)
	}
	body, raw := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/search/jql", res.StatusCode, res.Header, strings.NewReader(string(raw))); err != nil {
		t.Errorf("GET /search/jql non conforme: %v", err)
	}
	if _, ok := body["isLast"]; !ok {
		t.Error("risposta /search/jql deve avere isLast")
	}
	issues, _ := body["issues"].([]any)
	if len(issues) != 1 {
		t.Errorf("attesa 1 issue, got %d", len(issues))
	}

	// POST /search/jql con fields
	res2 := doJSON(t, srv, http.MethodPost, jwt, "/rest/api/3/search/jql", map[string]any{
		"jql": "project = SR", "fields": []string{"summary", "status"}, "maxResults": 10,
	})
	if res2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res2.Body)
		res2.Body.Close()
		t.Fatalf("POST /search/jql status = %d: %s", res2.StatusCode, b)
	}
	_, raw2 := decodeBody(t, res2)
	if err := v.ValidateResponse(http.MethodPost, "/rest/api/3/search/jql", res2.StatusCode, res2.Header, strings.NewReader(string(raw2))); err != nil {
		t.Errorf("POST /search/jql non conforme: %v", err)
	}

	// legacy POST /search offset-paginato
	res3 := doJSON(t, srv, http.MethodPost, jwt, "/rest/api/3/search", map[string]any{"jql": "project = SR"})
	if res3.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res3.Body)
		res3.Body.Close()
		t.Fatalf("POST /search status = %d: %s", res3.StatusCode, b)
	}
	body3, raw3 := decodeBody(t, res3)
	if err := v.ValidateResponse(http.MethodPost, "/rest/api/3/search", res3.StatusCode, res3.Header, strings.NewReader(string(raw3))); err != nil {
		t.Errorf("POST /search non conforme: %v", err)
	}
	if body3["total"] == nil || body3["startAt"] == nil {
		t.Error("risposta legacy /search deve avere total/startAt")
	}

	// legacy GET /search
	res3b := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/search?"+q.Encode(), nil)
	if res3b.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res3b.Body)
		res3b.Body.Close()
		t.Fatalf("GET /search status = %d: %s", res3b.StatusCode, b)
	}
	_, raw3b := decodeBody(t, res3b)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/search", res3b.StatusCode, res3b.Header, strings.NewReader(string(raw3b))); err != nil {
		t.Errorf("GET /search non conforme: %v", err)
	}

	// approximate-count
	res4 := doJSON(t, srv, http.MethodPost, jwt, "/rest/api/3/search/approximate-count", map[string]any{"jql": "project = SR"})
	if res4.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res4.Body)
		res4.Body.Close()
		t.Fatalf("POST /search/approximate-count status = %d: %s", res4.StatusCode, b)
	}
	body4, raw4 := decodeBody(t, res4)
	if err := v.ValidateResponse(http.MethodPost, "/rest/api/3/search/approximate-count", res4.StatusCode, res4.Header, strings.NewReader(string(raw4))); err != nil {
		t.Errorf("POST /search/approximate-count non conforme: %v", err)
	}
	if n, ok := body4["count"].(float64); !ok || n != 1 {
		t.Errorf("count = %v, want 1", body4["count"])
	}
}

func TestSearchJQL_InvalidReturns400(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	q := url.Values{"jql": {"project ="}}
	res := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/search/jql?"+q.Encode(), nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, want 400: %s", res.StatusCode, b)
	}
}

func TestAutocomplete_ConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	res := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/jql/autocompletedata", nil)
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("status = %d: %s", res.StatusCode, b)
	}
	v := MustLoad(t, specPath)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/jql/autocompletedata", res.StatusCode, res.Header, res.Body); err != nil {
		t.Errorf("GET /jql/autocompletedata non conforme: %v", err)
	}
}

func TestFilters_CRUDConformsToContract(t *testing.T) {
	srv, authSvc := newTestServer(t)
	jwt := registerAndLogin(t, authSvc)
	v := MustLoad(t, specPath)

	// create
	res := doJSON(t, srv, http.MethodPost, jwt, "/rest/api/3/filter", map[string]any{
		"name": "My open", "jql": "assignee = currentUser()", "description": "mine",
	})
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("POST /filter status = %d, want 200: %s", res.StatusCode, b)
	}
	created, rawCreated := decodeBody(t, res)
	if err := v.ValidateResponse(http.MethodPost, "/rest/api/3/filter", res.StatusCode, res.Header, strings.NewReader(string(rawCreated))); err != nil {
		t.Errorf("POST /filter non conforme: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("filtro creato senza id")
	}

	// get
	resG := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/filter/"+id, nil)
	if resG.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resG.Body)
		resG.Body.Close()
		t.Fatalf("GET /filter/%s status = %d: %s", id, resG.StatusCode, b)
	}
	_, rawG := decodeBody(t, resG)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/filter/{id}", resG.StatusCode, resG.Header, strings.NewReader(string(rawG))); err != nil {
		t.Errorf("GET /filter/{id} non conforme: %v", err)
	}

	// update (PUT)
	resU := doJSON(t, srv, http.MethodPut, jwt, "/rest/api/3/filter/"+id, map[string]any{
		"name": "My open (renamed)", "jql": "assignee = currentUser()", "description": "mine, renamed",
	})
	if resU.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resU.Body)
		resU.Body.Close()
		t.Fatalf("PUT /filter/%s status = %d: %s", id, resU.StatusCode, b)
	}
	_, rawU := decodeBody(t, resU)
	if err := v.ValidateResponse(http.MethodPut, "/rest/api/3/filter/{id}", resU.StatusCode, resU.Header, strings.NewReader(string(rawU))); err != nil {
		t.Errorf("PUT /filter/{id} non conforme: %v", err)
	}

	// favourite PUT
	resF := doJSON(t, srv, http.MethodPut, jwt, "/rest/api/3/filter/"+id+"/favourite", nil)
	if resF.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resF.Body)
		resF.Body.Close()
		t.Fatalf("PUT /filter/%s/favourite status = %d: %s", id, resF.StatusCode, b)
	}
	fav, rawF := decodeBody(t, resF)
	if err := v.ValidateResponse(http.MethodPut, "/rest/api/3/filter/{id}/favourite", resF.StatusCode, resF.Header, strings.NewReader(string(rawF))); err != nil {
		t.Errorf("PUT /filter/{id}/favourite non conforme: %v", err)
	}
	if fav["favourite"] != true {
		t.Error("dopo PUT favourite, favourite deve essere true")
	}

	// filter/favourite (elenco preferiti dell'utente corrente)
	resFavList := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/filter/favourite", nil)
	if resFavList.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resFavList.Body)
		resFavList.Body.Close()
		t.Fatalf("GET /filter/favourite status = %d: %s", resFavList.StatusCode, b)
	}
	rawFavList, err := io.ReadAll(resFavList.Body)
	resFavList.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/filter/favourite", resFavList.StatusCode, resFavList.Header, strings.NewReader(string(rawFavList))); err != nil {
		t.Errorf("GET /filter/favourite non conforme: %v", err)
	}

	// favourite DELETE (rimuove dai preferiti)
	resFD := doJSON(t, srv, http.MethodDelete, jwt, "/rest/api/3/filter/"+id+"/favourite", nil)
	if resFD.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resFD.Body)
		resFD.Body.Close()
		t.Fatalf("DELETE /filter/%s/favourite status = %d: %s", id, resFD.StatusCode, b)
	}
	favD, rawFD := decodeBody(t, resFD)
	if err := v.ValidateResponse(http.MethodDelete, "/rest/api/3/filter/{id}/favourite", resFD.StatusCode, resFD.Header, strings.NewReader(string(rawFD))); err != nil {
		t.Errorf("DELETE /filter/{id}/favourite non conforme: %v", err)
	}
	if favD["favourite"] != false {
		t.Error("dopo DELETE favourite, favourite deve essere false")
	}

	// filter/search paginato
	resS := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/filter/search", nil)
	if resS.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resS.Body)
		resS.Body.Close()
		t.Fatalf("GET /filter/search status = %d: %s", resS.StatusCode, b)
	}
	bodyS, rawS := decodeBody(t, resS)
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/filter/search", resS.StatusCode, resS.Header, strings.NewReader(string(rawS))); err != nil {
		t.Errorf("GET /filter/search non conforme: %v", err)
	}
	values, _ := bodyS["values"].([]any)
	if len(values) < 1 {
		t.Errorf("GET /filter/search values = %d, want >= 1", len(values))
	}

	// filter/my (elenco non paginato)
	resMy := doJSON(t, srv, http.MethodGet, jwt, "/rest/api/3/filter/my", nil)
	if resMy.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resMy.Body)
		resMy.Body.Close()
		t.Fatalf("GET /filter/my status = %d: %s", resMy.StatusCode, b)
	}
	rawMy, err := io.ReadAll(resMy.Body)
	resMy.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ValidateResponse(http.MethodGet, "/rest/api/3/filter/my", resMy.StatusCode, resMy.Header, strings.NewReader(string(rawMy))); err != nil {
		t.Errorf("GET /filter/my non conforme: %v", err)
	}
	var myFilters []any
	if err := json.Unmarshal(rawMy, &myFilters); err != nil {
		t.Fatal(err)
	}
	if len(myFilters) < 1 {
		t.Errorf("GET /filter/my len = %d, want >= 1", len(myFilters))
	}

	// delete
	resD := doJSON(t, srv, http.MethodDelete, jwt, "/rest/api/3/filter/"+id, nil)
	defer resD.Body.Close()
	if resD.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resD.Body)
		t.Fatalf("DELETE /filter/%s status = %d, want 204: %s", id, resD.StatusCode, b)
	}
}
