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

// ErrNotFound is returned by the project-scoped sprint operations when the
// sprint id does not exist in that project (missing or belonging to another
// project — indistinguishable by design).
var ErrNotFound = errors.New("sprint not found")

// ErrInvalidTarget is returned by Complete when moveToSprintID is not a sprint
// of the same project (missing or foreign — indistinguishable by design).
var ErrInvalidTarget = errors.New("target sprint not found in project")

// GetByID loads the sprint id of projectID; an id from another project is
// ErrNotFound. Any other DB error is propagated as-is (handler → 500).
func (s *Service) GetByID(projectID, id string) (*Sprint, error) {
	return getByID(s.db, projectID, id)
}

// getByID is GetByID on an arbitrary handle, so it can run inside a
// transaction.
func getByID(db *gorm.DB, projectID, id string) (*Sprint, error) {
	var sp Sprint
	if err := db.First(&sp, "id = ? AND project_id = ?", id, projectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
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

// Update renames/re-goals the sprint id of projectID.
func (s *Service) Update(projectID, id, name, goal string) (*Sprint, error) {
	sp, err := s.GetByID(projectID, id)
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

// Start activates the sprint sprintID of projectID.
func (s *Service) Start(projectID, sprintID string) (*Sprint, error) {
	sp, err := s.GetByID(projectID, sprintID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sp.State = StateActive
	sp.StartDate = &now
	if err := s.db.Save(sp).Error; err != nil {
		return nil, err
	}
	if s.notifier != nil {
		s.notifier.NotifySprintStarted(sp.ProjectID, sp.ID, sp.Name, "")
	}
	return sp, nil
}

// Complete chiude lo sprint e gestisce le issue incomplete (categoria != 'done'):
//   - moveToSprintID != nil → riassegnate allo sprint target (UUID)
//   - altrimenti moveOpenToBacklog → sprint_id NULL (backlog)
//   - altrimenti restano sullo sprint chiuso
//
// sprintID must belong to projectID, otherwise ErrNotFound and nothing changes;
// moveToSprintID must belong to projectID too, otherwise ErrInvalidTarget and
// nothing changes (the handler checks this as well — this is the last line of
// defence in front of the cross-project UPDATE).
//
// The state change and the issue moves are ONE transaction: if moving the
// issues fails the sprint stays active, instead of being closed with its open
// issues stranded on it. An issue whose status_id is NULL (no workflow status
// assigned yet) is open, so the "not done" filter is written NULL-safe —
// `status_id NOT IN (...)` alone is unknown for NULL and silently skipped it.
func (s *Service) Complete(projectID, sprintID string, moveOpenToBacklog bool, moveToSprintID *string) (*Sprint, error) {
	var sp *Sprint
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		sp, err = getByID(tx, projectID, sprintID)
		if err != nil {
			return err
		}
		if moveToSprintID != nil {
			if _, err := getByID(tx, projectID, *moveToSprintID); err != nil {
				if errors.Is(err, ErrNotFound) {
					return ErrInvalidTarget
				}
				return err
			}
		}
		now := time.Now()
		sp.State = StateClosed
		sp.EndDate = &now
		sp.CompleteDate = &now
		if err := tx.Save(sp).Error; err != nil {
			return err
		}
		const notDone = "(status_id IS NULL OR status_id NOT IN (SELECT id FROM workflow_statuses WHERE category = 'done'))"
		switch {
		case moveToSprintID != nil:
			return tx.Exec("UPDATE issues SET sprint_id = ? WHERE sprint_id = ? AND "+notDone, *moveToSprintID, sprintID).Error
		case moveOpenToBacklog:
			return tx.Exec("UPDATE issues SET sprint_id = NULL WHERE sprint_id = ? AND "+notDone, sprintID).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.notifier != nil {
		s.notifier.NotifySprintCompleted(sp.ProjectID, sp.ID, sp.Name, "")
	}
	return sp, nil
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
	return s.GetByID(sp.ProjectID, id)
}

func (s *Service) DB() *gorm.DB { return s.db }
