package v3

import (
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/rest/api/3/project/search", nil)
	startAt, maxResults := ParsePagination(r, 50, 100)
	if startAt != 0 || maxResults != 50 {
		t.Errorf("got %d,%d want 0,50", startAt, maxResults)
	}
}

func TestParsePagination_CapAndNegatives(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?startAt=-5&maxResults=9999", nil)
	startAt, maxResults := ParsePagination(r, 50, 100)
	if startAt != 0 || maxResults != 100 {
		t.Errorf("got %d,%d want 0,100", startAt, maxResults)
	}
}

func TestParseExpand(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?expand=description,lead,issueTypes", nil)
	e := ParseExpand(r)
	if !e.Has("lead") || e.Has("url") {
		t.Errorf("unexpected expand: %v", e)
	}
	// Il valore expand va rieccheggiato nella risposta come stringa.
	if e.String() != "description,lead,issueTypes" {
		t.Errorf("String() = %q", e.String())
	}
}

func TestParseFields(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?fields=summary,status,-comment", nil)
	f := ParseFields(r)
	if !f.Include("summary") || !f.Include("status") {
		t.Error("summary/status should be included")
	}
	if f.Include("comment") {
		t.Error("-comment should be excluded")
	}
	r2 := httptest.NewRequest("GET", "/x", nil)
	if !ParseFields(r2).Include("anything") {
		t.Error("no fields param means all fields (Jira default *navigable)")
	}
}
