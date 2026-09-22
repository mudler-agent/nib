package tui

import (
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mudler/nib/theme"
)

// The streamed reply is revealed at a steady pace instead of one jump per
// provider chunk. Providers deliver content in bursts (a few tokens, a
// pause, a whole line), so drawing each burst as it lands looks jerky. The
// transcript keeps the full text (reconciliation in responseMsg/parkMsg is
// unchanged); only the render of the streaming tail is cut at
// streamShown, and a tick advances that cut toward the end.
const (
	// streamRevealInterval is one reveal frame, about 33 fps. A faster rate
	// adds full-transcript renders without a visible gain in a terminal.
	streamRevealInterval = 30 * time.Millisecond
	// streamRevealDrainFrames is how many frames a backlog takes to drain:
	// each frame reveals 1/N of what is still hidden, so a large burst
	// shows fast and the end of it slows down (an ease-out). With 8 frames
	// the text stays at most about 250ms behind the model.
	streamRevealDrainFrames = 8
)

// streamTickMsg advances the reveal of the streaming reply by one frame.
type streamTickMsg struct{}

func streamTick() tea.Cmd {
	return tea.Tick(streamRevealInterval, func(time.Time) tea.Msg { return streamTickMsg{} })
}

// streamBacklog reports whether the streaming tail has text not yet shown.
func (m Model) streamBacklog() bool {
	return m.streamingActive && len(m.messages) > 0 && m.streamShown < len(m.messages[len(m.messages)-1].Content)
}

// startStreamReveal returns the tick Cmd when a backlog exists and no tick is
// already pending. Only one tick is in flight at a time, so bursts of deltas
// do not multiply the frame rate.
func (m *Model) startStreamReveal() tea.Cmd {
	if m.streamTicking || !m.streamBacklog() {
		return nil
	}
	m.streamTicking = true
	return streamTick()
}

// advanceStreamReveal moves streamShown forward by one frame. It counts in
// runes and always stops on a rune boundary, so a multi-byte character is
// never cut in half.
func (m *Model) advanceStreamReveal() {
	text := m.messages[len(m.messages)-1].Content
	hidden := text[m.streamShown:]
	n := utf8.RuneCountInString(hidden)
	step := (n + streamRevealDrainFrames - 1) / streamRevealDrainFrames
	for i := 0; i < step && m.streamShown < len(text); i++ {
		_, size := utf8.DecodeRuneInString(text[m.streamShown:])
		m.streamShown += size
	}
}

// visibleStreamContent is the part of the streaming tail to draw this frame.
func (m Model) visibleStreamContent(content string) string {
	if m.streamShown >= len(content) {
		return content
	}
	return content[:m.streamShown]
}

// renderStreaming renders the visible part of the streaming reply so it looks
// like the final glamour pass will, and nothing jumps when the turn ends.
//
// The settled blocks (everything up to the last block boundary) go through
// the cached renderMarkdown. The open block at the end changes every frame,
// and rendering it on its own would get the gap before it wrong: glamour
// puts no blank line after a heading, and a code block brings its own padded
// one. So the open block is rendered together with the last settled block,
// and the lines of that settled block are dropped from the result. The gap
// is then glamour's own, and each frame renders two blocks, not the reply.
//
// A code block that is still open is closed for the render, so it shows as
// code while it streams rather than as raw text that snaps into a code block
// at the end. The pulsing cursor goes at the end of the last line.
func (m *Model) renderStreaming(visible string, width int) string {
	settled, open, lastStart, inFence := splitSettled(visible)
	if inFence {
		open += "\n" + fenceClose(open)
	}
	var out string
	switch {
	case strings.TrimSpace(open) == "":
		out = m.renderMarkdown(settled, width)
	case strings.TrimSpace(settled) == "":
		out = renderMarkdownWith(m.markdownFor(width), open, width)
	default:
		out = m.renderMarkdown(settled, width) + "\n" + m.renderOpenBlock(settled[lastStart:], open, width)
	}
	return appendCursor(out, theme.StreamCursorAt(time.Since(m.streamStart)), width)
}

// renderOpenBlock renders open as it appears after last in one document,
// without the lines of last itself.
func (m *Model) renderOpenBlock(last, open string, width int) string {
	skip := strings.Count(m.renderMarkdown(last, width), "\n") + 1
	lines := strings.Split(renderMarkdownWith(m.markdownFor(width), last+open, width), "\n")
	if skip >= len(lines) {
		return renderMarkdownWith(m.markdownFor(width), open, width)
	}
	return strings.Join(lines[skip:], "\n")
}

// splitSettled cuts text at its last block boundary: a blank line outside a
// code fence, or the line that closes a fence. settled is complete markdown
// that will not change as more text arrives; open is the block still being
// written. lastStart is where the last block of settled begins. inFence
// reports that open is inside a code fence that has not closed yet.
func splitSettled(text string) (settled, open string, lastStart int, inFence bool) {
	cut, prev := 0, 0
	fence := ""
	pos := 0
	boundary := func(at int) {
		if at > cut && strings.TrimSpace(text[cut:at]) != "" {
			prev = cut
		}
		cut = at
	}
	for pos < len(text) {
		end := strings.IndexByte(text[pos:], '\n')
		if end < 0 {
			break // the last line is incomplete: it can never be a boundary
		}
		line := text[pos : pos+end]
		next := pos + end + 1
		trimmed := strings.TrimSpace(line)
		switch {
		case fence == "" && isFence(trimmed):
			fence = trimmed[:3]
		case fence != "" && strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "":
			fence = ""
			boundary(next)
		case fence == "" && trimmed == "":
			boundary(next)
		}
		pos = next
	}
	if fence == "" && pos < len(text) && isFence(strings.TrimSpace(text[pos:])) {
		fence = strings.TrimSpace(text[pos:])[:3]
	}
	return text[:cut], text[cut:], prev, fence != ""
}

func isFence(line string) bool {
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

// fenceClose returns the closing line for the fence that opens block.
func fenceClose(block string) string {
	if strings.HasPrefix(strings.TrimSpace(block), "~~~") {
		return "~~~"
	}
	return "```"
}

// appendCursor puts cursor after the last visible cell of out. glamour pads
// wrapped lines with spaces up to the width, so the padding is cut first. A
// trailing blank line (the padding a code block ends with) is kept, and the
// cursor goes on the last line that has text. When that line is already full
// the cursor starts a new line.
func appendCursor(out, cursor string, width int) string {
	lines := strings.Split(out, "\n")
	i := len(lines) - 1
	for i > 0 && strings.TrimSpace(ansi.Strip(lines[i])) == "" {
		i--
	}
	visible := strings.TrimRight(ansi.Strip(lines[i]), " ")
	w := ansi.StringWidth(visible)
	line := ansi.Truncate(lines[i], w, "")
	if w+ansi.StringWidth(cursor) > width {
		lines[i] = line + "\n" + cursor
	} else {
		lines[i] = line + cursor
	}
	return strings.Join(lines, "\n")
}
