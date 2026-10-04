package publication

import (
	"html"
	"regexp"
	"strings"
)

var codeLanguage = regexp.MustCompile(`^[A-Za-z0-9_+.-]*$`)

// text preserves complete code spans and fenced blocks. Everything outside code
// remains literal text, so agent prose cannot inject HTML, links or mentions.
func text(value string) string {
	var result strings.Builder
	plainStart := 0
	for pos := 0; pos < len(value); {
		if pos == 0 || value[pos-1] == '\n' {
			if end := fencedEnd(value, pos); end > pos {
				result.WriteString(plainText(value[plainStart:pos]))
				// Fields may have a host prefix such as "Suggested fix: ". A
				// fence needs its own paragraph rather than following that label.
				result.WriteString("\n\n")
				result.WriteString(value[pos:end])
				result.WriteString("\n\n")
				pos, plainStart = end, end
				continue
			}
		}
		if value[pos] == '`' {
			count := runLength(value, pos, '`')
			if end := spanEnd(value, pos+count, count); end > 0 {
				result.WriteString(plainText(value[plainStart:pos]))
				result.WriteString(value[pos:end])
				pos, plainStart = end, end
				continue
			}
			pos += count
		} else {
			pos++
		}
	}
	result.WriteString(plainText(value[plainStart:]))
	return result.String()
}

func plainText(value string) string {
	// Escape Markdown before HTML: otherwise escaping '#' breaks entities such
	// as &#39; into visible &\#39; instead of a normal apostrophe.
	replacer := strings.NewReplacer("\\", "\\\\", "`", "\\`", "~", "\\~", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "#", "\\#", "!", "\\!")
	value = html.EscapeString(replacer.Replace(value))
	return strings.ReplaceAll(value, "@", "&#64;")
}

func runLength(value string, start int, marker byte) int {
	end := start
	for end < len(value) && value[end] == marker {
		end++
	}
	return end - start
}

func spanEnd(value string, pos, width int) int {
	for pos < len(value) && value[pos] != '\n' && value[pos] != '\r' {
		if value[pos] == '`' {
			count := runLength(value, pos, '`')
			if count == width {
				return pos + count
			}
			pos += count
		} else {
			pos++
		}
	}
	return 0
}

// fencedEnd recognizes a complete block with a simple optional language tag.
// Incomplete blocks fall back to literal text and cannot swallow host sections.
func fencedEnd(value string, start int) int {
	lineEnd := strings.IndexByte(value[start:], '\n')
	if lineEnd < 0 {
		return 0
	}
	lineEnd += start
	opening := strings.TrimSuffix(value[start:lineEnd], "\r")
	trimmed := strings.TrimLeft(opening, " ")
	if len(opening)-len(trimmed) > 3 || len(trimmed) < 3 || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0
	}
	marker := trimmed[0]
	width := runLength(trimmed, 0, marker)
	if width < 3 || !codeLanguage.MatchString(strings.TrimSpace(trimmed[width:])) {
		return 0
	}
	for pos := lineEnd + 1; pos < len(value); {
		end := strings.IndexByte(value[pos:], '\n')
		if end < 0 {
			end = len(value)
		} else {
			end += pos
		}
		line := strings.TrimSuffix(value[pos:end], "\r")
		trimmed := strings.TrimLeft(line, " ")
		count := runLength(trimmed, 0, marker)
		if len(line)-len(trimmed) <= 3 && count >= width && strings.TrimSpace(trimmed[count:]) == "" {
			return end
		}
		pos = end + 1
	}
	return 0
}
