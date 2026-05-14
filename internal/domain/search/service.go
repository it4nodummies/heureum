package search

import (
	"github.com/open-jira/open-jira/internal/domain/issue"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Search(query string) ([]issue.Issue, error) {
	q := Parse(query)
	db := s.db.Where("is_archived = ?", false)
	db = q.Apply(db)
	if q.Label != "" {
		db = db.Where("id IN (SELECT il.issue_id FROM issue_labels il JOIN labels l ON il.label_id = l.id WHERE l.name = ?)", q.Label)
	}
	var issues []issue.Issue
	if err := db.Order("created_at DESC").Find(&issues).Error; err != nil {
		return nil, err
	}
	return issues, nil
}
