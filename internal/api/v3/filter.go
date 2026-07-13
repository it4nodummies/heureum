package v3

import (
	"fmt"
	"net/url"
)

// Filter è lo schema del contratto Jira per un filtro salvato.
type Filter struct {
	Self             string `json:"self"`
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Owner            *User  `json:"owner,omitempty"`
	JQL              string `json:"jql,omitempty"`
	ViewURL          string `json:"viewUrl,omitempty"`
	SearchURL        string `json:"searchUrl,omitempty"`
	Favourite        bool   `json:"favourite"`
	FavouritedCount  int64  `json:"favouritedCount"`
	SharePermissions []any  `json:"sharePermissions"`
	EditPermissions  []any  `json:"editPermissions"`
}

// PageBeanFilterDetails è la risposta paginata di /filter/search.
type PageBeanFilterDetails struct {
	Self       string   `json:"self,omitempty"`
	MaxResults int      `json:"maxResults"`
	StartAt    int64    `json:"startAt"`
	Total      int64    `json:"total"`
	IsLast     bool     `json:"isLast"`
	Values     []Filter `json:"values"`
}

// FilterInput porta i dati necessari a costruire un Filter di risposta.
type FilterInput struct {
	ID          string
	Name        string
	Description string
	JQL         string
	Favourite   bool
	Owner       *User
	BaseURL     string
}

// JiraFilter costruisce il Filter di risposta conforme.
func JiraFilter(in FilterInput) Filter {
	return Filter{
		Self:             fmt.Sprintf("%s/rest/api/3/filter/%s", in.BaseURL, in.ID),
		ID:               in.ID,
		Name:             in.Name,
		Description:      in.Description,
		Owner:            in.Owner,
		JQL:              in.JQL,
		ViewURL:          fmt.Sprintf("%s/issues/?filter=%s", in.BaseURL, in.ID),
		SearchURL:        fmt.Sprintf("%s/rest/api/3/search?jql=%s", in.BaseURL, url.QueryEscape(in.JQL)),
		Favourite:        in.Favourite,
		SharePermissions: []any{},
		EditPermissions:  []any{},
	}
}
