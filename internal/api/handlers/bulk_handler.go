package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/it4nodummies/heureum/internal/api/middleware"
	v3 "github.com/it4nodummies/heureum/internal/api/v3"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/permission"
)

// bulkBody è il corpo di POST /rest/api/3/issues/bulk: un insieme di chiavi
// issue e un set parziale di campi da applicare a ciascuna (o delete).
type bulkBody struct {
	Keys   []string `json:"keys"`
	Fields struct {
		Assignee *struct {
			AccountID string `json:"accountId"`
		} `json:"assignee"`
		Priority *struct {
			ID string `json:"id"`
		} `json:"priority"`
		Labels *[]string `json:"labels"`
	} `json:"fields"`
	Delete bool `json:"delete"`
}

// bulkResult è l'esito per-chiave del batch. Error è omesso quando ok è true.
type bulkResult struct {
	Key   string `json:"key"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// BulkUpdate implementa POST /rest/api/3/issues/bulk (estensione Heureum, non
// nello spec Jira). Applica un set parziale di campi — o cancella — una LISTA
// di issue, con AUTORIZZAZIONE PER-ISSUE eseguita IN-HANDLER: per ogni chiave
// si risolve il progetto della issue e si chiama chk.RequireProject(uid,
// projectID, EditIssues). Una chiave sconosciuta o negata diventa un fallimento
// PER-CHIAVE (ok:false + error), NON un 403 sull'intera richiesta. La rotta è
// registrata SENZA il decoratore Enforce (non può filtrare una lista nel body),
// esattamente come POST /rest/api/3/issues/rank. Mirror del pattern
// RequireProject in-handler di IssueLinkHandler.Create/Delete.
//
// NOTA (unassign): issue.Service.Update SA azzerare assignee_id (un accountId
// vuoto viene scritto come NULL), ma è il decode a puntatore di QUESTO handler a
// non poterlo esprimere: un assigneeID nil significa "non toccare" e un JSON
// `assignee:null` è indistinguibile da un campo omesso. Perciò questo endpoint
// supporta solo l'IMPOSTAZIONE di un accountId non vuoto; la rimozione
// dell'assegnatario (unassign) NON è supportata qui.
func (h *IssueHandler) BulkUpdate(w http.ResponseWriter, r *http.Request) {
	var body bulkBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		v3.WriteError(w, http.StatusBadRequest, []string{"Invalid request body."}, nil)
		return
	}
	uid := middleware.UserIDFromContext(r.Context())

	// Precompute i campi comuni a tutte le issue una volta sola.
	var priority *issue.Priority
	if body.Fields.Priority != nil {
		if e := priorityEnumForID(body.Fields.Priority.ID); e != "" {
			p := issue.Priority(e)
			priority = &p
		}
	}
	var assigneeID *string
	if body.Fields.Assignee != nil && body.Fields.Assignee.AccountID != "" {
		accountID := body.Fields.Assignee.AccountID
		assigneeID = &accountID
	}

	results := make([]bulkResult, 0, len(body.Keys))
	for _, key := range body.Keys {
		iss, err := h.resolveIssue(key)
		if err != nil || iss == nil {
			results = append(results, bulkResult{Key: key, OK: false, Error: "not found"})
			continue
		}
		perm := permission.EditIssues
		if body.Delete {
			perm = permission.DeleteIssues
		}
		if err := h.chk.RequireProject(uid, iss.ProjectID, perm); err != nil {
			results = append(results, bulkResult{Key: key, OK: false, Error: "forbidden"})
			continue
		}
		if body.Delete {
			if err := h.svc.Delete(iss.Key); err != nil {
				results = append(results, bulkResult{Key: key, OK: false, Error: err.Error()})
				continue
			}
			results = append(results, bulkResult{Key: iss.Key, OK: true})
			continue
		}
		if body.Fields.Labels != nil {
			if err := h.svc.SetLabels(iss.ID, iss.ProjectID, *body.Fields.Labels); err != nil {
				results = append(results, bulkResult{Key: key, OK: false, Error: err.Error()})
				continue
			}
		}
		if _, err := h.svc.Update(iss.Key, nil, nil, priority, assigneeID, nil, nil); err != nil {
			results = append(results, bulkResult{Key: key, OK: false, Error: err.Error()})
			continue
		}
		results = append(results, bulkResult{Key: iss.Key, OK: true})
	}

	v3.WriteJSON(w, http.StatusOK, map[string]any{"results": results})
}
