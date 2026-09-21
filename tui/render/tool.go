package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mudler/nib/internal/textdiff"
	"github.com/mudler/nib/theme"
)

// ToolStatus is the outcome a tool block's header marks.
type ToolStatus int

const (
	ToolStatusUnknown ToolStatus = iota // no outcome to show (a plain separator)
	ToolStatusOK
	ToolStatusFailed
)

// toolIndent is how far a tool block's body sits beneath its header.
const toolIndent = "  "

// TranscriptDiffRows caps a diff shown in the transcript; ApprovalDiffRows caps
// one in the approval prompt, where the user decides on it and so gets more.
const (
	TranscriptDiffRows = 16
	ApprovalDiffRows   = 30
)

// ToolBlock renders a RoleTool message: a header line (status mark, the call
// summary with its verb emphasized, and dim metadata), then the body beneath
// it at a two-cell indent — the diff when the message carries one, otherwise
// the text output. It is the same on both surfaces, so it lives here rather
// than in each presenter.
func ToolBlock(m Message, w int) string {
	var b strings.Builder
	b.WriteString(ToolHeader(m.Label, m.Meta, m.AgentID, m.Status, w))
	b.WriteString("\n")
	bodyW := w - lipgloss.Width(toolIndent)
	if m.Diff != nil && len(m.Diff.Lines) > 0 {
		for _, row := range DiffRows(*m.Diff, bodyW, TranscriptDiffRows) {
			b.WriteString(toolIndent + row + "\n")
		}
		return b.String()
	}
	if strings.TrimSpace(m.Content) == "" {
		return b.String()
	}
	style := theme.ToolOutput
	if m.Status == ToolStatusFailed {
		// A failure's output is what the user needs to read next; do not dim it.
		style = lipgloss.NewStyle()
	}
	wrapped := Wrap(m.Content, bodyW)
	for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
		b.WriteString(toolIndent + style.Render(line) + "\n")
	}
	return b.String()
}

// ToolHeader renders a tool block's one-line header, truncated to w: the
// status mark, the sub-agent tag when agentID is set, the summary label and
// the dim meta. The label's first word is the verb ("edit", "read", "grep")
// and is shown at full strength; the rest is dim. A shell command ("$ …")
// instead dims the "$" and shows the command itself at full strength, since
// the command is what the user scans for.
func ToolHeader(label, meta, agentID string, status ToolStatus, w int) string {
	var mark string
	switch status {
	case ToolStatusOK:
		mark = theme.ToolOK.Render(theme.Check)
	case ToolStatusFailed:
		mark = theme.ToolFailed.Render(theme.Cross)
	default:
		mark = theme.Subtle.Render(theme.Sep)
	}
	head := mark + " "
	if agentID != "" {
		head += theme.Subtle.Render(theme.SubAgent+" "+ShortID(agentID)) + theme.SepStyle.Render(" "+theme.Sep+" ")
	}
	room := w - lipgloss.Width(head)
	if meta != "" {
		room -= lipgloss.Width(meta) + 2
	}
	label = TruncateRunes(label, max(room, 1))

	switch {
	case strings.HasPrefix(label, "$ "):
		head += theme.ToolDetail.Render("$") + " " + theme.ToolVerb.Render(label[2:])
	default:
		verb, rest, _ := strings.Cut(label, " ")
		head += theme.ToolVerb.Render(verb)
		if rest != "" {
			head += " " + theme.ToolDetail.Render(rest)
		}
	}
	if meta != "" {
		head += "  " + theme.Meta.Render(meta)
	}
	return head
}

// DiffStat summarizes a diff as "+N -M" (ASCII, as git prints it), leaving out a side with no lines.
func DiffStat(d textdiff.Diff) string {
	var parts []string
	if d.Added > 0 {
		parts = append(parts, "+"+strconv.Itoa(d.Added))
	}
	if d.Removed > 0 {
		parts = append(parts, "-"+strconv.Itoa(d.Removed))
	}
	return strings.Join(parts, " ")
}

// DiffRows renders a diff as styled rows exactly w cells wide: a dim line
// number gutter (left out for a fragment diff, which has no numbers), then a
// sign column and the line text. Added and removed rows are tinted across the
// full width so the change reads as a band; context rows are dim; a Gap row is
// a dim ellipsis. Tabs expand to four spaces so the tint has no holes, and a
// line too long for w is cut with an ellipsis rather than wrapped, which keeps
// one row per line. At most maxRows rows are shown (maxRows <= 0 shows all);
// past that a fold line says how many were left out.
func DiffRows(d textdiff.Diff, w, maxRows int) []string {
	numW := 0
	for _, l := range d.Lines {
		if n := max(l.OldNo, l.NewNo); n > 0 {
			numW = max(numW, len(strconv.Itoa(n)))
		}
	}
	gutter := 0
	if numW > 0 {
		gutter = numW + 1
	}
	textW := max(w-gutter-2, 1) // sign + space

	lines := d.Lines
	hidden := 0
	if maxRows > 0 && len(lines) > maxRows {
		hidden = len(lines) - (maxRows - 1)
		lines = lines[:maxRows-1]
	}
	rows := make([]string, 0, len(lines)+1)
	for _, l := range lines {
		num := ""
		if numW > 0 {
			n := l.NewNo
			if l.Kind == textdiff.Del {
				n = l.OldNo
			}
			s := ""
			if n > 0 {
				s = strconv.Itoa(n)
			}
			num = theme.DiffLineNo.Render(fmt.Sprintf("%*s", numW, s)) + " "
		}
		if l.Kind == textdiff.Gap {
			rows = append(rows, strings.Repeat(" ", gutter)+theme.DiffLineNo.Render(theme.DiffGap))
			continue
		}
		text := TruncateRunes(strings.ReplaceAll(l.Text, "\t", "    "), textW)
		pad := strings.Repeat(" ", max(textW-lipgloss.Width(text), 0))
		switch l.Kind {
		case textdiff.Add:
			rows = append(rows, num+theme.DiffAddSign.Render("+ ")+theme.DiffAdd.Render(text+pad))
		case textdiff.Del:
			rows = append(rows, num+theme.DiffDelSign.Render("- ")+theme.DiffDel.Render(text+pad))
		default:
			rows = append(rows, num+"  "+theme.DiffContext.Render(text))
		}
	}
	if hidden > 0 {
		rows = append(rows, strings.Repeat(" ", gutter)+theme.Hint.Render(fmt.Sprintf(theme.DiffMore, hidden)))
	}
	return rows
}
