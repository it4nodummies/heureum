package issue

import (
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AttachmentService struct {
	db *gorm.DB
}

func NewAttachmentService(db *gorm.DB) *AttachmentService {
	return &AttachmentService{db: db}
}

func (s *AttachmentService) UploadAttachment(issueID, uploaderID, filename string, file multipart.File) (*IssueAttachment, error) {
	uploadDir := "./data/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, err
	}

	id := uuid.New().String()
	ext := filepath.Ext(filename)
	storedName := id + ext
	filePath := filepath.Join(uploadDir, storedName)

	dst, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		return nil, err
	}

	att := &IssueAttachment{
		ID:         id,
		IssueID:    issueID,
		Filename:   filename,
		FilePath:   filePath,
		FileSize:   written,
		UploaderID: &uploaderID,
	}
	if err := s.db.Create(att).Error; err != nil {
		return nil, err
	}
	return att, nil
}

func (s *AttachmentService) GetAttachments(issueID string) ([]IssueAttachment, error) {
	var atts []IssueAttachment
	err := s.db.Where("issue_id = ?", issueID).Order("created_at DESC").Find(&atts).Error
	return atts, err
}

func (s *AttachmentService) DeleteAttachment(attachmentID string) error {
	var att IssueAttachment
	if err := s.db.Where("id = ?", attachmentID).First(&att).Error; err != nil {
		return err
	}
	os.Remove(att.FilePath)
	return s.db.Delete(&att).Error
}
