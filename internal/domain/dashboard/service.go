package dashboard

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Dashboard struct {
	ID         string    `gorm:"primaryKey;type:text" json:"id"`
	Name       string    `gorm:"type:text;not null" json:"name"`
	OwnerID    string    `gorm:"type:text;not null;index" json:"owner_id"`
	IsPublic   bool      `gorm:"default:false" json:"is_public"`
	LayoutJSON string    `gorm:"type:text;default:'{}'" json:"layout_json"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type DashboardWidget struct {
	ID           string `gorm:"primaryKey;type:text" json:"id"`
	DashboardID  string `gorm:"type:text;not null;index" json:"dashboard_id"`
	WidgetType   string `gorm:"type:text;not null" json:"widget_type"`
	ConfigJSON   string `gorm:"type:text;default:'{}'" json:"config_json"`
	PositionJSON string `gorm:"type:text;default:'{}'" json:"position_json"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) CreateDashboard(ownerID, name string) (*Dashboard, error) {
	if name == "" {
		return nil, errors.New("dashboard name required")
	}
	d := &Dashboard{
		ID:      uuid.New().String(),
		Name:    name,
		OwnerID: ownerID,
	}
	if err := s.db.Create(d).Error; err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Service) GetDashboard(id string) (*Dashboard, error) {
	var d Dashboard
	if err := s.db.First(&d, "id = ?", id).Error; err != nil {
		return nil, errors.New("dashboard not found")
	}
	return &d, nil
}

func (s *Service) ListDashboards(ownerID string) ([]Dashboard, error) {
	var dashboards []Dashboard
	s.db.Where("owner_id = ? OR is_public = ?", ownerID, true).
		Order("created_at DESC").Find(&dashboards)
	return dashboards, nil
}

func (s *Service) UpdateDashboard(id, name string, isPublic *bool, layoutJSON *string) (*Dashboard, error) {
	d, err := s.GetDashboard(id)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if name != "" {
		updates["name"] = name
	}
	if isPublic != nil {
		updates["is_public"] = *isPublic
	}
	if layoutJSON != nil {
		updates["layout_json"] = *layoutJSON
	}
	if len(updates) > 0 {
		if err := s.db.Model(d).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetDashboard(id)
}

func (s *Service) DeleteDashboard(id string) error {
	return s.db.Delete(&Dashboard{}, "id = ?", id).Error
}

func (s *Service) AddWidget(dashboardID, widgetType string, configJSON string) (*DashboardWidget, error) {
	w := &DashboardWidget{
		ID:          uuid.New().String(),
		DashboardID: dashboardID,
		WidgetType:  widgetType,
		ConfigJSON:  configJSON,
	}
	if err := s.db.Create(w).Error; err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) RemoveWidget(widgetID string) error {
	return s.db.Delete(&DashboardWidget{}, "id = ?", widgetID).Error
}

func (s *Service) GetWidgets(dashboardID string) ([]DashboardWidget, error) {
	var widgets []DashboardWidget
	s.db.Where("dashboard_id = ?", dashboardID).Find(&widgets)
	return widgets, nil
}

func (s *Service) UpdateWidget(widgetID string, configJSON string, positionJSON string) (*DashboardWidget, error) {
	var w DashboardWidget
	if err := s.db.First(&w, "id = ?", widgetID).Error; err != nil {
		return nil, errors.New("widget not found")
	}
	updates := map[string]interface{}{}
	if configJSON != "" {
		updates["config_json"] = configJSON
	}
	if positionJSON != "" {
		updates["position_json"] = positionJSON
	}
	if len(updates) > 0 {
		if err := s.db.Model(&w).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return &w, nil
}

func (s *Service) GetAssignedIssues(userID string) ([]AssignedIssue, error) {
	var issues []AssignedIssue
	s.db.Raw(`
		SELECT i.id, i.key, i.title, i.priority, i.project_id, p.name as project_name,
			i.updated_at, ws.name as status_name
		FROM issues i
		JOIN projects p ON i.project_id = p.id
		LEFT JOIN workflow_statuses ws ON i.status_id = ws.id
		WHERE i.assignee_id = ? AND i.is_archived = FALSE
		ORDER BY i.updated_at DESC
		LIMIT 20
	`, userID).Scan(&issues)
	return issues, nil
}

type AssignedIssue struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Title       string    `json:"title"`
	Priority    string    `json:"priority"`
	ProjectID   string    `json:"project_id"`
	ProjectName string    `json:"project_name"`
	UpdatedAt   time.Time `json:"updated_at"`
	StatusName  string    `json:"status_name"`
}

func (s *Service) GetActivityFeed(userID string, limit int) ([]ActivityItem, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []ActivityItem
	s.db.Raw(`
		SELECT ih.id, ih.issue_id, i.key as issue_key, i.title as issue_title,
			ih.actor_id, u.display_name as actor_name,
			ih.field_name, ih.old_value, ih.new_value, ih.created_at
		FROM issue_history ih
		JOIN issues i ON ih.issue_id = i.id
		JOIN project_members pm ON i.project_id = pm.project_id AND pm.user_id = ?
		LEFT JOIN users u ON ih.actor_id = u.id
		ORDER BY ih.created_at DESC
		LIMIT ?
	`, userID, limit).Scan(&items)
	return items, nil
}

type ActivityItem struct {
	ID         string    `json:"id"`
	IssueID    string    `json:"issue_id"`
	IssueKey   string    `json:"issue_key"`
	IssueTitle string    `json:"issue_title"`
	ActorID    *string   `json:"actor_id,omitempty"`
	ActorName  string    `json:"actor_name"`
	FieldName  string    `json:"field_name"`
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	CreatedAt  time.Time `json:"created_at"`
}

func ParseWidgetConfig(configJSON string) (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	return config, nil
}
