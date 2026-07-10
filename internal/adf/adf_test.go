package adf

import (
	"encoding/json"
	"strings"
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

func TestValidate_NestedInvalidNode(t *testing.T) {
	doc := Node{Type: "doc", Version: 1, Content: []Node{
		{Type: "paragraph", Content: []Node{{Type: ""}}},
	}}
	if err := Validate(doc); err == nil {
		t.Error("expected error for nested node without type")
	}
}

// TestRoundTrip_LossyByDesign pins the documented tradeoff: Node drops
// unknown JSON fields and decodes numeric attrs as float64, so large
// integers lose precision. Callers needing exact payloads must keep the raw JSON.
func TestRoundTrip_LossyByDesign(t *testing.T) {
	const in = `{
	  "type": "doc", "version": 1, "unknownField": "keep me",
	  "content": [
	    {"type": "paragraph", "attrs": {"n": 12345678901234567},
	     "content": [{"type": "text", "text": "x"}]}
	  ]
	}`
	var doc Node
	if err := json.Unmarshal([]byte(in), &doc); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "unknownField") {
		t.Errorf("unknown fields are expected to be dropped, got %s", out)
	}
	if strings.Contains(string(out), "12345678901234567") {
		t.Errorf("large ints are expected to lose precision via float64, got %s", out)
	}
}

func TestRoundTrip_LinkMarkAttrs(t *testing.T) {
	const in = `{
	  "type": "doc", "version": 1,
	  "content": [
	    {"type": "paragraph", "content": [
	      {"type": "text", "text": "docs", "marks": [
	        {"type": "link", "attrs": {"href": "https://example.com"}}
	      ]}
	    ]}
	  ]
	}`
	var doc Node
	if err := json.Unmarshal([]byte(in), &doc); err != nil {
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
	marks := again.Content[0].Content[0].Marks
	if len(marks) != 1 || marks[0].Type != "link" {
		t.Fatalf("mark lost in round trip: %+v", again)
	}
	if href, _ := marks[0].Attrs["href"].(string); href != "https://example.com" {
		t.Errorf("href = %q", href)
	}
}

func TestFromText_CRLF(t *testing.T) {
	doc := FromText("first\r\nsecond")
	if err := Validate(doc); err != nil {
		t.Fatal(err)
	}
	if got := PlainText(doc); got != "first\nsecond" {
		t.Errorf("PlainText = %q", got)
	}
	if len(doc.Content) != 2 {
		t.Errorf("expected 2 paragraphs, got %d", len(doc.Content))
	}
}

func TestFromText_BlankLineRoundTrip(t *testing.T) {
	doc := FromText("a\n\nb")
	if err := Validate(doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Content) != 3 {
		t.Fatalf("expected 3 paragraphs, got %d", len(doc.Content))
	}
	if got := PlainText(doc); got != "a\n\nb" {
		t.Errorf("PlainText = %q", got)
	}
}

func TestPlainText_NestedBlocks(t *testing.T) {
	doc := Node{Type: "doc", Version: 1, Content: []Node{
		{Type: "bulletList", Content: []Node{
			{Type: "listItem", Content: []Node{
				{Type: "paragraph", Content: []Node{{Type: "text", Text: "Buy milk"}}},
			}},
			{Type: "listItem", Content: []Node{
				{Type: "paragraph", Content: []Node{{Type: "text", Text: "Buy eggs"}}},
			}},
		}},
	}}
	if got := PlainText(doc); got != "Buy milk\nBuy eggs" {
		t.Errorf("PlainText = %q", got)
	}
}
