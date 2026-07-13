package v3

import (
	"fmt"

	"github.com/open-jira/open-jira/internal/domain/user"
)

// Votes è lo schema v3 dei voti di una issue (additionalProperties:false).
type Votes struct {
	Self     string `json:"self"`
	Votes    int    `json:"votes"`
	HasVoted bool   `json:"hasVoted"`
	Voters   []User `json:"voters"`
}

// JiraVotes mappa lo stato dei voti di un issue nella forma Jira v3.
func JiraVotes(issueKey, baseURL string, count int, hasVoted bool, voters []user.User) Votes {
	vs := Votes{
		Self:     fmt.Sprintf("%s/rest/api/3/issue/%s/votes", baseURL, issueKey),
		Votes:    count,
		HasVoted: hasVoted,
		Voters:   make([]User, 0, len(voters)),
	}
	for _, u := range voters {
		vs.Voters = append(vs.Voters, JiraUser(u, baseURL))
	}
	return vs
}

// Watchers è lo schema v3 dei watcher di una issue (additionalProperties:false).
type Watchers struct {
	Self       string `json:"self"`
	IsWatching bool   `json:"isWatching"`
	WatchCount int    `json:"watchCount"`
	Watchers   []User `json:"watchers"`
}

// JiraWatchers mappa lo stato dei watcher di un issue nella forma Jira v3.
func JiraWatchers(issueKey, baseURL string, isWatching bool, watchers []user.User) Watchers {
	ws := Watchers{
		Self:       fmt.Sprintf("%s/rest/api/3/issue/%s/watchers", baseURL, issueKey),
		IsWatching: isWatching,
		WatchCount: len(watchers),
		Watchers:   make([]User, 0, len(watchers)),
	}
	for _, u := range watchers {
		ws.Watchers = append(ws.Watchers, JiraUser(u, baseURL))
	}
	return ws
}

// LinkTypeRef è lo schema v3 IssueLinkType (additionalProperties:false).
type LinkTypeRef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
	Self    string `json:"self,omitempty"`
}

// LinkedIssueRef è lo schema v3 LinkedIssue (additionalProperties:false).
type LinkedIssueRef struct {
	ID     string         `json:"id"`
	Key    string         `json:"key"`
	Self   string         `json:"self"`
	Fields map[string]any `json:"fields,omitempty"`
}

// IssueLinkV3 è lo schema v3 IssueLink (additionalProperties:false).
type IssueLinkV3 struct {
	ID           string          `json:"id"`
	Self         string          `json:"self"`
	Type         LinkTypeRef     `json:"type"`
	InwardIssue  *LinkedIssueRef `json:"inwardIssue,omitempty"`
	OutwardIssue *LinkedIssueRef `json:"outwardIssue,omitempty"`
}

// JiraLinkType mappa il nostro LinkType interno alla forma v3.
func JiraLinkType(internal, baseURL string) LinkTypeRef {
	switch internal {
	case "blocks":
		return LinkTypeRef{ID: "1", Name: "Blocks", Inward: "is blocked by", Outward: "blocks", Self: baseURL + "/rest/api/3/issueLinkType/1"}
	case "is_blocked":
		return LinkTypeRef{ID: "1", Name: "Blocks", Inward: "is blocked by", Outward: "blocks", Self: baseURL + "/rest/api/3/issueLinkType/1"}
	case "duplicates":
		return LinkTypeRef{ID: "2", Name: "Duplicate", Inward: "is duplicated by", Outward: "duplicates", Self: baseURL + "/rest/api/3/issueLinkType/2"}
	default:
		return LinkTypeRef{ID: "3", Name: "Relates", Inward: "relates to", Outward: "relates to", Self: baseURL + "/rest/api/3/issueLinkType/3"}
	}
}

// LinkTypeForName mappa un name Jira ("Blocks","Duplicate","Relates") al LinkType interno.
func LinkTypeForName(name string) string {
	switch name {
	case "Blocks":
		return "blocks"
	case "Duplicate":
		return "duplicates"
	default:
		return "relates"
	}
}

// LinkedIssue costruisce un LinkedIssueRef v3 con summary/status minimali in fields.
func LinkedIssue(id, key, self, summary, status string) LinkedIssueRef {
	return LinkedIssueRef{ID: id, Key: key, Self: self, Fields: map[string]any{"summary": summary, "status": map[string]any{"name": status}}}
}
