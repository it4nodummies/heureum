package handlers

import (
	"encoding/json"
	"net/http"

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
}

func NewFilterHandler(svc *search.FilterService, db *gorm.DB, baseURL string) *FilterHandler {
	return &FilterHandler{svc: svc, db: db, baseURL: baseURL}
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
	v3.WriteJSON(w, http.StatusOK, h.toFilter(f))
}

func (h *FilterHandler) Update(w http.ResponseWriter, r *http.Request) {
	var b filterBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	f, err := h.svc.Update(r.PathValue("id"), b.Name, b.Description, b.JQL, nil)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"filter not found"}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusOK, h.toFilter(f))
}

func (h *FilterHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.PathValue("id")); err != nil {
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
