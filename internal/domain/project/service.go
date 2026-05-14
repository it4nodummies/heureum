package project

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/open-jira/open-jira/internal/domain/user"
)

type Service struct {
	db   *gorm.DB
	lead *user.User
}

func NewService(db *gorm.DB, lead *user.User) *Service {
	return &Service{db: db, lead: lead}
}

func (s *Service) Create(name, key, description string, pType Type) (*Project, error) {
	key = strings.ToUpper(key)
	if len(key) < 2 || len(key) > 10 {
		return nil, errors.New("project key must be 2-10 characters")
	}
	var existing Project
	if s.db.Where("key = ?", key).First(&existing).Error == nil {
		return nil, errors.New("project key already exists")
	}
	p := &Project{
		ID:          uuid.New().String(),
		Name:        name,
		Key:         key,
		Description: description,
		Type:        pType,
	}
	if s.lead != nil {
		p.LeadUserID = &s.lead.ID
	}
	if err := s.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) GetByKey(key string) (*Project, error) {
	var p Project
	if err := s.db.Where("key = ?", strings.ToUpper(key)).First(&p).Error; err != nil {
		return nil, errors.New("project not found")
	}
	return &p, nil
}

func (s *Service) GetByID(id string) (*Project, error) {
	var p Project
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, errors.New("project not found")
	}
	return &p, nil
}

func (s *Service) List(archived bool) ([]Project, error) {
	var projects []Project
	query := s.db
	if !archived {
		query = query.Where("is_archived = ?", false)
	}
	if err := query.Order("created_at DESC").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

func (s *Service) Update(key string, name, description string) (*Project, error) {
	p, err := s.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if name != "" {
		p.Name = name
	}
	p.Description = description
	if err := s.db.Save(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) Archive(key string) error {
	return s.db.Model(&Project{}).Where("key = ?", strings.ToUpper(key)).Updates(map[string]interface{}{
		"is_archived": true,
	}).Error
}

func (s *Service) AddMember(projectID, userID string, role MemberRole) error {
	return s.db.Create(&ProjectMember{ProjectID: projectID, UserID: userID, Role: role}).Error
}

func (s *Service) RemoveMember(projectID, userID string) error {
	return s.db.Where("project_id = ? AND user_id = ?", projectID, userID).Delete(&ProjectMember{}).Error
}

func (s *Service) ListMembers(projectID string) ([]ProjectMember, error) {
	var members []ProjectMember
	if err := s.db.Where("project_id = ?", projectID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (s *Service) DB() *gorm.DB { return s.db }
