package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/it4nodummies/heureum/internal/api/authz"
	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
)

type AttachmentHandler struct {
	svc      *issue.AttachmentService
	issueSvc *issue.Service
	chk      *authz.Checker
}

func NewAttachmentHandler(svc *issue.AttachmentService, issueSvc *issue.Service, chk *authz.Checker) *AttachmentHandler {
	return &AttachmentHandler{svc: svc, issueSvc: issueSvc, chk: chk}
}

func (h *AttachmentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	iss, err := h.issueSvc.GetByKey(r.PathValue("issueIdOrKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, `{"error":"failed to parse form"}`, http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"file is required"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	uploaderID := middleware.UserIDFromContext(r.Context())
	att, err := h.svc.UploadAttachment(iss.ID, uploaderID, header.Filename, file)
	if err != nil {
		http.Error(w, `{"error":"failed to upload attachment"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v3.AttachmentFrom(att.ID, att.Filename, att.FileSize, att.CreatedAt))
}

func (h *AttachmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	att, err := h.svc.GetAttachment(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"attachment not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v3.AttachmentFrom(att.ID, att.Filename, att.FileSize, att.CreatedAt))
}

// ListForIssue restituisce gli allegati di una issue in shape v3
// (GET /rest/api/3/issue/{issueIdOrKey}/attachments). Non è una rotta del
// contratto ufficiale Jira (che espone gli allegati solo dentro
// fields.attachment dell'issue), ma è comoda per il frontend: vedi Round 13
// Task 5.
func (h *AttachmentHandler) ListForIssue(w http.ResponseWriter, r *http.Request) {
	iss, err := h.issueSvc.GetByKey(r.PathValue("issueIdOrKey"))
	if err != nil {
		http.Error(w, `{"error":"issue not found"}`, http.StatusNotFound)
		return
	}
	atts, err := h.svc.GetAttachments(iss.ID)
	if err != nil {
		http.Error(w, `{"error":"failed to list attachments"}`, http.StatusInternalServerError)
		return
	}
	out := make([]v3.Attachment, 0, len(atts))
	for _, att := range atts {
		out = append(out, v3.AttachmentFrom(att.ID, att.Filename, att.FileSize, att.CreatedAt))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// Delete: two-hop authorization (attachment -> issue -> project), enforced
// in-handler because DELETE /attachment/{id} has no single path-resolvable
// project (left unwrapped by the router decorator, per the Round 11 plan).
func (h *AttachmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	att, err := h.svc.GetAttachment(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"attachment not found"}`, http.StatusNotFound)
		return
	}
	var iss issue.Issue
	if err := h.issueSvc.DB().First(&iss, "id = ?", att.IssueID).Error; err != nil {
		http.Error(w, `{"error":"attachment not found"}`, http.StatusNotFound)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())
	if err := h.chk.RequireProject(uid, iss.ProjectID, permission.EditIssues); err != nil {
		authz.WriteForbidden(w)
		return
	}
	if err := h.svc.DeleteAttachment(att.ID); err != nil {
		http.Error(w, `{"error":"attachment not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AttachmentHandler) ServeFile(w http.ResponseWriter, r *http.Request) {
	att, err := h.svc.GetAttachment(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"attachment not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", "inline; filename=\""+filepath.Base(att.Filename)+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, att.FilePath)
}

func (h *AttachmentHandler) Meta(w http.ResponseWriter, r *http.Request) {
	meta := map[string]interface{}{
		"enabled":         true,
		"uploadLimit":     10485760,
		"allowedFormats":  []string{"*"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}
