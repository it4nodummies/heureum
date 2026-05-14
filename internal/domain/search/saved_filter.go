package search

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SavedFilter struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID *string   `gorm:"type:text" json:"project_id,omitempty"`
	OwnerID   string    `gorm:"type:text;not null" json:"owner_id"`
	Name      string    `gorm:"type:text;not null" json:"name"`
	JQL       string    `gorm:"type:text;default:''" json:"jql"`
	IsShared  bool      `gorm:"default:false" json:"is_shared"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type FilterService struct {
	db *gorm.DB
}

func NewFilterService(db *gorm.DB) *FilterService {
	return &FilterService{db: db}
}

func (s *FilterService) List(userID string) ([]SavedFilter, error) {
	var filters []SavedFilter
	if err := s.db.Where("owner_id = ? OR is_shared = ?", userID, true).Order("created_at DESC").Find(&filters).Error; err != nil {
		return nil, err
	}
	return filters, nil
}

func (s *FilterService) Create(ownerID string, projectID *string, name, jql string, isShared bool) (*SavedFilter, error) {
	f := &SavedFilter{
		ID:        uuid.New().String(),
		OwnerID:   ownerID,
		ProjectID: projectID,
		Name:      name,
		JQL:       jql,
		IsShared:  isShared,
	}
	if err := s.db.Create(f).Error; err != nil {
		return nil, err
	}
	return f, nil
}

func (s *FilterService) Get(id string) (*SavedFilter, error) {
	var f SavedFilter
	if err := s.db.Where("id = ?", id).First(&f).Error; err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *FilterService) Delete(id string) error {
	return s.db.Where("id = ?", id).Delete(&SavedFilter{}).Error
}
