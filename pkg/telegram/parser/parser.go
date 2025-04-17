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

// Parsef is a convenience function that formats the input string using [fmt.Sprintf] and Parse.
func Parsef(format string, a ...any) string {
	return Parse(fmt.Sprintf(format, a...))
}

// Patternf is a convenience function that returns a function that formats the input string using [fmt.Sprintf] and Parse.
func Patternf(pattern string, defaults ...any) func(a ...any) string {
	needed := len(defaults)
	return func(a ...any) string {
		got := len(a)
		args := make([]interface{}, needed)
		for i := range needed {
			if i > got {
				args[i] = defaults[i]
			} else {
				args[i] = a[i]
			}
		}
		return Parse(fmt.Sprintf(pattern, args...))
	}
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
