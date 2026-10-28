package git

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GitProviderConfig is the per-project git integration. The three secret
// columns are json:"-" so no serialization path can leak them: handlers
// expose a dedicated DTO (has_token + webhook_url) instead.
//
//   - TokenEncrypted: provider API token, AES-256-GCM ciphertext ("v1:" prefix,
//     see crypto.go). Rows written before R27 hold plaintext and are
//     re-encrypted by EnsureBackfill at startup.
//   - WebhookSecret: HMAC secret shared with the provider, used ONLY to verify
//     inbound webhook signatures.
//   - WebhookURLToken: random token that is the path segment of the inbound
//     webhook URL; the only thing the inbound route resolves a provider by.
//     Nullable (*string) because pre-R27 rows have none until backfilled.
type GitProviderConfig struct {
	ID              string       `gorm:"primaryKey;type:text" json:"id"`
	ProjectID       string       `gorm:"type:text;not null;index" json:"project_id"`
	ProviderType    ProviderType `gorm:"type:text;not null" json:"provider_type"`
	BaseURL         string       `gorm:"type:text;not null" json:"base_url"`
	TokenEncrypted  string       `gorm:"column:token_encrypted;type:text;default:''" json:"-"`
	WebhookSecret   string       `gorm:"column:webhook_secret;type:text;default:''" json:"-"`
	WebhookURLToken *string      `gorm:"column:webhook_url_token;type:text" json:"-"`
	CreatedAt       time.Time    `gorm:"autoCreateTime" json:"created_at"`
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
	db  *gorm.DB
	key []byte
}

// NewConfigService builds the git config service. key is the 32-byte
// AES-256 key for provider tokens, derived once via KeyFromSecret(APP_SECRET).
func NewConfigService(db *gorm.DB, key []byte) *ConfigService {
	return &ConfigService{db: db, key: key}
}

// CreateProvider stores a provider with its API token encrypted at rest and
// a freshly generated webhook URL token. An empty token is stored as "" (no
// token) rather than as the encryption of an empty string.
func (s *ConfigService) CreateProvider(projectID, providerType, baseURL, token, webhookSecret string) (*GitProviderConfig, error) {
	urlToken, err := newURLToken()
	if err != nil {
		return nil, err
	}
	cfg := &GitProviderConfig{
		ID:              uuid.New().String(),
		ProjectID:       projectID,
		ProviderType:    ProviderType(providerType),
		BaseURL:         baseURL,
		WebhookSecret:   webhookSecret,
		WebhookURLToken: &urlToken,
	}
	if token != "" {
		enc, err := Encrypt(s.key, token)
		if err != nil {
			return nil, err
		}
		cfg.TokenEncrypted = enc
	}
	if err := s.db.Create(cfg).Error; err != nil {
		return nil, err
	}
	return cfg, nil
}

// Token returns the provider's API token in clear, decrypting the stored
// ciphertext. "" when no token is configured. This is the only path that
// should ever read TokenEncrypted for use.
func (s *ConfigService) Token(cfg *GitProviderConfig) (string, error) {
	if cfg.TokenEncrypted == "" {
		return "", nil
	}
	return Decrypt(s.key, cfg.TokenEncrypted)
}

// BackfillStats summarises what EnsureBackfill changed (counts only — never
// token values).
type BackfillStats struct {
	Encrypted int
	URLTokens int
}

// EnsureBackfill is run at server startup, after migrations, and is
// idempotent: rows with a NULL webhook_url_token get one, and rows whose
// token_encrypted is legacy plaintext (no "v1:" prefix, non-empty) are
// encrypted in place. A second run finds nothing to do.
func (s *ConfigService) EnsureBackfill() (BackfillStats, error) {
	var stats BackfillStats
	var rows []GitProviderConfig
	if err := s.db.Where("webhook_url_token IS NULL OR (token_encrypted <> '' AND token_encrypted NOT LIKE ?)", cipherPrefix+"%").Find(&rows).Error; err != nil {
		return stats, err
	}
	for i := range rows {
		row := &rows[i]
		updates := map[string]interface{}{}
		if row.WebhookURLToken == nil {
			tok, err := newURLToken()
			if err != nil {
				return stats, err
			}
			updates["webhook_url_token"] = tok
			stats.URLTokens++
		}
		if row.TokenEncrypted != "" && !IsEncrypted(row.TokenEncrypted) {
			enc, err := Encrypt(s.key, row.TokenEncrypted)
			if err != nil {
				return stats, err
			}
			updates["token_encrypted"] = enc
			stats.Encrypted++
		}
		if len(updates) == 0 {
			continue
		}
		if err := s.db.Model(&GitProviderConfig{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return stats, err
		}
	}
	return stats, nil
}

func (s *ConfigService) GetProvider(projectID string) (*GitProviderConfig, error) {
	var cfg GitProviderConfig
	if err := s.db.Where("project_id = ?", projectID).First(&cfg).Error; err != nil {
		return nil, errors.New("git provider not found for project")
	}
	return &cfg, nil
}

// FindByWebhookURLToken resolves the provider targeted by an inbound webhook
// from the URL path token. The HMAC webhook_secret is deliberately NOT a
// lookup key: it is only ever used for signature verification.
func (s *ConfigService) FindByWebhookURLToken(token string) (*GitProviderConfig, error) {
	if token == "" {
		return nil, errors.New("invalid webhook token")
	}
	var cfg GitProviderConfig
	if err := s.db.Where("webhook_url_token = ?", token).First(&cfg).Error; err != nil {
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
		ID:       uuid.New().String(),
		IssueID:  issueID,
		PRNumber: prNumber,
		Title:    title,
		URL:      url,
		State:    state,
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
