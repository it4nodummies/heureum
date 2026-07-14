package handlers

import (
	v3 "github.com/open-jira/open-jira/internal/api/v3"
	"github.com/open-jira/open-jira/internal/domain/issue"
)

// renderIssueList costruisce gli IssueBean (con proiezione dei fields) per una
// lista di issue di dominio, riusando IssueHandler.buildIssueInput. Condiviso tra
// la ricerca (Round 4) e gli endpoint agile (Round 5).
func renderIssueList(issueH *IssueHandler, issues []issue.Issue, fields []string) ([]map[string]any, error) {
	// fields == nil significa "nessuna proiezione richiesta esplicitamente":
	// per gli endpoint agile (board/backlog/sprint) vogliamo comunque tutti i
	// campi (a differenza di /search, dove il chiamante passa già un default
	// esplicito). ParseFieldsFromList(nil) altrimenti risolverebbe a "nessun
	// campo" (limited=true, include vuoto), lasciando "fields": {} per ogni
	// issue — bug osservato nel Round 5 (board senza card, backlog senza
	// summary/status).
	if fields == nil {
		fields = []string{"*all"}
	}
	f := v3.ParseFieldsFromList(fields)
	out := make([]map[string]any, 0, len(issues))
	for i := range issues {
		bean := v3.JiraIssue(issueH.buildIssueInput(&issues[i]))
		m, err := v3.ProjectIssue(bean, f)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
