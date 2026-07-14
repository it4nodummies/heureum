package authz

import (
	"net/http"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
)

// WriteForbidden scrive una risposta 403 nel formato errore v3 standard.
// Esportata per essere riusata dagli handler che applicano l'enforcement
// in-handler (rotte body-based, round successivo) invece che via decorator.
func WriteForbidden(w http.ResponseWriter) {
	v3.WriteError(w, http.StatusForbidden, []string{"you do not have permission to perform this action"}, nil)
}

// Enforce richiede permKey sul progetto risolto da resolve per l'utente
// autenticato. Se il target non è risolvibile (ok=false) passa direttamente
// al handler, che risponderà con il proprio 404 naturale — evita di rivelare
// tramite un 403 l'esistenza di una risorsa che l'utente non potrebbe vedere.
func (c *Checker) Enforce(permKey string, resolve Resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := middleware.UserIDFromContext(r.Context())
		projectID, ok := resolve(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if err := c.RequireProject(uid, projectID, permKey); err != nil {
			WriteForbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// EnforceNotFound richiede permKey sul progetto risolto da resolve, come
// Enforce, ma nega con 404 invece di 403 (semantica Jira sulle letture: non
// rivelare tramite un 403 l'esistenza di una risorsa che l'utente non
// potrebbe vedere). Resolver ok=false → pass-through al handler, invariato.
func (c *Checker) EnforceNotFound(permKey string, resolve Resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := middleware.UserIDFromContext(r.Context())
		projectID, ok := resolve(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if err := c.RequireProject(uid, projectID, permKey); err != nil {
			v3.WriteError(w, http.StatusNotFound, []string{"the resource does not exist or you do not have permission to view it"}, nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// EnforceGlobalAdmin richiede che l'utente autenticato sia admin globale
// (flag is_admin), a prescindere da qualunque progetto.
func (c *Checker) EnforceGlobalAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := middleware.UserIDFromContext(r.Context())
		if err := c.RequireGlobalAdmin(uid); err != nil {
			WriteForbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}
