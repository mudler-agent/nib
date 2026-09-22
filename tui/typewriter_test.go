package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/mudler/nib/theme"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// drainStreamReveal delivers reveal ticks until the streaming tail is fully
// drawn, failing if that takes an unbounded number of frames.
func drainStreamReveal(t *testing.T, m Model) Model {
	t.Helper()
	for i := 0; m.streamBacklog(); i++ {
		if i > 1000 {
			t.Fatal("stream reveal never caught up")
		}
		next, _ := m.Update(streamTickMsg{})
		m = next.(Model)
	}
	return m
}

func streamModel() Model {
	return Model{
		viewport:  viewport.New(80, 20),
		width:     80,
		loading:   true,
		presenter: testPresenter(),
	}
}

// A burst is drawn over several frames, not all at once, and every frame
// shows a prefix of the reply.
func TestStreamRevealDrawsBurstOverSeveralFrames(t *testing.T) {
	reply := strings.Repeat("abcdefghij", 20)
	next, cmd := streamModel().Update(content(reply))
	m := next.(Model)
	if cmd == nil {
		t.Fatal("content delta did not schedule a reveal tick")
	}
	if strings.Contains(m.viewport.View(), "abcdefghij") {
		t.Fatal("burst drawn in full before any reveal tick")
	}

	prev := 0
	frames := 0
	for m.streamBacklog() {
		next, _ = m.Update(streamTickMsg{})
		m = next.(Model)
		if m.streamShown <= prev {
			t.Fatalf("frame %d did not advance the reveal (%d -> %d)", frames, prev, m.streamShown)
		}
		prev = m.streamShown
		frames++
	}
	if frames < 2 || frames > 100 {
		t.Fatalf("burst revealed in %d frames, want a short animation", frames)
	}
	if got := assistantMessages(m); len(got) != 1 || got[0] != reply {
		t.Fatalf("transcript = %+v, want the full reply kept unchanged", got)
	}
}

// Only one tick is in flight however many deltas arrive before it fires:
// a second delta sees streamTicking and schedules nothing new.
func TestStreamRevealKeepsOneTickInFlight(t *testing.T) {
	next, _ := streamModel().Update(content("hello"))
	m := next.(Model)
	if !m.streamTicking {
		t.Fatal("first delta did not schedule a reveal tick")
	}
	if cmd := m.startStreamReveal(); cmd != nil {
		t.Fatal("a second reveal tick was scheduled while one is pending")
	}
}

// The reveal never cuts a multi-byte rune in half.
func TestStreamRevealStopsOnRuneBoundaries(t *testing.T) {
	var next tea.Model = streamModel()
	next, _ = next.(Model).Update(content(strings.Repeat("héllo wörld 日本語 ", 10)))
	m := next.(Model)
	for m.streamBacklog() {
		next, _ = m.Update(streamTickMsg{})
		m = next.(Model)
		shown := m.visibleStreamContent(m.messages[len(m.messages)-1].Content)
		if !utf8.ValidString(shown) {
			t.Fatalf("visible prefix is not valid UTF-8: %q", shown)
		}
	}
}

// The end of the turn draws the whole reply at once; a late tick is a no-op.
func TestStreamRevealFinishesAtTurnEnd(t *testing.T) {
	next, _ := streamModel().Update(content("partial reply that is long enough to lag"))
	next, _ = next.(Model).Update(responseMsg{content: "final reply"})
	m := next.(Model)
	if !strings.Contains(m.viewport.View(), "final reply") {
		t.Fatalf("finalized reply not drawn in full: %q", m.viewport.View())
	}
	next, _ = m.Update(streamTickMsg{})
	if next.(Model).streamTicking {
		t.Fatal("a tick after the turn ended scheduled another tick")
	}
}

func TestSplitSettled(t *testing.T) {
	for _, tc := range []struct {
		name, text, settled, last string
		inFence                   bool
	}{
		{"one open paragraph", "hello wor", "", "", false},
		{"paragraph then open one", "first para\n\nsecond", "first para\n\n", "first para\n\n", false},
		{"last settled block", "a\n\nb\n\nc", "a\n\nb\n\n", "b\n\n", false},
		{"blank line inside a fence is not a boundary", "```go\na\n\nb", "", "", true},
		{"closed fence settles", "```go\na\n```\nnext", "```go\na\n```\n", "```go\na\n```\n", false},
		{"fence opening on the last line", "para\n\n```", "para\n\n", "para\n\n", true},
		{"tilde fence", "~~~\nx\n~~~\n", "~~~\nx\n~~~\n", "~~~\nx\n~~~\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settled, open, lastStart, inFence := splitSettled(tc.text)
			if settled != tc.settled || settled[lastStart:] != tc.last || inFence != tc.inFence || settled+open != tc.text {
				t.Fatalf("splitSettled(%q) = %q, %q, last %q, %v; want settled %q, last %q, inFence %v", tc.text, settled, open, settled[lastStart:], inFence, tc.settled, tc.last, tc.inFence)
			}
		})
	}
}

// At every point of the reveal, the settled part of the streaming render
// matches the final render line for line, so nothing jumps: not when a block
// settles, and not when the turn ends and glamour takes over.
func TestStreamingRenderMatchesFinalRender(t *testing.T) {
	reply := "# Title\n\nSome **bold** text that is long enough to wrap across more than one line of the terminal.\n\n- one\n- two\n\n```go\nfunc main() {}\n```\n\n## Next\n\nThe end."
	m := streamModel()
	const width = 60
	final := strings.Split(ansi.Strip(m.renderMarkdown(reply, width)), "\n")
	for n := 1; n <= len(reply); n++ {
		visible := reply[:n]
		settled, _, _, _ := splitSettled(visible)
		if settled == "" {
			continue
		}
		got := strings.Split(ansi.Strip(m.renderStreaming(visible, width)), "\n")
		// Every line of the settled render, and of the open block's
		// complete lines, must equal the same line of the final render.
		want := strings.Split(ansi.Strip(m.renderMarkdown(settled, width)), "\n")
		for i := range want {
			line := strings.TrimSuffix(strings.TrimRight(at(got, i), " "), theme.StreamCursor)
			if strings.TrimRight(line, " ") != strings.TrimRight(final[i], " ") {
				t.Fatalf("at %d bytes, line %d = %q, final has %q", n, i, at(got, i), final[i])
			}
		}
	}
	got := normalize(strings.TrimSuffix(normalize(ansi.Strip(m.renderStreaming(reply, width))), theme.StreamCursor))
	if want := normalize(strings.Join(final, "\n")); got != want {
		t.Fatalf("fully revealed render differs from the final one\n--- streaming\n%s\n--- final\n%s", got, want)
	}
}

// normalize drops the trailing padding glamour adds to lines, which the
// reader never sees.
func normalize(s string) string {
	lines := strings.Split(strings.TrimRight(s, " \n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

func at(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<missing>"
}

// An open code block renders as code while it streams: the fence markers
// are not drawn as text.
func TestStreamingRenderClosesAnOpenFence(t *testing.T) {
	m := streamModel()
	out := ansi.Strip(m.renderStreaming("look:\n\n```go\nfunc main() {", 60))
	if strings.Contains(out, "```") {
		t.Fatalf("open fence drawn as raw text: %q", out)
	}
	if !strings.Contains(out, "func main() {") {
		t.Fatalf("open code block missing: %q", out)
	}
}

// The cursor sits at the end of the streaming text and goes away with the
// turn.
func TestStreamCursorShownOnlyWhileStreaming(t *testing.T) {
	m := drainStreamReveal(t, func() Model { n, _ := streamModel().Update(content("hi there")); return n.(Model) }())
	if !strings.Contains(m.viewport.View(), "hi there"+theme.StreamCursor) {
		t.Fatalf("cursor not at the end of the streaming text: %q", m.viewport.View())
	}
	n, _ := m.Update(responseMsg{content: "hi there"})
	if strings.Contains(n.(Model).viewport.View(), theme.StreamCursor) {
		t.Fatal("cursor still drawn after the turn ended")
	}
}

// Inside an open code block the cursor follows the code, not the padding
// line glamour puts under the block.
func TestStreamCursorFollowsOpenCode(t *testing.T) {
	m := streamModel()
	out := ansi.Strip(m.renderStreaming("```go\nfunc main() {", 60))
	if !strings.Contains(out, "func main() {"+theme.StreamCursor) {
		t.Fatalf("cursor not after the open code: %q", out)
	}
}
