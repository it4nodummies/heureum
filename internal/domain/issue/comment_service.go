package issue

import (
	"errors"
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
	NotifyUsersMentionedByIDs(mentionedUserIDs []string, commenterID, issueKey, issueTitle string) error
	ResolveUserIDsByUsernames(usernames []string) ([]string, error)
}

func NewCommentService(db *gorm.DB) *CommentService {
	return &CommentService{db: db}
}

func (s *CommentService) SetNotifier(n CommentNotifier) {
	s.notifier = n
}

// AddComment crea un commento. adfMentionUserIDs sono gli user id estratti dai
// nodi ADF mention (attrs.id) dal chiamante (comment_handler); vengono uniti
// alle mention testuali @username e notificati una sola volta (dedupe lato
// notifier).
func (s *CommentService) AddComment(issueID, authorID, bodyJSON string, adfMentionUserIDs ...string) (*Comment, error) {
	// comments.author_id and issue_history.actor_id are nullable FKs to
	// users.id: a blank author (e.g. system-generated git commit comments,
	// git_handler passes "") must be stored as SQL NULL, not "", which would
	// violate the FK on Postgres.
	var authorPtr *string
	if authorID != "" {
		authorPtr = &authorID
	}
	c := &Comment{
		ID:       uuid.New().String(),
		IssueID:  issueID,
		AuthorID: authorPtr,
		BodyJSON: bodyJSON,
	}
	if err := s.db.Create(c).Error; err != nil {
		return nil, err
	}
	h := &IssueHistory{
		ID:        uuid.New().String(),
		IssueID:   issueID,
		ActorID:   authorPtr,
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
			var mentionUserIDs []string
			if mentions := ParseMentions(bodyJSON); len(mentions) > 0 {
				if userIDs, err := s.notifier.ResolveUserIDsByUsernames(mentions); err == nil {
					mentionUserIDs = append(mentionUserIDs, userIDs...)
				}
			}
			mentionUserIDs = append(mentionUserIDs, adfMentionUserIDs...)
			if len(mentionUserIDs) > 0 {
				s.notifier.NotifyUsersMentionedByIDs(mentionUserIDs, authorID, issueKey, issueTitle)
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

// ErrCommentNotFound is returned by the issue-scoped comment lookups when the
// comment does not exist, is deleted, or belongs to a different issue. Callers
// map it to 404 so a comment id from another issue/project is indistinguishable
// from a missing one.
var ErrCommentNotFound = errors.New("comment not found")

// GetComment loads a live comment bound to issueID: a comment id that belongs
// to another issue is reported as not found, never returned.
func (s *CommentService) GetComment(issueID, commentID string) (*Comment, error) {
	var c Comment
	err := s.db.Where("id = ? AND issue_id = ? AND is_deleted = ?", commentID, issueID, false).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCommentNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateComment replaces the body of a comment bound to issueID.
func (s *CommentService) UpdateComment(issueID, commentID string, bodyJSON string) (*Comment, error) {
	res := s.db.Model(&Comment{}).Where("id = ? AND issue_id = ? AND is_deleted = ?", commentID, issueID, false).Update("body_json", bodyJSON)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrCommentNotFound
	}
	return s.GetComment(issueID, commentID)
}

func (s *CommentService) GetCommentsByIDs(commentIDs []string) ([]Comment, error) {
	var comments []Comment
	err := s.db.Where("id IN ? AND is_deleted = ?", commentIDs, false).
		Order("created_at ASC").Find(&comments).Error
	return comments, err
}

// SoftDeleteComment marks a comment bound to issueID as deleted.
func (s *CommentService) SoftDeleteComment(issueID, commentID string) error {
	res := s.db.Model(&Comment{}).Where("id = ? AND issue_id = ? AND is_deleted = ?", commentID, issueID, false).Update("is_deleted", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCommentNotFound
	}
	return nil
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
