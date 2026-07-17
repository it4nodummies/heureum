package authz

import (
	"net/http"
	"strconv"

	"github.com/it4nodummies/heureum/internal/domain/issue"
)

// Resolver estrae il progetto rilevante per la richiesta. ok=false indica che
// la risorsa target non è stata trovata: il decorator Enforce passa allora al
// handler, che risponderà con il proprio 404 naturale (nessun info-leak sulla
// presenza/assenza della risorsa dovuto all'enforcement).
type Resolver func(r *http.Request) (projectID string, ok bool)

// ByKey risolve il progetto dal path param {key} (project key, es. "PROJ").
func (c *Checker) ByKey(r *http.Request) (string, bool) {
	p, err := c.projects.GetByKey(r.PathValue("key"))
	if err != nil {
		return "", false
	}
	return p.ID, true
}

// ByProjectID risolve il progetto dal path param {projectID}, già l'UUID
// interno del progetto (verifica solo che esista).
func (c *Checker) ByProjectID(r *http.Request) (string, bool) {
	id := r.PathValue("projectID")
	if id == "" {
		return "", false
	}
	if _, err := c.projects.GetByID(id); err != nil {
		return "", false
	}
	return id, true
}

// ByIssueParam risolve il progetto a partire da un path param che identifica
// una issue: se numerico prova prima GetBySeqID, altrimenti (o in fallback)
// GetByKey. Usato per {issueKey} e {issueIdOrKey}.
func (c *Checker) ByIssueParam(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		v := r.PathValue(param)
		if v == "" {
			return "", false
		}
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			if iss, err := c.issues.GetBySeqID(n); err == nil {
				return iss.ProjectID, true
			}
		}
		if iss, err := c.issues.GetByKey(v); err == nil {
			return iss.ProjectID, true
		}
		return "", false
	}
}

// ByIssueUUID risolve il progetto dal path param {issueID}, l'UUID interno
// della issue (usato dalle rotte custom-values). Non esiste un GetByID per
// UUID sul servizio issue, quindi si interroga il DB direttamente.
func (c *Checker) ByIssueUUID(r *http.Request) (string, bool) {
	id := r.PathValue("issueID")
	if id == "" {
		return "", false
	}
	var iss issue.Issue
	if err := c.issues.DB().First(&iss, "id = ?", id).Error; err != nil {
		return "", false
	}
	return iss.ProjectID, true
}

// ByBoardSeq risolve il progetto dal path param board (seq_id numerico, es.
// {boardId}) via board.GetBySeqID.
func (c *Checker) ByBoardSeq(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		n, err := strconv.ParseInt(r.PathValue(param), 10, 64)
		if err != nil {
			return "", false
		}
		b, err := c.boards.GetBySeqID(n)
		if err != nil {
			return "", false
		}
		return b.ProjectID, true
	}
}

// BySprintSeq risolve il progetto dal path param sprint (seq_id numerico, es.
// {sprintId}) via sprint.GetBySeqID.
func (c *Checker) BySprintSeq(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		n, err := strconv.ParseInt(r.PathValue(param), 10, 64)
		if err != nil {
			return "", false
		}
		sp, err := c.sprints.GetBySeqID(n)
		if err != nil {
			return "", false
		}
		return sp.ProjectID, true
	}
}

// ByAutomationRule risolve il progetto dal path param regola (UUID, es.
// {ruleID}) via automation.GetRule.
func (c *Checker) ByAutomationRule(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		id := r.PathValue(param)
		if id == "" {
			return "", false
		}
		rule, err := c.autos.GetRule(id)
		if err != nil {
			return "", false
		}
		return rule.ProjectID, true
	}
}

// ByCustomField risolve il progetto dal path param custom field (UUID, es.
// {fieldID}) via customfield.GetField.
func (c *Checker) ByCustomField(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		id := r.PathValue(param)
		if id == "" {
			return "", false
		}
		f, err := c.cfs.GetField(id)
		if err != nil {
			return "", false
		}
		return f.ProjectID, true
	}
}

// ByVersion risolve il progetto dal path param version (UUID, es. {id})
// via version.Get -> ProjectID (mirror di ByCustomField).
func (c *Checker) ByVersion(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		id := r.PathValue(param)
		if id == "" {
			return "", false
		}
		v, err := c.versions.Get(id)
		if err != nil {
			return "", false
		}
		return v.ProjectID, true
	}
}

// ByAttachment risolve il progetto a due hop dal path param allegato (UUID,
// es. {id}): attachment -> issue (via IssueID) -> project. Il servizio
// attachment non è montato sul Checker, quindi si interroga direttamente la
// tabella issue_attachments via issues.DB() (stesso pattern di ByIssueUUID).
func (c *Checker) ByAttachment(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		id := r.PathValue(param)
		if id == "" {
			return "", false
		}
		var att issue.IssueAttachment
		if err := c.issues.DB().First(&att, "id = ?", id).Error; err != nil {
			return "", false
		}
		var iss issue.Issue
		if err := c.issues.DB().First(&iss, "id = ?", att.IssueID).Error; err != nil {
			return "", false
		}
		return iss.ProjectID, true
	}
}

// ByIssueLink risolve il progetto a due hop dal path param link (UUID, es.
// {linkId}): link -> issue sorgente (SourceID) -> project. Stesso pattern di
// issuelink_handler.go (Get/issueByID): il permesso si valuta sul progetto
// della issue sorgente del link.
func (c *Checker) ByIssueLink(param string) Resolver {
	return func(r *http.Request) (string, bool) {
		id := r.PathValue(param)
		if id == "" {
			return "", false
		}
		link, err := c.issues.GetLink(id)
		if err != nil {
			return "", false
		}
		var iss issue.Issue
		if err := c.issues.DB().First(&iss, "id = ?", link.SourceID).Error; err != nil {
			return "", false
		}
		return iss.ProjectID, true
	}
}
