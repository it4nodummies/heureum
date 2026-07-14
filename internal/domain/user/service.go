package user

import "gorm.io/gorm"

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) DB() *gorm.DB { return s.db }

func (s *Service) GetByID(id string) (*User, error) {
	var u User
	if err := s.db.First(&u, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// Search cerca utenti attivi per displayName o email (case-insensitive).
func (s *Service) Search(query string, limit int) ([]User, error) {
	var users []User
	like := "%" + query + "%"
	q := s.db.Where("is_active = ? AND (LOWER(display_name) LIKE LOWER(?) OR LOWER(email) LIKE LOWER(?))", true, like, like).Order("display_name ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// AssignableForProject: utenti membri del progetto (assegnabili).
func (s *Service) AssignableForProject(projectID string, query string, limit int) ([]User, error) {
	var users []User
	q := s.db.
		Joins("JOIN project_members pm ON pm.user_id = users.id").
		Where("pm.project_id = ? AND users.is_active = ?", projectID, true)
	if query != "" {
		like := "%" + query + "%"
		q = q.Where("LOWER(users.display_name) LIKE LOWER(?) OR LOWER(users.email) LIKE LOWER(?)", like, like)
	}
	q = q.Order("users.display_name ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateProfile aggiorna i campi profilo (nil = invariato).
func (s *Service) UpdateProfile(id string, displayName, timeZone, locale, avatarURL *string) (*User, error) {
	updates := map[string]any{}
	if displayName != nil {
		updates["display_name"] = *displayName
	}
	if timeZone != nil {
		updates["time_zone"] = *timeZone
	}
	if locale != nil {
		updates["locale"] = *locale
	}
	if avatarURL != nil {
		updates["avatar_url"] = *avatarURL
	}
	if len(updates) > 0 {
		if err := s.db.Model(&User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetByID(id)
}
