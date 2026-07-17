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
	return s.CreateFull(projectID, name, goal, nil, nil, nil)
}

// CreateFull crea uno sprint con tutti i campi agili, assegnando il seq_id.
func (s *Service) CreateFull(projectID, name, goal string, originBoardID *int64, start, end *time.Time) (*Sprint, error) {
	var maxSeq int64
	if err := s.db.Model(&Sprint{}).Select("COALESCE(MAX(seq_id), 0)").Scan(&maxSeq).Error; err != nil {
		return nil, err
	}
	sp := &Sprint{
		ID:            uuid.New().String(),
		ProjectID:     projectID,
		Name:          name,
		Goal:          goal,
		State:         StateFuture,
		SeqID:         maxSeq + 1,
		OriginBoardID: originBoardID,
		StartDate:     start,
		EndDate:       end,
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

func (s *Service) GetBySeqID(seqID int64) (*Sprint, error) {
	var sp Sprint
	if err := s.db.Where("seq_id = ?", seqID).First(&sp).Error; err != nil {
		return nil, err
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

// Complete chiude lo sprint e gestisce le issue incomplete (categoria != 'done'):
//   - moveToSprintID != nil → riassegnate allo sprint target (UUID)
//   - altrimenti moveOpenToBacklog → sprint_id NULL (backlog)
//   - altrimenti restano sullo sprint chiuso
func (s *Service) Complete(sprintID string, moveOpenToBacklog bool, moveToSprintID *string) (*Sprint, error) {
	var sp Sprint
	if err := s.db.First(&sp, "id = ?", sprintID).Error; err != nil {
		return nil, errors.New("sprint not found")
	}
	now := time.Now()
	sp.State = StateClosed
	sp.EndDate = &now
	sp.CompleteDate = &now
	if err := s.db.Save(&sp).Error; err != nil {
		return nil, err
	}
	switch {
	case moveToSprintID != nil:
		s.db.Exec("UPDATE issues SET sprint_id = ? WHERE sprint_id = ? AND status_id NOT IN (SELECT id FROM workflow_statuses WHERE category = 'done')", *moveToSprintID, sprintID)
	case moveOpenToBacklog:
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

// UpdateFull aggiorna i campi modificabili di uno sprint (name/goal/state/date).
// I puntatori nil lasciano il campo invariato.
func (s *Service) UpdateFull(id string, name, goal, state *string, start, end *time.Time) (*Sprint, error) {
	var sp Sprint
	if err := s.db.Where("id = ?", id).First(&sp).Error; err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if name != nil {
		updates["name"] = *name
	}
	if goal != nil {
		updates["goal"] = *goal
	}
	if state != nil {
		updates["state"] = *state
		if *state == string(StateClosed) {
			updates["complete_date"] = time.Now()
		}
	}
	if start != nil {
		updates["start_date"] = *start
	}
	if end != nil {
		updates["end_date"] = *end
	}
	if len(updates) > 0 {
		if err := s.db.Model(&sp).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetByID(id)
}

func (s *Service) DB() *gorm.DB { return s.db }
