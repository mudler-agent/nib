package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// fillMessages adds enough short messages that the content exceeds the viewport
// height, so scrolling is meaningful.
func fillMessages(m *Model, n int) {
	for i := 0; i < n; i++ {
		m.appendMessage(ChatMessage{Role: "user", Content: "history line"})
	}
}

func TestUpdateViewportPreservesScrollWhenNotAtBottom(t *testing.T) {
	m := newTestModel(Model{viewport: viewport.New(40, 4), width: 40})
	fillMessages(&m, 40)
	m.updateViewport()
	if !m.viewport.AtBottom() {
		t.Fatal("precondition: a fresh render should follow to the bottom")
	}

	// User scrolls up to the top.
	m.viewport.SetYOffset(0)
	if m.viewport.AtBottom() {
		t.Fatal("precondition: should not be at bottom after scrolling to top")
	}

	// A re-render (spinner tick, status change, streamed token) must NOT yank the
	// user back to the bottom while they're reading history.
	m = withMessages(m, ChatMessage{Role: "agent", Content: "newly arrived"})
	m.updateViewport()

	if m.viewport.YOffset != 0 {
		t.Fatalf("scroll position not preserved: YOffset = %d, want 0", m.viewport.YOffset)
	}
	if m.viewport.AtBottom() {
		t.Fatal("re-render snapped to bottom while the user was scrolled up")
	}
}

func TestUpdateViewportFollowsWhenAtBottom(t *testing.T) {
	m := newTestModel(Model{viewport: viewport.New(40, 4), width: 40})
	fillMessages(&m, 40)
	m.updateViewport()
	m.viewport.GotoBottom()

	// New content while parked at the bottom should keep following.
	for i := 0; i < 5; i++ {
		m = withMessages(m, ChatMessage{Role: "agent", Content: "streamed"})
	}
	m.updateViewport()

	if !m.viewport.AtBottom() {
		t.Fatal("expected to keep following to the bottom when already at the bottom")
	}
}

// TestResizeRewrapsAndClamps verifies a window resize re-renders at the new
// width and leaves the scroll offset inside the new content bounds.
func TestResizeRewrapsAndClamps(t *testing.T) {
	m := newTestModel(Model{
		viewport: viewport.New(80, 10),
		textarea: textarea.New(),
		width:    80,
		height:   24,
	})
	// One long message so the wrap width visibly changes the line count.
	m = withMessages(m, ChatMessage{
		Role:    "assistant",
		Content: strings.Repeat("wrap me across several lines. ", 40),
	})
	m.updateViewport()
	m.viewport.GotoBottom()
	wide := m.viewport.TotalLineCount()

	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	nm := next.(Model)

	if nm.viewport.Width != 40 {
		t.Fatalf("viewport width = %d, want 40", nm.viewport.Width)
	}
	// The real guard: without the updateViewport() call the viewport still holds
	// content wrapped to the old 80-column width. Halving the width must produce
	// more wrapped lines.
	narrow := nm.viewport.TotalLineCount()
	if narrow <= wide {
		t.Errorf("content not re-wrapped for the narrower width: %d lines at width 40, %d at width 80", narrow, wide)
	}
	if nm.viewport.YOffset > narrow {
		t.Errorf("scroll offset %d stranded past content (%d lines)", nm.viewport.YOffset, narrow)
	}
}

// TestResizeAtBottomStaysFollowing covers the whole-branch review finding:
// updateDimensions() used to shrink m.viewport.Height BEFORE updateViewport()
// captured wasAtBottom against the still-old content, so a user pinned to the
// bottom could read as scrolled-up the moment the terminal shrank and get
// stranded in scrollback.
func TestResizeAtBottomStaysFollowing(t *testing.T) {
	m := newTestModel(Model{
		viewport: viewport.New(80, 10),
		textarea: textarea.New(),
		width:    80,
		height:   24,
	})
	for i := 0; i < 60; i++ {
		m = withMessages(m, ChatMessage{Role: "user", Content: "history line"})
	}
	m.updateViewport()
	m.viewport.GotoBottom()
	if !m.viewport.AtBottom() {
		t.Fatal("precondition: expected to be at bottom before resize")
	}

	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	nm := next.(Model)

	if !nm.viewport.AtBottom() {
		t.Error("shrinking the terminal while pinned to the bottom stranded the user in scrollback")
	}
}

// TestUpdateViewportFollowSnapsToBottom covers the reported bug: the user
// scrolls up to re-read history, sends a message, and the reply streams in below
// the fold because the preserve-scroll guard saw wasAtBottom == false.
func TestUpdateViewportFollowSnapsToBottom(t *testing.T) {
	m := newTestModel(Model{viewport: viewport.New(40, 4), width: 40})
	fillMessages(&m, 40)
	m.updateViewport()
	m.viewport.SetYOffset(0)

	m = withMessages(m, ChatMessage{Role: "user", Content: "a new question"})
	m.updateViewportFollow()

	if !m.viewport.AtBottom() {
		t.Fatal("a user-initiated update must follow to the bottom even when scrolled up")
	}
}

// TestForceFollowIsOneShot ensures the flag does not pin the viewport to the
// bottom forever — the next passive re-render must respect the user's scroll.
func TestForceFollowIsOneShot(t *testing.T) {
	m := newTestModel(Model{viewport: viewport.New(40, 4), width: 40})
	fillMessages(&m, 40)
	m.updateViewportFollow()

	m.viewport.SetYOffset(0)
	m = withMessages(m, ChatMessage{Role: "agent", Content: "streamed"})
	m.updateViewport()

	if m.viewport.YOffset != 0 {
		t.Fatalf("passive re-render after a forced follow moved the viewport: YOffset = %d, want 0", m.viewport.YOffset)
	}
}

// TestEndKeyJumpsToBottom gives the user a way back once they are scrolled up.
func TestEndKeyJumpsToBottom(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyEnd},
		{Type: tea.KeyRunes, Runes: []rune{'G'}},
	} {
		m := newTestModel(Model{viewport: viewport.New(40, 4), width: 40, textarea: textarea.New()})
		fillMessages(&m, 40)
		m.updateViewport()
		m.viewport.SetYOffset(0)

		next, _ := m.Update(key)
		if !next.(Model).viewport.AtBottom() {
			t.Errorf("key %v did not jump to the bottom", key)
		}
	}
}

// TestGAtBottomFallsThroughToComposer guards the whole-branch review finding:
// the `G` shortcut used to fire whenever the composer was empty, which is
// exactly the state the user's FIRST keystroke of a new message finds it in.
// At the bottom there is nowhere for `G` to jump back from, so it must reach
// the textarea like any other rune.
func TestGAtBottomFallsThroughToComposer(t *testing.T) {
	ta := textarea.New()
	ta.Focus()

	m := newTestModel(Model{viewport: viewport.New(40, 4), width: 40, textarea: ta})
	fillMessages(&m, 40)
	m.updateViewport()
	m.viewport.GotoBottom()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	nm := next.(Model)

	if nm.textarea.Value() != "G" {
		t.Errorf("G at the bottom did not reach the composer: got %q, want %q", nm.textarea.Value(), "G")
	}
}

// TestEndKeyRespectsComposerText guards the regression a reviewer caught: End
// is bubbles/textarea's binding for line-end. Hijacking it unconditionally
// would silently break cursor movement whenever the user is mid-message. With
// text in the composer, End must reach the textarea (not jump the viewport)
// and must not alter the composer's content.
func TestEndKeyRespectsComposerText(t *testing.T) {
	ta := textarea.New()
	ta.Focus()
	ta.SetValue("still typing")

	m := newTestModel(Model{viewport: viewport.New(40, 4), width: 40, textarea: ta})
	fillMessages(&m, 40)
	m.updateViewport()
	m.viewport.SetYOffset(0)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	nm := next.(Model)

	if nm.viewport.AtBottom() {
		t.Error("End with a non-empty composer jumped the viewport instead of reaching the textarea")
	}
	if nm.textarea.Value() != "still typing" {
		t.Errorf("End with a non-empty composer altered the composer content: got %q, want %q", nm.textarea.Value(), "still typing")
	}
}
