package board

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) DB() *gorm.DB { return s.db }

// Create crea una board assegnando il prossimo seq_id (max+1, da 1).
func (s *Service) Create(projectID, name, boardType string, filterID *string) (*Board, error) {
	var maxSeq int64
	if err := s.db.Model(&Board{}).Select("COALESCE(MAX(seq_id), 0)").Scan(&maxSeq).Error; err != nil {
		return nil, err
	}
	b := &Board{
		ID:        uuid.NewString(),
		SeqID:     maxSeq + 1,
		Name:      name,
		Type:      boardType,
		ProjectID: projectID,
		FilterID:  filterID,
	}
	if err := s.db.Create(b).Error; err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Service) GetBySeqID(seqID int64) (*Board, error) {
	var b Board
	if err := s.db.Where("seq_id = ?", seqID).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) GetByID(id string) (*Board, error) {
	var b Board
	if err := s.db.Where("id = ?", id).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) ListByProject(projectID string) ([]Board, error) {
	var boards []Board
	if err := s.db.Where("project_id = ?", projectID).Order("seq_id ASC").Find(&boards).Error; err != nil {
		return nil, err
	}
	return boards, nil
}

// List restituisce tutte le board con paginazione offset, più il totale.
func (s *Service) List(offset, limit int) ([]Board, int, error) {
	var total int64
	if err := s.db.Model(&Board{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var boards []Board
	q := s.db.Order("seq_id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&boards).Error; err != nil {
		return nil, 0, err
	}
	return boards, int(total), nil
}

func (s *Service) Delete(id string) error {
	return s.db.Where("id = ?", id).Delete(&Board{}).Error
}
