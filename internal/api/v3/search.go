package v3

import (
	"encoding/base64"
	"strconv"
)

// SearchResults è la risposta del legacy POST/GET /rest/api/3/search (offset).
type SearchResults struct {
	Issues          []map[string]any `json:"issues"`
	StartAt         int              `json:"startAt"`
	MaxResults      int              `json:"maxResults"`
	Total           int              `json:"total"`
	WarningMessages []string         `json:"warningMessages,omitempty"`
}

// SearchAndReconcileResults è la risposta di GET/POST /rest/api/3/search/jql
// (token-paginato: nessun total/startAt).
type SearchAndReconcileResults struct {
	Issues        []map[string]any `json:"issues"`
	NextPageToken string           `json:"nextPageToken,omitempty"`
	IsLast        bool             `json:"isLast"`
}

// EncodeCursor codifica un offset in un token opaco base64url.
func EncodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("o:" + strconv.Itoa(offset)))
}

// DecodeCursor decodifica un token prodotto da EncodeCursor. Token vuoto => 0.
func DecodeCursor(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	s := string(b)
	if len(s) < 3 || s[:2] != "o:" {
		return 0, strconv.ErrSyntax
	}
	return strconv.Atoi(s[2:])
}

// FieldReferenceData e FunctionReferenceData: shape del contratto JQLReferenceData.
// Nota: orderable/searchable/isList sono STRINGHE "true"/"false" nello schema Jira.
type FieldReferenceData struct {
	Value       string   `json:"value"`
	DisplayName string   `json:"displayName"`
	Orderable   string   `json:"orderable"`
	Searchable  string   `json:"searchable"`
	Operators   []string `json:"operators"`
	Types       []string `json:"types"`
}

type FunctionReferenceData struct {
	Value       string   `json:"value"`
	DisplayName string   `json:"displayName"`
	IsList      string   `json:"isList"`
	Types       []string `json:"types"`
}

type JQLReferenceData struct {
	VisibleFieldNames    []FieldReferenceData    `json:"visibleFieldNames"`
	VisibleFunctionNames []FunctionReferenceData `json:"visibleFunctionNames"`
	JQLReservedWords     []string                `json:"jqlReservedWords"`
}

// AutocompleteData descrive i campi/funzioni JQL supportati dal nostro engine.
func AutocompleteData() JQLReferenceData {
	field := func(v, dn string, orderable bool, ops []string) FieldReferenceData {
		o := "false"
		if orderable {
			o = "true"
		}
		return FieldReferenceData{Value: v, DisplayName: dn, Orderable: o, Searchable: "true", Operators: ops, Types: []string{"java.lang.String"}}
	}
	eq := []string{"=", "!=", "in", "not in"}
	txt := []string{"~", "!~"}
	return JQLReferenceData{
		VisibleFieldNames: []FieldReferenceData{
			field("project", "Project", false, eq),
			field("status", "Status", true, eq),
			field("assignee", "Assignee", true, append([]string{"is", "is not"}, eq...)),
			field("reporter", "Reporter", false, eq),
			field("priority", "Priority", true, eq),
			field("type", "Type", false, eq),
			field("labels", "Labels", false, eq),
			field("summary", "Summary", true, txt),
			field("text", "Text", false, txt),
			field("resolution", "Resolution", false, []string{"is", "is not"}),
			field("created", "Created", true, []string{"=", "!=", ">", ">=", "<", "<="}),
			field("updated", "Updated", true, []string{"=", "!=", ">", ">=", "<", "<="}),
			field("key", "Key", true, eq),
		},
		VisibleFunctionNames: []FunctionReferenceData{
			{Value: "currentUser()", DisplayName: "currentUser()", IsList: "false", Types: []string{"com.atlassian.jira.user.ApplicationUser"}},
		},
		JQLReservedWords: []string{"and", "or", "not", "in", "is", "empty", "null", "order", "by", "asc", "desc"},
	}
}
