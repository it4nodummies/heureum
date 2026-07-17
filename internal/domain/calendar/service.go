package calendar

import (
	"time"

	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) GetCalendarData(projectID string, year, month int) (*CalendarData, error) {
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	var issues []CalendarIssue
	s.db.Raw(`
		SELECT i.id, i.key, i.title, i.priority, ws.name AS status, i.due_date, i.start_date
		FROM issues i
		LEFT JOIN workflow_statuses ws ON i.status_id = ws.id
		WHERE i.project_id = ? AND i.is_archived = false
			AND ((i.due_date IS NOT NULL AND i.due_date >= ? AND i.due_date <= ?)
				OR (i.start_date IS NOT NULL AND i.start_date >= ? AND i.start_date <= ?))
		ORDER BY i.due_date ASC
	`, projectID, firstDay, lastDay, firstDay, lastDay).Scan(&issues)

	issueMap := make(map[int][]CalendarIssue)
	for _, iss := range issues {
		// Prefer the due date; fall back to start date for issues that only
		// have a start_date in this month (otherwise they'd be fetched but
		// never placed on a day).
		var day int
		switch {
		case iss.DueDate != nil && int(iss.DueDate.Month()) == month && iss.DueDate.Year() == year:
			day = iss.DueDate.Day()
		case iss.StartDate != nil && int(iss.StartDate.Month()) == month && iss.StartDate.Year() == year:
			day = iss.StartDate.Day()
		default:
			continue
		}
		issueMap[day] = append(issueMap[day], iss)
	}

	days := make([]CalendarDay, 0, lastDay.Day())
	for d := 1; d <= lastDay.Day(); d++ {
		date := time.Date(year, time.Month(month), d, 0, 0, 0, 0, time.UTC)
		entries := issueMap[d]
		if entries == nil {
			entries = []CalendarIssue{}
		}
		days = append(days, CalendarDay{
			Date:   date.Format("2006-01-02"),
			Day:    d,
			Issues: entries,
		})
	}

	return &CalendarData{
		Year:      year,
		Month:     month,
		Days:      days,
		TotalDays: lastDay.Day(),
	}, nil
}
