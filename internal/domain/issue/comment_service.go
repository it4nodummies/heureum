package issue

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CommentService struct {
	db       *gorm.DB
	notifier CommentNotifier
}

type CommentNotifier interface {
	NotifyIssueCommented(issueID, commenterID, issueKey, issueTitle, commentPreview string) error
	NotifyUsersMentioned(mentionedUserIDs []string, commenterID, issueKey, issueTitle string) error
	ResolveUserIDsByUsernames(usernames []string) ([]string, error)
}

func NewCommentService(db *gorm.DB) *CommentService {
	return &CommentService{db: db}
}

func (s *CommentService) SetNotifier(n CommentNotifier) {
	s.notifier = n
}

func (s *CommentService) AddComment(issueID, authorID, bodyJSON string) (*Comment, error) {
	c := &Comment{
		ID:       uuid.New().String(),
		IssueID:  issueID,
		AuthorID: &authorID,
		BodyJSON: bodyJSON,
	}
	if err := s.db.Create(c).Error; err != nil {
		return nil, err
	}
	h := &IssueHistory{
		ID:        uuid.New().String(),
		IssueID:   issueID,
		ActorID:   &authorID,
		FieldName: "comment",
		OldValue:  "",
		NewValue:  c.ID,
	}
	s.db.Create(h)
	if s.notifier != nil {
		var result struct {
			Key       string `gorm:"column:key"`
			Title     string `gorm:"column:title"`
			ProjectID string `gorm:"column:project_id"`
		}
		s.db.Table("issues").Where("id = ?", issueID).Select("key", "title", "project_id").Scan(&result)
		issueKey := result.Key
		issueTitle := result.Title
		if issueKey != "" {
			s.notifier.NotifyIssueCommented(issueID, authorID, issueKey, issueTitle, bodyJSON)
			mentions := ParseMentions(bodyJSON)
			if len(mentions) > 0 {
				userIDs, _ := s.notifier.ResolveUserIDsByUsernames(mentions)
				s.notifier.NotifyUsersMentioned(userIDs, authorID, issueKey, issueTitle)
			}
		}
	}
	return c, nil
}

func (s *CommentService) GetComments(issueID string) ([]Comment, error) {
	var comments []Comment
	err := s.db.Where("issue_id = ? AND is_deleted = ?", issueID, false).
		Order("created_at ASC").Find(&comments).Error
	return comments, err
}

func (s *CommentService) GetComment(commentID string) (*Comment, error) {
	var c Comment
	if err := s.db.Where("id = ? AND is_deleted = ?", commentID, false).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *CommentService) UpdateComment(commentID string, bodyJSON string) (*Comment, error) {
	if err := s.db.Model(&Comment{}).Where("id = ?", commentID).Update("body_json", bodyJSON).Error; err != nil {
		return nil, err
	}
	return s.GetComment(commentID)
}

func (s *CommentService) GetCommentsByIDs(commentIDs []string) ([]Comment, error) {
	var comments []Comment
	err := s.db.Where("id IN ? AND is_deleted = ?", commentIDs, false).
		Order("created_at ASC").Find(&comments).Error
	return comments, err
}

func (s *CommentService) SoftDeleteComment(commentID string) error {
	return s.db.Model(&Comment{}).Where("id = ?", commentID).Update("is_deleted", true).Error
}

var mentionRe = regexp.MustCompile(`@(\w+)`)

func ParseMentions(bodyJSON string) []string {
	matches := mentionRe.FindAllStringSubmatch(bodyJSON, -1)
	seen := map[string]bool{}
	var mentions []string
	for _, m := range matches {
		username := strings.ToLower(m[1])
		if !seen[username] {
			seen[username] = true
			mentions = append(mentions, username)
		}
	}
	return mentions
}
