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
