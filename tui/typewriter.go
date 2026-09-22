package tui

import (
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
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
