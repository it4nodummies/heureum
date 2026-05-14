package notification

type NotificationSetting struct {
	UserID    string `gorm:"primaryKey;type:text" json:"user_id"`
	ProjectID string `gorm:"primaryKey;type:text;default:''" json:"project_id"`
	EventType string `gorm:"primaryKey;type:text" json:"event_type"`
	ViaEmail  bool   `gorm:"default:true" json:"via_email"`
	ViaApp    bool   `gorm:"default:true" json:"via_app"`
}

func (s *Service) GetSettings(userID string) ([]NotificationSetting, error) {
	var settings []NotificationSetting
	if err := s.db.Where("user_id = ?", userID).Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func (s *Service) UpdateSetting(userID, projectID, eventType string, viaEmail, viaApp bool) error {
	setting := NotificationSetting{
		UserID:    userID,
		ProjectID: projectID,
		EventType: eventType,
		ViaEmail:  viaEmail,
		ViaApp:    viaApp,
	}
	return s.db.Where("user_id = ? AND project_id = ? AND event_type = ?", userID, projectID, eventType).
		Assign(&setting).
		FirstOrCreate(&setting).Error
}
