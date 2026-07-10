package v3

import "time"

const jiraTimeLayout = "2006-01-02T15:04:05.000-0700"

// JiraTime formatta un time.Time nel formato di Jira Cloud. Zero time → "".
func JiraTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(jiraTimeLayout)
}
