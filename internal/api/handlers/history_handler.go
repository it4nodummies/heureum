package handlers

import (
	"net/http"
	"strconv"

	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"gorm.io/gorm"
)

// HistoryHandler serve l'endpoint Jira v3 del changelog di un issue.
type HistoryHandler struct {
	db       *gorm.DB
	issueSvc *issue.Service
	baseURL  string
}

func NewHistoryHandler(db *gorm.DB, issueSvc *issue.Service, baseURL string) *HistoryHandler {
	return &HistoryHandler{db: db, issueSvc: issueSvc, baseURL: baseURL}
}

// resolve trova l'issue dal path param issueKey, provando prima come ID
// numerico sequenziale e poi come key (es. "DEMO-1").
func (h *HistoryHandler) resolve(r *http.Request) *issue.Issue {
	k := r.PathValue("issueKey")
	if n, err := strconv.ParseInt(k, 10, 64); err == nil {
		if iss, err := h.issueSvc.GetBySeqID(n); err == nil {
			return iss
		}
		return nil
	}
	iss, err := h.issueSvc.GetByKey(k)
	if err != nil {
		return nil
	}
	return iss
}

// GetHistory restituisce il changelog dell'issue come PageBeanChangelog.
func (h *HistoryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	history, _ := h.issueSvc.GetHistory(iss.ID)

	values := make([]v3.Changelog, 0, len(history))
	for _, item := range history {
		cl := v3.Changelog{
			ID:      item.ID,
			Created: v3.JiraTime(item.CreatedAt),
			Items: []v3.ChangeItem{
				{
					Field:      item.FieldName,
					Fieldtype:  "jira",
					FromString: item.OldValue,
					ToString:   item.NewValue,
				},
			},
		}
		if item.ActorID != nil && *item.ActorID != "" {
			var u user.User
			if err := h.db.Where("id = ?", *item.ActorID).First(&u).Error; err == nil {
				ju := v3.JiraUser(u, h.baseURL)
				cl.Author = &ju
			}
		}
		values = append(values, cl)
	}

	v3.WritePage(w, http.StatusOK, v3.Page[v3.Changelog]{
		StartAt:    0,
		MaxResults: len(values),
		Total:      len(values),
		Values:     values,
	})
}
