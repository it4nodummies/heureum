package issue

import (
	"time"

	"gorm.io/gorm"
)

// Vote registra il voto di un utente su un issue (Jira "issue votes").
type Vote struct {
	IssueID   string    `gorm:"primaryKey;type:text" json:"issue_id"`
	UserID    string    `gorm:"primaryKey;type:text" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Vote) TableName() string { return "issue_votes" }

// VoteService gestisce l'aggiunta, rimozione e lettura dei voti su un issue.
type VoteService struct{ db *gorm.DB }

func NewVoteService(db *gorm.DB) *VoteService { return &VoteService{db: db} }

// Add registra il voto di userID su issueID. È idempotente: votare più volte
// non crea righe duplicate né errori.
func (s *VoteService) Add(issueID, userID string) error {
	return s.db.Where(Vote{IssueID: issueID, UserID: userID}).FirstOrCreate(&Vote{IssueID: issueID, UserID: userID}).Error
}

// Remove rimuove il voto di userID su issueID, se presente.
func (s *VoteService) Remove(issueID, userID string) error {
	return s.db.Delete(&Vote{}, "issue_id = ? AND user_id = ?", issueID, userID).Error
}

// Count restituisce il numero di voti su issueID.
func (s *VoteService) Count(issueID string) int {
	var n int64
	s.db.Model(&Vote{}).Where("issue_id = ?", issueID).Count(&n)
	return int(n)
}

// HasVoted indica se userID ha già votato issueID.
func (s *VoteService) HasVoted(issueID, userID string) bool {
	var n int64
	s.db.Model(&Vote{}).Where("issue_id = ? AND user_id = ?", issueID, userID).Count(&n)
	return n > 0
}

// Voters restituisce gli ID degli utenti che hanno votato issueID.
func (s *VoteService) Voters(issueID string) []string {
	var ids []string
	s.db.Model(&Vote{}).Where("issue_id = ?", issueID).Pluck("user_id", &ids)
	return ids
}
