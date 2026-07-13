package issue

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RemoteLink rappresenta un link Jira verso una risorsa in un sistema
// remoto (Jira "remote issue link"), es. un documento Confluence o un
// ticket in un altro tracker.
type RemoteLink struct {
	ID           string    `gorm:"primaryKey;type:text" json:"id"`
	IssueID      string    `gorm:"type:text;not null;index" json:"issue_id"`
	GlobalID     string    `gorm:"column:global_id;type:text;default:''" json:"global_id"`
	URL          string    `gorm:"type:text;not null;default:''" json:"url"`
	Title        string    `gorm:"type:text;not null;default:''" json:"title"`
	Summary      string    `gorm:"type:text;default:''" json:"summary"`
	Relationship string    `gorm:"type:text;default:''" json:"relationship"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (RemoteLink) TableName() string { return "issue_remote_links" }

// RemoteLinkService gestisce la creazione, lettura e rimozione dei remote
// issue link su un issue.
type RemoteLinkService struct{ db *gorm.DB }

func NewRemoteLinkService(db *gorm.DB) *RemoteLinkService { return &RemoteLinkService{db: db} }

// Add crea un remote link per issueID. globalID può essere vuoto.
func (s *RemoteLinkService) Add(issueID, globalID, url, title, summary, relationship string) (*RemoteLink, error) {
	rl := &RemoteLink{ID: uuid.NewString(), IssueID: issueID, GlobalID: globalID, URL: url, Title: title, Summary: summary, Relationship: relationship}
	if err := s.db.Create(rl).Error; err != nil {
		return nil, err
	}
	return rl, nil
}

// ListByIssue restituisce i remote link di issueID in ordine di creazione.
func (s *RemoteLinkService) ListByIssue(issueID string) ([]RemoteLink, error) {
	var out []RemoteLink
	err := s.db.Where("issue_id = ?", issueID).Order("created_at asc").Find(&out).Error
	return out, err
}

// Delete rimuove il remote link con l'id dato, ma solo se appartiene a
// issueID. Restituisce il numero di righe eliminate: il chiamante lo usa per
// distinguere "eliminato" da "non trovato / non appartenente all'issue".
func (s *RemoteLinkService) Delete(issueID, id string) (int64, error) {
	res := s.db.Delete(&RemoteLink{}, "issue_id = ? AND id = ?", issueID, id)
	return res.RowsAffected, res.Error
}
