package v3

import (
	"fmt"

	"github.com/open-jira/open-jira/internal/adf"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

// Worklog è la rappresentazione Jira v3 di un worklog (schema "Worklog" nel contratto).
type Worklog struct {
	Self             string    `json:"self"`
	ID               string    `json:"id"`
	IssueID          string    `json:"issueId"`
	Author           *User     `json:"author,omitempty"`
	UpdateAuthor     *User     `json:"updateAuthor,omitempty"`
	Comment          *adf.Node `json:"comment,omitempty"`
	Created          string    `json:"created"`
	Updated          string    `json:"updated"`
	Started          string    `json:"started"`
	TimeSpent        string    `json:"timeSpent"`
	TimeSpentSeconds int       `json:"timeSpentSeconds"`
}

// PageOfWorklogs è la forma paginata restituita da GET .../worklog.
type PageOfWorklogs struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Worklogs   []Worklog `json:"worklogs"`
}

// formatSeconds converte i secondi in una stringa timeSpent in stile Jira
// (es. "1h 30m"). Zero o negativo → "0m".
func formatSeconds(sec int) string {
	if sec <= 0 {
		return "0m"
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

// JiraWorklog mappa il modello interno issue.Worklog nella forma Jira v3.
// bodyADF (definita in comment.go) interpreta CommentJSON come nodo ADF.
func JiraWorklog(wl issue.Worklog, author *user.User, baseURL string) Worklog {
	jw := Worklog{
		Self:             fmt.Sprintf("%s/rest/api/3/issue/%s/worklog/%s", baseURL, wl.IssueID, wl.ID),
		ID:               wl.ID,
		IssueID:          wl.IssueID,
		Comment:          bodyADF(wl.CommentJSON),
		Created:          JiraTime(wl.CreatedAt),
		Updated:          JiraTime(wl.UpdatedAt),
		TimeSpent:        formatSeconds(wl.TimeSpentSeconds),
		TimeSpentSeconds: wl.TimeSpentSeconds,
	}
	if wl.Started != nil {
		jw.Started = JiraTime(*wl.Started)
	}
	if author != nil {
		u := JiraUser(*author, baseURL)
		jw.Author = &u
		jw.UpdateAuthor = &u
	}
	return jw
}
