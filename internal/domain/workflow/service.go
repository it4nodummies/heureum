package workflow

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) CreateWorkflow(projectID, name string) (*Workflow, error) {
	wf := &Workflow{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Name:      name,
	}
	if err := s.db.Create(wf).Error; err != nil {
		return nil, err
	}
	return wf, nil
}

func (s *Service) GetWorkflow(projectID string) (*Workflow, error) {
	var wf Workflow
	if err := s.db.Preload("Statuses").Where("project_id = ?", projectID).First(&wf).Error; err != nil {
		return nil, errors.New("workflow not found")
	}
	return &wf, nil
}

func (s *Service) AddStatus(workflowID, name string, category StatusCategory, color string) (*WorkflowStatus, error) {
	if color == "" {
		color = "#6B7280"
	}
	var maxPos int
	s.db.Model(&WorkflowStatus{}).Where("workflow_id = ?", workflowID).Select("COALESCE(MAX(position), -1)").Scan(&maxPos)

	status := &WorkflowStatus{
		ID:         uuid.New().String(),
		WorkflowID: workflowID,
		Name:       name,
		Category:   category,
		Color:      color,
		Position:   maxPos + 1,
	}
	if err := s.db.Create(status).Error; err != nil {
		return nil, err
	}
	return status, nil
}

func (s *Service) UpdateStatus(statusID, name string, category StatusCategory, color string) (*WorkflowStatus, error) {
	var status WorkflowStatus
	if err := s.db.First(&status, "id = ?", statusID).Error; err != nil {
		return nil, errors.New("status not found")
	}
	status.Name = name
	status.Category = category
	status.Color = color
	if err := s.db.Save(&status).Error; err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *Service) RemoveStatus(statusID string) error {
	return s.db.Delete(&WorkflowStatus{}, "id = ?", statusID).Error
}

func (s *Service) AddTransition(workflowID, fromStatusID, toStatusID string) (*WorkflowTransition, error) {
	tr := &WorkflowTransition{
		ID:           uuid.New().String(),
		WorkflowID:   workflowID,
		FromStatusID: fromStatusID,
		ToStatusID:   toStatusID,
	}
	if err := s.db.Create(tr).Error; err != nil {
		return nil, err
	}
	return tr, nil
}

func (s *Service) RemoveTransition(transitionID string) error {
	return s.db.Delete(&WorkflowTransition{}, "id = ?", transitionID).Error
}

func (s *Service) GetTransitions(workflowID string) ([]WorkflowTransition, error) {
	var transitions []WorkflowTransition
	s.db.Where("workflow_id = ?", workflowID).Find(&transitions)
	return transitions, nil
}

func (s *Service) ValidateTransition(workflowID, fromStatusID, toStatusID string) error {
	var count int64
	s.db.Model(&WorkflowTransition{}).Where("workflow_id = ? AND from_status_id = ? AND to_status_id = ?", workflowID, fromStatusID, toStatusID).Count(&count)
	if count == 0 {
		return errors.New("invalid transition")
	}
	return nil
}

func (s *Service) CreateDefaultWorkflow(projectID string) (*Workflow, error) {
	wf, err := s.CreateWorkflow(projectID, "Default Workflow")
	if err != nil {
		return nil, err
	}

	todo, _ := s.AddStatus(wf.ID, "TO DO", CategoryTodo, "#6B7280")
	inProg, _ := s.AddStatus(wf.ID, "IN PROGRESS", CategoryInProgress, "#3B82F6")
	done, _ := s.AddStatus(wf.ID, "DONE", CategoryDone, "#10B981")

	s.AddTransition(wf.ID, todo.ID, inProg.ID)
	s.AddTransition(wf.ID, inProg.ID, todo.ID)
	s.AddTransition(wf.ID, inProg.ID, done.ID)

	return s.GetWorkflow(projectID)
}
