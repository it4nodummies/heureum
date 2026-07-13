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
