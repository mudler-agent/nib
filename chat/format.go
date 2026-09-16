package chat

import (
	"fmt"
	"strings"
)

// PreviewResult formats a tool result for compact display: it renders the
// result for human reading (FormatToolResult — a purpose-built formatter when
// name has one, flattened rows for an unrecognized JSON object, or the raw
// text unchanged), trims surrounding whitespace, then truncates to at most
// maxLines lines, appending a "… N more lines" note when it had to cut.
// Returns "" for empty/whitespace input. maxLines <= 0 means no line limit.
func PreviewResult(name, s string, maxLines int) string {
	s = strings.TrimRight(strings.TrimSpace(FormatToolResult(name, strings.TrimSpace(s))), "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		extra := len(lines) - maxLines
		noun := "lines"
		if extra == 1 {
			noun = "line"
		}
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("… %d more %s", extra, noun))
	}
	return strings.Join(lines, "\n")
}
