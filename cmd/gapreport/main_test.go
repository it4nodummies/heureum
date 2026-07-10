package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExtractRoutes(t *testing.T) {
	src := `
		mux.HandleFunc("POST /rest/api/3/auth/login", authH.Login)
		mux.Handle("GET /rest/api/3/project/{key}", authMw(http.HandlerFunc(projectH.Get)))
	`
	routes := extractRoutes(src)
	want := []Route{
		{Method: "POST", Path: "/rest/api/3/auth/login"},
		{Method: "GET", Path: "/rest/api/3/project/{key}"},
	}
	if len(routes) != len(want) {
		t.Fatalf("got %d routes, want %d", len(routes), len(want))
	}
	for i := range want {
		if routes[i] != want[i] {
			t.Errorf("route %d: got %+v, want %+v", i, routes[i], want[i])
		}
	}
}

func TestExtractRoutesAllMethods(t *testing.T) {
	// La regex deve riconoscere lo stesso insieme di metodi di httpMethods.
	src := `
		mux.HandleFunc("HEAD /a", h)
		mux.HandleFunc("OPTIONS /b", h)
		mux.HandleFunc("TRACE /c", h)
	`
	routes := extractRoutes(src)
	want := []Route{
		{Method: "HEAD", Path: "/a"},
		{Method: "OPTIONS", Path: "/b"},
		{Method: "TRACE", Path: "/c"},
	}
	if !reflect.DeepEqual(routes, want) {
		t.Errorf("got %+v, want %+v", routes, want)
	}
	for m := range httpMethods {
		if !routeRe.MatchString(`mux.HandleFunc("` + m + ` /x", h)`) {
			t.Errorf("routeRe does not match method %s present in httpMethods", m)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	// I nomi dei parametri non contano nel confronto: {key} e {projectIdOrKey} sono equivalenti.
	if normalizePath("/rest/api/3/project/{key}") != normalizePath("/rest/api/3/project/{projectIdOrKey}") {
		t.Error("normalized paths should match regardless of param names")
	}
}

func TestDiffRoutes(t *testing.T) {
	implemented := map[string]bool{
		"GET /rest/api/3/myself":        true, // match esatto
		"GET /rest/api/3/project/{key}": true, // match con nome parametro diverso
		"POST /rest/api/3/auth/login":   true, // fuori contratto
	}
	specs := map[string]string{
		"GET /rest/api/3/myself":                   "Get current user",
		"GET /rest/api/3/project/{projectIdOrKey}": "Get project",
		"DELETE /rest/api/3/issue/{issueIdOrKey}":  "Delete issue", // mancante
	}
	matched, missing, extra := diffRoutes(implemented, specs)

	wantMatched := []string{
		"GET /rest/api/3/myself",
		"GET /rest/api/3/project/{projectIdOrKey}",
	}
	wantMissing := []string{"DELETE /rest/api/3/issue/{issueIdOrKey}"}
	wantExtra := []string{"POST /rest/api/3/auth/login"}

	if !reflect.DeepEqual(matched, wantMatched) {
		t.Errorf("matched: got %v, want %v", matched, wantMatched)
	}
	if !reflect.DeepEqual(missing, wantMissing) {
		t.Errorf("missing: got %v, want %v", missing, wantMissing)
	}
	if !reflect.DeepEqual(extra, wantExtra) {
		t.Errorf("extra: got %v, want %v", extra, wantExtra)
	}
}

func TestLoadSpecRoutes(t *testing.T) {
	fixture := `{
		"paths": {
			"/rest/api/3/myself": {
				"get": {"summary": "Get current user", "tags": ["Myself"]}
			},
			"/rest/api/3/issue/{issueIdOrKey}": {
				"parameters": [{"name": "issueIdOrKey", "in": "path"}],
				"delete": {"summary": "Delete issue"},
				"put": {"summary": 42}
			}
		}
	}`
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := loadSpecRoutes(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{
		"GET /rest/api/3/myself":                  "Get current user",
		"DELETE /rest/api/3/issue/{issueIdOrKey}": "Delete issue",
	}
	// L'op malformata (put con summary numerico) deve produrre solo un warning
	// su stderr, non un errore né una entry; "parameters" va ignorato.
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadSpecRoutesInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadSpecRoutes(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q should mention the spec file path %q", err.Error(), path)
	}
}
