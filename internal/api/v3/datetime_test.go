package v3

import (
	"testing"
	"time"
)

func TestJiraTime(t *testing.T) {
	loc := time.FixedZone("CET", 2*3600)
	ts := time.Date(2026, 7, 10, 15, 4, 5, 123_000_000, loc)
	got := JiraTime(ts)
	if got != "2026-07-10T15:04:05.123+02:00" {
		t.Errorf("JiraTime = %q", got)
	}
}

func TestJiraTime_Zero(t *testing.T) {
	if JiraTime(time.Time{}) != "" {
		t.Error("zero time must render empty string")
	}
}
