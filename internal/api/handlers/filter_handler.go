package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/search"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"gorm.io/gorm"
)

type FilterHandler struct {
	svc     *search.FilterService
	db      *gorm.DB
	baseURL string
	chk     *authz.Checker
}

func NewFilterHandler(svc *search.FilterService, db *gorm.DB, baseURL string, chk *authz.Checker) *FilterHandler {
	return &FilterHandler{svc: svc, db: db, baseURL: baseURL, chk: chk}
}

// requireOwner carica il filtro id e verifica che l'utente autenticato ne sia
// il proprietario (o admin globale). Scrive 404 se il filtro non esiste, 403
// se esiste ma appartiene a un altro utente. Ritorna (filtro, true) solo se
// l'operazione può procedere.
func (h *FilterHandler) requireOwner(w http.ResponseWriter, r *http.Request, id string) (*search.SavedFilter, bool) {
	f, err := h.svc.Get(id)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return nil, false
	}
	uid := middleware.UserIDFromContext(r.Context())
	if f.OwnerID != uid && !h.chk.IsGlobalAdmin(uid) {
		authz.WriteForbidden(w)
		return nil, false
	}
	return f, true
}

// ownerRef costruisce lo User v3 del proprietario (nil se non trovato).
func (h *FilterHandler) ownerRef(ownerID string) *v3.User {
	var u user.User
	if h.db.First(&u, "id = ?", ownerID).Error != nil {
		return nil
	}
	ju := v3.JiraUser(u, h.baseURL)
	return &ju
}

func (h *FilterHandler) toFilter(f *search.SavedFilter) v3.Filter {
	return v3.JiraFilter(v3.FilterInput{
		ID: f.ID, Name: f.Name, Description: f.Description, JQL: f.JQL,
		Favourite: f.IsFavourite, Owner: h.ownerRef(f.OwnerID), BaseURL: h.baseURL,
	})
}

type filterBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	JQL         string `json:"jql"`
}

func (h *FilterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var b filterBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if b.Name == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"name is required"}, nil)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	f, err := h.svc.Create(uid, nil, b.Name, b.Description, b.JQL, false)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to create filter"}, nil)
		return
	}
	// Il contratto Jira Cloud per POST /rest/api/3/filter risponde 200 (non
	// 201): a differenza di /issue o /project, la creazione di un filtro non
	// è modellata come risorsa REST "Created" nello spec ufficiale.
	v3.WriteJSON(w, http.StatusOK, h.toFilter(f))
}

func (h *FilterHandler) Get(w http.ResponseWriter, r *http.Request) {
	f, err := h.svc.Get(r.PathValue("id"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	if f.OwnerID != uid && !f.IsShared && !h.chk.IsGlobalAdmin(uid) {
		v3.WriteError(w, http.StatusNotFound, []string{"the resource does not exist or you do not have permission to view it"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, h.toFilter(f))
}

func (h *FilterHandler) Update(w http.ResponseWriter, r *http.Request) {
	var b filterBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	id := r.PathValue("id")
	if _, ok := h.requireOwner(w, r, id); !ok {
		return
	}
	f, err := h.svc.Update(id, b.Name, b.Description, b.JQL, nil)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, h.toFilter(f))
}

func (h *FilterHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := h.requireOwner(w, r, id); !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete filter"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FilterHandler) Search(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	startAt, maxResults := v3.ParsePagination(r, 50, 100)
	filters, total, err := h.svc.Search(uid, startAt, maxResults)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to search filters"}, nil)
		return
	}
	values := make([]v3.Filter, 0, len(filters))
	for i := range filters {
		values = append(values, h.toFilter(&filters[i]))
	}
	v3.WriteJSON(w, http.StatusOK, v3.PageBeanFilterDetails{
		MaxResults: maxResults, StartAt: int64(startAt), Total: int64(total),
		IsLast: startAt+len(values) >= total, Values: values,
	})
}

func (h *FilterHandler) My(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	filters, err := h.svc.List(uid) // List esistente: owner o shared
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list filters"}, nil)
		return
	}
	h.writeArray(w, filters)
}

func (h *FilterHandler) Favourite(w http.ResponseWriter, r *http.Request) {
	uid := middleware.UserIDFromContext(r.Context())
	filters, err := h.svc.ListFavourites(uid)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to list favourites"}, nil)
		return
	}
	h.writeArray(w, filters)
}

func (h *FilterHandler) AddFavourite(w http.ResponseWriter, r *http.Request) {
	h.setFav(w, r, true)
}

func (h *FilterHandler) RemoveFavourite(w http.ResponseWriter, r *http.Request) {
	h.setFav(w, r, false)
}

func (h *FilterHandler) setFav(w http.ResponseWriter, r *http.Request, fav bool) {
	id := r.PathValue("id")
	if _, ok := h.requireOwner(w, r, id); !ok {
		return
	}
	if err := h.svc.ToggleFavourite(id, fav); err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return
	}
	f, err := h.svc.Get(id)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, h.toFilter(f))
}

func (h *FilterHandler) writeArray(w http.ResponseWriter, filters []search.SavedFilter) {
	out := make([]v3.Filter, 0, len(filters))
	for i := range filters {
		out = append(out, h.toFilter(&filters[i]))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}
