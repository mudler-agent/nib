package tui

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mudler/nib/theme"
	"github.com/muesli/termenv"

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
		next, _ := m.Update(animTickMsg{})
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
		next, _ = m.Update(animTickMsg{})
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

// A delta leaves something to animate; once the reply is drawn and the new
// entry has faded in, nothing is, so the clock can sleep.
func TestAnimatingUntilRevealedAndFadedIn(t *testing.T) {
	next, _ := streamModel().Update(content("hello"))
	m := next.(Model)
	if !m.animating() {
		t.Fatal("a new delta left nothing to animate")
	}
	m = drainStreamReveal(t, m)
	for i := range m.messages {
		m.messages[i].arrived = time.Now().Add(-time.Second)
	}
	if !m.streamingActive || m.animating() {
		t.Fatalf("streaming=%v animating=%v, want nothing left to animate while caught up", m.streamingActive, m.animating())
	}
}

// The clock ticks while wanted and stops once a frame has nothing moving.
func TestAnimClockTicksOnlyWhileWanted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newAnimClock(ctx)
	c.want(true)
	select {
	case <-c.ticks:
	case <-time.After(time.Second):
		t.Fatal("no tick while wanted")
	}
	c.want(false)
	// At most the tick already in flight arrives after that.
	deadline := time.After(5 * streamRevealInterval)
	got := 0
	for {
		select {
		case <-c.ticks:
			got++
			continue
		case <-deadline:
		}
		break
	}
	if got > 1 {
		t.Fatalf("%d ticks after the clock was stopped, want at most 1", got)
	}
}

// The reveal never cuts a multi-byte rune in half.
func TestStreamRevealStopsOnRuneBoundaries(t *testing.T) {
	var next tea.Model = streamModel()
	next, _ = next.(Model).Update(content(strings.Repeat("héllo wörld 日本語 ", 10)))
	m := next.(Model)
	for m.streamBacklog() {
		next, _ = m.Update(animTickMsg{})
		m = next.(Model)
		shown := m.visibleStreamContent(m.messages[len(m.messages)-1].Content)
		if !utf8.ValidString(shown) {
			t.Fatalf("visible prefix is not valid UTF-8: %q", shown)
		}
	}
}

// The reveal goes on after the turn ends, on the final text, so the end of a
// fast stream does not land at once; then the entry gets its final render.
func TestStreamRevealContinuesAfterTurnEnd(t *testing.T) {
	next, _ := streamModel().Update(content("partial"))
	next, _ = next.(Model).Update(responseMsg{content: "partial reply that is long enough to take a few frames"})
	m := next.(Model)
	if strings.Contains(m.viewport.View(), "take a few frames") {
		t.Fatal("rest of the reply drawn at once at turn end")
	}
	m = drainStreamReveal(t, m)
	if out := m.viewport.View(); !strings.Contains(out, "take a few frames") || strings.Contains(out, theme.StreamCursor) {
		t.Fatalf("after the reveal: %q, want the full reply and no cursor", out)
	}
	if m.revealIdx != 0 {
		t.Fatal("reveal not cleared once the reply was fully drawn")
	}
}

// A reply that did not stream is revealed too, from its first character.
func TestNonStreamedReplyIsRevealed(t *testing.T) {
	next, _ := streamModel().Update(responseMsg{content: "a reply that arrived in one piece, all at once"})
	m := next.(Model)
	if strings.Contains(m.viewport.View(), "all at once") {
		t.Fatal("non-streamed reply drawn at once")
	}
	m = drainStreamReveal(t, m)
	if !strings.Contains(m.viewport.View(), "all at once") {
		t.Fatalf("non-streamed reply not drawn after its reveal: %q", m.viewport.View())
	}
}

// A new entry's chrome fades in; after theme.FadeDuration it is at full ink.
func TestNewEntryFadesIn(t *testing.T) {
	m := streamModel()
	m.appendMessage(ChatMessage{Role: "user", Content: "hi"})
	if a := m.arriving(m.messages[0]); a <= 0.5 {
		t.Fatalf("arriving = %v right after append, want close to 1", a)
	}
	m.messages[0].arrived = time.Now().Add(-theme.FadeDuration)
	if a := m.arriving(m.messages[0]); a != 0 {
		t.Fatalf("arriving = %v after the fade, want 0", a)
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
	if strings.Contains(drainStreamReveal(t, n.(Model)).viewport.View(), theme.StreamCursor) {
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

// A sub-agent's thread line fades in with its own entry; the lines before
// it, already in, are not faded again.
func TestAgentThreadLineFadesInAlone(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	m := streamModel()
	old := time.Now().Add(-time.Second)
	run := []ChatMessage{
		{Role: "agent_tool", AgentID: "a1", Content: "read one.go", arrived: old},
		{Role: "agent_tool", AgentID: "a1", Content: "read two.go", arrived: old},
	}
	var settled strings.Builder
	m.renderAgentThreadRun(&settled, run, 80, false, false)

	run[1].arrived = time.Now()
	var arriving strings.Builder
	m.renderAgentThreadRun(&arriving, run, 80, false, false)

	s, a := strings.Split(settled.String(), "\n"), strings.Split(arriving.String(), "\n")
	if a[0] != s[0] {
		t.Fatal("an older thread line was faded again when a new one arrived")
	}
	if a[1] == s[1] || ansi.Strip(a[1]) != ansi.Strip(s[1]) {
		t.Fatalf("the new thread line is not faded, or its text changed: %q", a[1])
	}
}
