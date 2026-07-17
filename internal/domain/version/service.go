package version

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) Create(projectID, name, description string, startDate, releaseDate *time.Time) (*Version, error) {
	if name == "" {
		return nil, errors.New("version name is required")
	}
	v := &Version{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		StartDate:   startDate,
		ReleaseDate: releaseDate,
	}
	if err := s.db.Create(v).Error; err != nil {
		return nil, err
	}
	return v, nil
}

func (s *Service) Get(id string) (*Version, error) {
	var v Version
	if err := s.db.First(&v, "id = ?", id).Error; err != nil {
		return nil, errors.New("version not found")
	}
	return &v, nil
}

func (s *Service) ListByProject(projectID string) ([]Version, error) {
	var versions []Version
	s.db.Where("project_id = ?", projectID).Order("created_at ASC").Find(&versions)
	return versions, nil
}

// Update applies only the non-nil fields (nil = unchanged) via an updates map,
// then returns the refreshed version.
func (s *Service) Update(id string, name, description *string, released, archived *bool, startDate, releaseDate *time.Time) (*Version, error) {
	updates := map[string]interface{}{}
	if name != nil {
		updates["name"] = *name
	}
	if description != nil {
		updates["description"] = *description
	}
	if released != nil {
		updates["released"] = *released
	}
	if archived != nil {
		updates["archived"] = *archived
	}
	if startDate != nil {
		updates["start_date"] = *startDate
	}
	if releaseDate != nil {
		updates["release_date"] = *releaseDate
	}
	if len(updates) > 0 {
		if err := s.db.Model(&Version{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Get(id)
}

// Delete removes the version and any issue_versions pivot rows referencing it.
func (s *Service) Delete(id string) error {
	if err := s.db.Where("version_id = ?", id).Delete(&IssueVersion{}).Error; err != nil {
		return err
	}
	return s.db.Where("id = ?", id).Delete(&Version{}).Error
}

// SetFixVersions reconciles the issue_versions pivot for an issue with the
// desired set of version ids (mirrors issue.Service.SetLabels): adds missing
// links and removes the extras.
func (s *Service) SetFixVersions(issueID string, versionIDs []string) error {
	current, err := s.currentVersionIDs(issueID)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, id := range versionIDs {
		if id != "" {
			want[id] = true
		}
	}
	have := map[string]bool{}
	for _, id := range current {
		have[id] = true
	}
	for _, id := range current {
		if !want[id] {
			if err := s.db.Where("issue_id = ? AND version_id = ?", issueID, id).Delete(&IssueVersion{}).Error; err != nil {
				return err
			}
		}
	}
	for id := range want {
		if !have[id] {
			if err := s.db.Create(&IssueVersion{IssueID: issueID, VersionID: id}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) currentVersionIDs(issueID string) ([]string, error) {
	var pivots []IssueVersion
	if err := s.db.Where("issue_id = ?", issueID).Find(&pivots).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(pivots))
	for _, p := range pivots {
		ids = append(ids, p.VersionID)
	}
	return ids, nil
}

// GetFixVersions returns the versions linked to an issue via the pivot.
func (s *Service) GetFixVersions(issueID string) ([]Version, error) {
	var versions []Version
	err := s.db.
		Joins("JOIN issue_versions ON issue_versions.version_id = versions.id").
		Where("issue_versions.issue_id = ?", issueID).
		Order("versions.created_at ASC").
		Find(&versions).Error
	if err != nil {
		return nil, err
	}
	return versions, nil
}

// ProgressCounts returns (done, total) issue counts for a version: total = all
// non-archived issues linked via the pivot; done = those whose status's
// workflow category is 'done'.
func (s *Service) ProgressCounts(versionID string) (done, total int, err error) {
	var totalCount, doneCount int64

	if err = s.db.Table("issue_versions").
		Joins("JOIN issues ON issues.id = issue_versions.issue_id").
		Where("issue_versions.version_id = ? AND issues.is_archived = ?", versionID, false).
		Count(&totalCount).Error; err != nil {
		return 0, 0, err
	}

	if err = s.db.Table("issue_versions").
		Joins("JOIN issues ON issues.id = issue_versions.issue_id").
		Joins("JOIN workflow_statuses ON workflow_statuses.id = issues.status_id").
		Where("issue_versions.version_id = ? AND issues.is_archived = ? AND workflow_statuses.category = ?", versionID, false, "done").
		Count(&doneCount).Error; err != nil {
		return 0, 0, err
	}

	return int(doneCount), int(totalCount), nil
}

func (s *Service) DB() *gorm.DB { return s.db }
