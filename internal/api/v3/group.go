package v3

import "fmt"

// GroupRef è lo shape conforme di un gruppo Jira.
type GroupRef struct {
	Name    string `json:"name"`
	GroupID string `json:"groupId"`
	Self    string `json:"self"`
}

func JiraGroup(id, name, baseURL string) GroupRef {
	return GroupRef{Name: name, GroupID: id, Self: fmt.Sprintf("%s/rest/api/3/group?groupId=%s", baseURL, id)}
}

// FoundGroup / FoundGroups per /groups/picker.
type FoundGroup struct {
	Name    string `json:"name"`
	GroupID string `json:"groupId"`
}
type FoundGroups struct {
	Header string       `json:"header"`
	Total  int          `json:"total"`
	Groups []FoundGroup `json:"groups"`
}
