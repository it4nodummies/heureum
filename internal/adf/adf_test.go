package adf

import (
	"encoding/json"
	"testing"
)

const sample = `{
  "type": "doc", "version": 1,
  "content": [
    {"type": "paragraph", "content": [
      {"type": "text", "text": "Hello "},
      {"type": "text", "text": "world", "marks": [{"type": "strong"}]}
    ]},
    {"type": "paragraph", "content": [{"type": "text", "text": "Second line"}]}
  ]
}`

func TestRoundTrip(t *testing.T) {
	var doc Node
	if err := json.Unmarshal([]byte(sample), &doc); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var again Node
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if again.Type != "doc" || again.Version != 1 || len(again.Content) != 2 {
		t.Errorf("round trip lost data: %+v", again)
	}
}

func TestPlainText(t *testing.T) {
	var doc Node
	_ = json.Unmarshal([]byte(sample), &doc)
	if got := PlainText(doc); got != "Hello world\nSecond line" {
		t.Errorf("PlainText = %q", got)
	}
}

func TestFromText(t *testing.T) {
	doc := FromText("Just a note")
	if err := Validate(doc); err != nil {
		t.Fatal(err)
	}
	if PlainText(doc) != "Just a note" {
		t.Errorf("PlainText = %q", PlainText(doc))
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(Node{Type: "paragraph"}); err == nil {
		t.Error("root must be doc")
	}
	if err := Validate(Node{Type: "doc", Version: 2}); err == nil {
		t.Error("version must be 1")
	}
}
