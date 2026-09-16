package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mudler/nib/tui/render/full"
	"github.com/mudler/nib/tui/render/inline"
)

// mouseTestModel builds a full-screen (mouse-capable) Model sized like a real
// terminal, mid-turn with a long enough reasoning trace that the collapsed
// box renders several rows — a hit test that's off by one row would still
// fail visibly rather than accidentally lining up by luck.
func mouseTestModel() Model {
	m := newTestModel(Model{
		viewport:           viewport.New(80, 20),
		textarea:           textarea.New(),
		width:              80,
		height:             30,
		presenter:          full.New(),
		loading:            true,
		reasoning:          longTrace(20),
		reasoningCollapsed: true,
	})
	m.updateDimensions()
	m.updateViewport()
	return m
}

// boxRow returns the terminal-relative Y that lands on the model's own
// recorded reasoning-box span (its first row), deriving the chrome height
// from Presenter.HeaderHeight — the same query View's layout budget and
// reasoningBoxHit both use — rather than a hardcoded layout constant.
//
// It is the exact inverse of the mapping reasoningBoxHit applies, so on its
// own it proves only that the two are inverses: a wrong coordinate mapping
// cancels out and the click still "lands". What it cannot cancel out is WHERE
// the span ends, which is why TestReasoningBoxHitBoundaries drives the rows
// either side of the span through the same helper — those depend on the span's
// real height, not just on the offset.
func (m Model) boxRow() int {
	return m.reasoningSpanStart + m.presenter.HeaderHeight(m.viewState()) - m.viewport.YOffset
}

// rowFor maps a content-relative viewport row to its terminal-relative Y, the
// same translation boxRow does for the span's first row.
func (m Model) rowFor(contentRow int) int {
	return contentRow + m.presenter.HeaderHeight(m.viewState()) - m.viewport.YOffset
}

// TestReasoningBoxHitBoundaries pins the edges of the hit-test, which boxRow
// alone cannot: the last row of the box must hit, and the rows immediately
// before and after it must miss. An off-by-one at either end is a box whose
// clickable area does not match the box the user can see.
func TestReasoningBoxHitBoundaries(t *testing.T) {
	m := mouseTestModel()
	if m.reasoningSpanStart >= m.reasoningSpanEnd {
		t.Fatal("precondition: reasoning box did not record a span")
	}
	if m.reasoningSpanEnd-m.reasoningSpanStart < 2 {
		t.Fatalf("precondition: span %d..%d is too short for a boundary test to say anything",
			m.reasoningSpanStart, m.reasoningSpanEnd)
	}

	cases := []struct {
		name string
		row  int
		want bool
	}{
		{"first row of the span", m.rowFor(m.reasoningSpanStart), true},
		{"last row of the span", m.rowFor(m.reasoningSpanEnd - 1), true},
		{"the row after the span", m.rowFor(m.reasoningSpanEnd), false},
		{"the row before the span", m.rowFor(m.reasoningSpanStart - 1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.reasoningBoxHit(tc.row); got != tc.want {
				t.Errorf("reasoningBoxHit(y=%d) = %v, want %v (span %d..%d, header %d rows)",
					tc.row, got, tc.want, m.reasoningSpanStart, m.reasoningSpanEnd,
					m.presenter.HeaderHeight(m.viewState()))
			}
		})
	}

	// And the same boundaries as real clicks: only the rows inside the span
	// toggle the collapse state.
	if after := clickAt(m, m.rowFor(m.reasoningSpanEnd)); after.reasoningCollapsed != m.reasoningCollapsed {
		t.Error("a click one row below the box toggled the collapse state")
	}
	if after := clickAt(m, m.rowFor(m.reasoningSpanEnd-1)); after.reasoningCollapsed == m.reasoningCollapsed {
		t.Error("a click on the box's last row did not toggle the collapse state")
	}
}

// clickAt synthesizes a left-button press at the given terminal row and runs
// it through Update.
func clickAt(m Model, row int) Model {
	next, _ := m.Update(tea.MouseMsg{Y: row, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	return next.(Model)
}

// TestClickInsideReasoningBoxTogglesCollapse is the feature itself: a left
// click landing on the box's recorded row span toggles reasoningCollapsed,
// and the span is recomputed for the model's new (expanded) state so a
// second click on the same logical spot collapses it back.
func TestClickInsideReasoningBoxTogglesCollapse(t *testing.T) {
	m := mouseTestModel()
	if !m.reasoningCollapsed {
		t.Fatal("precondition: model should start collapsed")
	}
	if m.reasoningSpanStart >= m.reasoningSpanEnd {
		t.Fatal("precondition: reasoning box did not record a span")
	}

	m = clickAt(m, m.boxRow())
	if m.reasoningCollapsed {
		t.Error("click inside the reasoning box did not expand it")
	}
	if m.reasoningSpanStart >= m.reasoningSpanEnd {
		t.Fatal("expanded box did not record a span")
	}

	// The box is now taller (expanded); boxRow() recomputes against the fresh
	// span rather than reusing a stale one.
	m = clickAt(m, m.boxRow())
	if !m.reasoningCollapsed {
		t.Error("second click inside the reasoning box did not collapse it")
	}
}

// TestClickOutsideReasoningBoxDoesNotToggle proves the hit-test is bounded:
// a click on the header row (well above the box) must not toggle anything.
func TestClickOutsideReasoningBoxDoesNotToggle(t *testing.T) {
	m := mouseTestModel()
	before := m.reasoningCollapsed

	m = clickAt(m, 0) // the header's brand/cwd row, above the body entirely
	if m.reasoningCollapsed != before {
		t.Error("click outside the reasoning box toggled the collapse state")
	}
}

// TestInlinePresenterIgnoresClicks: the inline widget never enables mouse
// reporting (Caps().Mouse == false), so even a synthesized click landing
// exactly on the box's row must be a no-op — this is full-screen-only by
// construction, not merely by the terminal never sending the event.
func TestInlinePresenterIgnoresClicks(t *testing.T) {
	m := newTestModel(Model{
		viewport:           viewport.New(80, 20),
		textarea:           textarea.New(),
		width:              80,
		height:             30,
		presenter:          inline.New(),
		loading:            true,
		reasoning:          longTrace(20),
		reasoningCollapsed: true,
	})
	m.updateDimensions()
	m.updateViewport()
	before := m.reasoningCollapsed
	if m.reasoningSpanStart >= m.reasoningSpanEnd {
		t.Fatal("precondition: reasoning box did not record a span")
	}

	m = clickAt(m, m.boxRow())
	if m.reasoningCollapsed != before {
		t.Error("inline presenter (no mouse reporting) must ignore a click")
	}
}

// TestWheelStillReachesViewport is the regression the brief warns about
// hardest: adding the click case must not shadow the existing wheel-scroll
// fallback (m.viewport.Update(msg) at the bottom of Update).
func TestWheelStillReachesViewport(t *testing.T) {
	m := mouseTestModel()
	for i := 0; i < 60; i++ {
		m = withMessages(m, ChatMessage{Role: "user", Content: "history line"})
	}
	m.updateViewport()
	if !m.viewport.AtBottom() {
		t.Fatal("precondition: viewport should be pinned to the bottom")
	}
	before := m.viewport.YOffset
	if before == 0 {
		t.Fatal("precondition: content must be taller than the viewport for the wheel to have anywhere to scroll")
	}

	next, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	got := next.(Model)
	if got.viewport.YOffset >= before {
		t.Errorf("wheel-up did not reach the viewport: YOffset before=%d after=%d", before, got.viewport.YOffset)
	}
}
