package v3

import (
	"encoding/json"
	"fmt"

	"github.com/it4nodummies/heureum/internal/adf"
	"github.com/it4nodummies/heureum/internal/domain/issue"
	"github.com/it4nodummies/heureum/internal/domain/user"
)

// ProjectRef è la rappresentazione v3 minimale di un progetto, referenziata
// da IssueFields.Project.
type ProjectRef struct {
	Self string `json:"self"`
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// ParentRef è la rappresentazione v3 minimale dell'issue padre.
type ParentRef struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// TimeTracking modella il campo timetracking di IssueFields.
type TimeTracking struct {
	OriginalEstimateSeconds  int `json:"originalEstimateSeconds,omitempty"`
	TimeSpentSeconds         int `json:"timeSpentSeconds,omitempty"`
	RemainingEstimateSeconds int `json:"remainingEstimateSeconds,omitempty"`
}

// IssueFields è il contenuto free-form di IssueBean.fields (il contratto non
// valida stray keys qui). Nomi di campo Jira ufficiali.
type IssueFields struct {
	Summary      string         `json:"summary"`
	Description  *adf.Node      `json:"description,omitempty"`
	IssueType    *IssueTypeRef  `json:"issuetype,omitempty"`
	Status       *StatusRef     `json:"status,omitempty"`
	Priority     *PriorityRef   `json:"priority,omitempty"`
	Assignee     *User          `json:"assignee,omitempty"`
	Reporter     *User          `json:"reporter,omitempty"`
	Resolution   *ResolutionRef `json:"resolution,omitempty"`
	Project      *ProjectRef    `json:"project,omitempty"`
	Parent       *ParentRef     `json:"parent,omitempty"`
	Labels       []string       `json:"labels"`
	Created      string         `json:"created"`
	Updated      string         `json:"updated"`
	DueDate      string         `json:"duedate,omitempty"`
	StoryPoints  *int           `json:"customfield_10016,omitempty"`
	TimeTracking *TimeTracking  `json:"timetracking,omitempty"`
}

// IssueBean è la forma Jira v3 ufficiale di un'issue (schema IssueBean).
type IssueBean struct {
	Self   string      `json:"self"`
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

// IssueInput raccoglie l'issue di dominio e le entità collegate necessarie a
// costruire un IssueBean v3 completo.
type IssueInput struct {
	Issue      issue.Issue
	BaseURL    string
	Assignee   *user.User
	Reporter   *user.User
	IssueType  *issue.IssueType
	Status     *StatusRef
	Resolution *ResolutionRef
	Project    *ProjectRef
	Parent     *ParentRef
	Labels     []string
}

// descriptionADF converte il DescriptionJSON grezzo dell'issue in un nodo
// ADF: se è già un documento ADF valido lo usa tal quale, altrimenti tratta
// il contenuto come testo semplice (supportando anche il legacy
// {"content":"..."}). Stringhe vuote o "{}" restituiscono nil.
func descriptionADF(descJSON string) *adf.Node {
	if descJSON == "" || descJSON == "{}" {
		return nil
	}
	var node adf.Node
	if err := json.Unmarshal([]byte(descJSON), &node); err == nil && node.Type == "doc" {
		return &node
	}
	var legacy struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(descJSON), &legacy); err == nil {
		// Il payload legacy {"content": "..."} è valido: una content vuota
		// significa "nessuna descrizione", non testo letterale da mostrare.
		if legacy.Content == "" {
			return nil
		}
		doc := adf.FromText(legacy.Content)
		return &doc
	}
	doc := adf.FromText(descJSON)
	return &doc
}

// JiraIssue costruisce l'IssueBean v3 ufficiale per un'issue di dominio,
// applicando i default Jira (issuetype "Task", status "To Do") quando le
// entità collegate non sono fornite in IssueInput.
func JiraIssue(in IssueInput) IssueBean {
	iss := in.Issue
	fields := IssueFields{
		Summary:     iss.Title,
		Description: descriptionADF(iss.DescriptionJSON),
		Priority:    issuePtr(PriorityForEnum(string(iss.Priority), in.BaseURL)),
		Labels:      in.Labels,
		Created:     JiraTime(iss.CreatedAt),
		Updated:     JiraTime(iss.UpdatedAt),
	}
	if fields.Labels == nil {
		fields.Labels = []string{}
	}

	if in.IssueType != nil {
		it := JiraIssueType(in.IssueType.ID, in.IssueType.Name, in.BaseURL+"/static/issuetype-"+in.IssueType.Icon+".svg", in.IssueType.IsSubtask, in.BaseURL)
		fields.IssueType = &it
	} else {
		it := JiraIssueType("0", "Task", in.BaseURL+"/static/issuetype-task.svg", false, in.BaseURL)
		fields.IssueType = &it
	}

	if in.Status != nil {
		fields.Status = in.Status
	} else {
		s := JiraStatus("0", "To Do", "todo", in.BaseURL)
		fields.Status = &s
	}

	if in.Assignee != nil {
		u := JiraUser(*in.Assignee, in.BaseURL)
		fields.Assignee = &u
	}
	if in.Reporter != nil {
		u := JiraUser(*in.Reporter, in.BaseURL)
		fields.Reporter = &u
	}

	fields.Resolution = in.Resolution
	fields.Project = in.Project
	fields.Parent = in.Parent

	if iss.DueDate != nil {
		fields.DueDate = iss.DueDate.Format("2006-01-02")
	}
	if iss.StoryPoints > 0 {
		fields.StoryPoints = issuePtr(iss.StoryPoints)
	}
	if iss.OriginalEstimate > 0 || iss.TimeSpent > 0 || iss.RemainingEstimate > 0 {
		fields.TimeTracking = &TimeTracking{
			OriginalEstimateSeconds:  iss.OriginalEstimate,
			TimeSpentSeconds:         iss.TimeSpent,
			RemainingEstimateSeconds: iss.RemainingEstimate,
		}
	}

	return IssueBean{
		Self:   fmt.Sprintf("%s/rest/api/3/issue/%d", in.BaseURL, iss.SeqID),
		ID:     fmt.Sprintf("%d", iss.SeqID),
		Key:    iss.Key,
		Fields: fields,
	}
}

// issuePtr restituisce un puntatore al valore passato; helper locale per
// evitare variabili temporanee quando si popolano campi opzionali *T.
func issuePtr[T any](v T) *T { return &v }
