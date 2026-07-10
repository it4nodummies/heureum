package main

import "testing"

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

func TestNormalizePath(t *testing.T) {
	// I nomi dei parametri non contano nel confronto: {key} e {projectIdOrKey} sono equivalenti.
	if normalizePath("/rest/api/3/project/{key}") != normalizePath("/rest/api/3/project/{projectIdOrKey}") {
		t.Error("normalized paths should match regardless of param names")
	}
}
