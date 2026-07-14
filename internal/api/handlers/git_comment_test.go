package handlers

import (
	"encoding/json"
	"testing"
)

// TestBuildCommitCommentADF_ValidJSON verifies that the ADF body produced for
// the auto-generated "commit referenced this issue" comment is always valid
// JSON, even when the commit message contains control bytes or other
// characters that Go's %q string-literal escaping would render as invalid
// JSON escapes (e.g. "\x1b" is not a legal JSON string escape).
func TestBuildCommitCommentADF_ValidJSON(t *testing.T) {
	tests := []struct {
		name    string
		short   string
		message string
	}{
		{
			name:    "plain message",
			short:   "abcd1234",
			message: "fix login bug",
		},
		{
			name:    "message with ESC control byte",
			short:   "abcd1234",
			message: "fix\x1b bug",
		},
		{
			name:    "message with bell and vertical tab",
			short:   "deadbeef",
			message: "weird\x07message\x0bhere",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := buildCommitCommentADF(tt.short, tt.message)
			if err != nil {
				t.Fatalf("buildCommitCommentADF returned error: %v", err)
			}

			var doc struct {
				Type    string `json:"type"`
				Version int    `json:"version"`
				Content []struct {
					Type    string `json:"type"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"content"`
			}

			if err := json.Unmarshal([]byte(body), &doc); err != nil {
				t.Fatalf("body is not valid JSON: %v\nbody: %s", err, body)
			}

			if doc.Type != "doc" || doc.Version != 1 {
				t.Fatalf("unexpected doc envelope: %+v", doc)
			}
			if len(doc.Content) != 1 || len(doc.Content[0].Content) != 1 {
				t.Fatalf("unexpected ADF structure: %+v", doc)
			}

			gotText := doc.Content[0].Content[0].Text
			wantText := "Commit " + tt.short + " referenced this issue: " + tt.message
			if gotText != wantText {
				t.Fatalf("text round-trip mismatch:\n got:  %q\n want: %q", gotText, wantText)
			}
		})
	}
}
