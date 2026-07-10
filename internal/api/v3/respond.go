// Package v3 implementa le convenzioni di risposta della Jira Cloud REST API v3:
// formato errori {errorMessages, errors}, paginazione {startAt, maxResults, total, isLast, values}.
package v3

import (
	"encoding/json"
	"log"
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
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("v3: encode response: %v", err)
	}
}

type Page[T any] struct {
	StartAt    int `json:"startAt"`
	MaxResults int `json:"maxResults"`
	Total      int `json:"total"`
	Values     []T `json:"values"`
}

type pageBody[T any] struct {
	Page[T]
	IsLast bool `json:"isLast"`
}

func WritePage[T any](w http.ResponseWriter, status int, p Page[T]) {
	if p.Values == nil {
		p.Values = []T{}
	}
	WriteJSON(w, status, pageBody[T]{Page: p, IsLast: p.StartAt+len(p.Values) >= p.Total})
}
