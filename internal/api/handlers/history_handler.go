package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
	"gorm.io/gorm"
)

type HistoryEntry struct {
	ID           string  `json:"id"`
	IssueID      string  `json:"issue_id"`
	ActorID      *string `json:"actor_id,omitempty"`
	ActorName    string  `json:"actor_name"`
	FieldName    string  `json:"field_name"`
	OldValue     string  `json:"old_value"`
	NewValue     string  `json:"new_value"`
	CreatedAt    string  `json:"created_at"`
}

type HistoryHandler struct {
	db       *gorm.DB
	issueSvc *issue.Service
}

func NewHistoryHandler(db *gorm.DB, issueSvc *issue.Service) *HistoryHandler {
	return &HistoryHandler{db: db, issueSvc: issueSvc}
}

func (h *HistoryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	iss, err := h.issueSvc.GetByKey(r.PathValue("issueKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	history, _ := h.issueSvc.GetHistory(iss.ID)

	entries := make([]HistoryEntry, 0, len(history))
	for _, item := range history {
		entry := HistoryEntry{
			ID:        item.ID,
			IssueID:   item.IssueID,
			ActorID:   item.ActorID,
			FieldName: item.FieldName,
			OldValue:  item.OldValue,
			NewValue:  item.NewValue,
			CreatedAt: item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if item.ActorID != nil && *item.ActorID != "" {
			var u user.User
			if err := h.db.Where("id = ?", *item.ActorID).First(&u).Error; err == nil {
				entry.ActorName = u.Username
			}
		}
		entries = append(entries, entry)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
