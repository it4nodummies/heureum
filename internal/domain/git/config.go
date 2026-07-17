package git

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GitProviderConfig struct {
	ID            string       `gorm:"primaryKey;type:text" json:"id"`
	ProjectID     string       `gorm:"type:text;not null;index" json:"project_id"`
	ProviderType  ProviderType `gorm:"type:text;not null" json:"provider_type"`
	BaseURL       string       `gorm:"type:text;not null" json:"base_url"`
	TokenEncrypted string      `gorm:"type:text;default:''" json:"token_encrypted"`
	WebhookSecret string       `gorm:"type:text;default:''" json:"webhook_secret"`
	CreatedAt     time.Time    `gorm:"autoCreateTime" json:"created_at"`
}

func (GitProviderConfig) TableName() string { return "git_providers" }

type IssueCommit struct {
	ID          string     `gorm:"primaryKey;type:text" json:"id"`
	IssueID     string     `gorm:"type:text;not null;index;uniqueIndex:idx_issue_commits_issue_sha" json:"issue_id"`
	ProviderID  *string    `gorm:"type:text" json:"provider_id,omitempty"`
	CommitSHA   string     `gorm:"type:text;not null;uniqueIndex:idx_issue_commits_issue_sha" json:"commit_sha"`
	Message     string     `gorm:"type:text;default:''" json:"message"`
	Author      string     `gorm:"type:text;default:''" json:"author"`
	CommittedAt *time.Time `json:"committed_at,omitempty"`
}

func (IssueCommit) TableName() string { return "issue_commits" }

type IssueBranch struct {
	ID         string  `gorm:"primaryKey;type:text" json:"id"`
	IssueID    string  `gorm:"type:text;not null;index" json:"issue_id"`
	ProviderID *string `gorm:"type:text" json:"provider_id,omitempty"`
	BranchName string  `gorm:"type:text;not null" json:"branch_name"`
	RepoURL    string  `gorm:"type:text;default:''" json:"repo_url"`
}

func (IssueBranch) TableName() string { return "issue_branches" }

type IssuePullRequest struct {
	ID         string     `gorm:"primaryKey;type:text" json:"id"`
	IssueID    string     `gorm:"type:text;not null;index" json:"issue_id"`
	ProviderID *string    `gorm:"type:text" json:"provider_id,omitempty"`
	PRNumber   int        `gorm:"not null" json:"pr_number"`
	Title      string     `gorm:"type:text;not null" json:"title"`
	URL        string     `gorm:"type:text;default:''" json:"url"`
	State      string     `gorm:"type:text;not null;default:'open'" json:"state"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	MergedAt   *time.Time `json:"merged_at,omitempty"`
}

func (IssuePullRequest) TableName() string { return "issue_pull_requests" }

type ConfigService struct {
	db *gorm.DB
}

func NewConfigService(db *gorm.DB) *ConfigService {
	return &ConfigService{db: db}
}

func (s *ConfigService) CreateProvider(projectID, providerType, baseURL, token, webhookSecret string) (*GitProviderConfig, error) {
	cfg := &GitProviderConfig{
		ID:             uuid.New().String(),
		ProjectID:      projectID,
		ProviderType:   ProviderType(providerType),
		BaseURL:        baseURL,
		TokenEncrypted: token,
		WebhookSecret:  webhookSecret,
	}
	if err := s.db.Create(cfg).Error; err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *ConfigService) GetProvider(projectID string) (*GitProviderConfig, error) {
	var cfg GitProviderConfig
	if err := s.db.Where("project_id = ?", projectID).First(&cfg).Error; err != nil {
		return nil, errors.New("git provider not found for project")
	}
	return &cfg, nil
}

func (s *ConfigService) FindByWebhookToken(token string) (*GitProviderConfig, error) {
	var cfg GitProviderConfig
	if err := s.db.Where("webhook_secret = ?", token).First(&cfg).Error; err != nil {
		return nil, errors.New("invalid webhook token")
	}
	return &cfg, nil
}

func (s *ConfigService) DeleteProvider(providerID string) error {
	return s.db.Delete(&GitProviderConfig{}, "id = ?", providerID).Error
}

func (s *ConfigService) GetProviderByID(id string) (*GitProviderConfig, error) {
	var cfg GitProviderConfig
	if err := s.db.First(&cfg, "id = ?", id).Error; err != nil {
		return nil, errors.New("git provider not found")
	}
	return &cfg, nil
}

// LinkCommit records a commit reference for an issue, deduping on
// (issue_id, commit_sha). It returns created=true only when a new row was
// inserted; a repeated SHA returns (false, nil) so callers can skip
// re-commenting. A unique-violation on insert (e.g. a concurrent race) is also
// treated as (false, nil).
func (s *ConfigService) LinkCommit(issueID, providerID, sha, message, author string) (created bool, err error) {
	var existing IssueCommit
	err = s.db.Where("issue_id = ? AND commit_sha = ?", issueID, sha).First(&existing).Error
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}

	c := &IssueCommit{
		ID:        uuid.New().String(),
		IssueID:   issueID,
		CommitSHA: sha,
		Message:   message,
		Author:    author,
	}
	if providerID != "" {
		c.ProviderID = &providerID
	}
	if err := s.db.Create(c).Error; err != nil {
		// Defensive: a concurrent insert of the same (issue_id, sha) hits the
		// unique index. Treat that as "already linked", not an error.
		if errors.Is(err, gorm.ErrDuplicatedKey) || isUniqueViolation(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isUniqueViolation reports whether err is a unique-constraint failure across
// the supported drivers (checked textually to avoid driver-specific deps).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "duplicate entry")
}

func (s *ConfigService) LinkBranch(issueID, providerID, branchName, repoURL string) error {
	b := &IssueBranch{
		ID:         uuid.New().String(),
		IssueID:    issueID,
		BranchName: branchName,
		RepoURL:    repoURL,
	}
	if providerID != "" {
		b.ProviderID = &providerID
	}
	return s.db.Create(b).Error
}

func (s *ConfigService) LinkPullRequest(issueID, providerID string, prNumber int, title, url, state string) error {
	pr := &IssuePullRequest{
		ID:         uuid.New().String(),
		IssueID:    issueID,
		PRNumber:   prNumber,
		Title:      title,
		URL:        url,
		State:      state,
	}
	if providerID != "" {
		pr.ProviderID = &providerID
	}
	return s.db.Create(pr).Error
}

func (s *ConfigService) UpdatePullRequestState(prID string, state string, mergedAt *time.Time) error {
	updates := map[string]interface{}{"state": state}
	if mergedAt != nil {
		updates["merged_at"] = mergedAt
	}
	return s.db.Model(&IssuePullRequest{}).Where("id = ?", prID).Updates(updates).Error
}

func (s *ConfigService) FindPullRequestByNumber(providerID string, prNumber int) (*IssuePullRequest, error) {
	var pr IssuePullRequest
	if err := s.db.Where("provider_id = ? AND pr_number = ?", providerID, prNumber).First(&pr).Error; err != nil {
		return nil, err
	}
	return &pr, nil
}

func (s *ConfigService) GetIssueCommits(issueID string) ([]IssueCommit, error) {
	var commits []IssueCommit
	s.db.Where("issue_id = ?", issueID).Order("committed_at DESC").Find(&commits)
	return commits, nil
}

func (s *ConfigService) GetIssueBranches(issueID string) ([]IssueBranch, error) {
	var branches []IssueBranch
	s.db.Where("issue_id = ?", issueID).Find(&branches)
	return branches, nil
}

func (s *ConfigService) GetIssuePullRequests(issueID string) ([]IssuePullRequest, error) {
	var prs []IssuePullRequest
	s.db.Where("issue_id = ?", issueID).Order("created_at DESC").Find(&prs)
	return prs, nil
}

func (s *ConfigService) DB() *gorm.DB { return s.db }

func ProviderForType(pt ProviderType) GitProvider {
	switch pt {
	case ProviderForgejo, ProviderGitea:
		return NewForgejoProvider()
	case ProviderGitLab:
		return NewGitLabProvider()
	case ProviderGitHub:
		return NewGitHubProvider()
	default:
		return &noopProvider{}
	}
}
