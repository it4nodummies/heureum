package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"gorm.io/gorm"
)

// ProjectCategoryHandler implements the Jira v3 projectCategory endpoints.
type ProjectCategoryHandler struct {
	db      *gorm.DB
	baseURL string
}

func NewProjectCategoryHandler(db *gorm.DB, baseURL string) *ProjectCategoryHandler {
	return &ProjectCategoryHandler{db: db, baseURL: baseURL}
}

func (h *ProjectCategoryHandler) jira(c project.ProjectCategory) v3.ProjectCategory {
	return v3.ProjectCategory{
		Self:        h.baseURL + "/rest/api/3/projectCategory/" + c.ID,
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
	}
}

// List implements GET /rest/api/3/projectCategory.
func (h *ProjectCategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	var cats []project.ProjectCategory
	if err := h.db.Find(&cats).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to list categories."}, nil)
		return
	}
	out := make([]v3.ProjectCategory, 0, len(cats))
	for _, c := range cats {
		out = append(out, h.jira(c))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// Create implements POST /rest/api/3/projectCategory.
func (h *ProjectCategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	if req.Name == "" {
		v3.WriteError(w, http.StatusBadRequest, nil, map[string]string{"name": "The category name must not be empty."})
		return
	}
	c := project.ProjectCategory{ID: uuid.NewString(), Name: req.Name, Description: req.Description}
	if err := h.db.Create(&c).Error; err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"Failed to create category."}, nil)
		return
	}
	v3.WriteJSON(w, http.StatusCreated, h.jira(c))
}
