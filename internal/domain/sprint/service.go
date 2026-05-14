package sprint

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db       *gorm.DB
	notifier SprintNotifier
}

type SprintNotifier interface {
	NotifySprintStarted(projectID, sprintID, sprintName string, startedByUserID string) error
	NotifySprintCompleted(projectID, sprintID, sprintName string, completedByUserID string) error
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) SetNotifier(n SprintNotifier) { s.notifier = n }

func (s *Service) Create(projectID, name, goal string) (*Sprint, error) {
	if name == "" {
		return nil, errors.New("sprint name required")
	}
	sp := &Sprint{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Name:      name,
		Goal:      goal,
		State:     StateFuture,
	}
	if err := s.db.Create(sp).Error; err != nil {
		return nil, err
	}
	return sp, nil
}

func (s *Service) GetByID(id string) (*Sprint, error) {
	var sp Sprint
	if err := s.db.First(&sp, "id = ?", id).Error; err != nil {
		return nil, errors.New("sprint not found")
	}
	return &sp, nil
}

func (s *Service) Update(id, name, goal string) (*Sprint, error) {
	sp, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		sp.Name = name
	}
	sp.Goal = goal
	if err := s.db.Save(sp).Error; err != nil {
		return nil, err
	}
	return sp, nil
}

func (s *Service) Start(sprintID string) (*Sprint, error) {
	var sp Sprint
	if err := s.db.First(&sp, "id = ?", sprintID).Error; err != nil {
		return nil, errors.New("sprint not found")
	}
	now := time.Now()
	sp.State = StateActive
	sp.StartDate = &now
	if err := s.db.Save(&sp).Error; err != nil {
		return nil, err
	}
	if s.notifier != nil {
		s.notifier.NotifySprintStarted(sp.ProjectID, sp.ID, sp.Name, "")
	}
	return &sp, nil
}

func (s *Service) Complete(sprintID string, moveOpenToBacklog bool) (*Sprint, error) {
	var sp Sprint
	if err := s.db.First(&sp, "id = ?", sprintID).Error; err != nil {
		return nil, errors.New("sprint not found")
	}
	now := time.Now()
	sp.State = StateClosed
	sp.EndDate = &now
	if err := s.db.Save(&sp).Error; err != nil {
		return nil, err
	}
	if moveOpenToBacklog {
		s.db.Exec("UPDATE issues SET sprint_id = NULL WHERE sprint_id = ? AND status_id NOT IN (SELECT id FROM workflow_statuses WHERE category = 'done')", sprintID)
	}
	if s.notifier != nil {
		s.notifier.NotifySprintCompleted(sp.ProjectID, sp.ID, sp.Name, "")
	}
	return &sp, nil
}

func (s *Service) ListByProject(projectID string) ([]Sprint, error) {
	var sprints []Sprint
	s.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&sprints)
	return sprints, nil
}

func (s *Service) GetActive(projectID string) (*Sprint, error) {
	var sp Sprint
	if err := s.db.Where("project_id = ? AND state = ?", projectID, StateActive).First(&sp).Error; err != nil {
		return nil, errors.New("no active sprint")
	}
	return &sp, nil
}

func (s *Service) AddIssue(sprintID, issueID string) error {
	return s.db.Table("issues").Where("id = ?", issueID).Update("sprint_id", sprintID).Error
}

func (s *Service) RemoveIssue(issueID string) error {
	return s.db.Table("issues").Where("id = ?", issueID).Update("sprint_id", nil).Error
}

func (s *Service) DB() *gorm.DB { return s.db }
