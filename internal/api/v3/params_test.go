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
	t.Run("negative startAt clamped to zero", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/x?startAt=-5&maxResults=50", nil)
		startAt, maxResults := ParsePagination(r, 50, 100)
		if startAt != 0 || maxResults != 50 {
			t.Errorf("got %d,%d want 0,50", startAt, maxResults)
		}
	})
	t.Run("maxResults capped at capMax", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/x?startAt=0&maxResults=9999", nil)
		startAt, maxResults := ParsePagination(r, 50, 100)
		if startAt != 0 || maxResults != 100 {
			t.Errorf("got %d,%d want 0,100", startAt, maxResults)
		}
	})
}

func TestParsePagination_CapBoundary(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?maxResults=100", nil)
	_, maxResults := ParsePagination(r, 50, 100)
	if maxResults != 100 {
		t.Errorf("maxResults = %d, want 100 (exactly capMax must be kept)", maxResults)
	}
}

func TestParsePagination_Garbage(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?startAt=abc&maxResults=xyz", nil)
	startAt, maxResults := ParsePagination(r, 50, 100)
	if startAt != 0 || maxResults != 50 {
		t.Errorf("got %d,%d want 0,50 (garbage values fall back to defaults)", startAt, maxResults)
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

func TestParseExpand_Empty(t *testing.T) {
	r := httptest.NewRequest("GET", "/x", nil)
	e := ParseExpand(r)
	if e.Has("anything") {
		t.Error("Has should be false for any name when expand param is absent")
	}
	if e.String() != "" {
		t.Errorf("String() = %q, want \"\"", e.String())
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

func TestFields_ZeroValueIncludesAll(t *testing.T) {
	var f Fields
	if !f.Include("x") {
		t.Error("zero-value Fields must include all fields")
	}
}
