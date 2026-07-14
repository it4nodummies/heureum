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
	if err := s.db.Preload("Statuses").Preload("Transitions").Where("project_id = ?", projectID).First(&wf).Error; err != nil {
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

// AddTransition crea una transizione from→to con nome e regole base.
func (s *Service) AddTransition(workflowID, fromStatusID, toStatusID, name string, requireAssignee, setResolution bool) (*WorkflowTransition, error) {
	tr := &WorkflowTransition{
		ID:              uuid.New().String(),
		WorkflowID:      workflowID,
		FromStatusID:    fromStatusID,
		ToStatusID:      toStatusID,
		Name:            name,
		RequireAssignee: requireAssignee,
		SetResolution:   setResolution,
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

// GetTransitionByID carica una singola transizione.
func (s *Service) GetTransitionByID(id string) (*WorkflowTransition, error) {
	var tr WorkflowTransition
	if err := s.db.Where("id = ?", id).First(&tr).Error; err != nil {
		return nil, err
	}
	return &tr, nil
}

// GetAvailableTransitions restituisce le transizioni uscenti da fromStatusID.
func (s *Service) GetAvailableTransitions(workflowID, fromStatusID string) ([]WorkflowTransition, error) {
	var trs []WorkflowTransition
	if err := s.db.Where("workflow_id = ? AND from_status_id = ?", workflowID, fromStatusID).Find(&trs).Error; err != nil {
		return nil, err
	}
	return trs, nil
}

// UpdateTransition aggiorna nome e regole di una transizione (puntatori nil = invariato).
func (s *Service) UpdateTransition(id string, name *string, requireAssignee, setResolution *bool) (*WorkflowTransition, error) {
	updates := map[string]any{}
	if name != nil {
		updates["name"] = *name
	}
	if requireAssignee != nil {
		updates["require_assignee"] = *requireAssignee
	}
	if setResolution != nil {
		updates["set_resolution"] = *setResolution
	}
	if len(updates) > 0 {
		if err := s.db.Model(&WorkflowTransition{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetTransitionByID(id)
}

// ReorderStatuses riassegna la position degli stati secondo l'ordine dato.
func (s *Service) ReorderStatuses(workflowID string, orderedStatusIDs []string) error {
	for i, id := range orderedStatusIDs {
		if err := s.db.Model(&WorkflowStatus{}).Where("workflow_id = ? AND id = ?", workflowID, id).Update("position", i).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ValidateTransition(workflowID, fromStatusID, toStatusID string) error {
	var count int64
	s.db.Model(&WorkflowTransition{}).Where("workflow_id = ? AND from_status_id = ? AND to_status_id = ?", workflowID, fromStatusID, toStatusID).Count(&count)
	if count == 0 {
		return errors.New("invalid transition")
	}
	return nil
}

func (s *Service) ListAllStatuses() ([]WorkflowStatus, error) {
	var statuses []WorkflowStatus
	if err := s.db.Order("position ASC").Find(&statuses).Error; err != nil {
		return nil, err
	}
	return statuses, nil
}

func (s *Service) GetStatus(idOrName string) (*WorkflowStatus, error) {
	var status WorkflowStatus
	if err := s.db.Where("id = ? OR name = ?", idOrName, idOrName).First(&status).Error; err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *Service) GetWorkflowByProjectID(projectID string) (*Workflow, error) {
	return s.GetWorkflow(projectID)
}

func (s *Service) CreateDefaultWorkflow(projectID string) (*Workflow, error) {
	wf, err := s.CreateWorkflow(projectID, "Default Workflow")
	if err != nil {
		return nil, err
	}

	todo, _ := s.AddStatus(wf.ID, "TO DO", CategoryTodo, "#6B7280")
	inProg, _ := s.AddStatus(wf.ID, "IN PROGRESS", CategoryInProgress, "#3B82F6")
	done, _ := s.AddStatus(wf.ID, "DONE", CategoryDone, "#10B981")

	s.AddTransition(wf.ID, todo.ID, inProg.ID, "Start Progress", false, false)
	s.AddTransition(wf.ID, inProg.ID, todo.ID, "Stop Progress", false, false)
	s.AddTransition(wf.ID, inProg.ID, done.ID, "Done", false, true)

	return s.GetWorkflow(projectID)
}
