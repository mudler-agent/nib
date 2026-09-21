package tui

import (
	"fmt"
	"strings"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// todoFooterRow builds the footer row for the ephemeral todo list.
// Returns ok=false when the list is empty or the session has no todo list.
func todoFooterRow(tl *chat.TodoList) (render.FooterRow, bool) {
	if tl == nil {
		return render.FooterRow{}, false
	}
	items := tl.Items()
	if len(items) == 0 {
		return render.FooterRow{}, false
	}

	done, total := tl.Counts()
	text := todoPreview(items, done, total)
	return render.FooterRow{
		Glyph: theme.Todo,
		Text:  text,
		Kind:  render.FooterTodo,
	}, true
}

// todoPreview renders a compact single-line preview of the todo list for the
// footer: "2/5 ✓ read files ◐ writing tests · build project"
//
// Completed items show ✓, the in_progress item shows ◐, and pending items are
// listed plainly. Cancelled items are struck-through (rendered as ~). The list
// is truncated with an ellipsis if it would overflow a reasonable width.
func todoPreview(items []chat.TodoItem, done, total int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d/%d", done, total)

	for _, it := range items {
		b.WriteString(" ")
		switch it.Status {
		case chat.TodoCompleted:
			b.WriteString("✓")
		case chat.TodoActive:
			b.WriteString("◐")
		case chat.TodoCancelled:
			b.WriteString("~")
		default:
			b.WriteString("○")
		}
		b.WriteString(" ")
		b.WriteString(truncateLabel(it.Content, 30))
	}
	return b.String()
}

// truncateLabel shortens a label to maxRunes, appending "…" if truncated.
func truncateLabel(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes-1]) + "…"
}
