package git

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newConfigDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&IssueCommit{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestLinkCommitDedupesRepeatedSHA(t *testing.T) {
	svc := NewConfigService(newConfigDB(t))

	created, err := svc.LinkCommit("issue-1", "prov-1", "sha1", "PROJ-1: first", "dev")
	if err != nil {
		t.Fatalf("first LinkCommit: %v", err)
	}
	if !created {
		t.Fatalf("first LinkCommit: expected created=true, got false")
	}

	created, err = svc.LinkCommit("issue-1", "prov-1", "sha1", "PROJ-1: first", "dev")
	if err != nil {
		t.Fatalf("second LinkCommit: %v", err)
	}
	if created {
		t.Fatalf("second LinkCommit: expected created=false, got true")
	}

	var count int64
	svc.DB().Model(&IssueCommit{}).Where("issue_id = ? AND commit_sha = ?", "issue-1", "sha1").Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 issue_commits row, got %d", count)
	}
}
