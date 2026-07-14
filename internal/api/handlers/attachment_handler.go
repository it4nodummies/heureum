package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	"github.com/it4nodummies/heureum/internal/domain/issue"
)

type AttachmentHandler struct {
	svc      *issue.AttachmentService
	issueSvc *issue.Service
}

func NewAttachmentHandler(svc *issue.AttachmentService, issueSvc *issue.Service) *AttachmentHandler {
	return &AttachmentHandler{svc: svc, issueSvc: issueSvc}
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
	json.NewEncoder(w).Encode(att)
}

func (h *AttachmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	att, err := h.svc.GetAttachment(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"attachment not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(att)
}

func (h *AttachmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteAttachment(r.PathValue("id")); err != nil {
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
