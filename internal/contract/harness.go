// Package contract valida le risposte del nostro server contro
// l'OpenAPI ufficiale di Jira Cloud (docs/contracts/).
package contract

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

type Validator struct {
	router routers.Router
}

func NewValidator(specPath string) (*Validator, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}
	// Lo spec Atlassian ha servers con host cloud: azzeriamo per far matchare i path relativi.
	doc.Servers = openapi3.Servers{&openapi3.Server{URL: "/"}}
	// Lo spec Atlassian ufficiale ha alcune inconsistenze minori (path
	// ambigui, example/default non conformi al proprio schema) che
	// farebbero fallire doc.Validate(). Usiamo il router gorillamux, che
	// non richiama la validazione dello spec: quello che ci interessa qui è
	// il matching di rotte e la validazione di richieste/risposte, non la
	// conformità pedante dello spec stesso.
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("build router: %w", err)
	}
	return &Validator{router: router}, nil
}

// loadEntry garantisce semantica once-per-key per la cache dei Validator.
type loadEntry struct {
	once sync.Once
	v    *Validator
	err  error
}

var cache sync.Map // specPath -> *loadEntry

// Load restituisce un Validator condiviso e cachato per specPath: il parse
// dello spec (~2.3MB) e la costruzione del router avvengono una sola volta
// per test binary.
func Load(specPath string) (*Validator, error) {
	e, _ := cache.LoadOrStore(specPath, &loadEntry{})
	entry := e.(*loadEntry)
	entry.once.Do(func() {
		entry.v, entry.err = NewValidator(specPath)
	})
	return entry.v, entry.err
}

// MustLoad è Load per i test: fallisce il test se lo spec non carica.
func MustLoad(tb testing.TB, specPath string) *Validator {
	tb.Helper()
	v, err := Load(specPath)
	if err != nil {
		tb.Fatalf("load contract spec %s: %v", specPath, err)
	}
	return v
}

// ValidateResponse verifica che (method, path) esista nel contratto e che
// status/header/body rispettino lo schema della risposta.
func (v *Validator) ValidateResponse(method, path string, status int, header http.Header, body io.Reader) error {
	u, err := url.Parse(path)
	if err != nil {
		return err
	}
	// Header di richiesta vuoto: validiamo solo la risposta e l'auth è
	// disattivata (NoopAuthenticationFunc), quindi gli header di richiesta
	// sono irrilevanti.
	req := &http.Request{Method: method, URL: u, Header: http.Header{}}
	route, pathParams, err := v.router.FindRoute(req)
	if err != nil {
		return fmt.Errorf("route %s %s not in contract: %w", method, path, err)
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
			Options: &openapi3filter.Options{
				AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			},
		},
		Status: status,
		Header: header,
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read response body for %s %s: %w", method, path, err)
	}
	input.SetBodyBytes(b)
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		return fmt.Errorf("response %s %s status=%d: %w", method, path, status, err)
	}
	return nil
}
