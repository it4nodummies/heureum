package v3

import (
	"mime"
	"path/filepath"
	"time"
)

// Attachment è la shape v3 di un allegato di issue (Jira Cloud "Attachment"
// schema, sottoinsieme rilevante: self/thumbnail/author non ancora esposti —
// vedi nota in AttachmentFrom).
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Created  string `json:"created"`
	Content  string `json:"content"`
}

// AttachmentFrom costruisce la shape v3 a partire dai campi grezzi del model
// issue.IssueAttachment (che non ha un campo mimeType): il mime type viene
// inferito dall'estensione del filename via mime.TypeByExtension, con
// fallback a "application/octet-stream" quando l'estensione non è nota.
// author è omesso: risolvere l'uploader (issue.IssueAttachment.UploaderID)
// a un v3.User qui richiederebbe iniettare uno UserService in questa
// funzione pura di shaping; lasciato fuori per questo task (vedi plan).
func AttachmentFrom(id, filename string, size int64, created time.Time) Attachment {
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return Attachment{
		ID:       id,
		Filename: filename,
		Size:     size,
		MimeType: mimeType,
		Created:  JiraTime(created),
		Content:  "/rest/api/3/attachment/content/" + id,
	}
}
