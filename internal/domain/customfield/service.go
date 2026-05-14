package customfield

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) CreateField(projectID, name string, fieldType FieldType) (*CustomField, error) {
	if name == "" {
		return nil, errors.New("field name is required")
	}
	if fieldType == "" {
		return nil, errors.New("field type is required")
	}
	f := &CustomField{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Name:      name,
		FieldType: fieldType,
	}
	if err := s.db.Create(f).Error; err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Service) GetField(id string) (*CustomField, error) {
	var f CustomField
	if err := s.db.First(&f, "id = ?", id).Error; err != nil {
		return nil, errors.New("custom field not found")
	}
	return &f, nil
}

func (s *Service) ListFields(projectID string) ([]CustomField, error) {
	var fields []CustomField
	s.db.Where("project_id = ?", projectID).Order("created_at ASC").Find(&fields)
	return fields, nil
}

func (s *Service) DeleteField(id string) error {
	return s.db.Where("id = ?", id).Delete(&CustomField{}).Error
}

func (s *Service) AddOption(fieldID, value string) (*CustomFieldOption, error) {
	if value == "" {
		return nil, errors.New("option value is required")
	}
	var count int64
	s.db.Model(&CustomFieldOption{}).Where("field_id = ?", fieldID).Count(&count)
	o := &CustomFieldOption{
		ID:       uuid.New().String(),
		FieldID:  fieldID,
		Value:    value,
		Position: int(count),
	}
	if err := s.db.Create(o).Error; err != nil {
		return nil, err
	}
	return o, nil
}

func (s *Service) RemoveOption(id string) error {
	return s.db.Where("id = ?", id).Delete(&CustomFieldOption{}).Error
}

func (s *Service) ListOptions(fieldID string) ([]CustomFieldOption, error) {
	var opts []CustomFieldOption
	s.db.Where("field_id = ?", fieldID).Order("position ASC").Find(&opts)
	return opts, nil
}

func (s *Service) SetValue(issueID, fieldID string, value interface{}) error {
	cv := &IssueCustomValue{
		IssueID: issueID,
		FieldID: fieldID,
	}
	s.db.Where("issue_id = ? AND field_id = ?", issueID, fieldID).FirstOrCreate(cv)

	switch v := value.(type) {
	case string:
		cv.ValueText = v
	case float64:
		cv.ValueNumber = &v
	default:
		return errors.New("unsupported value type for custom field")
	}

	return s.db.Save(cv).Error
}

func (s *Service) SetOptionValue(issueID, fieldID, optionID string) error {
	cv := &IssueCustomValue{IssueID: issueID, FieldID: fieldID}
	s.db.Where("issue_id = ? AND field_id = ?", issueID, fieldID).FirstOrCreate(cv)
	cv.ValueText = ""
	cv.ValueNumber = nil
	cv.OptionID = &optionID
	return s.db.Save(cv).Error
}

func (s *Service) GetValues(issueID string) ([]IssueCustomValue, error) {
	var values []IssueCustomValue
	s.db.Where("issue_id = ?", issueID).Find(&values)
	return values, nil
}

func (s *Service) DB() *gorm.DB { return s.db }
