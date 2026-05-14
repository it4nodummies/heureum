package issue

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CommentService struct {
	db *gorm.DB
}

func NewCommentService(db *gorm.DB) *CommentService {
	return &CommentService{db: db}
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
	return c, nil
}

func (s *CommentService) GetComments(issueID string) ([]Comment, error) {
	var comments []Comment
	err := s.db.Where("issue_id = ? AND is_deleted = ?", issueID, false).
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
