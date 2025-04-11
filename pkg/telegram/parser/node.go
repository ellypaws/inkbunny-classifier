package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Node represents an AST node.
type Node interface {
	String() string
}

// TextNode holds plain text.
type TextNode struct {
	Text string
	Raw  bool
}

func (n *TextNode) String() string {
	if n.Raw {
		return n.Text
	}
	return escapeTelegram(n.Text)
}

// FormattingNode represents a formatting block (bold, italic, underline, etc.).
type FormattingNode struct {
	Format   string // e.g. "*", "_", "__", "~~" or "||"
	Children []Node
}

func (n *FormattingNode) String() string {
	var inner strings.Builder
	for _, child := range n.Children {
		inner.WriteString(child.String())
	}
	// Map Discord tokens to Telegram MarkdownV2 equivalents.
	switch n.Format {
	case "~~":
		return "~" + inner.String() + "~"
	case "**":
		return "*" + inner.String() + "*"
	case "*":
		return "_" + inner.String() + "_"
	default:
		return n.Format + inner.String() + n.Format
	}
}

// CodeNode represents inline code.
type CodeNode struct {
	Text string
}

func (n *CodeNode) String() string {
	return "`" + escapeTelegram(n.Text) + "`"
}

// CodeBlockNode represents a code block.
type CodeBlockNode struct {
	Text string
}

func (n *CodeBlockNode) String() string {
	if strings.TrimSpace(n.Text) == "" {
		return ""
	}
	text := strings.ReplaceAll(n.Text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "`", "\\`")
	return "```" + text + "```"
}

// QuoteBlockNode represents a quote block.
type QuoteBlockNode struct {
	Children []Node
}

func (n *QuoteBlockNode) String() string {
	var inner strings.Builder
	for _, child := range n.Children {
		inner.WriteString(child.String())
	}
	return ">" + inner.String() + "\n"
}

type HeaderNode struct {
	Level    int
	Children []Node
}

func (n *HeaderNode) String() string {
	var inner strings.Builder
	for _, child := range n.Children {
		inner.WriteString(child.String())
	}
	switch n.Level {
	case 1:
		return ">*" + strings.Trim(inner.String(), "*") + "*\n"
	case 2:
		return ">" + inner.String() + "\n"
	case 3:
		return "*" + strings.Trim(inner.String(), "*") + "*\n"
	default:
		return inner.String()
	}
}

// LinkNode represents an inline link [text](url).
type LinkNode struct {
	Text string
	URL  string
}

func (n *LinkNode) String() string {
	url := strings.ReplaceAll(n.URL, "\\", "\\\\")
	url = strings.ReplaceAll(url, ")", "\\)")
	return fmt.Sprintf("[%s](%s)", escapeTelegram(n.Text), url)
}

// MentionNode represents a mention (user, channel, role).
type MentionNode struct {
	Text string
}

func (n *MentionNode) String() string {
	return escapeTelegram(n.Text)
}

// TimestampNode represents a Discord timestamp.
type TimestampNode struct {
	Timestamp int64
	Style     string
}

func (n *TimestampNode) String() string {
	utc := time.Unix(n.Timestamp, 0).UTC()
	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		return n.format(utc)
	}

	return fmt.Sprintf(`*%s* \(%s\)`, n.format(utc), n.format(utc.In(est)))
}

func (n *TimestampNode) format(t time.Time) string {
	var formatted string
	switch n.Style {
	case "t":
		formatted = t.Format("3:04 PM MST")
	case "T":
		formatted = t.Format("3:04:05 PM MST")
	case "d":
		formatted = t.Format("02/01/2006")
	case "D":
		formatted = t.Format("January 02, 2006")
	case "f":
		formatted = t.Format("January 02, 2006 3:04 PM MST")
	case "F":
		formatted = t.Format("Monday, January 02, 2006 3:04 PM MST")
	case "R":
		return formatRelativeFull(t)
	default:
		formatted = t.Format(time.RFC3339)
	}
	return escapeTelegram(formatted)
}

func formatRelativeFull(t time.Time) string {
	return fmt.Sprintf(`*%s* \(%s\)`, formatRelativeTime(t), escapeTelegram(t.Format("January 02, 2006 3:04 PM MST")))
}

func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)
	if diff < 0 {
		diff = -diff
		switch {
		case diff < time.Minute:
			return "in a few seconds"
		case diff < time.Hour:
			return fmt.Sprintf("in %d minutes", int(diff.Minutes()))
		case diff < 24*time.Hour:
			return fmt.Sprintf("in %d hours", int(diff.Hours()))
		case diff < 30*24*time.Hour:
			return fmt.Sprintf("in %d days", int(diff.Hours()/24))
		case diff < 365*24*time.Hour:
			return fmt.Sprintf("in %d months", int(diff.Hours()/(24*30)))
		default:
			return fmt.Sprintf("in %d years", int(diff.Hours()/(24*365)))
		}
	}
	switch {
	case diff < time.Minute:
		return "a few seconds ago"
	case diff < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(diff.Hours()))
	case diff < 30*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	case diff < 365*24*time.Hour:
		return fmt.Sprintf("%d months ago", int(diff.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%d years ago", int(diff.Hours()/(24*365)))
	}
}

// escapeTelegram escapes Telegram MarkdownV2 special characters in text,
// skipping those that are already escaped.
func escapeTelegram(text string) string {
	specialChars := map[rune]bool{
		'_': true, '*': true, '[': true, ']': true, '(': true, ')': true,
		'~': true, '`': true, '>': true, '#': true, '+': true, '-': true,
		'=': true, '|': true, '{': true, '}': true, '.': true, '!': true,
	}
	var sb strings.Builder
	// Here, we use a range loop (which is rune aware) for escaping.
	for i, r := range text {
		if specialChars[r] && !isEscaped(text, i) {
			sb.WriteRune('\\')
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// isEscaped returns true if the character at pos in text is escaped by an odd number of preceding backslashes.
// Note: pos is a byte index.
func isEscaped(text string, pos int) bool {
	count := 0
	for pos-1 >= 0 && text[pos-1] == '\\' {
		count++
		pos--
	}
	return count%2 == 1
}

// findClosing searches for token in text starting at byte offset start,
// decoding UTF-8 runes as it goes.
func findClosing(text string, start int, token string) int {
	for j := start; j < len(text); {
		if strings.HasPrefix(text[j:], token) && !isEscaped(text, j) {
			return j
		}
		_, size := utf8.DecodeRuneInString(text[j:])
		j += size
	}
	return -1
}

// buildAST parses the input string into an AST using UTF-8 aware decoding.
func buildAST(input string) []Node {
	var (
		nodes []Node
		i     int
	)
	for i < len(input) {
		r, size := utf8.DecodeRuneInString(input[i:])

		// Check for escaped tokens.
		if r == '\\' && i+size < len(input) {
			_, size2 := utf8.DecodeRuneInString(input[i+size:])
			nodes = append(nodes, &TextNode{Text: input[i : i+size+size2], Raw: true})
			i += size + size2
			continue
		}

		// Handle markers such as [[TIMESTAMP:...]], [[MENTION:...]], [[CHANNEL:...]]
		if strings.HasPrefix(input[i:], "[[TIMESTAMP:") {
			end := findClosing(input, i+len("[[TIMESTAMP:"), "]]")
			if end != -1 {
				marker := input[i : end+len("]]")]
				parts := strings.Split(strings.Trim(marker, "[]"), ":")
				if len(parts) == 3 {
					ts, err := strconv.ParseInt(parts[1], 10, 64)
					if err == nil {
						nodes = append(nodes, &TimestampNode{Timestamp: ts, Style: parts[2]})
						i = end + len("]]")
						continue
					}
				}
			}
		}
		if strings.HasPrefix(input[i:], "[[MENTION:") {
			end := findClosing(input, i+len("[[MENTION:"), "]]")
			if end != -1 {
				marker := input[i : end+len("]]")]
				content := strings.TrimPrefix(strings.TrimSuffix(marker, "]]"), "[[MENTION:")
				nodes = append(nodes, &MentionNode{Text: content})
				i = end + len("]]")
				continue
			}
		}
		if strings.HasPrefix(input[i:], "[[CHANNEL:") {
			end := findClosing(input, i+len("[[CHANNEL:"), "]]")
			if end != -1 {
				marker := input[i : end+len("]]")]
				content := strings.TrimPrefix(strings.TrimSuffix(marker, "]]"), "[[CHANNEL:")
				nodes = append(nodes, &MentionNode{Text: content})
				i = end + len("]]")
				continue
			}
		}

		// Code block: ```...```
		if strings.HasPrefix(input[i:], "```") {
			j := i + len("```")
			end := findClosing(input, j, "```")
			if end != -1 {
				code := input[j:end]
				nodes = append(nodes, &CodeBlockNode{Text: code})
				i = end + len("```")
				continue
			}
		}

		// Inline code: `...`
		if r == '`' {
			end := findClosing(input, i+size, "`")
			if end != -1 {
				code := input[i+size : end]
				nodes = append(nodes, &CodeNode{Text: code})
				i = end + len("`")
				continue
			}
		}

		// Inline link: [text](url)
		if r == '[' {
			closeBracket := findClosing(input, i+size, "]")
			if closeBracket != -1 && closeBracket+1 < len(input) && input[closeBracket+1] == '(' {
				closeParen := findClosing(input, closeBracket+2, ")")
				if closeParen != -1 {
					linkText := input[i+size : closeBracket]
					linkURL := input[closeBracket+2 : closeParen]
					nodes = append(nodes, &LinkNode{Text: linkText, URL: linkURL})
					i = closeParen + len(")")
					continue
				}
			}
		}

		// Multi-character formatting tokens: **, __, ||, ~~
		if strings.HasPrefix(input[i:], "**") || strings.HasPrefix(input[i:], "__") ||
			strings.HasPrefix(input[i:], "||") || strings.HasPrefix(input[i:], "~~") {
			token := input[i : i+2]
			end := findClosing(input, i+2, token)
			if end != -1 {
				innerContent := input[i+2 : end]
				children := buildAST(innerContent)
				nodes = append(nodes, &FormattingNode{Format: token, Children: children})
				i = end + 2
				continue
			}
		}

		// Header: # ...
		if strings.HasPrefix(input[i:], "### ") {
			end := findClosing(input, i+len("### "), "\n")
			if end != -1 {
				children := buildAST(input[i+len("### ") : end])
				nodes = append(nodes, &HeaderNode{Level: 3, Children: children})
				i = end + 1
				continue
			}
		}
		if strings.HasPrefix(input[i:], "## ") {
			end := findClosing(input, i+len("## "), "\n")
			if end != -1 {
				children := buildAST(input[i+len("## ") : end])
				nodes = append(nodes, &HeaderNode{Level: 2, Children: children})
				i = end + 1
				continue
			}
		}
		if strings.HasPrefix(input[i:], "# ") {
			end := findClosing(input, i+len("# "), "\n")
			if end != -1 {
				children := buildAST(input[i+len("# ") : end])
				nodes = append(nodes, &HeaderNode{Level: 1, Children: children})
				i = end + 1
				continue
			}
		}

		// Quote block: > ...
		if (i == 0 || input[i-1] == '\n') && strings.HasPrefix(input[i:], "> ") {
			end := findClosing(input, i+len("> "), "\n")
			if end != -1 {
				children := buildAST(input[i+len("> ") : end])
				nodes = append(nodes, &QuoteBlockNode{Children: children})
				i = end + 1
				continue
			}
		}

		// Single-character formatting tokens: * and _
		if r == '*' || r == '_' {
			token := input[i : i+size]
			end := findClosing(input, i+size, token)
			if end != -1 {
				innerContent := input[i+size : end]
				children := buildAST(innerContent)
				nodes = append(nodes, &FormattingNode{Format: token, Children: children})
				i = end + size
				continue
			}
		}

		// Default: treat current rune as plain text.
		nodes = append(nodes, &TextNode{Text: input[i : i+size]})
		i += size
	}
	return nodes
}
