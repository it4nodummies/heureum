package group

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) Create(name string) (*Group, error) {
	g := &Group{ID: uuid.NewString(), Name: name}
	if err := s.db.Create(g).Error; err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Service) FindByName(name string) (*Group, error) {
	var g Group
	if err := s.db.Where("name = ?", name).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Service) Delete(id string) error {
	return s.db.Where("id = ?", id).Delete(&Group{}).Error
}

// AddUser è idempotente (ON CONFLICT DO NOTHING via clause).
func (s *Service) AddUser(groupID, userID string) error {
	return s.db.
		Where("group_id = ? AND user_id = ?", groupID, userID).
		FirstOrCreate(&GroupMember{GroupID: groupID, UserID: userID}).Error
}

func (s *Service) RemoveUser(groupID, userID string) error {
	return s.db.Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&GroupMember{}).Error
}

// MemberIDs restituisce gli id utente membri del gruppo (paginati) + il totale.
func (s *Service) MemberIDs(groupID string, offset, limit int) ([]string, int, error) {
	var total int64
	if err := s.db.Model(&GroupMember{}).Where("group_id = ?", groupID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var members []GroupMember
	q := s.db.Where("group_id = ?", groupID).Order("user_id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&members).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	return ids, int(total), nil
}

// Search trova gruppi il cui nome contiene la query (case-insensitive).
func (s *Service) Search(query string, limit int) ([]Group, error) {
	var groups []Group
	q := s.db.Where("LOWER(name) LIKE LOWER(?)", "%"+query+"%").Order("name ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}
