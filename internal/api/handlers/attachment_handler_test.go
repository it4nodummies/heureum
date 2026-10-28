package handlers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/project"
	"github.com/it4nodummies/heureum/internal/domain/user"
	"github.com/it4nodummies/heureum/internal/domain/workflow"
)

func setupAttachmentHandler(t *testing.T) (*AttachmentHandler, *issue.Issue, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&user.User{}, &project.Project{}, &workflow.WorkflowStatus{},
		&issue.Issue{}, &issue.IssueAttachment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	uid := uuid.New().String()
	if err := db.Create(&user.User{ID: uid, Email: "up@example.com", Username: "up", DisplayName: "Up", IsActive: true}).Error; err != nil {
		t.Fatal(err)
	}
	p := &project.Project{ID: uuid.New().String(), Key: "ATT", Name: "Att", SeqID: 10000}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	issueSvc := issue.NewService(db)
	iss, err := issueSvc.Create("ATT", p.ID, "with attachment", "", issue.PriorityMedium, nil, nil)
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	attSvc := issue.NewAttachmentService(db, t.TempDir())
	return NewAttachmentHandler(attSvc, issueSvc, nil), iss, uid
}

// multipartUpload builds a multipart/form-data body with one "file" part of
// size bytes.
func multipartUpload(t *testing.T, size int) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "blob.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(fw, bytes.NewReader(make([]byte, size)), int64(size)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func postAttachment(t *testing.T, h *AttachmentHandler, key, uid string, size int) *httptest.ResponseRecorder {
	t.Helper()
	body, ctype := multipartUpload(t, size)
	req := httptest.NewRequest("POST", "/rest/api/3/issue/"+key+"/attachments", body)
	req.Header.Set("Content-Type", ctype)
	req.SetPathValue("issueIdOrKey", key)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uid))
	rec := httptest.NewRecorder()
	h.Upload(rec, req)
	return rec
}

// TestAttachmentUpload_RejectsOver10MB: /attachment/meta advertises a 10 MB
// uploadLimit; the upload route must actually enforce it (413) instead of
// buffering whatever the client sends.
func TestAttachmentUpload_RejectsOver10MB(t *testing.T) {
	h, iss, uid := setupAttachmentHandler(t)

	rec := postAttachment(t, h, iss.Key, uid, 11<<20)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over-limit upload: status = %d, want 413, body: %s", rec.Code, rec.Body.String())
	}
	var n int64
	h.issueSvc.DB().Model(&issue.IssueAttachment{}).Count(&n)
	if n != 0 {
		t.Errorf("attachment rows = %d, want 0 (over-limit upload must not be stored)", n)
	}
}

// TestAttachmentUpload_AcceptsFileAtAdvertisedLimit: a file of exactly the
// advertised uploadLimit (10 MB) still goes through — the multipart framing
// around it must not push a legitimate upload over the cap.
func TestAttachmentUpload_AcceptsFileAtAdvertisedLimit(t *testing.T) {
	h, iss, uid := setupAttachmentHandler(t)

	const size = maxAttachmentBytes
	rec := postAttachment(t, h, iss.Key, uid, size)
	if rec.Code != http.StatusCreated {
		t.Fatalf("10 MB upload: status = %d, want 201, body: %s", rec.Code, rec.Body.String())
	}
	var att issue.IssueAttachment
	if err := h.issueSvc.DB().First(&att).Error; err != nil {
		t.Fatalf("attachment not stored: %v", err)
	}
	if att.FileSize != size {
		t.Errorf("stored file_size = %d, want %d", att.FileSize, size)
	}
}
