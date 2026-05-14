package timeline

import (
	"time"

	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) GetTimelineData(projectID, zoom string) (*TimelineData, error) {
	var sprints []SprintIssueCount
	s.db.Raw(`
		SELECT sp.id AS sprint_id, sp.name, sp.start_date AS start, sp.end_date AS end,
			COUNT(i.id) AS total,
			COUNT(CASE WHEN ws.category = 'done' THEN 1 END) AS done
		FROM sprints sp
		LEFT JOIN issues i ON i.sprint_id = sp.id
		LEFT JOIN workflow_statuses ws ON i.status_id = ws.id
		WHERE sp.project_id = ?
		GROUP BY sp.id, sp.name, sp.start_date, sp.end_date
		ORDER BY sp.start_date ASC
	`, projectID).Scan(&sprints)

	var earliest, latest time.Time
	bars := make([]TimelineBar, 0)

	for _, sp := range sprints {
		start := time.Now().AddDate(0, -1, 0)
		end := time.Now().AddDate(0, 1, 0)
		if sp.Start != nil {
			start = *sp.Start
		}
		if sp.End != nil {
			end = *sp.End
		}

		progress := 0.0
		if sp.Total > 0 {
			progress = float64(sp.Done) / float64(sp.Total) * 100
		}

		color := "#3B82F6"
		if progress >= 100 {
			color = "#10B981"
		} else if progress > 0 {
			color = "#F59E0B"
		}

		bars = append(bars, TimelineBar{
			ID:        sp.SprintID,
			Name:      sp.Name,
			Type:      "sprint",
			StartDate: &start,
			EndDate:   &end,
			Progress:  progress,
			Color:     color,
		})

		if earliest.IsZero() || start.Before(earliest) {
			earliest = start
		}
		if latest.IsZero() || end.After(latest) {
			latest = end
		}
	}

	var epicBars []TimelineBar
	s.db.Raw(`
		SELECT i.id, i.title AS name, i.start_date, i.due_date
		FROM issues i
		JOIN issue_types it ON i.type_id = it.id
		WHERE i.project_id = ? AND lower(it.name) = 'epic' AND i.is_archived = false
	`, projectID).Scan(&epicBars)

	for i := range epicBars {
		epicBars[i].Type = "epic"
		epicBars[i].Color = "#8B5CF6"
	}

	if earliest.IsZero() {
		earliest = time.Now().AddDate(0, -1, 0)
	}
	if latest.IsZero() {
		latest = time.Now().AddDate(0, 1, 0)
	}

	earliest = earliest.AddDate(0, 0, -7)
	latest = latest.AddDate(0, 0, 7)

	allBars := make([]TimelineBar, 0)
	allBars = append(allBars, epicBars...)
	allBars = append(allBars, bars...)

	headers := s.generateHeaders(earliest, latest, zoom)

	return &TimelineData{
		ProjectID: projectID,
		Zoom:      zoom,
		StartDate: earliest,
		EndDate:   latest,
		Bars:      allBars,
		Headers:   headers,
	}, nil
}

func (s *Service) generateHeaders(start, end time.Time, zoom string) []string {
	var headers []string
	current := start
	switch zoom {
	case "quarters":
		current = time.Date(start.Year(), time.Month(((int(start.Month())-1)/3)*3+1), 1, 0, 0, 0, 0, start.Location())
		for current.Before(end) || current.Equal(end) {
			headers = append(headers, current.Format("2006-Q1"))
			current = current.AddDate(0, 3, 0)
		}
	case "months":
		current = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
		for current.Before(end) || current.Equal(end) {
			headers = append(headers, current.Format("Jan 2006"))
			current = current.AddDate(0, 1, 0)
		}
	default:
		current = time.Date(start.Year(), start.Month(), start.Day()-int(start.Weekday()), 0, 0, 0, 0, start.Location())
		for current.Before(end) || current.Equal(end) {
			headers = append(headers, current.Format("Jan 02"))
			current = current.AddDate(0, 0, 7)
		}
	}
	return headers
}
