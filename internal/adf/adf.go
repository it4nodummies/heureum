// Package adf implements the Atlassian Document Format used by Jira Cloud v3
// for descriptions and comments. https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
package adf

import (
	"errors"
	"strings"
)

// Mark is an inline formatting annotation applied to a text node (e.g.
// strong, em, link). Attrs holds mark-specific attributes such as a link's
// "href".
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Node is a single ADF node; a document is a tree of Nodes rooted at a node
// of type "doc". Attrs holds node-specific attributes (e.g. a heading's
// "level"); Marks holds inline formatting on text nodes.
//
// Round-trip fidelity: Node is a lossy model by design. Unknown JSON fields
// are dropped on unmarshal, and numeric attrs are decoded as float64, so
// large integers (beyond 2^53) lose precision. Callers that must preserve
// exact client payloads byte-for-byte (as planned for issue descriptions)
// should store the original raw JSON alongside the parsed Node.
type Node struct {
	Type    string         `json:"type"`
	Version int            `json:"version,omitempty"` // only on root "doc" node
	Text    string         `json:"text,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

// blockTypes are ADF node types whose rendered text forms separate lines in
// PlainText output.
var blockTypes = map[string]bool{
	"paragraph": true, "heading": true, "blockquote": true,
	"bulletList": true, "orderedList": true, "listItem": true,
	"codeBlock": true, "rule": true, "table": true, "tableRow": true, "tableCell": true,
	"mediaGroup": true, "panel": true,
}

// FromText constructs a minimal ADF document from plain text
// (one paragraph per line). CRLF line endings are normalized to LF.
func FromText(text string) Node {
	var paras []Node
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		p := Node{Type: "paragraph"}
		if line != "" {
			p.Content = []Node{{Type: "text", Text: line}}
		}
		paras = append(paras, p)
	}
	return Node{Type: "doc", Version: 1, Content: paras}
}

// PlainText extracts plain text from the document; block nodes are separated
// by newlines. Empty block nodes (e.g. empty paragraphs) produce empty lines,
// so FromText and PlainText round-trip.
func PlainText(n Node) string {
	return strings.Join(segments(n), "\n")
}

// segments renders the children of n as text lines: each block child
// contributes its own segments recursively, while consecutive inline children
// are concatenated into a single segment. A node whose children produce no
// segments yields one empty segment, preserving empty blocks as blank lines.
func segments(n Node) []string {
	var segs []string
	var cur strings.Builder
	inline := false
	flush := func() {
		segs = append(segs, cur.String())
		cur.Reset()
		inline = false
	}
	for _, child := range n.Content {
		if blockTypes[child.Type] {
			if inline {
				flush()
			}
			segs = append(segs, segments(child)...)
		} else {
			cur.WriteString(inlineText(child))
			inline = true
		}
	}
	if inline {
		flush()
	}
	if len(segs) == 0 {
		return []string{""}
	}
	return segs
}

func inlineText(n Node) string {
	if n.Type == "text" {
		return n.Text
	}
	var b strings.Builder
	for _, child := range n.Content {
		b.WriteString(inlineText(child))
	}
	return b.String()
}

// Validate verifies basic structural constraints of an ADF document.
func Validate(n Node) error {
	if n.Type != "doc" {
		return errors.New("adf: root node must have type \"doc\"")
	}
	if n.Version != 1 {
		return errors.New("adf: version must be 1")
	}
	return validateChildren(n.Content)
}

func validateChildren(nodes []Node) error {
	for _, n := range nodes {
		if n.Type == "" {
			return errors.New("adf: node without type")
		}
		if n.Type == "text" && n.Text == "" {
			return errors.New("adf: text node without text")
		}
		if err := validateChildren(n.Content); err != nil {
			return err
		}
	}
	return nil
}
