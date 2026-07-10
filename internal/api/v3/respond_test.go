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
	WritePage(rec, 200, Page{StartAt: 0, MaxResults: 50, Total: 2, Values: []string{"a", "b"}})
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
