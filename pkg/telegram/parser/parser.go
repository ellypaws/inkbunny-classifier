package parser

import (
	"fmt"
	"strings"
)

// Parse is the entry point. It first pre-processes the text to replace Discord-specific
// markers (timestamps, mentions) with temporary markers, builds the AST, then renders it.
func Parse(text string) string {
	// Build AST from the resulting text.
	if !strings.HasSuffix(text, "\n") {
		text = fmt.Sprintf("%s\n", text)
	}
	nodes := buildAST(text)
	// Render AST back into a Telegram MarkdownV2 string.
	return strings.TrimSpace(renderNodes(nodes, len(text)))
}

// renderNodes concatenates the rendered output of all AST nodes.
func renderNodes(nodes []Node, length int) string {
	var sb strings.Builder
	sb.Grow(length)
	for _, n := range nodes {
		sb.WriteString(n.String())
	}
	return sb.String()
}
