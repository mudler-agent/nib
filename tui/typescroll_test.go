package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mudler/nib/tui/render/inline"
	"github.com/mudler/nib/types"
)

// TestTypingDoesNotScrollTranscript: every keystroke also reaches the
// transcript viewport, so its bindings must not include keys the composer
// types or edits with. The bubbles default binds b/u/k (up) and space/f/d/j
// (down), which scrolled the transcript while the user typed a message.
func TestTypingDoesNotScrollTranscript(t *testing.T) {
	m := NewModel(context.Background(), types.Config{}, 40, nil, inline.New())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = next.(Model)
	fillMessages(&m, 80)
	m.updateViewport()
	if !m.viewport.AtBottom() {
		t.Fatal("precondition: transcript should be pinned to the bottom")
	}
	want := m.viewport.YOffset

	keys := []tea.KeyMsg{{Type: tea.KeySpace, Runes: []rune{' '}}, {Type: tea.KeyCtrlU}, {Type: tea.KeyCtrlD}}
	for _, r := range "buk fdj hl" {
		keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	for _, k := range keys {
		next, _ = m.Update(k)
		m = next.(Model)
		if m.viewport.YOffset != want {
			t.Fatalf("key %q moved the transcript: YOffset = %d, want %d", k.String(), m.viewport.YOffset, want)
		}
	}

	// Page keys still scroll: they are not composer keys.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = next.(Model)
	if m.viewport.YOffset >= want {
		t.Fatalf("PgUp did not scroll up: YOffset = %d, was %d", m.viewport.YOffset, want)
	}
}
