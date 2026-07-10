package v3

import "fmt"

// PriorityRef è la rappresentazione v3 di una priorità (schema Priority).
type PriorityRef struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	StatusColor string `json:"statusColor,omitempty"`
}

var priorityOrder = []struct{ id, name, color string }{
	{"1", "Highest", "#CD1317"},
	{"2", "High", "#E9494A"},
	{"3", "Medium", "#E97F33"},
	{"4", "Low", "#2A8735"},
	{"5", "Lowest", "#57A55A"},
}

// enumToID mappa il nostro enum interno all'id priorità Jira standard.
var enumToID = map[string]string{"highest": "1", "high": "2", "medium": "3", "low": "4", "lowest": "5"}

// StandardPriorities restituisce le 5 priorità standard di Jira.
func StandardPriorities(baseURL string) []PriorityRef {
	out := make([]PriorityRef, 0, len(priorityOrder))
	for _, p := range priorityOrder {
		out = append(out, PriorityRef{
			Self:        fmt.Sprintf("%s/rest/api/3/priority/%s", baseURL, p.id),
			ID:          p.id,
			Name:        p.name,
			StatusColor: p.color,
			IconURL:     fmt.Sprintf("%s/static/priority-%s.svg", baseURL, p.id),
		})
	}
	return out
}

// PriorityForEnum mappa l'enum interno (highest..lowest) alla priorità Jira; ignoto → Medium.
func PriorityForEnum(enum, baseURL string) PriorityRef {
	id := enumToID[enum]
	if id == "" {
		id = "3"
	}
	for _, p := range StandardPriorities(baseURL) {
		if p.ID == id {
			return p
		}
	}
	return StandardPriorities(baseURL)[2]
}

// StatusCategoryRef modella lo statusCategory v3.
type StatusCategoryRef struct {
	Self      string `json:"self"`
	ID        int    `json:"id"`
	Key       string `json:"key"`
	ColorName string `json:"colorName"`
	Name      string `json:"name"`
}

// StatusRef modella uno status v3 con la sua categoria.
type StatusRef struct {
	Self           string            `json:"self"`
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	IconURL        string            `json:"iconUrl,omitempty"`
	StatusCategory StatusCategoryRef `json:"statusCategory"`
}

// categoryFor mappa la categoria interna (todo|inprogress|done) allo statusCategory Jira.
func categoryFor(internal, baseURL string) StatusCategoryRef {
	switch internal {
	case "done":
		return StatusCategoryRef{Self: baseURL + "/rest/api/3/statuscategory/3", ID: 3, Key: "done", ColorName: "green", Name: "Done"}
	case "inprogress":
		return StatusCategoryRef{Self: baseURL + "/rest/api/3/statuscategory/4", ID: 4, Key: "indeterminate", ColorName: "yellow", Name: "In Progress"}
	default:
		return StatusCategoryRef{Self: baseURL + "/rest/api/3/statuscategory/2", ID: 2, Key: "new", ColorName: "blue-gray", Name: "To Do"}
	}
}

// JiraStatus costruisce uno StatusRef v3.
func JiraStatus(id, name, internalCategory, baseURL string) StatusRef {
	return StatusRef{
		Self:           fmt.Sprintf("%s/rest/api/3/status/%s", baseURL, id),
		ID:             id,
		Name:           name,
		StatusCategory: categoryFor(internalCategory, baseURL),
	}
}

// IssueTypeRef modella un issue type v3 (schema IssueTypeDetails, campi essenziali).
type IssueTypeRef struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	Subtask     bool   `json:"subtask"`
}

// JiraIssueType costruisce un IssueTypeRef v3.
func JiraIssueType(id, name, iconURL string, subtask bool, baseURL string) IssueTypeRef {
	return IssueTypeRef{
		Self:    fmt.Sprintf("%s/rest/api/3/issuetype/%s", baseURL, id),
		ID:      id,
		Name:    name,
		IconURL: iconURL,
		Subtask: subtask,
	}
}

// ResolutionRef modella una resolution v3.
type ResolutionRef struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// JiraResolution costruisce una ResolutionRef v3.
func JiraResolution(id, name, desc, baseURL string) ResolutionRef {
	return ResolutionRef{
		Self:        fmt.Sprintf("%s/rest/api/3/resolution/%s", baseURL, id),
		ID:          id,
		Name:        name,
		Description: desc,
	}
}
