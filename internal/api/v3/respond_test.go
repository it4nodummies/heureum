package v3

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteError_JiraShape(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, 404, []string{"Issue does not exist or you do not have permission to see it."}, nil)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	var body struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.ErrorMessages) != 1 {
		t.Fatalf("errorMessages = %v", body.ErrorMessages)
	}
	// Jira serializza sempre entrambe le chiavi, anche vuote.
	if body.Errors == nil {
		t.Error("errors must be {} not null")
	}
}

func TestWriteError_FieldErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, 400, nil, map[string]string{"summary": "Summary is required."})
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if string(body["errorMessages"]) != "[]" {
		t.Errorf("errorMessages = %s, want []", body["errorMessages"])
	}
}

func TestWritePage(t *testing.T) {
	rec := httptest.NewRecorder()
	WritePage(rec, 200, Page[string]{StartAt: 0, MaxResults: 50, Total: 2, Values: []string{"a", "b"}})
	var body struct {
		StartAt    int      `json:"startAt"`
		MaxResults int      `json:"maxResults"`
		Total      int      `json:"total"`
		IsLast     bool     `json:"isLast"`
		Values     []string `json:"values"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.IsLast || body.Total != 2 || len(body.Values) != 2 {
		t.Errorf("unexpected page: %+v", body)
	}
}

func TestWritePage_PartialPageNotLast(t *testing.T) {
	rec := httptest.NewRecorder()
	WritePage(rec, 200, Page[string]{StartAt: 0, MaxResults: 2, Total: 5, Values: []string{"a", "b"}})
	var body struct {
		IsLast bool     `json:"isLast"`
		Values []string `json:"values"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.IsLast {
		t.Errorf("isLast = true, want false for partial page (startAt=0, total=5, 2 values)")
	}
	if len(body.Values) != 2 {
		t.Errorf("values = %v, want 2 elements", body.Values)
	}
}

func TestWritePage_StructValues(t *testing.T) {
	type issue struct {
		Key     string `json:"key"`
		Summary string `json:"summary"`
	}
	rec := httptest.NewRecorder()
	WritePage(rec, 200, Page[issue]{
		StartAt:    1,
		MaxResults: 50,
		Total:      2,
		Values:     []issue{{Key: "PROJ-2", Summary: "Second issue"}},
	})
	var body struct {
		IsLast bool    `json:"isLast"`
		Values []issue `json:"values"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.IsLast {
		t.Error("isLast = false, want true (startAt=1 + 1 value >= total=2)")
	}
	if len(body.Values) != 1 || body.Values[0].Key != "PROJ-2" {
		t.Errorf("unexpected values: %+v", body.Values)
	}
}
