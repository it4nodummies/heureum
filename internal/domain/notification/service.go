package notification

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Notification struct {
	ID        string `gorm:"primaryKey;type:text" json:"id"`
	UserID    string `gorm:"type:text;not null;index" json:"user_id"`
	Type      string `gorm:"type:text;not null" json:"type"`
	Title     string `gorm:"type:text;not null" json:"title"`
	Body      string `gorm:"type:text;default:''" json:"body"`
	Link      string `gorm:"type:text;default:''" json:"link"`
	IsRead    bool   `gorm:"default:false" json:"is_read"`
	CreatedAt int64  `gorm:"autoCreateTime" json:"created_at"`
}

type Service struct {
	db          *gorm.DB
	broadcaster func([]byte)
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) SetBroadcaster(fn func([]byte)) {
	s.broadcaster = fn
}

func (s *Service) Create(userID, notifType, title, body, link string) error {
	n := &Notification{
		ID:     uuid.New().String(),
		UserID: userID,
		Type:   notifType,
		Title:  title,
		Body:   body,
		Link:   link,
	}
	if err := s.db.Create(n).Error; err != nil {
		return err
	}
	if s.broadcaster != nil {
		msg, _ := json.Marshal(map[string]interface{}{
			"type":    "notification",
			"user_id": userID,
			"payload": n,
		})
		s.broadcaster(msg)
	}
	return nil
}

func (s *Service) CreateBatch(userIDs []string, notifType, title, body, link string) error {
	for _, uid := range userIDs {
		if err := s.Create(uid, notifType, title, body, link); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ResolveUserIDsByUsernames(usernames []string) ([]string, error) {
	if len(usernames) == 0 {
		return nil, nil
	}
	var userIDs []string
	if err := s.db.Table("users").Where("username IN ?", usernames).Pluck("id", &userIDs).Error; err != nil {
		return nil, err
	}
	return userIDs, nil
}

func (s *Service) ListByUser(userID string, unreadOnly bool) ([]Notification, error) {
	var notifs []Notification
	q := s.db.Where("user_id = ?", userID)
	if unreadOnly {
		q = q.Where("is_read = ?", false)
	}
	q.Order("created_at DESC").Limit(50).Find(&notifs)
	return notifs, nil
}

func (s *Service) MarkRead(notificationID string) error {
	return s.db.Model(&Notification{}).Where("id = ?", notificationID).Update("is_read", true).Error
}

func (s *Service) MarkAllRead(userID string) error {
	return s.db.Model(&Notification{}).Where("user_id = ?", userID).Update("is_read", true).Error
}

func (s *Service) GetUnreadCount(userID string) (int64, error) {
	var count int64
	s.db.Model(&Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&count)
	return count, nil
}

func (s *Service) GetIssueWatchers(issueID string, excludeUserID string) ([]string, error) {
	var watchers []string
	q := s.db.Table("issue_watchers").Where("issue_id = ?", issueID)
	if excludeUserID != "" {
		q = q.Where("user_id != ?", excludeUserID)
	}
	if err := q.Pluck("user_id", &watchers).Error; err != nil {
		return nil, err
	}
	return watchers, nil
}

func (s *Service) GetProjectMembers(projectID string, excludeUserID string) ([]string, error) {
	var members []string
	q := s.db.Table("project_members").Where("project_id = ?", projectID)
	if excludeUserID != "" {
		q = q.Where("user_id != ?", excludeUserID)
	}
	if err := q.Pluck("user_id", &members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (s *Service) GetIssueProjectIDAndKey(issueID string) (string, string, error) {
	var result struct {
		ProjectID string `gorm:"column:project_id"`
		Key       string `gorm:"column:key"`
	}
	if err := s.db.Table("issues").Where("id = ?", issueID).Select("project_id", "key").Scan(&result).Error; err != nil {
		return "", "", err
	}
	return result.ProjectID, result.Key, nil
}

func (s *Service) NotifyIssueAssigned(issueID, assigneeUserID, issueKey, issueTitle string) error {
	link := fmt.Sprintf("/issues/%s", issueKey)
	return s.Create(assigneeUserID, "assignment", "You were assigned to an issue",
		fmt.Sprintf("You were assigned to %s: %s", issueKey, issueTitle), link)
}

func (s *Service) NotifyIssueCommented(issueID, commenterID, issueKey, issueTitle, commentPreview string) error {
	watchers, err := s.GetIssueWatchers(issueID, commenterID)
	if err != nil {
		return err
	}
	if len(watchers) == 0 {
		return nil
	}
	link := fmt.Sprintf("/issues/%s", issueKey)
	body := commentPreview
	if len(body) > 200 {
		body = body[:200] + "..."
	}
	return s.CreateBatch(watchers, "comment", fmt.Sprintf("New comment on %s", issueKey),
		fmt.Sprintf("%s: %s", issueTitle, body), link)
}

// NotifyUsersMentionedByIDs notifica gli utenti citati passandone direttamente
// gli user id (es. dai nodi ADF mention, attrs.id = user id). Deduplica gli id e
// salta l'autore, così un utente citato più volte (o sia via @username testuale
// sia via nodo ADF) riceve una sola notifica "mention".
func (s *Service) NotifyUsersMentionedByIDs(userIDs []string, authorID, issueKey, issueTitle string) error {
	if len(userIDs) == 0 {
		return nil
	}
	seen := map[string]bool{}
	mentioned := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id == "" || id == authorID || seen[id] {
			continue
		}
		seen[id] = true
		mentioned = append(mentioned, id)
	}
	if len(mentioned) == 0 {
		return nil
	}
	link := fmt.Sprintf("/issues/%s", issueKey)
	return s.CreateBatch(mentioned, "mention", fmt.Sprintf("You were mentioned in %s", issueKey),
		fmt.Sprintf("Someone mentioned you in the comments of %s: %s", issueKey, issueTitle), link)
}

func (s *Service) NotifySprintStarted(projectID, sprintID, sprintName string, startedByUserID string) error {
	members, err := s.GetProjectMembers(projectID, startedByUserID)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}
	return s.CreateBatch(members, "sprint_started", "Sprint started",
		fmt.Sprintf("Sprint \"%s\" has started", sprintName), fmt.Sprintf("/sprints/%s", sprintID))
}

func (s *Service) NotifySprintCompleted(projectID, sprintID, sprintName string, completedByUserID string) error {
	members, err := s.GetProjectMembers(projectID, completedByUserID)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}
	return s.CreateBatch(members, "sprint_completed", "Sprint completed",
		fmt.Sprintf("Sprint \"%s\" has been completed", sprintName), fmt.Sprintf("/sprints/%s", sprintID))
}

func (s *Service) NotifyStatusChanged(issueID, newStatus, issueKey, issueTitle, actorUserID string) error {
	watchers, err := s.GetIssueWatchers(issueID, actorUserID)
	if err != nil {
		return err
	}
	if len(watchers) == 0 {
		return nil
	}
	link := fmt.Sprintf("/issues/%s", issueKey)
	return s.CreateBatch(watchers, "status_change", fmt.Sprintf("Status changed on %s", issueKey),
		fmt.Sprintf("%s status changed to %s", issueTitle, newStatus), link)
}
