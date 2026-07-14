package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
)

// RemoteLinkHandler serve gli endpoint Jira v3 dei remote issue link su un issue.
type RemoteLinkHandler struct {
	svc      *issue.RemoteLinkService
	issueSvc *issue.Service
	baseURL  string
}

func NewRemoteLinkHandler(svc *issue.RemoteLinkService, issueSvc *issue.Service, baseURL string) *RemoteLinkHandler {
	return &RemoteLinkHandler{svc: svc, issueSvc: issueSvc, baseURL: baseURL}
}

// resolve trova l'issue dal path param issueIdOrKey, provando prima come ID
// numerico sequenziale e poi come key (es. "DEMO-1").
func (h *RemoteLinkHandler) resolve(r *http.Request) *issue.Issue {
	k := r.PathValue("issueIdOrKey")
	if n, err := strconv.ParseInt(k, 10, 64); err == nil {
		if iss, err := h.issueSvc.GetBySeqID(n); err == nil {
			return iss
		}
		return nil
	}
	iss, err := h.issueSvc.GetByKey(k)
	if err != nil {
		return nil
	}
	return iss
}

// List restituisce i remote link dell'issue come array di RemoteLink v3.
func (h *RemoteLinkHandler) List(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	links, err := h.svc.ListByIssue(iss.ID)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to get remote links"}, nil)
		return
	}
	out := make([]v3.RemoteLink, 0, len(links))
	for _, rl := range links {
		out = append(out, v3.JiraRemoteLink(rl, h.baseURL))
	}
	v3.WriteJSON(w, http.StatusOK, out)
}

// createRemoteLinkRequest è il body accettato da POST .../remotelink, forma
// RemoteIssueLinkRequest del contratto Jira v3 (object.url/title/summary
// required, globalId e relationship opzionali).
type createRemoteLinkRequest struct {
	GlobalID     string `json:"globalId"`
	Relationship string `json:"relationship"`
	Object       struct {
		URL     string `json:"url"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
	} `json:"object"`
}

// Create crea un nuovo remote link sull'issue risolto dal path.
func (h *RemoteLinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	var req createRemoteLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"invalid request body"}, nil)
		return
	}
	if req.Object.URL == "" || req.Object.Title == "" {
		v3.WriteError(w, http.StatusBadRequest, []string{"object.url and object.title are required"}, nil)
		return
	}
	rl, err := h.svc.Add(iss.ID, req.GlobalID, req.Object.URL, req.Object.Title, req.Object.Summary, req.Relationship)
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to add remote link"}, nil)
		return
	}
	mapped := v3.JiraRemoteLink(*rl, h.baseURL)
	// RemoteIssueLinkIdentifies (additionalProperties:false, props id/self,
	// nessuno required): rispondiamo solo con "self" — coerente con la scelta
	// di omettere "id" (int, opzionale) per non forzare una conversione
	// UUID -> intero sul nostro identificatore interno.
	v3.WriteJSON(w, http.StatusCreated, struct {
		Self string `json:"self"`
	}{Self: mapped.Self})
}

// Delete rimuove il remote link identificato dal path param "id", ma solo se
// appartiene all'issue risolto dal path: un link di PROJ-1 non può essere
// eliminato tramite una richiesta scoping su PROJ-2. 404 se l'issue non
// esiste o se il link non appartiene ad esso.
func (h *RemoteLinkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	iss := h.resolve(r)
	if iss == nil {
		v3.WriteError(w, http.StatusNotFound, []string{"issue not found"}, nil)
		return
	}
	n, err := h.svc.Delete(iss.ID, r.PathValue("id"))
	if err != nil {
		v3.WriteError(w, http.StatusInternalServerError, []string{"failed to delete remote link"}, nil)
		return
	}
	if n == 0 {
		v3.WriteError(w, http.StatusNotFound, []string{"remote link not found"}, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
