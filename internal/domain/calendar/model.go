package calendar

import "time"

type CalendarIssue struct {
	ID        string     `json:"id"`
	Key       string     `json:"key"`
	Title     string     `json:"title"`
	Priority  string     `json:"priority"`
	Status    string     `json:"status"`
	DueDate   *time.Time `json:"due_date"`
	StartDate *time.Time `json:"start_date"`
}

type CalendarDay struct {
	Date   string          `json:"date"`
	Day    int             `json:"day"`
	Issues []CalendarIssue `json:"issues"`
}

type CalendarData struct {
	Year      int           `json:"year"`
	Month     int           `json:"month"`
	Days      []CalendarDay `json:"days"`
	TotalDays int           `json:"total_days"`
}
