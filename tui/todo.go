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
		b.WriteString(todoStatusGlyph(it.Status))
		b.WriteString(" ")
		b.WriteString(truncateLabel(it.Content, 30))
	}
	return b.String()
}

// renderTodoPanel renders the full todo list as a body-replacing panel
// (toggled with Ctrl+T). Mirrors the log viewer approach: the panel owns
// the body area entirely and suppresses the composer + footer.
func (m Model) renderTodoPanel() string {
	var b strings.Builder

	b.WriteString(theme.Brand.Render("todos"))
	b.WriteString("\n\n")

	if m.session == nil || m.session.TodoList() == nil {
		b.WriteString(theme.Help.Render("No todo list in this session."))
		return b.String()
	}

	items := m.session.TodoList().Items()
	if len(items) == 0 {
		b.WriteString(theme.Help.Render("The todo list is empty."))
		return b.String()
	}

	done, total := m.session.TodoList().Counts()
	b.WriteString(theme.Meta.Render(fmt.Sprintf("%d/%d completed", done, total)))
	b.WriteString("\n\n")

	w := m.width
	if w < 20 {
		w = 80
	}

	for i, it := range items {
		glyph := todoStatusGlyph(it.Status)
		label := it.Content

		var line string
		switch it.Status {
		case chat.TodoCompleted:
			line = theme.Done.Render(glyph) + " " + theme.Subtle.Render(label)
		case chat.TodoActive:
			line = theme.Running.Render(glyph) + " " + theme.Brand.Render(label)
		case chat.TodoCancelled:
			line = theme.Error.Render(glyph) + " " + theme.Subtle.Render(label)
		default:
			line = theme.Help.Render(glyph) + " " + label
		}

		b.WriteString(line)

		if it.ActiveForm != "" && it.Status == chat.TodoActive {
			b.WriteString(theme.Meta.Render(" — " + it.ActiveForm))
		}

		// Wrap long content to the panel width.
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// todoStatusGlyph returns the display glyph for a todo status.
func todoStatusGlyph(s chat.TodoStatus) string {
	switch s {
	case chat.TodoCompleted:
		return "✓"
	case chat.TodoActive:
		return "◐"
	case chat.TodoCancelled:
		return "~"
	default:
		return "○"
	}
}

// truncateLabel shortens a label to maxRunes, appending "…" if truncated.
func truncateLabel(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes-1]) + "…"
}
