package issue

// ListTypes returns the project's issue types, highest hierarchy level first
// (Epic, then Story/Task/Bug, then Subtask) and by name within a level — the
// order Jira's create dialog shows them. Backs Project.issueTypes (v3).
func (s *Service) ListTypes(projectID string) ([]IssueType, error) {
	var types []IssueType
	err := s.db.Where("project_id = ?", projectID).
		Order("hierarchy_level DESC, name ASC").
		Find(&types).Error
	return types, err
}
