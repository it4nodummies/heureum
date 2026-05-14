package search

import (
	"strings"

	"gorm.io/gorm"
)

type Query struct {
	ProjectKey string
	TypeName   string
	Status     string
	Assignee   string
	Priority   string
	Sprint     string
	Label      string
	Text       string
}

func Parse(query string) *Query {
	q := &Query{}
	parts := strings.Fields(query)
	for _, part := range parts {
		if strings.HasPrefix(part, "project=") {
			q.ProjectKey = strings.TrimPrefix(part, "project=")
		}
		if strings.HasPrefix(part, "status=") {
			q.Status = strings.TrimPrefix(part, "status=")
		}
		if strings.HasPrefix(part, "assignee=") {
			q.Assignee = strings.TrimPrefix(part, "assignee=")
		}
		if strings.HasPrefix(part, "priority=") {
			q.Priority = strings.TrimPrefix(part, "priority=")
		}
		if strings.HasPrefix(part, "type=") {
			q.TypeName = strings.TrimPrefix(part, "type=")
		}
	}
	if q.Text == "" && len(parts) > 0 && !strings.Contains(query, "=") {
		q.Text = query
	}
	return q
}

func (q *Query) Apply(db *gorm.DB) *gorm.DB {
	if q.ProjectKey != "" {
		db = db.Where("project_id IN (SELECT id FROM projects WHERE key = ?)", q.ProjectKey)
	}
	if q.Text != "" {
		db = db.Where("title LIKE ? OR description_json LIKE ?", "%"+q.Text+"%", "%"+q.Text+"%")
	}
	if q.Priority != "" {
		db = db.Where("priority = ?", q.Priority)
	}
	if q.Assignee != "" {
		db = db.Where("assignee_id IN (SELECT id FROM users WHERE username = ?)", q.Assignee)
	}
	if q.Status != "" {
		db = db.Where("status_id IN (SELECT id FROM workflow_statuses WHERE name = ?)", q.Status)
	}
	if q.TypeName != "" {
		db = db.Where("type_id IN (SELECT id FROM issue_types WHERE name = ?)", q.TypeName)
	}
	return db
}
