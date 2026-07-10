package issue

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db       *gorm.DB
	notifier Notifier
}

type Notifier interface {
	NotifyIssueAssigned(issueID, assigneeUserID, issueKey, issueTitle string) error
	NotifyStatusChanged(issueID, newStatus, issueKey, issueTitle, actorUserID string) error
}

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

func (s *Service) Create(projectKey, projectID, title, description string, priority Priority, parentID *string, typeID *string) (*Issue, error) {
	if title == "" {
		return nil, errors.New("title is required")
	}
	var maxIssue Issue
	s.db.Where("project_id = ?", projectID).Order("created_at DESC").Limit(1).Find(&maxIssue)
	seq := int64(1)
	if maxIssue.Key != "" {
		fmt.Sscanf(maxIssue.Key, projectKey+"-%d", &seq)
		seq++
	}
	key := fmt.Sprintf("%s-%d", projectKey, seq)
	seqID, err := s.nextSeqID()
	if err != nil {
		return nil, err
	}
	issue := &Issue{
		ID:              uuid.New().String(),
		ProjectID:       projectID,
		Key:             key,
		Title:           title,
		DescriptionJSON: fmt.Sprintf(`{"content":"%s"}`, description),
		Priority:        priority,
		ParentID:        parentID,
		TypeID:          typeID,
		Position:        float64(seq * 1000),
		SeqID:           seqID,
	}
	if err := s.db.Create(issue).Error; err != nil {
		return nil, err
	}
	s.logHistory(issue.ID, "", "created", "", key)
	return issue, nil
}

func (s *Service) GetByKey(key string) (*Issue, error) {
	var issue Issue
	if err := s.db.Where("key = ?", key).First(&issue).Error; err != nil {
		return nil, errors.New("issue not found")
	}
	return &issue, nil
}

func (s *Service) nextSeqID() (int64, error) {
	var max sql.NullInt64
	// nota: MAX+1 ha una race teorica sotto create concorrenti; accettabile a questa scala.
	if err := s.db.Model(&Issue{}).Select("COALESCE(MAX(seq_id), 9999)").Scan(&max).Error; err != nil {
		return 0, err
	}
	return max.Int64 + 1, nil
}

func (s *Service) GetBySeqID(id int64) (*Issue, error) {
	var i Issue
	if err := s.db.First(&i, "seq_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &i, nil
}

// GetLabels restituisce i nomi delle label associate a una issue.
func (s *Service) GetLabels(issueID string) ([]string, error) {
	var names []string
	err := s.db.Table("labels").
		Joins("JOIN issue_labels ON issue_labels.label_id = labels.id").
		Where("issue_labels.issue_id = ?", issueID).
		Pluck("labels.name", &names).Error
	return names, err
}

func (s *Service) Update(key string, title, descriptionJSON *string, priority *Priority, assigneeID, statusID *string, storyPoints *int) (*Issue, error) {
	issue, err := s.GetByKey(key)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if title != nil {
		s.logHistory(issue.ID, "", "title", issue.Title, *title)
		updates["title"] = *title
	}
	if descriptionJSON != nil {
		s.logHistory(issue.ID, "", "description", issue.DescriptionJSON, *descriptionJSON)
		updates["description_json"] = *descriptionJSON
	}
	if priority != nil {
		s.logHistory(issue.ID, "", "priority", string(issue.Priority), string(*priority))
		updates["priority"] = *priority
	}
	if assigneeID != nil {
		old := ""
		if issue.AssigneeID != nil {
			old = *issue.AssigneeID
		}
		s.logHistory(issue.ID, "", "assignee", old, *assigneeID)
		updates["assignee_id"] = *assigneeID
		if *assigneeID != "" && s.notifier != nil {
			s.notifier.NotifyIssueAssigned(issue.ID, *assigneeID, issue.Key, issue.Title)
		}
	}
	if statusID != nil {
		old := ""
		if issue.StatusID != nil {
			old = *issue.StatusID
		}
		s.logHistory(issue.ID, "", "status", old, *statusID)
		updates["status_id"] = *statusID
		if *statusID != old && s.notifier != nil {
			s.notifier.NotifyStatusChanged(issue.ID, *statusID, issue.Key, issue.Title, "")
		}
	}
	if storyPoints != nil {
		s.logHistory(issue.ID, "", "story_points", fmt.Sprintf("%d", issue.StoryPoints), fmt.Sprintf("%d", *storyPoints))
		updates["story_points"] = *storyPoints
	}
	if err := s.db.Model(issue).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetByKey(key)
}

func (s *Service) Delete(key string) error {
	return s.db.Model(&Issue{}).Where("key = ?", key).Update("is_archived", true).Error
}

func (s *Service) AddLabel(issueID, projectID, name, color string) (*Label, error) {
	label := Label{ID: uuid.New().String(), ProjectID: projectID, Name: name, Color: color}
	s.db.Where("project_id = ? AND name = ?", projectID, name).FirstOrCreate(&label)
	il := &IssueLabel{IssueID: issueID, LabelID: label.ID}
	if err := s.db.Create(il).Error; err != nil {
		return nil, err
	}
	return &label, nil
}

func (s *Service) RemoveLabel(issueID, labelID string) error {
	return s.db.Where("issue_id = ? AND label_id = ?", issueID, labelID).Delete(&IssueLabel{}).Error
}

func (s *Service) AddLink(sourceID, targetID string, linkType LinkType) (*IssueLink, error) {
	link := &IssueLink{ID: uuid.New().String(), SourceID: sourceID, TargetID: targetID, LinkType: linkType}
	if err := s.db.Create(link).Error; err != nil {
		return nil, err
	}
	return link, nil
}

func (s *Service) ListLinks(issueID string) ([]IssueLink, error) {
	var links []IssueLink
	s.db.Where("source_id = ? OR target_id = ?", issueID, issueID).Find(&links)
	return links, nil
}

func (s *Service) GetLink(linkID string) (*IssueLink, error) {
	var link IssueLink
	if err := s.db.Where("id = ?", linkID).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func (s *Service) DeleteLink(linkID string) error {
	return s.db.Where("id = ?", linkID).Delete(&IssueLink{}).Error
}

func (s *Service) ListByProject(projectID string, opts ...ListOption) ([]Issue, error) {
	q := s.db.Where("project_id = ? AND is_archived = ?", projectID, false)
	for _, o := range opts {
		q = o(q)
	}
	var issues []Issue
	if err := q.Order("position ASC").Find(&issues).Error; err != nil {
		return nil, err
	}
	return issues, nil
}

func (s *Service) GetChildren(parentID string) ([]Issue, error) {
	var issues []Issue
	s.db.Where("parent_id = ? AND is_archived = ?", parentID, false).Order("position ASC").Find(&issues)
	return issues, nil
}

func (s *Service) Watch(issueID, userID string) error {
	return s.db.Create(&IssueWatcher{IssueID: issueID, UserID: userID}).Error
}

func (s *Service) Unwatch(issueID, userID string) error {
	return s.db.Where("issue_id = ? AND user_id = ?", issueID, userID).Delete(&IssueWatcher{}).Error
}

func (s *Service) GetWatchers(issueID string) ([]IssueWatcher, error) {
	var watchers []IssueWatcher
	err := s.db.Where("issue_id = ?", issueID).Find(&watchers).Error
	return watchers, err
}

func (s *Service) GetHistory(issueID string) ([]IssueHistory, error) {
	var h []IssueHistory
	s.db.Where("issue_id = ?", issueID).Order("created_at DESC").Find(&h)
	return h, nil
}

func (s *Service) DB() *gorm.DB { return s.db }

func (s *Service) logHistory(issueID, actorID, field, oldVal, newVal string) {
	h := &IssueHistory{ID: uuid.New().String(), IssueID: issueID, ActorID: &actorID, FieldName: field, OldValue: oldVal, NewValue: newVal}
	s.db.Create(h)
}

type ListOption func(*gorm.DB) *gorm.DB

func WithStatus(statusID string) ListOption {
	return func(db *gorm.DB) *gorm.DB { return db.Where("status_id = ?", statusID) }
}
func WithAssignee(userID string) ListOption {
	return func(db *gorm.DB) *gorm.DB { return db.Where("assignee_id = ?", userID) }
}
func WithPriority(priority Priority) ListOption {
	return func(db *gorm.DB) *gorm.DB { return db.Where("priority = ?", priority) }
}
func WithSprint(sprintID string) ListOption {
	return func(db *gorm.DB) *gorm.DB { return db.Where("sprint_id = ?", sprintID) }
}

func WithNotArchived() ListOption {
	return func(db *gorm.DB) *gorm.DB { return db.Where("is_archived = ?", false) }
}
