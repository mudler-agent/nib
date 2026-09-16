package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
)

// TestNewOutputMarkerShowsWhenScrolledUp covers a spec requirement that never
// made it into any of the four tasks: a dim "new output" marker when content
// has arrived below the fold while the user is scrolled up reading history.
// Without it the preserve-scroll behaviour is silent — a reply lands and
// nothing tells the reader it happened.
func TestNewOutputMarkerShowsWhenScrolledUp(t *testing.T) {
	m := newTestModel(Model{viewport: viewport.New(40, 4), textarea: textarea.New(), width: 40, height: 24})
	fillMessages(&m, 40)
	m.updateViewport()
	m.viewport.SetYOffset(0)
	if m.viewport.AtBottom() {
		t.Fatal("precondition: should not be at bottom after scrolling to top")
	}

	out := m.View()
	if !strings.Contains(out, "new output") {
		t.Errorf("expected a \"new output\" marker in the footer while scrolled up, got:\n%s", out)
	}
}

// TestNewOutputMarkerHiddenAtBottom ensures the marker never shows once the
// user is caught back up — otherwise it would be permanent noise rather than
// a signal.
func TestNewOutputMarkerHiddenAtBottom(t *testing.T) {
	m := newTestModel(Model{viewport: viewport.New(40, 4), textarea: textarea.New(), width: 40, height: 24})
	fillMessages(&m, 40)
	m.updateViewport()
	m.viewport.GotoBottom()
	if !m.viewport.AtBottom() {
		t.Fatal("precondition: expected to be at bottom")
	}

	out := m.View()
	if strings.Contains(out, "new output") {
		t.Errorf("did not expect a \"new output\" marker while at the bottom, got:\n%s", out)
	}
}
