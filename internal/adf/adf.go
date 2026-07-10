// Package adf implements the Atlassian Document Format used by Jira Cloud v3
// for descriptions and comments. https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
package adf

import (
	"errors"
	"strings"
)

type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

type Node struct {
	Type    string         `json:"type"`
	Version int            `json:"version,omitempty"` // only on root "doc" node
	Text    string         `json:"text,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

// FromText constructs a minimal ADF document from plain text
// (one paragraph per line).
func FromText(text string) Node {
	var paras []Node
	for _, line := range strings.Split(text, "\n") {
		p := Node{Type: "paragraph"}
		if line != "" {
			p.Content = []Node{{Type: "text", Text: line}}
		}
		paras = append(paras, p)
	}
	return Node{Type: "doc", Version: 1, Content: paras}
}

// PlainText extracts plain text from the document; block nodes are separated by newlines.
func PlainText(n Node) string {
	var blocks []string
	for _, child := range n.Content {
		blocks = append(blocks, inlineText(child))
	}
	return strings.Join(blocks, "\n")
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
