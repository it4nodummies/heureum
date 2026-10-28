package contract

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/domain/git"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// TestGetProvider_ViewerForbidden: GET /project/{key}/git/providers è una
// lettura amministrativa (espone la configurazione dell'integrazione git),
// quindi deve richiedere AdministerProjects come i fratelli POST/DELETE — un
// viewer del progetto riceve 403, l'admin 200. Passa dal router reale perché
// il buco era nel wiring (BrowseProjects), non nell'handler.
func TestGetProvider_ViewerForbidden(t *testing.T) {
	srv, authSvc, db := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	bob := registerUserAndLogin(t, authSvc, "bob-viewer@example.com", "bob")

	createProjectViaAPI(t, srv, alice, "RD", "R&D")

	var bobUser user.User
	if err := db.Where("email = ?", "bob-viewer@example.com").First(&bobUser).Error; err != nil {
		t.Fatal(err)
	}
	res := doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/project/RD/members",
		map[string]any{"user_id": bobUser.ID, "role": "viewer"})
	if res.StatusCode/100 != 2 {
		_, raw := decodeBody(t, res)
		t.Fatalf("add viewer status = %d: %s", res.StatusCode, raw)
	}
	res.Body.Close()

	res = doJSON(t, srv, http.MethodPost, alice, "/rest/api/3/project/RD/git/providers",
		map[string]any{"provider_type": "github", "base_url": "https://github.com/acme/rd", "token": "ghp_secret", "webhook_secret": "hmac-secret-0123456789"})
	if res.StatusCode != http.StatusCreated {
		_, raw := decodeBody(t, res)
		t.Fatalf("configure provider status = %d: %s", res.StatusCode, raw)
	}
	res.Body.Close()

	// Viewer: 403 (stile Enforce dei fratelli POST/DELETE).
	res = doJSON(t, srv, http.MethodGet, bob, "/rest/api/3/project/RD/git/providers", nil)
	if res.StatusCode != http.StatusForbidden {
		_, raw := decodeBody(t, res)
		t.Errorf("viewer GET git/providers status = %d, want 403: %s", res.StatusCode, raw)
	} else {
		res.Body.Close()
	}

	// Admin: 200 e nessun segreto nel body.
	res = doJSON(t, srv, http.MethodGet, alice, "/rest/api/3/project/RD/git/providers", nil)
	if res.StatusCode != http.StatusOK {
		_, raw := decodeBody(t, res)
		t.Fatalf("admin GET git/providers status = %d, want 200: %s", res.StatusCode, raw)
	}
	body, raw := decodeBody(t, res)
	for _, k := range []string{"token_encrypted", "webhook_secret"} {
		if _, ok := body[k]; ok {
			t.Errorf("admin body exposes %q: %s", k, raw)
		}
	}
	if strings.Contains(string(raw), "ghp_secret") || strings.Contains(string(raw), "hmac-secret-0123456789") {
		t.Errorf("admin body contains a secret value: %s", raw)
	}
	if body["has_token"] != true {
		t.Errorf("has_token = %v, want true: %s", body["has_token"], raw)
	}
	if u, _ := body["webhook_url"].(string); !strings.Contains(u, "/rest/api/3/webhooks/git/") {
		t.Errorf("webhook_url = %q, want inbound git webhook URL", u)
	}
}

// gitWebhookFixture: two projects with one issue each (status set to the
// project's first non-done workflow status), a git provider on CC owned by
// carol — who has NO role in AA — and a poster that signs payloads with CC's
// HMAC secret and sends them to CC's inbound webhook URL.
type gitWebhookFixture struct {
	t        *testing.T
	srv      *httptest.Server
	db       *gorm.DB
	secret   string
	hookPath string
	aa, cc   issue.Issue
	aaStatus string
	ccStatus string
	ccDone   string
}

func newGitWebhookFixture(t *testing.T) *gitWebhookFixture {
	t.Helper()
	srv, authSvc, db := newTestServerDB(t)
	alice := registerAndLogin(t, authSvc)
	carol := registerUserAndLogin(t, authSvc, "carol@example.com", "carol")

	createProjectViaAPI(t, srv, alice, "AA", "Victim")
	createProjectViaAPI(t, srv, carol, "CC", "Attacker")
	aaKey := createIssueViaAPI(t, srv, alice, "AA", "victim issue")
	ccKey := createIssueViaAPI(t, srv, carol, "CC", "own issue")

	f := &gitWebhookFixture{t: t, srv: srv, db: db, secret: "carol-hmac-secret-0123456789"}
	if err := db.Where("key = ?", aaKey).First(&f.aa).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("key = ?", ccKey).First(&f.cc).Error; err != nil {
		t.Fatal(err)
	}
	f.aaStatus = f.statusFor(f.aa.ProjectID, "todo")
	f.ccStatus = f.statusFor(f.cc.ProjectID, "todo")
	f.ccDone = f.statusFor(f.cc.ProjectID, "done")
	for _, pair := range [][2]string{{f.aa.ID, f.aaStatus}, {f.cc.ID, f.ccStatus}} {
		if err := db.Model(&issue.Issue{}).Where("id = ?", pair[0]).Update("status_id", pair[1]).Error; err != nil {
			t.Fatal(err)
		}
	}

	res := doJSON(t, srv, http.MethodPost, carol, "/rest/api/3/project/CC/git/providers",
		map[string]any{"provider_type": "github", "base_url": "https://github.com/carol/cc", "token": "", "webhook_secret": f.secret})
	if res.StatusCode != http.StatusCreated {
		_, raw := decodeBody(t, res)
		t.Fatalf("configure provider status = %d: %s", res.StatusCode, raw)
	}
	body, _ := decodeBody(t, res)
	webhookURL, _ := body["webhook_url"].(string)
	i := strings.LastIndex(webhookURL, "/rest/api/3/webhooks/git/")
	if i < 0 {
		t.Fatalf("webhook_url = %q", webhookURL)
	}
	f.hookPath = webhookURL[i:]
	return f
}

// statusFor returns the id of the project's workflow status with that category.
func (f *gitWebhookFixture) statusFor(projectID, category string) string {
	f.t.Helper()
	var id string
	err := f.db.Table("workflow_statuses").
		Select("workflow_statuses.id").
		Joins("JOIN workflows ON workflows.id = workflow_statuses.workflow_id").
		Where("workflows.project_id = ? AND workflow_statuses.category = ?", projectID, category).
		Order("workflow_statuses.position").Limit(1).Scan(&id).Error
	if err != nil || id == "" {
		f.t.Fatalf("no %q status for project %s: %v", category, projectID, err)
	}
	return id
}

func (f *gitWebhookFixture) postSigned(payload string) {
	f.t.Helper()
	mac := hmac.New(sha256.New, []byte(f.secret))
	mac.Write([]byte(payload))
	req, _ := http.NewRequest(http.MethodPost, f.srv.URL+f.hookPath, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		f.t.Fatalf("signed webhook status = %d: %s", res.StatusCode, b)
	}
}

func (f *gitWebhookFixture) pushAndMergeReferencing(key string) {
	f.t.Helper()
	f.postSigned(`{"ref":"refs/heads/` + key + `-hotfix","repository":{"html_url":"https://github.com/carol/cc"},` +
		`"commits":[{"id":"0123456789abcdef0123456789abcdef01234567","message":"` + key + ` fix","author":{"username":"carol"}}]}`)
	f.postSigned(`{"action":"closed","repository":{"html_url":"https://github.com/carol/cc"},` +
		`"pull_request":{"number":7,"title":"` + key + ` done","html_url":"https://github.com/carol/cc/pull/7","state":"closed","merged":true,"head":{"ref":"` + key + `-hotfix"}}}`)
}

type gitLinks struct {
	comments, commits, branches, prs int64
	statusID                         string
}

func (f *gitWebhookFixture) linksOf(issueID string) gitLinks {
	f.t.Helper()
	var l gitLinks
	f.db.Model(&issue.Comment{}).Where("issue_id = ?", issueID).Count(&l.comments)
	f.db.Model(&git.IssueCommit{}).Where("issue_id = ?", issueID).Count(&l.commits)
	f.db.Model(&git.IssueBranch{}).Where("issue_id = ?", issueID).Count(&l.branches)
	f.db.Model(&git.IssuePullRequest{}).Where("issue_id = ?", issueID).Count(&l.prs)
	var iss issue.Issue
	if err := f.db.First(&iss, "id = ?", issueID).Error; err != nil {
		f.t.Fatal(err)
	}
	if iss.StatusID != nil {
		l.statusID = *iss.StatusID
	}
	return l
}

// TestGitWebhook_IgnoresIssuesOfOtherProjects: an inbound git event is
// authenticated by the provider's secret, and the provider belongs to ONE
// project. Keys extracted from commit messages / branch names / PR titles
// must only ever touch issues of that project: otherwise anyone who can
// create a project (any authenticated user) can configure a provider with
// their own secret and, with correctly signed events, add commit comments
// and links to any issue on the instance and — on "PR merged" — move it to a
// status from the attacker's workflow (cross-project status corruption).
// Foreign keys are skipped silently, like unknown ones; the webhook still
// returns 200.
func TestGitWebhook_IgnoresIssuesOfOtherProjects(t *testing.T) {
	f := newGitWebhookFixture(t)

	f.pushAndMergeReferencing(f.aa.Key)

	got := f.linksOf(f.aa.ID)
	if got.comments != 0 || got.commits != 0 || got.branches != 0 || got.prs != 0 {
		t.Errorf("%s touched by CC's provider: comments=%d commits=%d branches=%d prs=%d, want all 0",
			f.aa.Key, got.comments, got.commits, got.branches, got.prs)
	}
	if got.statusID != f.aaStatus {
		t.Errorf("%s status_id = %q, want unchanged %q (CC's provider must not transition it)", f.aa.Key, got.statusID, f.aaStatus)
	}
}

// TestGitWebhook_LinksIssuesOfOwnProject: the positive side of the check —
// the same events referencing the provider's own issue DO link the commit,
// the branch and the PR, add the commit comment, and auto-transition the
// issue to the project's done status on merge.
func TestGitWebhook_LinksIssuesOfOwnProject(t *testing.T) {
	f := newGitWebhookFixture(t)

	f.pushAndMergeReferencing(f.cc.Key)

	got := f.linksOf(f.cc.ID)
	if got.comments != 1 || got.commits != 1 || got.branches != 1 || got.prs != 1 {
		t.Errorf("%s: comments=%d commits=%d branches=%d prs=%d, want all 1",
			f.cc.Key, got.comments, got.commits, got.branches, got.prs)
	}
	if got.statusID != f.ccDone {
		t.Errorf("%s status_id = %q, want done status %q after PR merged", f.cc.Key, got.statusID, f.ccDone)
	}
}
