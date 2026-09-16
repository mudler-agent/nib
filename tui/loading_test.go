package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"

	"github.com/mudler/nib/theme"
)

// TestLoadingBlockRendersSpinner guards the bug where the spinner was
// constructed and ticked but never drawn, leaving the loading state with no
// visible motion at all.
func TestLoadingBlockRendersSpinner(t *testing.T) {
	s := spinner.New()
	s.Spinner = spinner.Spinner{Frames: theme.SpinnerFrames(), FPS: spinnerFPS}

	m := newTestModel(Model{
		viewport: viewport.New(80, 10),
		width:    80,
		spinner:  s,
		loading:  true,
	})
	m.updateViewport()

	out := m.viewport.View()
	frame := theme.SpinnerFrames()[0]
	if !strings.Contains(out, frame) {
		t.Errorf("loading block missing spinner frame %q; got:\n%s", frame, out)
	}
	if !strings.Contains(out, theme.VerbThinking) {
		t.Errorf("loading block missing status verb %q; got:\n%s", theme.VerbThinking, out)
	}
}
