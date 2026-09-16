package theme_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mudler/nib/theme"
)

func TestColorsAreSet(t *testing.T) {
	for name, c := range map[string]lipgloss.Color{
		"Accent": theme.Accent, "Sage": theme.Sage, "Danger": theme.Danger,
		"Dim": theme.Dim, "Faint": theme.Faint,
	} {
		if string(c) == "" {
			t.Errorf("%s color is empty", name)
		}
	}
}

func TestGlyphsHaveNoEmoji(t *testing.T) {
	for _, g := range []string{theme.Sep, theme.PromptGlyph, theme.ApprovalGutter, theme.MsgGutter, theme.SubAgent, theme.Cross} {
		for _, r := range g {
			if r >= 0x1F000 {
				t.Errorf("glyph %q contains emoji rune %U", g, r)
			}
		}
	}
}

func TestSpinnerFramesRespectRestrictedGlyphs(t *testing.T) {
	t.Setenv("NIB_ASCII", "1")
	for _, f := range theme.SpinnerFrames() {
		for _, r := range f {
			if r > 0xFF {
				t.Errorf("restricted spinner frame %q contains non-Latin-1 rune %U", f, r)
			}
		}
	}

	t.Setenv("NIB_ASCII", "0")
	full := theme.SpinnerFrames()
	if len(full) < 2 {
		t.Fatalf("unrestricted spinner needs multiple frames, got %d", len(full))
	}
}

func TestSpinnerFramesAreUniformWidth(t *testing.T) {
	// A frame that changes width makes the status line jitter on every tick.
	for _, env := range []string{"0", "1"} {
		t.Setenv("NIB_ASCII", env)
		frames := theme.SpinnerFrames()
		want := len([]rune(frames[0]))
		for _, f := range frames {
			if got := len([]rune(f)); got != want {
				t.Errorf("NIB_ASCII=%s: frame %q width %d, want %d", env, f, got, want)
			}
		}
	}
}

func TestEmptyStateCopyPresent(t *testing.T) {
	if theme.EmptyTagline == "" {
		t.Error("EmptyTagline is empty")
	}
	if len(theme.EmptyExamples) < 1 {
		t.Error("EmptyExamples is empty")
	}
	if !strings.Contains(theme.EmptySlash, "/") {
		t.Errorf("EmptySlash %q should mention /", theme.EmptySlash)
	}
}
