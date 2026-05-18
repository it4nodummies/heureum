package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/open-jira/open-jira/internal/domain/issue"
)

type IssueLinkHandler struct {
	issueSvc *issue.Service
}

func NewIssueLinkHandler(issueSvc *issue.Service) *IssueLinkHandler {
	return &IssueLinkHandler{issueSvc: issueSvc}
}

type jiraLinkRequest struct {
	Type         jiraLinkType  `json:"type"`
	InwardIssue  jiraLinkedIssue `json:"inwardIssue"`
	OutwardIssue jiraLinkedIssue `json:"outwardIssue"`
}

type jiraLinkType struct {
	Name string `json:"name"`
}

type jiraLinkedIssue struct {
	Key string `json:"key"`
}

func (h *IssueLinkHandler) Get(w http.ResponseWriter, r *http.Request) {
	link, err := h.issueSvc.GetLink(r.PathValue("linkId"))
	if err != nil {
		http.Error(w, `{"error":"link not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(link)
}

func (h *IssueLinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req jiraLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Type.Name == "" || req.InwardIssue.Key == "" || req.OutwardIssue.Key == "" {
		http.Error(w, `{"error":"type.name, inwardIssue.key, and outwardIssue.key are required"}`, http.StatusBadRequest)
		return
	}

	sourceIss, err := h.issueSvc.GetByKey(req.OutwardIssue.Key)
	if err != nil {
		http.Error(w, `{"error":"outwardIssue not found"}`, http.StatusNotFound)
		return
	}
	targetIss, err := h.issueSvc.GetByKey(req.InwardIssue.Key)
	if err != nil {
		http.Error(w, `{"error":"inwardIssue not found"}`, http.StatusNotFound)
		return
	}

	linkType := issue.LinkType(req.Type.Name)
	link, err := h.issueSvc.AddLink(sourceIss.ID, targetIss.ID, linkType)
	if err != nil {
		http.Error(w, `{"error":"failed to create link"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(link)
}

func (h *IssueLinkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.issueSvc.DeleteLink(r.PathValue("linkId")); err != nil {
		http.Error(w, `{"error":"link not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
