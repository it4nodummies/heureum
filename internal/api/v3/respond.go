// Package v3 implementa le convenzioni di risposta della Jira Cloud REST API v3:
// formato errori {errorMessages, errors}, paginazione {startAt, maxResults, total, isLast, values}.
package v3

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

// WriteError scrive un errore nel formato Jira v3. messages e fieldErrors
// possono essere nil: le chiavi vengono comunque serializzate vuote, come fa Jira.
func WriteError(w http.ResponseWriter, status int, messages []string, fieldErrors map[string]string) {
	if messages == nil {
		messages = []string{}
	}
	if fieldErrors == nil {
		fieldErrors = map[string]string{}
	}
	WriteJSON(w, status, errorBody{ErrorMessages: messages, Errors: fieldErrors})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type Page struct {
	StartAt    int `json:"startAt"`
	MaxResults int `json:"maxResults"`
	Total      int `json:"total"`
	Values     any `json:"values"`
}

type pageBody struct {
	Page
	IsLast bool `json:"isLast"`
}

func WritePage(w http.ResponseWriter, status int, p Page) {
	n := 0
	if vs, ok := p.Values.([]string); ok {
		n = len(vs)
	} else if raw, err := json.Marshal(p.Values); err == nil {
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil {
			n = len(arr)
		}
	}
	WriteJSON(w, status, pageBody{Page: p, IsLast: p.StartAt+n >= p.Total})
}
