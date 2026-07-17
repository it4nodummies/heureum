package timeline

import "time"

type TimelineBar struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	Progress  float64    `json:"progress"`
	ParentID  *string    `json:"parent_id,omitempty"`
	Color     string     `json:"color"`
}

type TimelineData struct {
	ProjectID string        `json:"project_id"`
	Zoom      string        `json:"zoom"`
	StartDate time.Time     `json:"start_date"`
	EndDate   time.Time     `json:"end_date"`
	Bars      []TimelineBar `json:"bars"`
	Headers   []string      `json:"headers"`
}

type SprintIssueCount struct {
	SprintID string
	Name     string
	Start    *time.Time
	End      *time.Time
	Total    int
	Done     int
}

type VersionProgress struct {
	VersionID string
	Name      string
	Start     *time.Time
	Release   *time.Time
	Total     int
	Done      int
}
