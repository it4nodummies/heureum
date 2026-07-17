package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/permission"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/version"
)

// VersionHandler serve gli endpoint v3 delle project version (release).
type VersionHandler struct {
	svc        *version.Service
	projectSvc *project.Service
	chk        *authz.Checker
	baseURL    string
}

func NewVersionHandler(svc *version.Service, projectSvc *project.Service, chk *authz.Checker, baseURL string) *VersionHandler {
	return &VersionHandler{svc: svc, projectSvc: projectSvc, chk: chk, baseURL: baseURL}
}

// parseVersionDate converte una stringa YYYY-MM-DD in *time.Time. Stringa vuota
// -> (nil, nil). Formato non valido -> errore.
func parseVersionDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// projectSeqID ritorna il seq_id del progetto proprietario di una version (0 se
// il progetto non è risolvibile).
func (h *VersionHandler) projectSeqID(projectID string) int64 {
	p, err := h.projectSvc.GetByID(projectID)
	if err != nil {
		return 0
	}
	return p.SeqID
}

// List: GET /rest/api/3/project/{key}/versions -> []JiraVersion.
func (h *VersionHandler) List(w http.ResponseWriter, r *http.Request) {
	p, err := h.projectSvc.GetByKey(r.PathValue("key"))
	if err != nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	versions, _ := h.svc.ListByProject(p.ID)
	out := make([]v3.JiraVersion, 0, len(versions))
	for _, v := range versions {
		out = append(out, v3.VersionFrom(v, p.SeqID, h.baseURL))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// Create: POST /rest/api/3/version. Autorizzazione in-handler (AdministerProjects
// sul progetto risolto dal body) perché la rotta non ha un path key risolvibile.
func (h *VersionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		StartDate   string `json:"startDate"`
		ReleaseDate string `json:"releaseDate"`
		ProjectID   int64  `json:"projectId"`
		Project     string `json:"project"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	// Risoluzione del progetto: preferisci projectId (seq_id), fallback a project (key).
	var p *project.Project
	var err error
	switch {
	case req.ProjectID != 0:
		p, err = h.projectSvc.GetBySeqID(req.ProjectID)
	case req.Project != "":
		p, err = h.projectSvc.GetByKey(req.Project)
	default:
		http.Error(w, `{"error":"projectId or project is required"}`, http.StatusBadRequest)
		return
	}
	if err != nil || p == nil {
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	uid := middleware.UserIDFromContext(r.Context())
	if err := h.chk.RequireProject(uid, p.ID, permission.AdministerProjects); err != nil {
		authz.WriteForbidden(w)
		return
	}

	startDate, err := parseVersionDate(req.StartDate)
	if err != nil {
		http.Error(w, `{"error":"invalid startDate (want YYYY-MM-DD)"}`, http.StatusBadRequest)
		return
	}
	releaseDate, err := parseVersionDate(req.ReleaseDate)
	if err != nil {
		http.Error(w, `{"error":"invalid releaseDate (want YYYY-MM-DD)"}`, http.StatusBadRequest)
		return
	}

	v, err := h.svc.Create(p.ID, req.Name, req.Description, startDate, releaseDate)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v3.VersionFrom(*v, p.SeqID, h.baseURL))
}

// Get: GET /rest/api/3/version/{id}.
func (h *VersionHandler) Get(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.Get(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"version not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v3.VersionFrom(*v, h.projectSeqID(v.ProjectID), h.baseURL))
}

// Update: PUT /rest/api/3/version/{id}. I campi assenti nel body restano invariati.
func (h *VersionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.svc.Get(id); err != nil {
		http.Error(w, `{"error":"version not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Released    *bool   `json:"released"`
		Archived    *bool   `json:"archived"`
		StartDate   *string `json:"startDate"`
		ReleaseDate *string `json:"releaseDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	var startDate, releaseDate *time.Time
	if req.StartDate != nil {
		d, err := parseVersionDate(*req.StartDate)
		if err != nil {
			http.Error(w, `{"error":"invalid startDate (want YYYY-MM-DD)"}`, http.StatusBadRequest)
			return
		}
		startDate = d
	}
	if req.ReleaseDate != nil {
		d, err := parseVersionDate(*req.ReleaseDate)
		if err != nil {
			http.Error(w, `{"error":"invalid releaseDate (want YYYY-MM-DD)"}`, http.StatusBadRequest)
			return
		}
		releaseDate = d
	}

	v, err := h.svc.Update(id, req.Name, req.Description, req.Released, req.Archived, startDate, releaseDate)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v3.VersionFrom(*v, h.projectSeqID(v.ProjectID), h.baseURL))
}

// Delete: DELETE /rest/api/3/version/{id}.
func (h *VersionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.svc.Get(id); err != nil {
		http.Error(w, `{"error":"version not found"}`, http.StatusNotFound)
		return
	}
	if err := h.svc.Delete(id); err != nil {
		http.Error(w, `{"error":"failed to delete version"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
