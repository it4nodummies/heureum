package v3

import "time"

// jiraTimeLayout usa l'offset con i due punti (RFC3339, es. +00:00). Jira Cloud
// emette storicamente l'offset senza due punti (+0000), ma quello non è un
// date-time valido per lo schema OpenAPI (che il validatore di contratto applica
// sui campi format:date-time, es. Comment.created); i client ISO8601 accettano
// entrambe le forme, quindi usiamo la forma conforme.
const jiraTimeLayout = "2006-01-02T15:04:05.000-07:00"

// JiraTime formatta un time.Time nel formato datetime di Jira. Zero time → "".
func JiraTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(jiraTimeLayout)
}
