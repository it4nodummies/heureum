package v3

import (
	"encoding/json"
	"fmt"

	"github.com/open-jira/open-jira/internal/adf"
	"github.com/open-jira/open-jira/internal/domain/issue"
	"github.com/open-jira/open-jira/internal/domain/user"
)

type Comment struct {
	Self         string    `json:"self"`
	ID           string    `json:"id"`
	Author       *User     `json:"author,omitempty"`
	Body         *adf.Node `json:"body,omitempty"`
	UpdateAuthor *User     `json:"updateAuthor,omitempty"`
	Created      string    `json:"created"`
	Updated      string    `json:"updated"`
}

type PageOfComments struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Comments   []Comment `json:"comments"`
}

// bodyADF interpreta un body memorizzato (doc ADF, {"content":"..."} legacy, o testo) come ADF.
func bodyADF(bodyJSON string) *adf.Node {
	if bodyJSON == "" || bodyJSON == "{}" {
		return nil
	}
	var node adf.Node
	if err := json.Unmarshal([]byte(bodyJSON), &node); err == nil && node.Type == "doc" {
		return &node
	}
	var legacy struct {
		Content string `json:"content"`
	}
	text := bodyJSON
	if err := json.Unmarshal([]byte(bodyJSON), &legacy); err == nil && legacy.Content != "" {
		text = legacy.Content
	}
	doc := adf.FromText(text)
	return &doc
}

func JiraComment(c issue.Comment, author, updateAuthor *user.User, baseURL string) Comment {
	jc := Comment{
		Self:    fmt.Sprintf("%s/rest/api/3/issue/%s/comment/%s", baseURL, c.IssueID, c.ID),
		ID:      c.ID,
		Body:    bodyADF(c.BodyJSON),
		Created: JiraTime(c.CreatedAt),
		Updated: JiraTime(c.UpdatedAt),
	}
	if author != nil {
		u := JiraUser(*author, baseURL)
		jc.Author = &u
	}
	if updateAuthor != nil {
		u := JiraUser(*updateAuthor, baseURL)
		jc.UpdateAuthor = &u
	}
	return jc
}

// ExtractMentions raccoglie gli accountId dai nodi ADF type:"mention" (attrs.id).
func ExtractMentions(bodyJSON string) []string {
	var node adf.Node
	if err := json.Unmarshal([]byte(bodyJSON), &node); err != nil || node.Type != "doc" {
		return nil
	}
	var out []string
	var walk func(n adf.Node)
	walk = func(n adf.Node) {
		if n.Type == "mention" {
			if id, ok := n.Attrs["id"].(string); ok && id != "" {
				out = append(out, id)
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(node)
	return out
}
