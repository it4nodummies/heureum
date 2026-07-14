package report

import (
	"fmt"
	"sort"
	"time"

	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/sprint"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

type BurndownData struct {
	Labels []string `json:"labels"`
	Ideal  []int    `json:"ideal"`
	Actual []int    `json:"actual"`
}

type ProjectSummary struct {
	IssueCountByStatus map[string]int `json:"issue_count_by_status"`
	CreatedLast7Days   int64          `json:"created_last_7_days"`
	UpdatedLast7Days   int64          `json:"updated_last_7_days"`
	CompletedLast7Days int64          `json:"completed_last_7_days"`
	ActiveSprint       *sprint.Sprint `json:"active_sprint,omitempty"`
}

type VelocityData struct {
	Sprints []SprintVelocity `json:"sprints"`
}

type SprintVelocity struct {
	SprintID      string `json:"sprint_id"`
	SprintName    string `json:"sprint_name"`
	Completed     int    `json:"completed"`
	TotalPlanned  int    `json:"total_planned"`
}

func (s *Service) GetBurndownData(sprintID string) (*BurndownData, error) {
	var sp sprint.Sprint
	if err := s.db.First(&sp, "id = ?", sprintID).Error; err != nil {
		return nil, fmt.Errorf("sprint not found")
	}

	start := time.Now()
	end := time.Now().Add(14 * 24 * time.Hour)
	if sp.StartDate != nil {
		start = *sp.StartDate
	}
	if sp.EndDate != nil {
		end = *sp.EndDate
	}

	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	if days > 60 {
		days = 60
	}

	labels := make([]string, days)
	for i := 0; i < days; i++ {
		d := start.Add(time.Duration(i) * 24 * time.Hour)
		if days <= 14 {
			labels[i] = d.Format("Jan 2")
		} else {
			labels[i] = fmt.Sprintf("Day %d", i+1)
		}
	}

	var sprintIssues []issue.Issue
	s.db.Where("sprint_id = ?", sprintID).Find(&sprintIssues)

	totalSP := 0
	issueEntries := make(map[string][]issue.IssueHistory)
	issueInitialSP := make(map[string]int)

	for _, iss := range sprintIssues {
		totalSP += iss.StoryPoints
		issueInitialSP[iss.ID] = iss.StoryPoints
		var history []issue.IssueHistory
		s.db.Where("issue_id = ? AND field_name IN ('story_points', 'status')",
			iss.ID).Order("created_at ASC").Find(&history)
		issueEntries[iss.ID] = history
	}

	ideal := make([]int, days)
	actual := make([]int, days)
	if totalSP == 0 {
		totalSP = 1
	}
	for i := 0; i < days; i++ {
		ideal[i] = totalSP - (totalSP*i)/days
		actual[i] = totalSP
	}

	if len(issueEntries) > 0 {
		var doneStatusID string
		s.db.Raw("SELECT ws.id FROM workflow_statuses ws JOIN workflows w ON ws.workflow_id = w.id WHERE w.project_id = ? AND ws.category = 'done' LIMIT 1",
			sp.ProjectID).Scan(&doneStatusID)

		for i := 0; i < days; i++ {
			dayEnd := start.Add(time.Duration(i+1) * 24 * time.Hour)
			remaining := 0
			for _, iss := range sprintIssues {
				inSprint := iss.CreatedAt.Before(dayEnd) || iss.CreatedAt.Equal(dayEnd)
				if !inSprint {
					continue
				}
				curSP := iss.StoryPoints
				done := false
				for _, h := range issueEntries[iss.ID] {
					if h.CreatedAt.After(dayEnd) {
						break
					}
					switch h.FieldName {
					case "story_points":
						var val int
						fmt.Sscanf(h.NewValue, "%d", &val)
						curSP = val
					case "status":
						if h.NewValue == doneStatusID {
							done = true
						}
						if h.OldValue == doneStatusID && h.NewValue != doneStatusID {
							done = false
						}
					}
				}
				if !done {
					remaining += curSP
				}
			}
			actual[i] = remaining
		}
	}

	return &BurndownData{Labels: labels, Ideal: ideal, Actual: actual}, nil
}

func (s *Service) GetProjectSummary(projectID string) (*ProjectSummary, error) {
	summary := &ProjectSummary{
		IssueCountByStatus: make(map[string]int),
	}

	type statusCount struct {
		Name  string
		Count int
	}
	var statusCounts []statusCount
	s.db.Raw(`
		SELECT COALESCE(ws.name, 'No Status') as name, COUNT(*) as count
		FROM issues i
		LEFT JOIN workflow_statuses ws ON i.status_id = ws.id
		WHERE i.project_id = ? AND i.is_archived = FALSE
		GROUP BY ws.name
	`, projectID).Scan(&statusCounts)
	for _, sc := range statusCounts {
		summary.IssueCountByStatus[sc.Name] = sc.Count
	}

	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)
	s.db.Model(&issue.Issue{}).Where("project_id = ? AND is_archived = FALSE AND created_at >= ?",
		projectID, sevenDaysAgo).Count(&summary.CreatedLast7Days)
	s.db.Model(&issue.Issue{}).Where("project_id = ? AND is_archived = FALSE AND updated_at >= ?",
		projectID, sevenDaysAgo).Count(&summary.UpdatedLast7Days)

	var doneStatusID string
	s.db.Raw(`
		SELECT ws.id FROM workflow_statuses ws
		JOIN workflows w ON ws.workflow_id = w.id
		WHERE w.project_id = ? AND ws.category = 'done'
		LIMIT 1
	`, projectID).Scan(&doneStatusID)

	if doneStatusID != "" {
		s.db.Model(&issue.Issue{}).Where("project_id = ? AND status_id = ? AND is_archived = FALSE AND updated_at >= ?",
			projectID, doneStatusID, sevenDaysAgo).Count(&summary.CompletedLast7Days)
	}

	var activeSprint sprint.Sprint
	if err := s.db.Where("project_id = ? AND state = ?", projectID, sprint.StateActive).
		First(&activeSprint).Error; err == nil {
		summary.ActiveSprint = &activeSprint
	}

	return summary, nil
}

func (s *Service) GetVelocity(projectID string) (*VelocityData, error) {
	var sprints []sprint.Sprint
	s.db.Where("project_id = ? AND state = 'closed'", projectID).
		Order("start_date DESC").Limit(10).Find(&sprints)

	var doneStatusID string
	s.db.Raw(`
		SELECT ws.id FROM workflow_statuses ws
		JOIN workflows w ON ws.workflow_id = w.id
		WHERE w.project_id = ? AND ws.category = 'done'
		LIMIT 1
	`, projectID).Scan(&doneStatusID)

	velocities := make([]SprintVelocity, 0, len(sprints))
	for _, sp := range sprints {
		var issues []issue.Issue
		s.db.Where("sprint_id = ?", sp.ID).Find(&issues)

		totalPlanned := 0
		completed := 0
		for _, iss := range issues {
			totalPlanned += iss.StoryPoints
			if iss.StatusID != nil && *iss.StatusID == doneStatusID {
				completed += iss.StoryPoints
			}
		}
		velocities = append(velocities, SprintVelocity{
			SprintID:     sp.ID,
			SprintName:   sp.Name,
			Completed:    completed,
			TotalPlanned: totalPlanned,
		})
	}

	sort.Slice(velocities, func(i, j int) bool {
		return velocities[i].SprintName < velocities[j].SprintName
	})

	return &VelocityData{Sprints: velocities}, nil
}

func (s *Service) GetBurnupData(sprintID string) (*BurndownData, error) {
	var sp sprint.Sprint
	if err := s.db.First(&sp, "id = ?", sprintID).Error; err != nil {
		return nil, fmt.Errorf("sprint not found")
	}

	start := time.Now()
	end := time.Now().Add(14 * 24 * time.Hour)
	if sp.StartDate != nil {
		start = *sp.StartDate
	}
	if sp.EndDate != nil {
		end = *sp.EndDate
	}

	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	if days > 60 {
		days = 60
	}

	var sprintIssues []issue.Issue
	s.db.Where("sprint_id = ?", sprintID).Find(&sprintIssues)

	totalSP := 0
	issueEntries := make(map[string][]issue.IssueHistory)
	for _, iss := range sprintIssues {
		totalSP += iss.StoryPoints
		var history []issue.IssueHistory
		s.db.Where("issue_id = ? AND field_name IN ('story_points', 'status')",
			iss.ID).Order("created_at ASC").Find(&history)
		issueEntries[iss.ID] = history
	}

	var doneStatusID string
	s.db.Raw(`
		SELECT ws.id FROM workflow_statuses ws
		JOIN workflows w ON ws.workflow_id = w.id
		WHERE w.project_id = ? AND ws.category = 'done'
		LIMIT 1
	`, sp.ProjectID).Scan(&doneStatusID)

	labels := make([]string, days)
	total := make([]int, days)
	completed := make([]int, days)

	for i := 0; i < days; i++ {
		d := start.Add(time.Duration(i) * 24 * time.Hour)
		labels[i] = d.Format("Jan 2")
		total[i] = totalSP
		comp := 0
		for _, iss := range sprintIssues {
			sp := iss.StoryPoints
			currentDone := iss.StatusID != nil && *iss.StatusID == doneStatusID
			for _, h := range issueEntries[iss.ID] {
				if h.CreatedAt.After(d) {
					break
				}
				switch h.FieldName {
				case "story_points":
					var val int
					fmt.Sscanf(h.NewValue, "%d", &val)
					sp = val
				case "status":
					currentDone = h.NewValue == doneStatusID
					if h.OldValue == doneStatusID && h.NewValue != doneStatusID {
						currentDone = false
					}
				}
			}
			if currentDone {
				comp += sp
			}
		}
		completed[i] = comp
	}

	return &BurndownData{Labels: labels, Ideal: total, Actual: completed}, nil
}

func (s *Service) GetCFD(projectID string) (*CFDData, error) {
	type DayCount struct {
		Date     string
		Category string
		Count    int
	}
	var dayCounts []DayCount
	s.db.Raw(`
		SELECT
			DATE(ih.created_at) as date,
			COALESCE(ws.category, 'inprogress') as category,
			COUNT(DISTINCT ih.issue_id) as count
		FROM issue_history ih
		JOIN issues i ON ih.issue_id = i.id
		LEFT JOIN workflow_statuses ws ON i.status_id = ws.id
		WHERE i.project_id = ? AND i.is_archived = FALSE AND ih.field_name IN ('status_id', 'created')
		GROUP BY DATE(ih.created_at), COALESCE(ws.category, 'inprogress')
		ORDER BY date ASC
	`, projectID).Scan(&dayCounts)

	if len(dayCounts) == 0 {
		return &CFDData{Categories: []string{"todo", "inprogress", "done"}, Dates: []string{}, Data: map[string][]int{}}, nil
	}

	dateSet := make(map[string]bool)
	for _, dc := range dayCounts {
		dateSet[dc.Date] = true
	}
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	cfd := &CFDData{
		Categories: []string{"todo", "inprogress", "done"},
		Dates:      dates,
		Data:       make(map[string][]int),
	}
	for _, cat := range cfd.Categories {
		cfd.Data[cat] = make([]int, len(dates))
	}

	cumulative := map[string]int{"todo": 0, "inprogress": 0, "done": 0}
	dateIndex := make(map[string]int)
	for i, d := range dates {
		dateIndex[d] = i
	}
	for _, dc := range dayCounts {
		cumulative[dc.Category] += dc.Count
	}
	cfd.Data["todo"] = make([]int, len(dates))
	cfd.Data["inprogress"] = make([]int, len(dates))
	cfd.Data["done"] = make([]int, len(dates))

	var projectIssues []issue.Issue
	s.db.Where("project_id = ? AND is_archived = FALSE", projectID).Find(&projectIssues)

	defaultCatCounts := map[string]int{"todo": 0, "inprogress": 0, "done": 0}
	for _, iss := range projectIssues {
		if iss.StatusID == nil {
			defaultCatCounts["inprogress"]++
		}
	}

	for _, cat := range cfd.Categories {
		for i := range dates {
			cfd.Data[cat][i] = cumulative[cat] + defaultCatCounts[cat]
		}
	}

	return cfd, nil
}

type CFDData struct {
	Categories []string         `json:"categories"`
	Dates      []string         `json:"dates"`
	Data       map[string][]int `json:"data"`
}
