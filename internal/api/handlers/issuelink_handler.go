package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
)

type IssueLinkHandler struct {
	issueSvc *issue.Service
	baseURL  string
}

func NewIssueLinkHandler(issueSvc *issue.Service, baseURL string) *IssueLinkHandler {
	return &IssueLinkHandler{issueSvc: issueSvc, baseURL: baseURL}
}

type jiraLinkRequest struct {
	Type         jiraLinkType    `json:"type"`
	InwardIssue  jiraLinkedIssue `json:"inwardIssue"`
	OutwardIssue jiraLinkedIssue `json:"outwardIssue"`
}

type jiraLinkType struct {
	Name string `json:"name"`
}

type jiraLinkedIssue struct {
	Key string `json:"key"`
}

// resolveIssue trova una issue per key o id (Jira accetta entrambi in LinkedIssue).
func (h *IssueLinkHandler) resolveIssue(ref jiraLinkedIssue) (*issue.Issue, error) {
	if ref.Key == "" {
		return nil, fmt.Errorf("issue key is required")
	}
	return h.issueSvc.GetByKey(ref.Key)
}

// toIssueLinkV3 costruisce la forma v3 di un IssueLink a partire dal record di
// dominio e dalle issue collegate (già risolte).
func (h *IssueLinkHandler) toIssueLinkV3(link *issue.IssueLink, outward, inward *issue.Issue) v3.IssueLinkV3 {
	self := fmt.Sprintf("%s/rest/api/3/issueLink/%s", h.baseURL, link.ID)
	out := v3.LinkedIssue(outward.ID, outward.Key, fmt.Sprintf("%s/rest/api/3/issue/%s", h.baseURL, outward.Key), outward.Title, "")
	in := v3.LinkedIssue(inward.ID, inward.Key, fmt.Sprintf("%s/rest/api/3/issue/%s", h.baseURL, inward.Key), inward.Title, "")
	return v3.IssueLinkV3{
		ID:           link.ID,
		Self:         self,
		Type:         v3.JiraLinkType(string(link.LinkType), h.baseURL),
		InwardIssue:  &in,
		OutwardIssue: &out,
	}
}

func (h *IssueLinkHandler) Get(w http.ResponseWriter, r *http.Request) {
	link, err := h.issueSvc.GetLink(r.PathValue("linkId"))
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"link not found"}, nil)
		return
	}

	outwardIss, oerr := h.issueByID(link.SourceID)
	inwardIss, ierr := h.issueByID(link.TargetID)
	if oerr != nil || ierr != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"linked issue not found"}, nil)
		return
	}

	v3.WriteJSON(w, http.StatusOK, h.toIssueLinkV3(link, outwardIss, inwardIss))
}

// issueByID recupera una issue tramite il suo ID interno (non la Key). Il
// service issue non espone un getter dedicato per ID, quindi usiamo la DB
// esposta da Service.DB(), come già fa IssueHandler per la status lookup.
func (h *IssueLinkHandler) issueByID(id string) (*issue.Issue, error) {
	var iss issue.Issue
	if err := h.issueSvc.DB().First(&iss, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &iss, nil
}

func (h *IssueLinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req jiraLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if req.Type.Name == "" || req.InwardIssue.Key == "" || req.OutwardIssue.Key == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"type.name, inwardIssue.key, and outwardIssue.key are required"}, nil)
		return
	}

	outwardIss, err := h.resolveIssue(req.OutwardIssue)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"outwardIssue not found"}, nil)
		return
	}
	inwardIss, err := h.resolveIssue(req.InwardIssue)
	if err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"inwardIssue not found"}, nil)
		return
	}

	internalType := issue.LinkType(v3.LinkTypeForName(req.Type.Name))
	if _, err := h.issueSvc.AddLink(outwardIss.ID, inwardIss.ID, internalType); err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to create link"}, nil)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *IssueLinkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.issueSvc.DeleteLink(r.PathValue("linkId")); err != nil {
		v3.WriteError(w, http.StatusNotFound, []string{"link not found"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
