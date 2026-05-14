package automation

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) CreateRule(projectID, name, triggerType, conditionsJSON, actionsJSON string) (*AutomationRule, error) {
	if name == "" {
		return nil, errors.New("rule name is required")
	}
	r := &AutomationRule{
		ID:             uuid.New().String(),
		ProjectID:      projectID,
		Name:           name,
		TriggerType:    triggerType,
		ConditionsJSON: conditionsJSON,
		ActionsJSON:    actionsJSON,
		IsActive:       true,
	}
	if err := s.db.Create(r).Error; err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Service) GetRule(id string) (*AutomationRule, error) {
	var r AutomationRule
	if err := s.db.First(&r, "id = ?", id).Error; err != nil {
		return nil, errors.New("automation rule not found")
	}
	return &r, nil
}

func (s *Service) ListRules(projectID string) ([]AutomationRule, error) {
	var rules []AutomationRule
	s.db.Where("project_id = ?", projectID).Order("created_at DESC").Find(&rules)
	return rules, nil
}

func (s *Service) UpdateRule(id string, name *string, isActive *bool, triggerType *string, conditionsJSON *string, actionsJSON *string) (*AutomationRule, error) {
	r, err := s.GetRule(id)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if name != nil {
		updates["name"] = *name
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}
	if triggerType != nil {
		updates["trigger_type"] = *triggerType
	}
	if conditionsJSON != nil {
		updates["conditions_json"] = *conditionsJSON
	}
	if actionsJSON != nil {
		updates["actions_json"] = *actionsJSON
	}
	if err := s.db.Model(r).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetRule(id)
}

func (s *Service) DeleteRule(id string) error {
	return s.db.Where("id = ?", id).Delete(&AutomationRule{}).Error
}

func (s *Service) ExecuteRule(ruleID, issueID string) (*AutomationRun, error) {
	r, err := s.GetRule(ruleID)
	if err != nil {
		return nil, err
	}
	return s.execute(r, issueID)
}

func (s *Service) TestRule(ruleID, issueID string) (*AutomationRun, error) {
	r, err := s.GetRule(ruleID)
	if err != nil {
		return nil, err
	}
	run, err := s.execute(r, issueID)
	run.Status = "test"
	run.Log = "Test execution: actions would be applied\n" + run.Log
	s.db.Save(run)
	return run, err
}

func (s *Service) execute(r *AutomationRule, issueID string) (*AutomationRun, error) {
	run := &AutomationRun{
		ID:      uuid.New().String(),
		RuleID:  r.ID,
		IssueID: &issueID,
		Status:  "success",
	}

	if !r.IsActive {
		run.Status = "skipped"
		run.Log = "Rule is inactive"
		s.db.Create(run)
		return run, nil
	}

	var conditions map[string]interface{}
	if err := json.Unmarshal([]byte(r.ConditionsJSON), &conditions); err != nil {
		run.Status = "error"
		run.Log = fmt.Sprintf("Invalid conditions JSON: %v", err)
		s.db.Create(run)
		return run, err
	}

	if len(conditions) > 0 {
		if !s.evaluateConditions(issueID, conditions) {
			run.Status = "skipped"
			run.Log = "Conditions not met"
			s.db.Create(run)
			return run, nil
		}
	}

	var actions []map[string]interface{}
	if err := json.Unmarshal([]byte(r.ActionsJSON), &actions); err != nil {
		run.Status = "error"
		run.Log = fmt.Sprintf("Invalid actions JSON: %v", err)
		s.db.Create(run)
		return run, err
	}

	var log string
	for _, action := range actions {
		actionType, _ := action["type"].(string)
		value, _ := action["value"].(string)
		switch actionType {
		case "set_assignee":
			s.db.Table("issues").Where("id = ?", issueID).Update("assignee_id", value)
			log += fmt.Sprintf("Set assignee to %s\n", value)
		case "add_label":
			s.applyLabel(issueID, value)
			log += fmt.Sprintf("Added label: %s\n", value)
		case "transition_issue":
			s.db.Table("issues").Where("id = ?", issueID).Update("status_id", value)
			log += fmt.Sprintf("Transitioned to status %s\n", value)
		case "add_comment":
			commentID := uuid.New().String()
			commentJSON := fmt.Sprintf(`{"content":"%s"}`, value)
			s.db.Exec("INSERT INTO comments (id, issue_id, body_json) VALUES (?, ?, ?)", commentID, issueID, commentJSON)
			log += fmt.Sprintf("Added comment: %s\n", value)
		default:
			log += fmt.Sprintf("Unknown action type: %s\n", actionType)
		}
	}
	run.Log = log
	s.db.Create(run)
	return run, nil
}

func (s *Service) evaluateConditions(issueID string, conditions map[string]interface{}) bool {
	var issue struct {
		Priority string
		Title    string
		StatusID *string
	}
	s.db.Table("issues").Where("id = ?", issueID).Select("priority", "title", "status_id").Scan(&issue)

	for field, expectedVal := range conditions {
		expected, _ := expectedVal.(string)
		switch field {
		case "priority":
			if issue.Priority != expected {
				return false
			}
		case "title_contains":
			if issue.Title == "" || !contains(issue.Title, expected) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (s *Service) applyLabel(issueID, labelName string) {
	var issue struct {
		ProjectID string
	}
	s.db.Table("issues").Where("id = ?", issueID).Select("project_id").Scan(&issue)
	labelID := uuid.New().String()
	s.db.Exec("INSERT INTO labels (id, project_id, name) VALUES (?, ?, ?) ON CONFLICT DO NOTHING", labelID, issue.ProjectID, labelName)
	var existingLabel string
	s.db.Table("labels").Where("project_id = ? AND name = ?", issue.ProjectID, labelName).Select("id").Scan(&existingLabel)
	if existingLabel != "" {
		labelID = existingLabel
	}
	s.db.Exec("INSERT INTO issue_labels (issue_id, label_id) VALUES (?, ?) ON CONFLICT DO NOTHING", issueID, labelID)
}

func (s *Service) ProcessRules(triggerType, issueID string) {
	var rules []AutomationRule
	s.db.Where("trigger_type = ? AND is_active = ?", triggerType, true).Find(&rules)
	for _, r := range rules {
		s.execute(&r, issueID)
	}
}

func (s *Service) ListRuns(ruleID string) ([]AutomationRun, error) {
	var runs []AutomationRun
	s.db.Where("rule_id = ?", ruleID).Order("triggered_at DESC").Limit(50).Find(&runs)
	return runs, nil
}

func (s *Service) DB() *gorm.DB { return s.db }

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
