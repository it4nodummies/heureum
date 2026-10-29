package issue

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/it4nodummies/heureum/internal/storetest"
	"gorm.io/gorm"
)

func newAttachmentTestDB(t *testing.T) *gorm.DB {
	return storetest.New(t)
}

// fakeMultipartFile adapts an io.Reader to multipart.File for tests that
// don't need Seek/ReadAt.
type fakeMultipartFile struct {
	*bytes.Reader
}

func (fakeMultipartFile) Close() error { return nil }

func TestUploadAttachment_WritesIntoConfiguredDir(t *testing.T) {
	db := newAttachmentTestDB(t)
	// issue_attachments.issue_id and .uploader_id are real FKs to issues(id)
	// and users(id): both rows must exist before UploadAttachment can insert.
	createTestIssue(t, db, "issue-1", "p1")
	createTestUser(t, db, "user-1")
	dir := t.TempDir()
	svc := NewAttachmentService(db, dir)

	content := []byte("hello attachment")
	att, err := svc.UploadAttachment("issue-1", "user-1", "notes.txt", fakeMultipartFile{bytes.NewReader(content)})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(filepath.Clean(att.FilePath), filepath.Clean(dir)) {
		t.Errorf("FilePath = %q, want it inside configured dir %q", att.FilePath, dir)
	}
	if _, err := os.Stat(att.FilePath); err != nil {
		t.Errorf("expected file to exist at %q: %v", att.FilePath, err)
	}
	got, err := os.ReadFile(att.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("file content = %q, want %q", got, content)
	}
}

func TestNewAttachmentService_EmptyDirFallsBackToDefault(t *testing.T) {
	db := newAttachmentTestDB(t)
	svc := NewAttachmentService(db, "")

	if svc.uploadDir != defaultUploadDir {
		t.Errorf("uploadDir = %q, want default %q", svc.uploadDir, defaultUploadDir)
	}
}
