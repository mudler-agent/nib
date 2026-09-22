package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

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
