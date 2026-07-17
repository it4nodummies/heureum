package issue

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Worklog registra il tempo dichiarato su un issue (time tracking Jira).
type Worklog struct {
	ID               string     `gorm:"primaryKey;type:text" json:"id"`
	IssueID          string     `gorm:"type:text;not null;index" json:"issue_id"`
	AuthorID         *string    `gorm:"type:text" json:"author_id,omitempty"`
	CommentJSON      string     `gorm:"column:comment_json;type:text;default:'{}'" json:"comment_json"`
	TimeSpentSeconds int        `gorm:"column:time_spent_seconds;default:0" json:"time_spent_seconds"`
	Started          *time.Time `json:"started,omitempty"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Worklog) TableName() string { return "issue_worklogs" }

// WorklogService gestisce la creazione, lettura e rimozione dei worklog.
type WorklogService struct{ db *gorm.DB }

func NewWorklogService(db *gorm.DB) *WorklogService { return &WorklogService{db: db} }

// Add crea un worklog per issueID e incrementa Issue.TimeSpent di seconds
// (il worklog è la fonte di verità del tempo loggato; TimeSpent è la somma
// denormalizzata usata dalla issue view e dal mapper v3 timetracking).
// authorID può essere vuoto (nessun autore), commentJSON vuoto viene
// normalizzato a "{}" (nessun commento ADF).
func (s *WorklogService) Add(issueID, authorID, commentJSON string, seconds int) (*Worklog, error) {
	now := time.Now()
	wl := &Worklog{
		ID:               uuid.NewString(),
		IssueID:          issueID,
		CommentJSON:      commentJSON,
		TimeSpentSeconds: seconds,
		Started:          &now,
	}
	if authorID != "" {
		wl.AuthorID = &authorID
	}
	if commentJSON == "" {
		wl.CommentJSON = "{}"
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(wl).Error; err != nil {
			return err
		}
		return tx.Model(&Issue{}).Where("id = ?", issueID).
			UpdateColumn("time_spent", gorm.Expr("time_spent + ?", seconds)).Error
	})
	if err != nil {
		return nil, err
	}
	return wl, nil
}

// ListByIssue restituisce i worklog di un issue, ordinati per data di creazione.
func (s *WorklogService) ListByIssue(issueID string) ([]Worklog, error) {
	var out []Worklog
	err := s.db.Where("issue_id = ?", issueID).Order("created_at asc").Find(&out).Error
	return out, err
}

// Get restituisce un worklog per ID.
func (s *WorklogService) Get(id string) (*Worklog, error) {
	var wl Worklog
	if err := s.db.First(&wl, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &wl, nil
}

// Delete rimuove un worklog per ID.
func (s *WorklogService) Delete(id string) error {
	return s.db.Delete(&Worklog{}, "id = ?", id).Error
}
